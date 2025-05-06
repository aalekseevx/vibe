// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

// Package cc implements an interceptor for bandwidth estimation that can be
// used with different BandwidthEstimators.
package cc

import (
	"math"
	"sync/atomic"
	"time"

	"github.com/pion/interceptor"
	"github.com/pion/rtcp"
	"github.com/pion/rtp"
	"github.com/pion/sdp/v3"

	"github.com/aalekseevx/vibe/bwe-test/gcc"
	"github.com/aalekseevx/vibe/bwe-test/ntp"

	"github.com/aalekseevx/vibe/bwe-test/cc"
)

type options struct {
	InitialRate int
	MinRate     int
	MaxRate     int
}

// Option can be used to set initial options on CC interceptors
type Option func(*options) error

// BandwidthEstimatorFactory creates new BandwidthEstimators
type BandwidthEstimatorFactory func() (BandwidthEstimator, error)

// BandwidthEstimator is the interface that will be returned by a
// NewPeerConnectionCallback and can be used to query current bandwidth
// metrics and add feedback manually.
type BandwidthEstimator interface {
	OnAcks(arrival time.Time, rtt time.Duration, acks []cc.Acknowledgment) int
}

// NewPeerConnectionCallback returns the BandwidthEstimator for the
// PeerConnection with id
type NewPeerConnectionCallback func(estimator *Interceptor)

// InterceptorFactory is a factory for CC interceptors
type InterceptorFactory struct {
	initialRate       int
	bweFactory        func() (BandwidthEstimator, error)
	addPeerConnection NewPeerConnectionCallback
}

// NewInterceptor returns a new CC interceptor factory
func NewInterceptor(factory BandwidthEstimatorFactory, opts ...Option) (*InterceptorFactory, error) {
	o := options{
		InitialRate: 100_000,
		MinRate:     50_000,
		MaxRate:     50_000_000,
	}

	for _, opt := range opts {
		if err := opt(&o); err != nil {
			return nil, err
		}
	}

	if factory == nil {
		factory = func() (BandwidthEstimator, error) {
			return gcc.NewSendSideController(o.InitialRate, o.MinRate, o.MaxRate), nil
		}
	}
	return &InterceptorFactory{
		initialRate:       o.InitialRate,
		bweFactory:        factory,
		addPeerConnection: nil,
	}, nil
}

// OnNewPeerConnection sets a callback that is called when a new CC interceptor
// is created.
func (f *InterceptorFactory) OnNewPeerConnection(cb NewPeerConnectionCallback) {
	f.addPeerConnection = cb
}

// NewInterceptor returns a new CC interceptor
func (f *InterceptorFactory) NewInterceptor(_ string) (interceptor.Interceptor, error) {
	bwe, err := f.bweFactory()
	if err != nil {
		return nil, err
	}
	i := &Interceptor{
		NoOp:            interceptor.NoOp{},
		feedbackAdapter: cc.NewFeedbackAdapter(),
		estimator:       bwe,
		target:          atomic.Int32{},
		feedback:        make(chan []rtcp.Packet),
		close:           make(chan struct{}),
	}
	i.target.Store(int32(f.initialRate))

	if f.addPeerConnection != nil {
		f.addPeerConnection(i)
	}
	return i, nil
}

// Interceptor implements Google Congestion Control
type Interceptor struct {
	interceptor.NoOp
	feedbackAdapter *cc.FeedbackAdapter
	estimator       BandwidthEstimator
	target          atomic.Int32
	feedback        chan []rtcp.Packet
	close           chan struct{}
}

// BindRTCPReader lets you modify any incoming RTCP packets. It is called once
// per sender/receiver, however this might change in the future. The returned
// method will be called once per packet batch.
func (c *Interceptor) BindRTCPReader(reader interceptor.RTCPReader) interceptor.RTCPReader {
	return interceptor.RTCPReaderFunc(func(b []byte, a interceptor.Attributes) (int, interceptor.Attributes, error) {
		i, attr, err := reader.Read(b, a)
		if err != nil {
			return 0, nil, err
		}
		buf := make([]byte, i)

		copy(buf, b[:i])

		if attr == nil {
			attr = make(interceptor.Attributes)
		}

		pkts, err := attr.GetRTCPPackets(buf[:i])
		if err != nil {
			return 0, nil, err
		}

		now := time.Now()

		for _, pkt := range pkts {
			var acks []cc.Acknowledgment
			var err error
			var feedbackSentTime time.Time
			switch fb := pkt.(type) {
			case *rtcp.TransportLayerCC:
				acks, err = c.feedbackAdapter.OnTransportCCFeedback(fb)
				if err != nil {
					return 0, nil, err
				}
				for i, ack := range acks {
					if i == 0 {
						feedbackSentTime = ack.Arrival
						continue
					}
					if ack.Arrival.After(feedbackSentTime) {
						feedbackSentTime = ack.Arrival
					}
				}
			case *rtcp.CCFeedbackReport:
				acks = c.feedbackAdapter.OnRFC8888Feedback(fb)
				feedbackSentTime = ntp.ToTime(uint64(fb.ReportTimestamp) << 16)
			default:
				continue
			}

			feedbackMinRTT := time.Duration(math.MaxInt)
			for _, ack := range acks {
				if ack.Arrival.IsZero() {
					continue
				}
				pendingTime := feedbackSentTime.Sub(ack.Arrival)
				rtt := now.Sub(ack.Departure) - pendingTime
				feedbackMinRTT = time.Duration(min(int(rtt), int(feedbackMinRTT)))
			}

			c.target.Store(int32(c.estimator.OnAcks(now, feedbackMinRTT, acks)))
		}

		return i, attr, nil
	})
}

// BindLocalStream lets you modify any outgoing RTP packets. It is called once
// for per LocalStream. The returned method will be called once per rtp packet.
func (c *Interceptor) BindLocalStream(info *interceptor.StreamInfo, writer interceptor.RTPWriter) interceptor.RTPWriter {
	var hdrExtID uint8
	for _, e := range info.RTPHeaderExtensions {
		if e.URI == sdp.TransportCCURI {
			hdrExtID = uint8(e.ID)
			break
		}
	}

	return interceptor.RTPWriterFunc(func(header *rtp.Header, payload []byte, attributes interceptor.Attributes) (int, error) {
		if hdrExtID != 0 {
			if attributes == nil {
				attributes = make(interceptor.Attributes)
			}
			attributes.Set(cc.TwccExtensionAttributesKey, hdrExtID)
		}
		if err := c.feedbackAdapter.OnSent(time.Now(), header, len(payload), attributes); err != nil {
			return 0, err
		}
		return writer.Write(header, payload, attributes)
	})
}

func (c *Interceptor) GetTargetBitrate() int {
	return int(c.target.Load())
}

// Close closes the interceptor and the associated bandwidth estimator.
func (c *Interceptor) Close() error {
	return nil
}

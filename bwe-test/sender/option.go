// SPDX-FileCopyrightText: 2025 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

// Package sender implements WebRTC sender functionality for bandwidth estimation tests.
package sender

import (
	"io"
	"time"

	"github.com/aalekseevx/vibe/bwe-test/packetdump"
	plogging "github.com/pion/logging"
	"github.com/pion/transport/v3/vnet"
	"github.com/pion/webrtc/v4"

	"github.com/aalekseevx/vibe/bwe-test/gcc"
	"github.com/aalekseevx/vibe/bwe-test/pacing"

	cc "github.com/aalekseevx/vibe/bwe-test/interceptorcc"

	"github.com/aalekseevx/vibe/bwe-test/logging"
)

// Option is a function that configures a Sender.
type Option func(*Sender) error

// PacketLogWriter returns an Option that configures RTP and RTCP packet logging.
func PacketLogWriter(rtpWriter, rtcpWriter io.Writer) Option {
	return func(sndr *Sender) error {
		formatter := logging.RTPFormatter{}
		rtpLogger, err := packetdump.NewSenderInterceptor(
			packetdump.RTPBinaryFormatter(formatter.RTPFormat),
			packetdump.RTPWriter(rtpWriter),
		)
		if err != nil {
			return err
		}
		rtcpLogger, err := packetdump.NewReceiverInterceptor(
			packetdump.RTCPFormatter(logging.RTCPFormat),
			packetdump.RTCPWriter(rtcpWriter),
		)
		if err != nil {
			return err
		}
		sndr.registry.Add(rtpLogger)
		sndr.registry.Add(rtcpLogger)

		return nil
	}
}

// DefaultInterceptors returns an Option that registers the default WebRTC interceptors.
func DefaultInterceptors() Option {
	return func(s *Sender) error {
		return webrtc.RegisterDefaultInterceptors(s.mediaEngine, s.registry)
	}
}

// CCLogWriter returns an Option that configures congestion control logging.
func CCLogWriter(w io.Writer) Option {
	return func(s *Sender) error {
		s.ccLogWriter = w

		return nil
	}
}

// GCC returns an Option that configures Google Congestion Control with the specified initial bitrate.
func GCC(initialBitrate, minBitrate, maxBitrate int) Option {
	return func(sndr *Sender) error {
		controller, err := cc.NewInterceptor(func() (cc.BandwidthEstimator, error) {
			return gcc.NewSendSideController(initialBitrate, minBitrate, maxBitrate), nil
		})
		if err != nil {
			return err
		}
		controller.OnNewPeerConnection(func(estimator *cc.Interceptor) {
			go func() {
				sndr.estimatorChan <- estimator
			}()
		})
		sndr.registry.Add(controller)
		if err = webrtc.ConfigureTWCCHeaderExtensionSender(sndr.mediaEngine, sndr.registry); err != nil {
			return err
		}

		return nil
	}
}

func Pacing() Option {
	return func(sndr *Sender) error {
		pacer := pacing.NewInterceptorFactory()
		sndr.pacerChan = make(chan *pacing.Interceptor)
		pacer.OnNewPeerConnection(func(pacer *pacing.Interceptor) {
			go func() {
				sndr.pacerChan <- pacer
			}()
		})
		sndr.registry.Add(pacer)
		return nil
	}
}

// SetVnet returns an Option that configures the virtual network for testing.
func SetVnet(v *vnet.Net, publicIPs []string) Option {
	return func(s *Sender) error {
		s.settingEngine.SetNet(v)
		s.settingEngine.SetICETimeouts(time.Second, time.Second, 200*time.Millisecond)
		s.settingEngine.SetNAT1To1IPs(publicIPs, webrtc.ICECandidateTypeHost)

		return nil
	}
}

// SetEncoderSource returns an Option that configures the encoder source.
func SetEncoderSource(source EncoderSource) Option {
	return func(s *Sender) error {
		s.sources = []MediaSource{source}
		s.bitrateAllocator = &encoderBitrateAllocator{
			source: source,
		}
		return nil
	}
}

// SetSimulcastSources returns an Option that configures the simulcast sources.
func SetSimulcastSources(sources []SimulcastSource) Option {
	return func(s *Sender) error {
		s.sources = make([]MediaSource, len(sources))
		for i, source := range sources {
			s.sources[i] = source
		}
		s.bitrateAllocator = &simulcastBitrateAllocator{
			sources: sources,
		}
		return nil
	}
}

// SetLoggerFactory returns an Option that configures the logger factory.
func SetLoggerFactory(loggerFactory plogging.LoggerFactory) Option {
	return func(s *Sender) error {
		s.settingEngine.LoggerFactory = loggerFactory
		s.log = loggerFactory.NewLogger("sender")

		return nil
	}
}

// SPDX-FileCopyrightText: 2025 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

package sender

import (
	"time"

	"github.com/pion/randutil"
	"github.com/pion/rtp"
)

// Use global random generator to properly seed by crypto grade random.
var globalMathRandomGenerator = randutil.NewMathRandomGenerator() // nolint:gochecknoglobals

type Packetizer struct {
	MTU         uint16
	PayloadType uint8
	SSRC        uint32
	Payloader   rtp.Payloader
	Sequencer   rtp.Sequencer
	Timestamp   uint32

	// Deprecated: will be removed in a future version.
	ClockRate uint32

	// put extension numbers in here. If they're 0, the extension is disabled (0 is not a legal extension number)
	extensionNumbers struct {
		AbsSendTime int // http://www.webrtc.org/experiments/rtp-hdrext/abs-send-time
	}
	timegen func() time.Time
}

// NewPacketizer returns a new instance of a Packetizer for a specific payloader.
func NewPacketizer(
	mtu uint16,
	pt uint8,
	ssrc uint32,
	csrc uint32,
	payloader rtp.Payloader,
	sequencer rtp.Sequencer,
	clockRate uint32,
) *Packetizer {
	return &Packetizer{
		MTU:         mtu,
		PayloadType: pt,
		SSRC:        ssrc,
		Payloader:   payloader,
		Sequencer:   sequencer,
		Timestamp:   globalMathRandomGenerator.Uint32(),
		ClockRate:   clockRate,
		timegen:     time.Now,
	}
}

// WithSSRC sets the SSRC for the Packetizer.
func WithSSRC(ssrc uint32) func(*Packetizer) {
	return func(p *Packetizer) {
		p.SSRC = ssrc
	}
}

// WithPayloadType sets the PayloadType for the Packetizer.
func WithPayloadType(pt uint8) func(*Packetizer) {
	return func(p *Packetizer) {
		p.PayloadType = pt
	}
}

// WithTimestamp sets the initial Timestamp for the Packetizer.
func WithTimestamp(timestamp uint32) func(*Packetizer) {
	return func(p *Packetizer) {
		p.Timestamp = timestamp
	}
}

// PacketizerOption is a function that configures a RTP Packetizer.
type PacketizerOption func(*Packetizer)

// NewPacketizerWithOptions returns a new instance of a Packetizer with the given options.
func NewPacketizerWithOptions(
	mtu uint16,
	payloader rtp.Payloader,
	sequencer rtp.Sequencer,
	clockRate uint32,
	options ...PacketizerOption,
) *Packetizer {
	PacketizerInstance := &Packetizer{
		MTU:       mtu,
		Payloader: payloader,
		Sequencer: sequencer,
		Timestamp: globalMathRandomGenerator.Uint32(),
		ClockRate: clockRate,
		timegen:   time.Now,
	}

	for _, option := range options {
		option(PacketizerInstance)
	}

	return PacketizerInstance
}

func (p *Packetizer) EnableAbsSendTime(value int) {
	p.extensionNumbers.AbsSendTime = value
}

// Packetize packetizes the payload of an RTP packet and returns one or more RTP packets.
func (p *Packetizer) Packetize(payload []byte, samples uint32, csrc uint32) []*rtp.Packet {
	// Guard against an empty payload
	if len(payload) == 0 {
		return nil
	}

	payloads := p.Payloader.Payload(p.MTU-12, payload)
	packets := make([]*rtp.Packet, len(payloads))

	for i, pp := range payloads {
		packets[i] = &rtp.Packet{
			Header: rtp.Header{
				Version:        2,
				Padding:        false,
				Extension:      false,
				Marker:         i == len(payloads)-1,
				PayloadType:    p.PayloadType,
				SequenceNumber: p.Sequencer.NextSequenceNumber(),
				Timestamp:      p.Timestamp, // Figure out how to do timestamps
				SSRC:           p.SSRC,
				CSRC:           []uint32{csrc},
			},
			Payload: pp,
		}
	}
	p.Timestamp += samples

	if len(packets) != 0 && p.extensionNumbers.AbsSendTime != 0 {
		sendTime := rtp.NewAbsSendTimeExtension(p.timegen())
		// apply http://www.webrtc.org/experiments/rtp-hdrext/abs-send-time
		b, err := sendTime.Marshal()
		if err != nil {
			return nil // never happens
		}
		err = packets[len(packets)-1].SetExtension(uint8(p.extensionNumbers.AbsSendTime), b) // nolint: gosec // G115
		if err != nil {
			return nil // never happens
		}
	}

	return packets
}

// GeneratePadding returns required padding-only packages.
func (p *Packetizer) GeneratePadding(samples uint32) []*rtp.Packet {
	// Guard against an empty payload
	if samples == 0 {
		return nil
	}

	packets := make([]*rtp.Packet, samples)

	for i := 0; i < int(samples); i++ {
		pp := make([]byte, 255)
		pp[254] = 255

		packets[i] = &rtp.Packet{
			Header: rtp.Header{
				Version:        2,
				Padding:        true,
				Extension:      false,
				Marker:         false,
				PayloadType:    p.PayloadType,
				SequenceNumber: p.Sequencer.NextSequenceNumber(),
				Timestamp:      p.Timestamp, // Use latest timestamp
				SSRC:           p.SSRC,
				CSRC:           []uint32{},
			},
			Payload: pp,
		}
	}

	return packets
}

// SkipSamples causes a gap in sample count between Packetize requests so the
// RTP payloads produced have a gap in timestamps.
func (p *Packetizer) SkipSamples(skippedSamples uint32) {
	p.Timestamp += skippedSamples
}

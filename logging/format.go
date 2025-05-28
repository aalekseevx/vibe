// SPDX-FileCopyrightText: 2025 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

// Package logging provides utilities for logging in bandwidth estimation tests.
package logging

import (
	"fmt"
	"time"

	"github.com/pion/interceptor"
	"github.com/pion/rtcp"
	"github.com/pion/rtp"

	"github.com/aalekseevx/vibe/bwe-test/sequencenumber"
)

// RTPFormatter formats RTP packets for logging.
type RTPFormatter struct {
	seqnr sequencenumber.Unwrapper
}

// RTPFormat formats an RTP packet as a string for logging.
func (f *RTPFormatter) RTPFormat(info *interceptor.StreamInfo, pkt *rtp.Packet, attr interceptor.Attributes) ([]byte, error) {
	var twcc rtp.TransportCCExtension
	unwrappedSeqNr := f.seqnr.Unwrap(pkt.SequenceNumber)
	var twccNr uint16
	if len(pkt.GetExtensionIDs()) > 0 {
		ext := pkt.GetExtension(pkt.GetExtensionIDs()[0])
		if err := twcc.Unmarshal(ext); err != nil {
			return nil, fmt.Errorf("Error unmarshaling TWCC extension: %w", err)
		}
		twccNr = twcc.TransportSequence
	}

	isRTX := info.SSRCRetransmission == pkt.SSRC
	isFEC := info.SSRCForwardErrorCorrection == pkt.SSRC

	trackID := pkt.CSRC[0]
	qualityID := pkt.CSRC[1]

	return []byte(fmt.Sprintf("%v,%v,%v,%v,%v,%v,%v,%v,%v,%v,%v,%v,%v\n",
		time.Now().UnixMilli(),
		pkt.PayloadType,
		pkt.SSRC,
		pkt.SequenceNumber,
		pkt.Timestamp,
		pkt.Marker,
		pkt.MarshalSize(),
		twccNr,
		unwrappedSeqNr,
		trackID,
		qualityID,
		isRTX,
		isFEC,
	)), nil
}

// RTCPFormat formats RTCP packets as a string for logging.
func RTCPFormat(pkts []rtcp.Packet, _ interceptor.Attributes) string {
	now := time.Now().UnixMilli()
	size := 0
	for _, pkt := range pkts {
		switch feedback := pkt.(type) {
		case *rtcp.TransportLayerCC:
			size += int(feedback.Len())
		case *rtcp.RawPacket:
			size += len(*feedback)
		}
	}

	return fmt.Sprintf("%v,%v\n", now, size)
}

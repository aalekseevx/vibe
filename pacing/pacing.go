package pacing

import (
	"io"
	"sync"
	"time"

	"github.com/aalekseevx/vibe/bwe-test/rtpbuffer"
	"github.com/pion/interceptor"
	"github.com/pion/logging"
	"github.com/pion/rtp"
)

type InterceptorFactory struct {
	InitialBitrate    int64
	StepDuration      time.Duration
	MaxBucketDuration time.Duration

	addPeerConnection NewPeerConnectionCallback
}

func NewInterceptorFactory(options ...Option) *InterceptorFactory {
	f := &InterceptorFactory{
		InitialBitrate:    1_000_000,
		StepDuration:      5 * time.Millisecond,
		MaxBucketDuration: 500 * time.Millisecond,
	}

	for _, o := range options {
		o(f)
	}

	return f
}

type NewPeerConnectionCallback func(pacer *Interceptor)

// OnNewPeerConnection sets a callback that is called when a new CC interceptor
// is created.
func (f *InterceptorFactory) OnNewPeerConnection(cb NewPeerConnectionCallback) {
	f.addPeerConnection = cb
}

func (f *InterceptorFactory) NewInterceptor(_ string) (interceptor.Interceptor, error) {
	p := &Interceptor{
		NoOp:   interceptor.NoOp{},
		mu:     sync.Mutex{},
		logger: logging.NewDefaultLoggerFactory().NewLogger("pacing"),

		intervalBudget: newIntervalBudget(f.InitialBitrate, f.MaxBucketDuration, time.Now()),

		writers:        make(map[uint32]interceptor.RTPWriter),
		roundRobin:     &roundRobin{},
		rtxSSRC:        make(map[uint32]uint32),
		rtxPayloadType: make(map[uint32]uint8),
		lastPacket:     make(map[uint32]rtp.Packet),
		packetFactory:  rtpbuffer.NewPacketFactoryCopy(),

		done: make(chan struct{}),
	}

	go p.run(f.StepDuration)

	f.addPeerConnection(p)

	return p, nil
}

type Interceptor struct {
	interceptor.NoOp
	logger logging.LeveledLogger

	mu sync.Mutex

	intervalBudget *intervalBudget

	writers        map[uint32]interceptor.RTPWriter
	roundRobin     *roundRobin
	rtxSSRC        map[uint32]uint32
	rtxPayloadType map[uint32]uint8
	lastPacket     map[uint32]rtp.Packet
	packetFactory  *rtpbuffer.PacketFactoryCopy

	done chan struct{}
}

func (p *Interceptor) SetTargetBitrate(target int64) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.intervalBudget.SetTargetRate(target, time.Now())
}

func (p *Interceptor) BindLocalStream(info *interceptor.StreamInfo, writer interceptor.RTPWriter) interceptor.RTPWriter {
	p.mu.Lock()
	p.writers[info.SSRC] = writer
	if info.PayloadTypeRetransmission != 0 && info.SSRCRetransmission != 0 {
		p.roundRobin.Add(info.SSRC)
		p.rtxPayloadType[info.SSRC] = info.PayloadTypeRetransmission
		p.rtxSSRC[info.SSRC] = info.SSRCRetransmission
	}
	p.mu.Unlock()

	return interceptor.RTPWriterFunc(func(header *rtp.Header, payload []byte, attributes interceptor.Attributes) (int, error) {
		packetSize := header.MarshalSize() + len(payload)
		p.mu.Lock()
		p.intervalBudget.UseBudget(packetSize, time.Now())
		if header.SSRC == info.SSRC {
			p.lastPacket[info.SSRC] = rtp.Packet{
				Header:  *header,
				Payload: payload,
			}
		}
		p.mu.Unlock()
		return writer.Write(header, payload, attributes)
	})
}

func (p *Interceptor) UnbindLocalStream(info *interceptor.StreamInfo) {
	p.mu.Lock()
	delete(p.writers, info.SSRC)
	p.roundRobin.Remove(info.SSRC)
	delete(p.rtxSSRC, info.SSRC)
	delete(p.rtxPayloadType, info.SSRC)
	delete(p.lastPacket, info.SSRC)
	p.mu.Unlock()
}

func (p *Interceptor) Close() error {
	close(p.done)
	return nil
}

func (p *Interceptor) run(step time.Duration) {
	ticker := time.NewTicker(step)
	defer ticker.Stop()

	for {
		select {
		case now := <-ticker.C:
			p.step(now)
		case <-p.done:
			return
		}
	}
}

func (p *Interceptor) step(now time.Time) {
	p.mu.Lock()
	defer p.mu.Unlock()

	availiableBytes := p.intervalBudget.BytesRemaining(now)
	paddingPackets := p.getPadding(availiableBytes)

	for _, padPkt := range paddingPackets {
		writer, exists := p.writers[padPkt.ssrc]
		if !exists {
			p.logger.Errorf("failed to find writer for ssrc %d", padPkt.ssrc)
			continue
		}

		header := padPkt.packet.Header()
		payload := padPkt.packet.Payload()
		packetSize := header.MarshalSize() + len(payload)
		p.intervalBudget.UseBudget(packetSize, now)
		_, err := writer.Write(header, payload, interceptor.Attributes{})
		padPkt.packet.Release()
		if err != nil && err != io.ErrClosedPipe {
			p.logger.Errorf("failed to write padding packet: %v", err)
		}
	}
}

type padding struct {
	packet *rtpbuffer.RetainablePacket
	ssrc   uint32
}

func (p *Interceptor) getPadding(maxBytes int) []padding {
	bytes := 0
	var result []padding

	if p.roundRobin.Size() == 0 {
		return nil
	}

	for bytes < maxBytes {
		packetFits := false

		for range p.roundRobin.Size() {
			ssrc, ok := p.roundRobin.Next()
			if !ok {
				break
			}

			lastPacket, ok := p.lastPacket[ssrc]
			if !ok {
				continue
			}

			rtxSSRC := p.rtxSSRC[ssrc]
			rtxPayloadType := p.rtxPayloadType[ssrc]

			size := lastPacket.MarshalSize()
			if size >= maxBytes-bytes {
				continue
			}

			packetFits = true
			packetCopy, err := p.packetFactory.NewPacket(&lastPacket.Header, lastPacket.Payload, rtxSSRC, rtxPayloadType)
			if err != nil {
				p.logger.Errorf("failed to create padding packet: %v", err)
				continue
			}

			bytes += size
			result = append(result, padding{
				packet: packetCopy,
				ssrc:   ssrc,
			})
		}
		if !packetFits {
			break
		}
	}

	return result
}

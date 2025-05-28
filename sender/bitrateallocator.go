package sender

import (
	"math"
	"sort"

	"github.com/aalekseevx/vibe/flexfec"
	"github.com/aalekseevx/vibe/pacing"
)

type fecParametersOption struct {
	numMediaPackets uint32
	numFecPackets   uint32
}

func (o fecParametersOption) overhead() float64 {
	return float64(o.numFecPackets) / float64(o.numMediaPackets)
}

type fecAllocator struct {
	options []fecParametersOption
}

func newFECAllocator() *fecAllocator {
	opts := make([]fecParametersOption, 0)
	for k := uint32(2); k < 10; k++ {
		for m := uint32(1); m < k; m++ {
			opts = append(opts, fecParametersOption{
				numMediaPackets: k,
				numFecPackets:   m,
			})
		}
	}
	opts = append(opts, fecParametersOption{
		numMediaPackets: 1,
		numFecPackets:   1,
	})
	opts = append(opts, fecParametersOption{
		numMediaPackets: 1,
		numFecPackets:   0,
	})

	sort.Slice(opts, func(i, j int) bool {
		return opts[i].overhead() > opts[j].overhead()
	})

	return &fecAllocator{
		options: opts,
	}
}

func (a *fecAllocator) Allocate(mediaBitrate int, targetBitrate int) fecParametersOption {
	for _, opt := range a.options {
		if float64(mediaBitrate)*(1+opt.overhead()) <= float64(targetBitrate) {
			return opt
		}
	}
	return a.options[len(a.options)-1]
}

type bitrateAllocator interface {
	SetTargetBitrate(bitrate int) error
	Initialize(pacer *pacing.Interceptor, fecSender *flexfec.FecInterceptor) error
}

type simulcastBitrateAllocator struct {
	sources      []SimulcastSource
	pacer        *pacing.Interceptor
	fecSender    *flexfec.FecInterceptor
	fecAllocator *fecAllocator
}

func (a *simulcastBitrateAllocator) Initialize(pacer *pacing.Interceptor, fecSender *flexfec.FecInterceptor) error {
	a.pacer = pacer
	a.fecSender = fecSender

	return nil
}

func (a *simulcastBitrateAllocator) SetTargetBitrate(targetBitrate int) error {
	qualities := make([][]Quality, 0, len(a.sources))
	currentBitrate := 0
	for _, source := range a.sources {
		sourceQualities := source.GetQualities()
		qualities = append(qualities, sourceQualities)
		for _, quality := range sourceQualities {
			if quality.Active {
				currentBitrate += quality.Bitrate
			}
		}
	}

	targetVideoBitrate := int(float64(targetBitrate) * 0.8)

	for currentBitrate < targetVideoBitrate {
		minimumBitrateWithAlternative := math.MaxInt
		minimumBitrateSourceWithAlternative := -1
		minimumBitrateSourceWithAlternativeQuality := -1
		for i, source := range a.sources {
			sourceQualities := source.GetQualities()
			for j, quality := range sourceQualities[:len(sourceQualities)-1] {
				if quality.Active && quality.Bitrate < minimumBitrateWithAlternative {
					minimumBitrateWithAlternative = quality.Bitrate
					minimumBitrateSourceWithAlternative = i
					minimumBitrateSourceWithAlternativeQuality = j
				}
			}
		}

		if minimumBitrateSourceWithAlternative == -1 {
			break
		}

		source := a.sources[minimumBitrateSourceWithAlternative]
		sourceQualities := source.GetQualities()
		nextQuality := sourceQualities[minimumBitrateSourceWithAlternativeQuality+1]
		bitrateDifference := nextQuality.Bitrate - minimumBitrateWithAlternative
		if currentBitrate+bitrateDifference > targetVideoBitrate {
			break
		}

		currentBitrate += bitrateDifference
		if err := source.SetQuality(nextQuality.Name); err != nil {
			return err
		}
	}

	for currentBitrate > targetVideoBitrate {
		maximumBitrate := 0
		maximumBitrateSource := -1
		maximumBitrateSourceQuality := -1
		for i, source := range a.sources {
			sourceQualities := source.GetQualities()
			for j := 1; j < len(sourceQualities); j++ {
				quality := sourceQualities[j]
				if quality.Active && quality.Bitrate > maximumBitrate {
					maximumBitrate = quality.Bitrate
					maximumBitrateSource = i
					maximumBitrateSourceQuality = j
				}
			}
		}

		if maximumBitrateSource == -1 {
			break
		}

		source := a.sources[maximumBitrateSource]
		sourceQualities := source.GetQualities()
		nextQuality := sourceQualities[maximumBitrateSourceQuality-1]
		bitrateDifference := nextQuality.Bitrate - maximumBitrate
		if currentBitrate+bitrateDifference < targetVideoBitrate {
			break
		}

		currentBitrate += bitrateDifference
		if err := source.SetQuality(nextQuality.Name); err != nil {
			return err
		}
	}

	if a.pacer != nil {
		a.pacer.SetTargetBitrate(int64(targetBitrate))
	}
	if a.fecSender != nil {
		opt := a.fecAllocator.Allocate(currentBitrate, targetBitrate)
		a.fecSender.SetFecParameters(opt.numMediaPackets, opt.numFecPackets)
	}

	return nil
}

type encoderBitrateAllocator struct {
	source EncoderSource
	pacer  *pacing.Interceptor
}

func (a *encoderBitrateAllocator) Initialize(pacer *pacing.Interceptor, fecSender *flexfec.FecInterceptor) error {
	a.pacer = pacer

	return nil
}

func (a *encoderBitrateAllocator) SetTargetBitrate(bitrate int) error {
	// Set the target bitrate for the encoder source
	a.source.SetTargetBitrate(bitrate)
	if a.pacer != nil {
		a.pacer.SetTargetBitrate(int64(bitrate))
	}
	return nil
}

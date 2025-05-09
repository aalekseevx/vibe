package sender

import "math"

type bitrateAllocator interface {
	SetTargetBitrate(bitrate int) error
}

type simulcastBitrateAllocator struct {
	sources []SimulcastSource
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
			for j, quality := range sourceQualities[1:] {
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

	return nil
}

type encoderBitrateAllocator struct {
	source EncoderSource
}

func (a *encoderBitrateAllocator) SetTargetBitrate(bitrate int) error {
	// Set the target bitrate for the encoder source
	a.source.SetTargetBitrate(bitrate)
	return nil
}

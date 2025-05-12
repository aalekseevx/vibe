package pacing

import "time"

type Option func(*InterceptorFactory)

func WithInitialBitrate(bitrate int64) Option {
	return func(o *InterceptorFactory) {
		o.InitialBitrate = bitrate
	}
}

func WithStepDuration(step time.Duration) Option {
	return func(o *InterceptorFactory) {
		o.StepDuration = step
	}
}

func WithMaxBucketDuration(duration time.Duration) Option {
	return func(o *InterceptorFactory) {
		o.MaxBucketDuration = duration
	}
}

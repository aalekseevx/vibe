package pacing

import (
	"time"
)

// https://chromium.googlesource.com/external/webrtc/+/master/modules/pacing/interval_budget.cc
type intervalBudget struct {
	Window        time.Duration
	lastIncreased time.Time

	TargetRateTokensPerMillisec int64
	MaxTokensInBudget           int64
	TokenRemaining              int64
}

func newIntervalBudget(initialTargetRateBps int64, window time.Duration, now time.Time) *intervalBudget {
	budget := &intervalBudget{
		Window:        window,
		lastIncreased: now,

		TargetRateTokensPerMillisec: 0,
		MaxTokensInBudget:           0,
		TokenRemaining:              0,
	}
	budget.SetTargetRate(initialTargetRateBps, now)
	return budget
}

func (b *intervalBudget) SetTargetRate(targetRateBitsPerSec int64, now time.Time) {
	b.increaseBudget(now)

	b.TargetRateTokensPerMillisec = targetRateBitsPerSec
	b.MaxTokensInBudget = (b.Window.Milliseconds() * targetRateBitsPerSec)
	b.TokenRemaining = clip(b.TokenRemaining, -b.MaxTokensInBudget, b.MaxTokensInBudget)
}

func (b *intervalBudget) UseBudget(bytes int, now time.Time) {
	b.increaseBudget(now)

	tokens := int64(bytes) * 8000
	b.TokenRemaining = max(b.TokenRemaining-tokens, -b.MaxTokensInBudget)
}

func (b *intervalBudget) BytesRemaining(now time.Time) int {
	b.increaseBudget(now)

	return int(max(b.TokenRemaining, 0) / 8000)
}

func (b *intervalBudget) TargetRate() int64 {
	return b.TargetRateTokensPerMillisec
}

func (b *intervalBudget) BudgetRatio(now time.Time) float64 {
	if b.MaxTokensInBudget == 0 {
		return 0
	}
	return float64(b.TokenRemaining) / float64(b.MaxTokensInBudget)
}

func (b *intervalBudget) increaseBudget(now time.Time) {
	deltaTime := now.Sub(b.lastIncreased)
	b.lastIncreased = now
	newTokens := b.TargetRateTokensPerMillisec * deltaTime.Milliseconds()
	b.TokenRemaining = min(b.TokenRemaining+newTokens, b.MaxTokensInBudget)
}

func clip(value, minValue, maxValue int64) int64 {
	return min(max(value, minValue), maxValue)
}

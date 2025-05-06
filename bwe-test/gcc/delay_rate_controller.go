// SPDX-FileCopyrightText: 2025 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

package gcc

import (
	"time"

	"github.com/pion/logging"

	"github.com/aalekseevx/vibe/bwe-test/cc"
)

const maxSamples = 1000

type DelayRateController struct {
	log         logging.LeveledLogger
	aga         *arrivalGroupAccumulator
	last        arrivalGroup
	kf          *kalmanFilter
	od          *overuseDetector
	rc          *rateController
	latestUsage usage
	samples     int
}

func NewDelayRateController(initialRate int) *DelayRateController {
	return &DelayRateController{
		log:         logging.NewDefaultLoggerFactory().NewLogger("bwe_delay_rate_controller"),
		aga:         newArrivalGroupAccumulator(),
		last:        []cc.Acknowledgment{},
		kf:          newKalmanFilter(),
		od:          newOveruseDetector(true),
		rc:          newRateController(initialRate),
		latestUsage: 0,
		samples:     0,
	}
}

func (c *DelayRateController) OnPacketAcked(ack cc.Acknowledgment) {
	next := c.aga.onPacketAcked(ack)
	if next == nil {
		return
	}
	if len(next) == 0 {
		// ignore empty groups, should never occur
		return
	}
	if len(c.last) == 0 {
		c.last = next
		return
	}

	prevSize := groupSize(c.last)
	nextSize := groupSize(next)
	sizeDelta := nextSize - prevSize

	interArrivalTime := next[len(next)-1].Arrival.Sub(c.last[len(c.last)-1].Arrival)
	interDepartureTime := next[len(next)-1].Departure.Sub(c.last[len(c.last)-1].Departure)
	interGroupDelay := interArrivalTime - interDepartureTime
	estimate := c.kf.update(float64(interGroupDelay.Milliseconds()), float64(sizeDelta))
	c.samples++
	c.latestUsage = c.od.update(ack.Arrival, estimate, c.samples)
	c.last = next
	c.log.Tracef(
		"ts=%v.%06d, seq=%v, size=%v, interArrivalTime=%v, interDepartureTime=%v, interGroupDelay=%v, estimate=%v, threshold=%v, usage=%v, state=%v",
		c.last[0].Departure.UTC().Format("2006/01/02 15:04:05"),
		c.last[0].Departure.UTC().Nanosecond()/1e3,
		next[0].SequenceNumber,
		nextSize,
		interArrivalTime.Microseconds(),
		interDepartureTime.Microseconds(),
		interGroupDelay.Microseconds(),
		estimate,
		c.od.delayThreshold,
		int(c.latestUsage),
		int(c.rc.s),
	)
}

func (c *DelayRateController) Update(ts time.Time, lastDeliveryRate int, rtt time.Duration) int {
	return c.rc.update(ts, c.latestUsage, lastDeliveryRate, rtt)
}

func groupSize(group arrivalGroup) int {
	sum := 0
	for _, ack := range group {
		sum += ack.Size
	}
	return sum
}

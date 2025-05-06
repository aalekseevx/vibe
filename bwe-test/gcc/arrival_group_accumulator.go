// SPDX-FileCopyrightText: 2025 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

package gcc

import (
	"time"

	"github.com/aalekseevx/vibe/bwe-test/cc"
)

type arrivalGroup []cc.Acknowledgment

type arrivalGroupAccumulator struct {
	next             arrivalGroup
	burstInterval    time.Duration
	maxBurstDuration time.Duration
}

func newArrivalGroupAccumulator() *arrivalGroupAccumulator {
	return &arrivalGroupAccumulator{
		next:             make([]cc.Acknowledgment, 0),
		burstInterval:    5 * time.Millisecond,
		maxBurstDuration: 100 * time.Millisecond,
	}
}

func (a *arrivalGroupAccumulator) onPacketAcked(ack cc.Acknowledgment) arrivalGroup {
	if len(a.next) == 0 {
		a.next = append(a.next, ack)
		return nil
	}

	if ack.Departure.Sub(a.next[0].Departure) < a.burstInterval {
		a.next = append(a.next, ack)
		return nil
	}

	sendTimeDelta := ack.Departure.Sub(a.next[0].Departure)
	arrivalTimeDeltaLast := ack.Arrival.Sub(a.next[len(a.next)-1].Arrival)
	arrivalTimeDeltaFirst := ack.Arrival.Sub(a.next[0].Arrival)
	propagationDelta := arrivalTimeDeltaFirst - sendTimeDelta

	if propagationDelta < 0 && arrivalTimeDeltaLast <= a.burstInterval && arrivalTimeDeltaFirst < a.maxBurstDuration {
		a.next = append(a.next, ack)
		return nil
	}

	group := make(arrivalGroup, len(a.next))
	copy(group, a.next)
	a.next = arrivalGroup{ack}
	return group
}

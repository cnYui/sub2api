package service

import (
	"errors"
	"sync/atomic"
)

type TrafficCreditReservationMetricsSnapshot struct {
	PreauthorizationSuccessTotal  uint64 `json:"preauthorization_success_total"`
	PreauthorizationRejectedTotal uint64 `json:"preauthorization_rejected_total"`
	DebtBlockedTotal              uint64 `json:"debt_blocked_total"`
	ReservationUnknownTotal       uint64 `json:"reservation_unknown_total"`
	ActualExceededReservedTotal   uint64 `json:"actual_exceeded_reserved_total"`
	StaleReservedReleasedTotal    uint64 `json:"stale_reserved_released_total"`
}

type trafficCreditReservationMetrics struct {
	preauthorizationSuccess  atomic.Uint64
	preauthorizationRejected atomic.Uint64
	debtBlocked              atomic.Uint64
	reservationUnknown       atomic.Uint64
	actualExceededReserved   atomic.Uint64
	staleReservedReleased    atomic.Uint64
}

var defaultTrafficCreditReservationMetrics trafficCreditReservationMetrics

func SnapshotTrafficCreditReservationMetrics() TrafficCreditReservationMetricsSnapshot {
	return TrafficCreditReservationMetricsSnapshot{
		PreauthorizationSuccessTotal:  defaultTrafficCreditReservationMetrics.preauthorizationSuccess.Load(),
		PreauthorizationRejectedTotal: defaultTrafficCreditReservationMetrics.preauthorizationRejected.Load(),
		DebtBlockedTotal:              defaultTrafficCreditReservationMetrics.debtBlocked.Load(),
		ReservationUnknownTotal:       defaultTrafficCreditReservationMetrics.reservationUnknown.Load(),
		ActualExceededReservedTotal:   defaultTrafficCreditReservationMetrics.actualExceededReserved.Load(),
		StaleReservedReleasedTotal:    defaultTrafficCreditReservationMetrics.staleReservedReleased.Load(),
	}
}

func recordTrafficCreditPreauthorizationSuccess() {
	defaultTrafficCreditReservationMetrics.preauthorizationSuccess.Add(1)
}

func recordTrafficCreditPreauthorizationRejected(err error) {
	defaultTrafficCreditReservationMetrics.preauthorizationRejected.Add(1)
	if errors.Is(err, ErrTrafficCreditDebtOutstanding) {
		defaultTrafficCreditReservationMetrics.debtBlocked.Add(1)
	}
}

func recordTrafficCreditReservationUnknown() {
	defaultTrafficCreditReservationMetrics.reservationUnknown.Add(1)
}

func recordTrafficCreditActualExceededReserved() {
	defaultTrafficCreditReservationMetrics.actualExceededReserved.Add(1)
}

func recordTrafficCreditStaleReservedReleased(count int) {
	if count <= 0 {
		return
	}
	defaultTrafficCreditReservationMetrics.staleReservedReleased.Add(uint64(count))
}

func resetTrafficCreditReservationMetricsForTest() {
	defaultTrafficCreditReservationMetrics.preauthorizationSuccess.Store(0)
	defaultTrafficCreditReservationMetrics.preauthorizationRejected.Store(0)
	defaultTrafficCreditReservationMetrics.debtBlocked.Store(0)
	defaultTrafficCreditReservationMetrics.reservationUnknown.Store(0)
	defaultTrafficCreditReservationMetrics.actualExceededReserved.Store(0)
	defaultTrafficCreditReservationMetrics.staleReservedReleased.Store(0)
}

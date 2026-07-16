package service

import "math"

type TrafficCreditPolicy struct {
	MinimumReserveUSD float64
}

func (p TrafficCreditPolicy) IsDepleted(remainingUSD float64) bool {
	return roundTrafficCreditAmount(remainingUSD) <= roundTrafficCreditAmount(p.MinimumReserveUSD)
}

func (p TrafficCreditPolicy) AvailableUSD(remainingUSD, reservedUSD float64) float64 {
	if p.IsDepleted(remainingUSD) {
		return 0
	}
	return roundTrafficCreditAmount(math.Max(remainingUSD-reservedUSD, 0))
}

func resolveTrafficCreditPolicy(policies []TrafficCreditPolicy) TrafficCreditPolicy {
	if len(policies) == 0 {
		return TrafficCreditPolicy{}
	}
	return policies[0]
}

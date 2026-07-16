package repository

import "github.com/Wei-Shaw/sub2api/internal/service"

func firstTrafficCreditPolicy(policies []service.TrafficCreditPolicy) service.TrafficCreditPolicy {
	if len(policies) == 0 {
		return service.TrafficCreditPolicy{}
	}
	return policies[0]
}

package types

import fmt "fmt"

// Builds the store key from the reward and host denom's
func (t TradeRoute) GetKey() []byte {
	return TradeRouteKeyFromDenoms(t.RewardDenomOnRewardZone, t.HostDenomOnHostZone)
}

// Human readable description for logging
func (t TradeRoute) Description() string {
	return fmt.Sprintf("TradeRoute from %s to %s", t.RewardDenomOnRewardZone, t.HostDenomOnHostZone)
}
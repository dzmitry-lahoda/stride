package keeper

import (
	"github.com/cosmos/cosmos-sdk/store/prefix"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"

	"github.com/Stride-Labs/stride/v4/x/ratelimit/types"
)

// Get the rate limit byte key built from the denom and channelId
func GetRateLimitItemKey(denom string, channelId string) []byte {
	return append(types.KeyPrefix(denom), types.KeyPrefix(channelId)...)
}

// The total value on a given path (aka, the denominator in the percentage calculation)
// is the total supply of the given denom
func (k Keeper) GetChannelValue(ctx sdk.Context, denom string) sdk.Int {
	return k.bankKeeper.GetSupply(ctx, denom).Amount
}

// Adds an amount to the flow in either the SEND or RECV direction
func (k Keeper) UpdateFlow(rateLimit types.RateLimit, direction types.PacketDirection, amount sdk.Int) error {
	switch direction {
	case types.PACKET_SEND:
		return rateLimit.Flow.AddOutflow(amount, *rateLimit.Quota)
	case types.PACKET_RECV:
		return rateLimit.Flow.AddInflow(amount, *rateLimit.Quota)
	default:
		return sdkerrors.Wrapf(sdkerrors.ErrInvalidRequest, "invalid packet direction (%s)", direction.String())
	}
}

// Checks whether the given packet will exceed the rate limit
// Called by OnRecvPacket and OnSendPacket
func (k Keeper) CheckRateLimitAndUpdateFlow(ctx sdk.Context, direction types.PacketDirection, denom string, channelId string, amount sdk.Int) error {
	// If there's no rate limit yet for this denom, no action is necessary
	rateLimit, found := k.GetRateLimit(ctx, denom, channelId)
	if !found {
		return nil
	}

	// Update the flow object with the change in amount
	if err := k.UpdateFlow(rateLimit, direction, amount); err != nil {
		return err
	}

	// If there's no quota error, update the rate limit object in the store with the new flow
	k.SetRateLimit(ctx, rateLimit)

	return nil
}

// Reset the rate limit after expiration
// The inflow and outflow should get reset to 1 and the channelValue should be updated
func (k Keeper) ResetRateLimit(ctx sdk.Context, denom string, channelId string) error {
	rateLimit, found := k.GetRateLimit(ctx, denom, channelId)
	if !found {
		return types.ErrRateLimitNotFound
	}

	flow := types.Flow{
		Inflow:       sdk.ZeroInt(),
		Outflow:      sdk.ZeroInt(),
		ChannelValue: k.GetChannelValue(ctx, denom),
	}
	rateLimit.Flow = &flow

	k.SetRateLimit(ctx, rateLimit)
	return nil
}

// Stores/Updates a rate limit object in the store
func (k Keeper) SetRateLimit(ctx sdk.Context, rateLimit types.RateLimit) {
	store := prefix.NewStore(ctx.KVStore(k.storeKey), types.RateLimitKeyPrefix)

	rateLimitKey := GetRateLimitItemKey(rateLimit.Path.Denom, rateLimit.Path.ChannelId)
	rateLimitValue := k.cdc.MustMarshal(&rateLimit)

	store.Set(rateLimitKey, rateLimitValue)
}

// Removes a rate limit object from the store using denom and channel-id
func (k Keeper) RemoveRateLimit(ctx sdk.Context, denom string, channelId string) {
	store := prefix.NewStore(ctx.KVStore(k.storeKey), types.RateLimitKeyPrefix)
	rateLimitKey := GetRateLimitItemKey(denom, channelId)
	store.Delete(rateLimitKey)
}

// Grabs and returns a rate limit object from the store using denom and channel-id
func (k Keeper) GetRateLimit(ctx sdk.Context, denom string, channelId string) (rateLimit types.RateLimit, found bool) {
	store := prefix.NewStore(ctx.KVStore(k.storeKey), types.RateLimitKeyPrefix)

	rateLimitKey := GetRateLimitItemKey(denom, channelId)
	rateLimitValue := store.Get(rateLimitKey)

	if len(rateLimitValue) == 0 {
		return rateLimit, false
	}

	k.cdc.MustUnmarshal(rateLimitValue, &rateLimit)
	return rateLimit, true
}

// Returns all rate limits stored
func (k Keeper) GetAllRateLimits(ctx sdk.Context) []types.RateLimit {
	store := prefix.NewStore(ctx.KVStore(k.storeKey), types.RateLimitKeyPrefix)

	iterator := store.Iterator(nil, nil)
	defer iterator.Close()

	allRateLimits := []types.RateLimit{}
	for ; iterator.Valid(); iterator.Next() {

		rateLimit := types.RateLimit{}
		k.cdc.MustUnmarshal(iterator.Value(), &rateLimit)
		allRateLimits = append(allRateLimits, rateLimit)
	}

	return allRateLimits
}
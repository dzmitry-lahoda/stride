package keeper_test

import (
	"time"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/gogoproto/proto"
	ibctesting "github.com/cosmos/ibc-go/v7/testing"

	epochtypes "github.com/Stride-Labs/stride/v16/x/epochs/types"
	icqtypes "github.com/Stride-Labs/stride/v16/x/interchainquery/types"
	"github.com/Stride-Labs/stride/v16/x/stakeibc/keeper"
	"github.com/Stride-Labs/stride/v16/x/stakeibc/types"
)

func (s *KeeperTestSuite) SetupTradeRewardBalanceCallbackTestCase() BalanceQueryCallbackTestCase {
	// Create the connection between Stride and HostChain with the withdrawal account initialized
	// TODO [DYDX]: Replace with trade route formatter
	tradeAccountOwner := types.FormatICAAccountOwner(HostChainId, types.ICAAccountType_CONVERTER_TRADE)
	tradeChannelId, tradePortId := s.CreateICAChannel(tradeAccountOwner)

	route := types.TradeRoute{
		RewardDenomOnRewardZone: RewardDenom,
		HostDenomOnHostZone:     HostDenom,

		RewardDenomOnTradeZone: "ibc/reward_on_trade",
		HostDenomOnTradeZone:   "ibc/host_on_trade",

		TradeAccount: types.ICAAccount{
			Address:      "trade-address",
			ConnectionId: ibctesting.FirstConnectionID,
		},

		TradeConfig: types.TradeConfig{
			PoolId:                 100,
			SwapPrice:              sdk.OneDec(),
			MinSwapAmount:          sdk.ZeroInt(),
			MaxSwapAmount:          sdkmath.NewInt(1000),
			MaxAllowedSwapLossRate: sdk.MustNewDecFromStr("0.1"),
		},
	}
	s.App.StakeibcKeeper.SetTradeRoute(s.Ctx, route)

	// Create and set the epoch tracker for timeouts
	timeoutDuration := time.Second * 30
	s.CreateEpochForICATimeout(epochtypes.STRIDE_EPOCH, timeoutDuration)

	// Build query object and serialized query response
	balance := sdkmath.NewInt(1_000_000)
	callbackDataBz, _ := proto.Marshal(&types.TradeRouteCallback{
		RewardDenom: RewardDenom,
		HostDenom:   HostDenom,
	})
	query := icqtypes.Query{CallbackData: callbackDataBz}
	queryResponse := s.CreateBalanceQueryResponse(balance.Int64(), route.RewardDenomOnTradeZone)

	// Get start sequence to test ICA submission
	startSequence := s.MustGetNextSequenceNumber(tradePortId, tradeChannelId)

	return BalanceQueryCallbackTestCase{
		TradeRoute: route,
		Balance:    balance,
		Response: ICQCallbackArgs{
			Query:        query,
			CallbackArgs: queryResponse,
		},
		ChannelID:     tradeChannelId,
		PortID:        tradePortId,
		StartSequence: startSequence,
	}
}

// Verify that a normal TradeRewardBalanceCallback does fire off the ICA for transfer
func (s *KeeperTestSuite) TestTradeRewardBalanceCallback_Successful() {
	tc := s.SetupTradeRewardBalanceCallbackTestCase()

	err := keeper.TradeRewardBalanceCallback(s.App.StakeibcKeeper, s.Ctx, tc.Response.CallbackArgs, tc.Response.Query)
	s.Require().NoError(err)

	// Confirm the sequence number was incremented
	endSequence := s.MustGetNextSequenceNumber(tc.PortID, tc.ChannelID)
	s.Require().Equal(endSequence, tc.StartSequence+1, "sequence number should increase after callback executed")
}

// Verify that if the amount returned by the ICQ response is less than the min_swap_amount, no trade happens
func (s *KeeperTestSuite) TestTradeRewardBalanceCallback_SuccessfulNoSwap() {
	tc := s.SetupTradeRewardBalanceCallbackTestCase()

	// Set min swap amount to be greater than the transfer amount
	route := tc.TradeRoute
	route.TradeConfig.MinSwapAmount = tc.Balance.Add(sdkmath.OneInt())
	s.App.StakeibcKeeper.SetTradeRoute(s.Ctx, route)

	err := keeper.TradeRewardBalanceCallback(s.App.StakeibcKeeper, s.Ctx, tc.Response.CallbackArgs, tc.Response.Query)
	s.Require().NoError(err)

	// ICA inside of TransferRewardTokensHostToTrade should not actually execute because of min_swap_amount
	// Confirm the sequence number was NOT incremented
	endSequence := s.MustGetNextSequenceNumber(tc.PortID, tc.ChannelID)
	s.Require().Equal(endSequence, tc.StartSequence, "sequence number should NOT have increased, no swap should happen")
}

func (s *KeeperTestSuite) TestTradeRewardBalanceCallback_ZeroBalance() {
	tc := s.SetupTradeRewardBalanceCallbackTestCase()

	// Replace the query response with a coin that has a zero amount
	tc.Response.CallbackArgs = s.CreateBalanceQueryResponse(0, tc.TradeRoute.HostDenomOnTradeZone)

	err := keeper.TradeRewardBalanceCallback(s.App.StakeibcKeeper, s.Ctx, tc.Response.CallbackArgs, tc.Response.Query)
	s.Require().NoError(err)

	// Confirm the sequence number was NOT incremented, meaning the trade ICA was not executed
	endSequence := s.MustGetNextSequenceNumber(tc.PortID, tc.ChannelID)
	s.Require().Equal(endSequence, tc.StartSequence, "sequence number should NOT have increased, no swap should happen")
}

func (s *KeeperTestSuite) TestTradeRewardBalanceCallback_InvalidArgs() {
	tc := s.SetupTradeRewardBalanceCallbackTestCase()

	// Submit callback with invalid callback args (so that it can't unmarshal into a coin)
	invalidArgs := []byte("random bytes")

	err := keeper.TradeRewardBalanceCallback(s.App.StakeibcKeeper, s.Ctx, invalidArgs, tc.Response.Query)
	s.Require().ErrorContains(err, "unable to determine balance from query response")
}

func (s *KeeperTestSuite) TestTradeRewardBalanceCallback_InvalidCallbackData() {
	tc := s.SetupTradeRewardBalanceCallbackTestCase()

	// Update the callback data so that it can't be successfully unmarshalled
	invalidQuery := tc.Response.Query
	invalidQuery.CallbackData = []byte("random bytes")

	err := keeper.TradeRewardBalanceCallback(s.App.StakeibcKeeper, s.Ctx, tc.Response.CallbackArgs, invalidQuery)
	s.Require().ErrorContains(err, "unable to unmarshal trade reward balance callback data")
}

func (s *KeeperTestSuite) TestTradeRewardBalanceCallback_TradeRouteNotFound() {
	tc := s.SetupTradeRewardBalanceCallbackTestCase()

	// Update the callback data so that it keys to a trade route that doesn't exist
	invalidCallbackDataBz, _ := proto.Marshal(&types.TradeRouteCallback{
		RewardDenom: RewardDenom,
		HostDenom:   "different-host-denom",
	})
	invalidQuery := tc.Response.Query
	invalidQuery.CallbackData = invalidCallbackDataBz

	err := keeper.TradeRewardBalanceCallback(s.App.StakeibcKeeper, s.Ctx, tc.Response.CallbackArgs, invalidQuery)
	s.Require().ErrorContains(err, "trade route not found")
}

func (s *KeeperTestSuite) TestTradeRewardBalanceCallback_FailedSubmitTx() {
	tc := s.SetupTradeRewardBalanceCallbackTestCase()

	// Remove connectionId from host ICAAccount on TradeRoute so the ICA tx fails
	invalidRoute := tc.TradeRoute
	invalidRoute.TradeAccount.ConnectionId = "bad-connection"
	s.App.StakeibcKeeper.SetTradeRoute(s.Ctx, invalidRoute)

	err := keeper.TradeRewardBalanceCallback(s.App.StakeibcKeeper, s.Ctx, tc.Response.CallbackArgs, tc.Response.Query)
	s.Require().ErrorContains(err, "Failed to submit ICA tx")
}
package types

import (
	"regexp"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

// Msg type for MsgUpdateRateLimit
const TypeMsgUpdateRateLimit = "update_rate_limit"

var _ sdk.Msg = &MsgUpdateRateLimit{}

func NewMsgUpdateRateLimit(creator string, denom string, channelId string, maxPercentSend uint64, maxPercentRecv uint64, durationHours uint64) *MsgUpdateRateLimit {
	return &MsgUpdateRateLimit{
		Creator:        creator,
		Denom:          denom,
		ChannelId:      channelId,
		MaxPercentSend: maxPercentSend,
		MaxPercentRecv: maxPercentRecv,
		DurationHours:  durationHours,
	}
}

func (msg *MsgUpdateRateLimit) Route() string {
	return RouterKey
}

func (msg *MsgUpdateRateLimit) Type() string {
	return TypeMsgUpdateRateLimit
}

func (msg *MsgUpdateRateLimit) GetSigners() []sdk.AccAddress {
	creator, err := sdk.AccAddressFromBech32(msg.Creator)
	if err != nil {
		panic(err)
	}
	return []sdk.AccAddress{creator}
}

func (msg *MsgUpdateRateLimit) GetSignBytes() []byte {
	bz := ModuleCdc.MustMarshalJSON(msg)
	return sdk.MustSortJSON(bz)
}

func (msg *MsgUpdateRateLimit) ValidateBasic() error {
	_, err := sdk.AccAddressFromBech32(msg.Creator)
	if err != nil {
		return sdkerrors.Wrapf(sdkerrors.ErrInvalidAddress, "invalid creator address (%s)", err)
	}

	if msg.Denom == "" {
		return sdkerrors.Wrapf(sdkerrors.ErrInvalidRequest, "invalid denom (%s)", msg.Denom)
	}

	matched, err := regexp.MatchString(`^channel-\d+$`, msg.ChannelId)
	if err != nil {
		return sdkerrors.Wrapf(sdkerrors.ErrInvalidRequest, "unable to verify channel-id (%s)", msg.ChannelId)
	}
	if !matched {
		return sdkerrors.Wrapf(sdkerrors.ErrInvalidRequest, "invalid channel-id (%s), must be of the format 'channel-{N}'", msg.ChannelId)
	}

	if msg.MaxPercentRecv > 100 || msg.MaxPercentSend > 100 {
		return sdkerrors.Wrapf(sdkerrors.ErrInvalidRequest, "percent must be between 0 and 100 (inclusively), Provided: Send - %d, Recv - %d", msg.MaxPercentSend, msg.MaxPercentRecv)
	}

	if msg.MaxPercentRecv == 0 && msg.MaxPercentSend == 0 {
		return sdkerrors.Wrapf(sdkerrors.ErrInvalidRequest, "either the max send or max receive threshold must be greater than 0")
	}

	if msg.DurationHours == 0 {
		return sdkerrors.Wrapf(sdkerrors.ErrInvalidRequest, "duration can not be zero")
	}

	return nil
}
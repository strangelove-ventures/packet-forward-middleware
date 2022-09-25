package types

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	clienttypes "github.com/cosmos/ibc-go/v5/modules/core/02-client/types"
	channeltypes "github.com/cosmos/ibc-go/v5/modules/core/04-channel/types"
)

// TransferKeeper defines the expected transfer keeper
type TransferKeeper interface {
	SendTransferWithResult(
		ctx sdk.Context,
		sourcePort string,
		sourceChannel string,
		token sdk.Coin,
		sender sdk.AccAddress,
		receiver string,
		timeoutHeight clienttypes.Height,
		timeoutTimestamp uint64,
	) (channeltypes.Packet, error)
}

// DistributionKeeper defines the expected distribution keeper
type DistributionKeeper interface {
	FundCommunityPool(ctx sdk.Context, amount sdk.Coins, sender sdk.AccAddress) error
}

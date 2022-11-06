package types

import (
	context "context"

	sdk "github.com/cosmos/cosmos-sdk/types"
	capabilitytypes "github.com/cosmos/cosmos-sdk/x/capability/types"
	"github.com/cosmos/ibc-go/v3/modules/apps/transfer/types"
	"github.com/cosmos/ibc-go/v3/modules/core/exported"
)

// TransferKeeper defines the expected transfer keeper
type TransferKeeper interface {
	Transfer(ctx context.Context, msg *types.MsgTransfer) (*types.MsgTransferResponse, error)
}

// ChannelKeeper defines the expected IBC channel keeper
type ChannelKeeper interface {
	LookupModuleByChannel(ctx sdk.Context, portID, channelID string) (string, *capabilitytypes.Capability, error)
	WriteAcknowledgement(
		ctx sdk.Context,
		chanCap *capabilitytypes.Capability,
		packet exported.PacketI,
		ack exported.Acknowledgement,
	) error
}

// DistributionKeeper defines the expected distribution keeper
type DistributionKeeper interface {
	FundCommunityPool(ctx sdk.Context, amount sdk.Coins, sender sdk.AccAddress) error
}

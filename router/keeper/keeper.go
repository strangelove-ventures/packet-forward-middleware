package keeper

import (
	"github.com/armon/go-metrics"
	"github.com/tendermint/tendermint/libs/log"

	"github.com/cosmos/cosmos-sdk/codec"
	storetypes "github.com/cosmos/cosmos-sdk/store/types"
	"github.com/cosmos/cosmos-sdk/telemetry"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	paramtypes "github.com/cosmos/cosmos-sdk/x/params/types"
	transfertypes "github.com/cosmos/ibc-go/v5/modules/apps/transfer/types"
	clienttypes "github.com/cosmos/ibc-go/v5/modules/core/02-client/types"
	host "github.com/cosmos/ibc-go/v5/modules/core/24-host"
	coretypes "github.com/cosmos/ibc-go/v5/modules/core/types"
	"github.com/strangelove-ventures/packet-forward-middleware/v2/router/parser"
	"github.com/strangelove-ventures/packet-forward-middleware/v2/router/types"
)

// Keeper defines the IBC fungible transfer keeper
type Keeper struct {
	storeKey   storetypes.StoreKey
	cdc        codec.BinaryCodec
	paramSpace paramtypes.Subspace

	transferKeeper types.TransferKeeper
	distrKeeper    types.DistributionKeeper
}

var (
	// Timeout height following IBC defaults
	DefaultTransferPacketTimeoutHeight = clienttypes.MustParseHeight(transfertypes.DefaultRelativePacketTimeoutHeight)
	// Timeout timestamp following IBC defaults
	DefaultTransferPacketTimeoutTimestamp = transfertypes.DefaultRelativePacketTimeoutTimestamp
)

// NewKeeper creates a new 29-fee Keeper instance
func NewKeeper(
	cdc codec.BinaryCodec, key storetypes.StoreKey, paramSpace paramtypes.Subspace,
	transferKeeper types.TransferKeeper, distrKeeper types.DistributionKeeper,
) Keeper {
	// set KeyTable if it has not already been set
	if !paramSpace.HasKeyTable() {
		paramSpace = paramSpace.WithKeyTable(types.ParamKeyTable())
	}

	return Keeper{
		cdc:            cdc,
		storeKey:       key,
		transferKeeper: transferKeeper,
		paramSpace:     paramSpace,
		distrKeeper:    distrKeeper,
	}
}

// Logger returns a module-specific logger.
func (k Keeper) Logger(ctx sdk.Context) log.Logger {
	return ctx.Logger().With("module", "x/"+host.ModuleName+"-"+types.ModuleName)
}

func (k Keeper) ForwardTransferPacket(ctx sdk.Context, parsedReceiver *parser.ParsedReceiver, token sdk.Coin, labels []metrics.Label) error {
	var err error
	feeAmount := sdk.NewDecFromInt(token.Amount).Mul(k.GetFeePercentage(ctx)).RoundInt()
	packetAmount := token.Amount.Sub(feeAmount)
	feeCoins := sdk.Coins{sdk.NewCoin(token.Denom, feeAmount)}
	packetCoin := sdk.NewCoin(token.Denom, packetAmount)

	// pay fees
	if feeAmount.IsPositive() {
		err = k.distrKeeper.FundCommunityPool(ctx, feeCoins, parsedReceiver.HostAccAddr)
		if err != nil {
			return sdkerrors.Wrapf(sdkerrors.ErrInsufficientFunds, err.Error())
		}
	}

	// send tokens to destination
	err = k.transferKeeper.SendTransfer(
		ctx,
		parsedReceiver.Port,
		parsedReceiver.Channel,
		packetCoin,
		parsedReceiver.HostAccAddr,
		parsedReceiver.Destination,
		DefaultTransferPacketTimeoutHeight,
		DefaultTransferPacketTimeoutTimestamp+uint64(ctx.BlockTime().UnixNano()),
	)
	if err != nil {
		return sdkerrors.Wrapf(sdkerrors.ErrInsufficientFunds, err.Error())
	}

	defer func() {
		telemetry.SetGaugeWithLabels(
			[]string{"tx", "msg", "ibc", "transfer"},
			float32(token.Amount.Int64()),
			[]metrics.Label{telemetry.NewLabel(coretypes.LabelDenom, token.Denom)},
		)

		telemetry.IncrCounterWithLabels(
			[]string{"ibc", types.ModuleName, "send"},
			1,
			labels,
		)
	}()
	return nil
}

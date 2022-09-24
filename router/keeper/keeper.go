package keeper

import (
	"fmt"

	"github.com/armon/go-metrics"
	"github.com/tendermint/tendermint/libs/log"

	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/telemetry"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	paramtypes "github.com/cosmos/cosmos-sdk/x/params/types"
	transfertypes "github.com/cosmos/ibc-go/v3/modules/apps/transfer/types"
	channeltypes "github.com/cosmos/ibc-go/v3/modules/core/04-channel/types"
	host "github.com/cosmos/ibc-go/v3/modules/core/24-host"
	coretypes "github.com/cosmos/ibc-go/v3/modules/core/types"
	"github.com/strangelove-ventures/packet-forward-middleware/v2/router/parser"
	"github.com/strangelove-ventures/packet-forward-middleware/v2/router/types"
)

// Keeper defines the IBC fungible transfer keeper
type Keeper struct {
	storeKey   sdk.StoreKey
	cdc        codec.BinaryCodec
	paramSpace paramtypes.Subspace

	transferKeeper types.TransferKeeper
	distrKeeper    types.DistributionKeeper
}

// NewKeeper creates a new 29-fee Keeper instance
func NewKeeper(
	cdc codec.BinaryCodec, key sdk.StoreKey, paramSpace paramtypes.Subspace,
	transferKeeper types.TransferKeeper, distrKeeper types.DistributionKeeper,
) Keeper {

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

func (k Keeper) ForwardTransferPacket(ctx sdk.Context, inFlightPacket *types.InFlightPacket, srcPacket channeltypes.Packet, srcPacketSender string, parsedReceiver *parser.ParsedReceiver, token sdk.Coin, labels []metrics.Label) error {
	var err error
	feeAmount := sdk.NewDecFromInt(token.Amount).Mul(k.GetFeePercentage(ctx)).RoundInt()
	packetAmount := token.Amount.Sub(feeAmount)
	feeCoins := sdk.Coins{sdk.NewCoin(token.Denom, feeAmount)}
	packetCoin := sdk.NewCoin(token.Denom, packetAmount)

	// pay fees
	if feeAmount.IsPositive() {
		if err := k.distrKeeper.FundCommunityPool(ctx, feeCoins, receiver); err != nil {
			return sdkerrors.Wrapf(sdkerrors.ErrInsufficientFunds, err.Error())
		}
	}

	// send tokens to destination
	sequence, err := k.transferKeeper.SendTransfer(
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
		// TODO refund to src chain
		return sdkerrors.Wrapf(sdkerrors.ErrInsufficientFunds, err.Error())
	}

	// Store the following information in keeper:
	// key - information about forwarded packet: src_channel (parsedReceiver.Channel), src_port (parsedReceiver.Port), sequence
	// value - information about original packet for refunding if necessary: retries, srcPacketSender, srcPacket.DestinationChannel, srcPacket.DestinationPort

	if inFlightPacket == nil {
		inFlightPacket = &types.InFlightPacket{
			OriginalSenderAddress: srcPacketSender,
			RefundChannelId:       srcPacket.DestinationChannel,
			RefundPortId:          srcPacket.DestinationPort,
			Retries:               0,
			MaxRetries:            int32(parsedReceiver.MaxRetries),
		}
	} else {
		inFlightPacket.Retries++
	}

	store := ctx.KVStore(k.storeKey)
	bz := k.cdc.MustMarshal(inFlightPacket)
	store.Set(types.RefundPacketKey(parsedReceiver.Channel, parsedReceiver.Port, sequence), bz)

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

func (k Keeper) HandleTimeout(ctx sdk.Context, packet channeltypes.Packet, relayer sdk.AccAddress) error {
	store := ctx.KVStore(k.storeKey)
	key := types.RefundPacketKey(packet.SourceChannel, packet.SourcePort, packet.Sequence)
	if !store.Has(key) {
		// not a forwarded packet, so ignore
		return nil
	}

	bz := store.Get(key)
	var inFlightPacket types.InFlightPacket
	k.cdc.MustUnmarshal(bz, &inFlightPacket)

	if inFlightPacket.Retries >= inFlightPacket.MaxRetries {
		return fmt.Errorf("giving up on packet on channel (%s) port (%s) after max retries: (%d)",
			inFlightPacket.RefundChannelId, inFlightPacket.RefundPortId, inFlightPacket.MaxRetries)
	}

	// Parse packet data
	var data transfertypes.FungibleTokenPacketData
	if err := transfertypes.ModuleCdc.UnmarshalJSON(packet.GetData(), &data); err != nil {
		return fmt.Errorf("error unmarshalling packet data: %w", err)
	}

	// send transfer again
	receiver := &parser.ParsedReceiver{
		HostAccAddr: sdk.AccAddress(data.Sender),
		Destination: data.Receiver,
		Channel:     packet.SourceChannel,
		Port:        packet.SourcePort,
		MaxRetries:  uint8(inFlightPacket.MaxRetries),
	}

	amount, ok := sdk.NewIntFromString(data.Amount)
	if !ok {
		return fmt.Errorf("error parsing amount from string for router retry: %s", data.Amount)
	}

	var token = sdk.NewCoin(data.Denom, amount)

	return k.ForwardTransferPacket(ctx, &inFlightPacket, channeltypes.Packet{}, "", receiver, token, nil)
}

func (k Keeper) RefundForwardedPacket(ctx sdk.Context, packet channeltypes.Packet, relayer sdk.AccAddress) error {
	store := ctx.KVStore(k.storeKey)
	key := types.RefundPacketKey(packet.SourceChannel, packet.SourcePort, packet.Sequence)
	if !store.Has(key) {
		return fmt.Errorf("called RefundForwardedPacket but no store key exists for that packet: %s", string(key))
	}

	bz := store.Get(key)
	var inFlightPacket types.InFlightPacket
	k.cdc.MustUnmarshal(bz, &inFlightPacket)

	// Parse packet data
	var data transfertypes.FungibleTokenPacketData
	if err := transfertypes.ModuleCdc.UnmarshalJSON(packet.GetData(), &data); err != nil {
		return fmt.Errorf("error unmarshalling packet data: %w", err)
	}

	amount, ok := sdk.NewIntFromString(data.Amount)
	if !ok {
		return fmt.Errorf("error parsing amount from string for router retry: %s", data.Amount)
	}

	var token = sdk.NewCoin(data.Denom, amount)

	_, err := k.transferKeeper.SendTransfer(
		ctx,
		inFlightPacket.RefundPortId,
		inFlightPacket.RefundChannelId,
		token,
		relayer,
		inFlightPacket.OriginalSenderAddress,
		DefaultTransferPacketTimeoutHeight,
		DefaultTransferPacketTimeoutTimestamp+uint64(ctx.BlockTime().UnixNano()),
	)

	return err
}

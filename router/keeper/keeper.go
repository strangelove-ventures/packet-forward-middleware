package keeper

import (
	"fmt"
	"time"

	"github.com/armon/go-metrics"
	"github.com/tendermint/tendermint/libs/log"

	"github.com/cosmos/cosmos-sdk/codec"
	storetypes "github.com/cosmos/cosmos-sdk/store/types"
	"github.com/cosmos/cosmos-sdk/telemetry"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	paramtypes "github.com/cosmos/cosmos-sdk/x/params/types"
	transfertypes "github.com/cosmos/ibc-go/v3/modules/apps/transfer/types"
	clienttypes "github.com/cosmos/ibc-go/v3/modules/core/02-client/types"
	channeltypes "github.com/cosmos/ibc-go/v3/modules/core/04-channel/types"
	host "github.com/cosmos/ibc-go/v3/modules/core/24-host"
	coretypes "github.com/cosmos/ibc-go/v3/modules/core/types"
	"github.com/strangelove-ventures/packet-forward-middleware/v3/router/types"
)

// Keeper defines the IBC fungible transfer keeper
type Keeper struct {
	storeKey   storetypes.StoreKey
	cdc        codec.BinaryCodec
	paramSpace paramtypes.Subspace

	transferKeeper types.TransferKeeper
	channelKeeper  types.ChannelKeeper
	distrKeeper    types.DistributionKeeper
}

type ForwardMetadata struct {
	Receiver string        `json:"receiver"`
	Port     string        `json:"port"`
	Channel  string        `json:"channel"`
	Timeout  time.Duration `json:"timeout"`
	Retries  *uint8        `json:"retries"`
}

var (
	// Timeout height following IBC defaults
	DefaultTransferPacketTimeoutHeight = clienttypes.Height{
		RevisionNumber: 0,
		RevisionHeight: 0,
	}
	// Timeout timestamp following IBC defaults
	DefaultForwardTransferPacketTimeoutTimestamp = time.Duration(transfertypes.DefaultRelativePacketTimeoutTimestamp) * time.Nanosecond

	// 28 day timeout for refund packets since funds are stuck in router module otherwise.
	DefaultRefundTransferPacketTimeoutTimestamp = 28 * 24 * time.Hour
)

// NewKeeper creates a new 29-fee Keeper instance
func NewKeeper(
	cdc codec.BinaryCodec, key storetypes.StoreKey, paramSpace paramtypes.Subspace,
	transferKeeper types.TransferKeeper, channelKeeper types.ChannelKeeper, distrKeeper types.DistributionKeeper,
) Keeper {
	// set KeyTable if it has not already been set
	if !paramSpace.HasKeyTable() {
		paramSpace = paramSpace.WithKeyTable(types.ParamKeyTable())
	}

	return Keeper{
		cdc:        cdc,
		storeKey:   key,
		paramSpace: paramSpace,

		transferKeeper: transferKeeper,
		channelKeeper:  channelKeeper,
		distrKeeper:    distrKeeper,
	}
}

// Logger returns a module-specific logger.
func (k Keeper) Logger(ctx sdk.Context) log.Logger {
	return ctx.Logger().With("module", "x/"+host.ModuleName+"-"+types.ModuleName)
}

func (k Keeper) ForwardTransferPacket(
	ctx sdk.Context,
	inFlightPacket *types.InFlightPacket,
	srcPacket channeltypes.Packet,
	srcPacketSender string,
	receiver string,
	metadata *ForwardMetadata,
	token sdk.Coin,
	maxRetries uint8,
	timeout time.Duration,
	labels []metrics.Label,
) error {
	var err error
	feeAmount := sdk.NewDecFromInt(token.Amount).Mul(k.GetFeePercentage(ctx)).RoundInt()
	packetAmount := token.Amount.Sub(feeAmount)
	feeCoins := sdk.Coins{sdk.NewCoin(token.Denom, feeAmount)}
	packetCoin := sdk.NewCoin(token.Denom, packetAmount)

	// pay fees
	if feeAmount.IsPositive() {
		hostAccAddr, err := sdk.AccAddressFromBech32(receiver)
		if err != nil {
			return err
		}
		err = k.distrKeeper.FundCommunityPool(ctx, feeCoins, hostAccAddr)
		if err != nil {
			k.Logger(ctx).Error("packetForwardMiddleware error funding community pool",
				"error", err,
			)
			return sdkerrors.Wrapf(sdkerrors.ErrInsufficientFunds, err.Error())
		}
	}

	// send tokens to destination
	res, err := k.transferKeeper.Transfer(
		sdk.WrapSDKContext(ctx),
		transfertypes.NewMsgTransfer(
			metadata.Port,
			metadata.Channel,
			packetCoin,
			receiver,
			metadata.Receiver,
			DefaultTransferPacketTimeoutHeight,
			uint64(ctx.BlockTime().UnixNano())+uint64(timeout.Nanoseconds()),
		),
	)
	if err != nil {
		k.Logger(ctx).Error("packetForwardMiddleware SendPacketTransfer error",
			"error", err,
			"amount", packetCoin.Amount.String(),
			"denom", packetCoin.Denom,
			"sender", receiver,
			"receiver", metadata.Receiver,
			"port", metadata.Port,
			"channel", metadata.Channel,
		)
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
			RetriesRemaining:      int32(maxRetries),
			Timeout:               uint64(timeout.Nanoseconds()),
		}
	} else {
		inFlightPacket.RetriesRemaining--
	}

	key := types.RefundPacketKey(metadata.Channel, metadata.Port, res.Sequence)
	store := ctx.KVStore(k.storeKey)
	bz := k.cdc.MustMarshal(inFlightPacket)
	store.Set(key, bz)

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

func (k Keeper) HandleTimeout(
	ctx sdk.Context,
	packet channeltypes.Packet,
) error {
	store := ctx.KVStore(k.storeKey)
	key := types.RefundPacketKey(packet.SourceChannel, packet.SourcePort, packet.Sequence)

	if !store.Has(key) {
		// not a forwarded packet, ignore.
		return nil
	}

	bz := store.Get(key)
	var inFlightPacket types.InFlightPacket
	k.cdc.MustUnmarshal(bz, &inFlightPacket)

	if inFlightPacket.RetriesRemaining <= 0 {
		k.Logger(ctx).Error("packetForwardMiddleware reached max retries for packet",
			"key", string(key),
			"original-sender-address", inFlightPacket.OriginalSenderAddress,
			"refund-channel-id", inFlightPacket.RefundChannelId,
			"refund-port-id", inFlightPacket.RefundPortId,
		)
		return fmt.Errorf("giving up on packet on channel (%s) port (%s) after max retries",
			inFlightPacket.RefundChannelId, inFlightPacket.RefundPortId)
	}

	// Parse packet data
	var data transfertypes.FungibleTokenPacketData
	if err := transfertypes.ModuleCdc.UnmarshalJSON(packet.GetData(), &data); err != nil {
		k.Logger(ctx).Error("packetForwardMiddleware error unmarshalling packet data",
			"key", string(key),
			"original-sender-address", inFlightPacket.OriginalSenderAddress,
			"refund-channel-id", inFlightPacket.RefundChannelId,
			"refund-port-id", inFlightPacket.RefundPortId,
			"retries-remaining", inFlightPacket.RetriesRemaining,
			"error", err,
		)
		return fmt.Errorf("error unmarshalling packet data: %w", err)
	}

	// send transfer again
	metadata := &ForwardMetadata{
		Receiver: data.Receiver,
		Channel:  packet.SourceChannel,
		Port:     packet.SourcePort,
	}

	amount, ok := sdk.NewIntFromString(data.Amount)
	if !ok {
		k.Logger(ctx).Error("packetForwardMiddleware error parsing amount from string for router retry",
			"key", string(key),
			"original-sender-address", inFlightPacket.OriginalSenderAddress,
			"refund-channel-id", inFlightPacket.RefundChannelId,
			"refund-port-id", inFlightPacket.RefundPortId,
			"retries-remaining", inFlightPacket.RetriesRemaining,
			"amount", data.Amount,
		)
		return fmt.Errorf("error parsing amount from string for router retry: %s", data.Amount)
	}

	denom := transfertypes.ParseDenomTrace(data.Denom).IBCDenom()

	var token = sdk.NewCoin(denom, amount)

	return k.ForwardTransferPacket(ctx, &inFlightPacket, channeltypes.Packet{}, "", data.Sender, metadata, token, uint8(inFlightPacket.RetriesRemaining), time.Duration(inFlightPacket.Timeout)*time.Nanosecond, nil)
}

func (k Keeper) RemoveInFlightPacket(ctx sdk.Context, packet channeltypes.Packet) {
	store := ctx.KVStore(k.storeKey)
	key := types.RefundPacketKey(packet.SourceChannel, packet.SourcePort, packet.Sequence)
	if !store.Has(key) {
		// not a forwarded packet, ignore.
		return
	}

	// done with packet key now, delete.
	store.Delete(key)
}

func (k Keeper) RefundForwardedPacket(ctx sdk.Context, packet channeltypes.Packet, timeout time.Duration) {
	store := ctx.KVStore(k.storeKey)
	key := types.RefundPacketKey(packet.SourceChannel, packet.SourcePort, packet.Sequence)
	if !store.Has(key) {
		// not a forwarded packet, ignore.
		return
	}

	bz := store.Get(key)

	// done with packet key now, delete.
	store.Delete(key)

	var inFlightPacket types.InFlightPacket
	k.cdc.MustUnmarshal(bz, &inFlightPacket)

	// Parse packet data
	var data transfertypes.FungibleTokenPacketData
	if err := transfertypes.ModuleCdc.UnmarshalJSON(packet.GetData(), &data); err != nil {
		k.Logger(ctx).Error("packetForwardMiddleware error unmarshalling packet data for refund",
			"key", string(key),
			"sequence", packet.Sequence,
			"channel-id", packet.SourceChannel,
			"port-id", packet.SourcePort,
			"original-sender-address", inFlightPacket.OriginalSenderAddress,
			"refund-channel-id", inFlightPacket.RefundChannelId,
			"refund-port-id", inFlightPacket.RefundPortId,
		)
		return
	}

	amount, ok := sdk.NewIntFromString(data.Amount)
	if !ok {
		k.Logger(ctx).Error("packetForwardMiddleware error parsing amount from string for refund",
			"key", string(key),
			"sequence", packet.Sequence,
			"channel-id", packet.SourceChannel,
			"port-id", packet.SourcePort,
			"original-sender-address", inFlightPacket.OriginalSenderAddress,
			"refund-channel-id", inFlightPacket.RefundChannelId,
			"refund-port-id", inFlightPacket.RefundPortId,
			"amount", data.Amount,
		)
		return
	}

	if _, err := k.transferKeeper.Transfer(
		sdk.WrapSDKContext(ctx),
		transfertypes.NewMsgTransfer(
			inFlightPacket.RefundPortId,
			inFlightPacket.RefundChannelId,
			sdk.NewCoin(transfertypes.ParseDenomTrace(data.Denom).IBCDenom(), amount),
			data.Sender,
			inFlightPacket.OriginalSenderAddress,
			DefaultTransferPacketTimeoutHeight,
			uint64(timeout.Nanoseconds())+uint64(ctx.BlockTime().UnixNano()),
		),
	); err != nil {
		k.Logger(ctx).Error("packetForwardMiddleware error sending packet transfer for refund",
			"key", string(key),
			"sequence", packet.Sequence,
			"channel-id", packet.SourceChannel,
			"port-id", packet.SourcePort,
			"original-sender-address", inFlightPacket.OriginalSenderAddress,
			"refund-channel-id", inFlightPacket.RefundChannelId,
			"refund-port-id", inFlightPacket.RefundPortId,
			"amount", data.Amount,
			"error", err,
		)
	}
}

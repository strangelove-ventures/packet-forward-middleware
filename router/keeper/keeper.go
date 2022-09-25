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
	transfertypes "github.com/cosmos/ibc-go/v5/modules/apps/transfer/types"
	clienttypes "github.com/cosmos/ibc-go/v5/modules/core/02-client/types"
	channeltypes "github.com/cosmos/ibc-go/v5/modules/core/04-channel/types"
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
	DefaultTransferPacketTimeoutHeight = clienttypes.Height{
		RevisionNumber: 0,
		RevisionHeight: 0,
	}
	// Timeout timestamp following IBC defaults
	DefaultTransferPacketTimeoutTimestamp = transfertypes.DefaultRelativePacketTimeoutTimestamp

	// 28 day timeout for refund packets since funds are stuck in router module otherwise.
	RefundTransferPacketTimeoutTimestamp = uint64((28 * 24 * time.Hour).Nanoseconds())
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

func (k Keeper) ForwardTransferPacket(ctx sdk.Context, inFlightPacket *types.InFlightPacket, srcPacket channeltypes.Packet, srcPacketSender string, parsedReceiver *parser.ParsedReceiver, token sdk.Coin, labels []metrics.Label) error {
	var err error
	feeAmount := sdk.NewDecFromInt(token.Amount).Mul(k.GetFeePercentage(ctx)).RoundInt()
	packetAmount := token.Amount.Sub(feeAmount)
	feeCoins := sdk.Coins{sdk.NewCoin(token.Denom, feeAmount)}
	packetCoin := sdk.NewCoin(token.Denom, packetAmount)

	// pay fees
	if feeAmount.IsPositive() {
		err = k.distrKeeper.FundCommunityPool(ctx, feeCoins, parsedReceiver.HostAccAddr)
		if err != nil {
			k.Logger(ctx).Error("packetForwardMiddleware error funding community pool",
				"error", err,
			)
			return sdkerrors.Wrapf(sdkerrors.ErrInsufficientFunds, err.Error())
		}
	}

	var timeoutTimestamp uint64
	timeout := parsedReceiver.Timeout.Nanoseconds()
	if timeout <= 0 {
		timeoutTimestamp = DefaultTransferPacketTimeoutTimestamp
	} else {
		timeoutTimestamp = uint64(timeoutTimestamp)
	}

	// send tokens to destination
	sequence, err := k.transferKeeper.SendPacketTransfer(
		ctx,
		parsedReceiver.Port,
		parsedReceiver.Channel,
		packetCoin,
		parsedReceiver.HostAccAddr,
		parsedReceiver.Destination,
		DefaultTransferPacketTimeoutHeight,
		timeoutTimestamp+uint64(ctx.BlockTime().UnixNano()),
	)
	if err != nil {
		k.Logger(ctx).Error("packetForwardMiddleware SendPacketTransfer error",
			"error", err,
			"amount", packetCoin.Amount.String(),
			"denom", packetCoin.Denom,
			"sender", parsedReceiver.HostAccAddr,
			"receiver", parsedReceiver.Destination,
			"port", parsedReceiver.Port,
			"channel", parsedReceiver.Channel,
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
			Retries:               0,
			MaxRetries:            int32(parsedReceiver.MaxRetries),
		}
	} else {
		inFlightPacket.Retries++
	}

	key := types.RefundPacketKey(parsedReceiver.Channel, parsedReceiver.Port, sequence)
	store := ctx.KVStore(k.storeKey)
	bz := k.cdc.MustMarshal(inFlightPacket)
	store.Set(key, bz)

	k.Logger(ctx).Debug("packetForwardMiddleware Saved forwarded packet info",
		"key", string(key),
		"original-sender-address", srcPacketSender,
		"refund-channel-id", srcPacket.DestinationChannel,
		"refund-port-id", srcPacket.DestinationPort,
		"max-retries", parsedReceiver.MaxRetries,
	)

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

func (k Keeper) HandleTimeout(ctx sdk.Context, packet channeltypes.Packet) error {
	store := ctx.KVStore(k.storeKey)
	key := types.RefundPacketKey(packet.SourceChannel, packet.SourcePort, packet.Sequence)

	k.Logger(ctx).Debug("packetForwardMiddleware observed timeout",
		"key", string(key),
	)

	if !store.Has(key) {
		// not a forwarded packet, so ignore
		return nil
	}

	bz := store.Get(key)
	var inFlightPacket types.InFlightPacket
	k.cdc.MustUnmarshal(bz, &inFlightPacket)

	if inFlightPacket.Retries >= inFlightPacket.MaxRetries {
		k.Logger(ctx).Error("packetForwardMiddleware reached max retries for packet",
			"key", string(key),
			"original-sender-address", inFlightPacket.OriginalSenderAddress,
			"refund-channel-id", inFlightPacket.RefundChannelId,
			"refund-port-id", inFlightPacket.RefundPortId,
			"max-retries", inFlightPacket.MaxRetries,
		)
		return fmt.Errorf("giving up on packet on channel (%s) port (%s) after max retries: (%d)",
			inFlightPacket.RefundChannelId, inFlightPacket.RefundPortId, inFlightPacket.MaxRetries)
	}

	// Parse packet data
	var data transfertypes.FungibleTokenPacketData
	if err := transfertypes.ModuleCdc.UnmarshalJSON(packet.GetData(), &data); err != nil {
		k.Logger(ctx).Error("packetForwardMiddleware error unmarshalling packet data",
			"key", string(key),
			"original-sender-address", inFlightPacket.OriginalSenderAddress,
			"refund-channel-id", inFlightPacket.RefundChannelId,
			"refund-port-id", inFlightPacket.RefundPortId,
			"max-retries", inFlightPacket.MaxRetries,
			"error", err,
		)
		return fmt.Errorf("error unmarshalling packet data: %w", err)
	}

	// send transfer again
	receiver := &parser.ParsedReceiver{
		HostAccAddr: sdk.MustAccAddressFromBech32(data.Sender),
		Destination: data.Receiver,
		Channel:     packet.SourceChannel,
		Port:        packet.SourcePort,
		MaxRetries:  uint8(inFlightPacket.MaxRetries),
	}

	amount, ok := sdk.NewIntFromString(data.Amount)
	if !ok {
		k.Logger(ctx).Error("packetForwardMiddleware error parsing amount from string for router retry",
			"key", string(key),
			"original-sender-address", inFlightPacket.OriginalSenderAddress,
			"refund-channel-id", inFlightPacket.RefundChannelId,
			"refund-port-id", inFlightPacket.RefundPortId,
			"max-retries", inFlightPacket.MaxRetries,
			"amount", data.Amount,
		)
		return fmt.Errorf("error parsing amount from string for router retry: %s", data.Amount)
	}

	var token = sdk.NewCoin(data.Denom, amount)

	return k.ForwardTransferPacket(ctx, &inFlightPacket, channeltypes.Packet{}, "", receiver, token, nil)
}

func (k Keeper) RemoveTrackedPacket(ctx sdk.Context, packet channeltypes.Packet) {
	store := ctx.KVStore(k.storeKey)
	key := types.RefundPacketKey(packet.SourceChannel, packet.SourcePort, packet.Sequence)
	if !store.Has(key) {
		// not a forwarded packet, so ignore
		return
	}

	store.Delete(key)
}

func (k Keeper) RefundForwardedPacket(ctx sdk.Context, packet channeltypes.Packet) error {
	store := ctx.KVStore(k.storeKey)
	key := types.RefundPacketKey(packet.SourceChannel, packet.SourcePort, packet.Sequence)
	if !store.Has(key) {
		k.Logger(ctx).Error("packetForwardMiddleware no store key exists for that packet",
			"key", string(key),
			"sequence", packet.Sequence,
			"channel-id", packet.SourceChannel,
			"port-id", packet.SourcePort,
		)
		return fmt.Errorf("called RefundForwardedPacket but no store key exists for that packet: %s", string(key))
	}

	bz := store.Get(key)
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
		return fmt.Errorf("error unmarshalling packet data: %w", err)
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
		return fmt.Errorf("error parsing amount from string for refund: %s", data.Amount)
	}

	denom := transfertypes.ParseDenomTrace(data.Denom).IBCDenom()

	var token = sdk.NewCoin(denom, amount)

	_, err := k.transferKeeper.SendPacketTransfer(
		ctx,
		inFlightPacket.RefundPortId,
		inFlightPacket.RefundChannelId,
		token,
		sdk.MustAccAddressFromBech32(data.Sender),
		inFlightPacket.OriginalSenderAddress,
		DefaultTransferPacketTimeoutHeight,
		RefundTransferPacketTimeoutTimestamp+uint64(ctx.BlockTime().UnixNano()),
	)

	if err != nil {
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

	k.RemoveTrackedPacket(ctx, packet)

	return err
}

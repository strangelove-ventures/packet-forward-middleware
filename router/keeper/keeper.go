package keeper

import (
	"encoding/json"
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

type PacketMetadata struct {
	Forward *ForwardMetadata `json:"forward"`
}

type ForwardMetadata struct {
	Receiver       string        `json:"receiver,omitempty"`
	Port           string        `json:"port,omitempty"`
	Channel        string        `json:"channel,omitempty"`
	Timeout        time.Duration `json:"timeout,omitempty"`
	Retries        *uint8        `json:"retries,omitempty"`
	Next           *string       `json:"next,omitempty"`
	RefundSequence *uint64       `json:"refund_sequence,omitempty"`
}

func (m *ForwardMetadata) Validate() error {
	if m.Receiver == "" {
		return fmt.Errorf("failed to validate forward metadata. receiver cannot be empty")
	}
	if err := host.PortIdentifierValidator(m.Port); err != nil {
		return fmt.Errorf("failed to validate forward metadata: %w", err)
	}
	if err := host.ChannelIdentifierValidator(m.Channel); err != nil {
		return fmt.Errorf("failed to validate forward metadata: %w", err)
	}

	return nil
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

	msgTransfer := transfertypes.NewMsgTransfer(
		metadata.Port,
		metadata.Channel,
		packetCoin,
		receiver,
		metadata.Receiver,
		DefaultTransferPacketTimeoutHeight,
		uint64(ctx.BlockTime().UnixNano())+uint64(timeout.Nanoseconds()),
	)

	// set memo for next transfer with next from this transfer.
	if metadata.Next != nil {
		msgTransfer.Memo = *metadata.Next
	}

	k.Logger(ctx).Debug("packetForwardMiddleware ForwardTransferPacket",
		"port", metadata.Port, "channel", metadata.Channel,
		"sender", receiver, "receiver", metadata.Receiver,
		"amount", packetCoin.Amount.String(), "denom", packetCoin.Denom,
	)

	// send tokens to destination
	res, err := k.transferKeeper.Transfer(
		sdk.WrapSDKContext(ctx),
		msgTransfer,
	)
	if err != nil {
		k.Logger(ctx).Error("packetForwardMiddleware ForwardTransferPacket error",
			"port", metadata.Port, "channel", metadata.Channel,
			"sender", receiver, "receiver", metadata.Receiver,
			"amount", packetCoin.Amount.String(), "denom", packetCoin.Denom,
			"error", err,
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
			RefundSequence:        srcPacket.Sequence,
			RefundDenom:           token.Denom,
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

	if data.Memo != "" {
		metadata.Next = &data.Memo
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

	// srcPacket and srcPacketSender are empty because inFlightPacket is non-nil.
	return k.ForwardTransferPacket(
		ctx,
		&inFlightPacket,
		channeltypes.Packet{},
		"",
		data.Sender,
		metadata,
		token,
		uint8(inFlightPacket.RetriesRemaining),
		time.Duration(inFlightPacket.Timeout)*time.Nanosecond,
		nil,
	)
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

// GetAndClearInFlightPacket will fetch an InFlightPacket from the store, remove it if it exists, and return it.
func (k Keeper) GetAndClearInFlightPacket(
	ctx sdk.Context,
	channel string,
	port string,
	sequence uint64,
) *types.InFlightPacket {
	store := ctx.KVStore(k.storeKey)
	key := types.RefundPacketKey(channel, port, sequence)
	if !store.Has(key) {
		// this is either not a forwarded packet, or it is the final destination for the refund.
		return nil
	}

	bz := store.Get(key)

	// done with packet key now, delete.
	store.Delete(key)

	var inFlightPacket types.InFlightPacket
	k.cdc.MustUnmarshal(bz, &inFlightPacket)
	return &inFlightPacket
}

func (k Keeper) RefundForwardedPacket(
	ctx sdk.Context,
	channel string,
	port string,
	refundSequence uint64,
	sender string,
	receiver string,
	token sdk.Coin,
	timeout time.Duration,
) {
	feeAmount := sdk.NewDecFromInt(token.Amount).Mul(k.GetFeePercentage(ctx)).RoundInt()
	packetAmount := token.Amount.Sub(feeAmount)
	feeCoins := sdk.Coins{sdk.NewCoin(token.Denom, feeAmount)}
	packetCoin := sdk.NewCoin(token.Denom, packetAmount)

	// pay fees
	if feeAmount.IsPositive() {
		hostAccAddr, err := sdk.AccAddressFromBech32(receiver)
		if err != nil {
			return
		}
		err = k.distrKeeper.FundCommunityPool(ctx, feeCoins, hostAccAddr)
		if err != nil {
			k.Logger(ctx).Error("packetForwardMiddleware error funding community pool",
				"error", err,
			)
			return
		}
	}

	msgTransfer := transfertypes.NewMsgTransfer(
		port,
		channel,
		packetCoin,
		sender,
		receiver,
		DefaultTransferPacketTimeoutHeight,
		uint64(ctx.BlockTime().UnixNano())+uint64(timeout.Nanoseconds()),
	)

	packetMemo := &PacketMetadata{
		Forward: &ForwardMetadata{
			RefundSequence: &refundSequence,
		},
	}

	memo, err := json.Marshal(packetMemo)
	if err != nil {
		k.Logger(ctx).Error("packetForwardMiddleware error marshaling json for multi-hop refund sequence",
			"refund-sequence", refundSequence,
			"channel-id", channel, "port-id", port,
			"sender", sender, "receiver", receiver,
			"amount", packetCoin.Amount, "denom", packetCoin.Denom,
			"error", err,
		)
		return
	}

	msgTransfer.Memo = string(memo)

	k.Logger(ctx).Debug("packetForwardMiddleware RefundForwardedPacket",
		"refund-sequence", refundSequence,
		"channel-id", channel, "port-id", port,
		"sender", sender, "receiver", receiver,
		"amount", packetCoin.Amount, "denom", packetCoin.Denom,
	)

	if _, err := k.transferKeeper.Transfer(
		sdk.WrapSDKContext(ctx),
		msgTransfer,
	); err != nil {
		k.Logger(ctx).Error("packetForwardMiddleware error sending packet transfer for multi-hop refund",
			"channel-id", channel,
			"port-id", port,
			"sender", sender,
			"receiver", receiver,
			"refund-sequence", refundSequence,
			"amount", token.Amount,
			"denom", token.Denom,
			"error", err,
		)
	}
}

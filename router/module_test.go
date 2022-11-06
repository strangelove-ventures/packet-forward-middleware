package router_test

import (
	"encoding/json"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	apptypes "github.com/cosmos/ibc-go/v3/modules/apps/transfer/types"
	transfertypes "github.com/cosmos/ibc-go/v3/modules/apps/transfer/types"
	channeltypes "github.com/cosmos/ibc-go/v3/modules/core/04-channel/types"
	"github.com/golang/mock/gomock"
	"github.com/strangelove-ventures/packet-forward-middleware/v3/router/keeper"
	"github.com/strangelove-ventures/packet-forward-middleware/v3/router/types"
	"github.com/strangelove-ventures/packet-forward-middleware/v3/test"
	"github.com/stretchr/testify/require"
)

var (
	testDenom  = "uatom"
	testAmount = "100"

	testSourcePort         = "transfer"
	testSourceChannel      = "channel-10"
	testDestinationPort    = "transfer"
	testDestinationChannel = "channel-11"
)

func makeIBCDenom(port, channel, denom string) string {
	prefixedDenom := transfertypes.GetDenomPrefix(port, channel) + denom
	return transfertypes.ParseDenomTrace(prefixedDenom).IBCDenom()
}

func emptyPacket() channeltypes.Packet {
	return channeltypes.Packet{}
}

func transferPacket(t *testing.T, receiver string, metadata *keeper.PacketMetadata) channeltypes.Packet {
	transferPacket := transfertypes.FungibleTokenPacketData{
		Denom:    testDenom,
		Amount:   testAmount,
		Receiver: receiver,
	}

	if metadata != nil {
		memo, err := json.Marshal(metadata)
		require.NoError(t, err)
		transferPacket.Memo = string(memo)
	}

	transferData, err := transfertypes.ModuleCdc.MarshalJSON(&transferPacket)
	require.NoError(t, err)

	return channeltypes.Packet{
		SourcePort:         testSourcePort,
		SourceChannel:      testSourceChannel,
		DestinationPort:    testDestinationPort,
		DestinationChannel: testDestinationChannel,
		Data:               transferData,
	}
}

func TestOnRecvPacket_EmptyPacket(t *testing.T) {
	ctl := gomock.NewController(t)
	defer ctl.Finish()
	setup := test.NewTestSetup(t, ctl)
	ctx := setup.Initializer.Ctx
	cdc := setup.Initializer.Marshaler
	routerModule := setup.RouterModule

	// Test data
	senderAccAddr := test.AccAddress()
	packet := emptyPacket()

	ack := routerModule.OnRecvPacket(ctx, packet, senderAccAddr)
	require.False(t, ack.Success())

	expectedAck := &channeltypes.Acknowledgement{}
	err := cdc.UnmarshalJSON(ack.Acknowledgement(), expectedAck)
	require.NoError(t, err)
	require.Equal(t, "EOF", expectedAck.GetError())
}

func TestOnRecvPacket_InvalidReceiver(t *testing.T) {
	ctl := gomock.NewController(t)
	defer ctl.Finish()
	setup := test.NewTestSetup(t, ctl)
	ctx := setup.Initializer.Ctx
	cdc := setup.Initializer.Marshaler
	routerModule := setup.RouterModule

	// Test data
	senderAccAddr := test.AccAddress()
	packet := transferPacket(t, "", nil)

	// Expected mocks
	gomock.InOrder(
		setup.Mocks.IBCModuleMock.EXPECT().OnRecvPacket(ctx, packet, senderAccAddr).
			Return(channeltypes.NewResultAcknowledgement([]byte("test"))),
	)

	ack := routerModule.OnRecvPacket(ctx, packet, senderAccAddr)
	require.True(t, ack.Success())

	expectedAck := &channeltypes.Acknowledgement{}
	err := cdc.UnmarshalJSON(ack.Acknowledgement(), expectedAck)
	require.NoError(t, err)
}

func TestOnRecvPacket_NoForward(t *testing.T) {
	ctl := gomock.NewController(t)
	defer ctl.Finish()
	setup := test.NewTestSetup(t, ctl)
	ctx := setup.Initializer.Ctx
	cdc := setup.Initializer.Marshaler
	routerModule := setup.RouterModule

	// Test data
	senderAccAddr := test.AccAddress()
	packet := transferPacket(t, "cosmos16plylpsgxechajltx9yeseqexzdzut9g8vla4k", nil)

	// Expected mocks
	gomock.InOrder(
		setup.Mocks.IBCModuleMock.EXPECT().OnRecvPacket(ctx, packet, senderAccAddr).
			Return(channeltypes.NewResultAcknowledgement([]byte("test"))),
	)

	ack := routerModule.OnRecvPacket(ctx, packet, senderAccAddr)
	require.True(t, ack.Success())

	expectedAck := &channeltypes.Acknowledgement{}
	err := cdc.UnmarshalJSON(ack.Acknowledgement(), expectedAck)
	require.NoError(t, err)
	require.Equal(t, "test", string(expectedAck.GetResult()))
}

func TestOnRecvPacket_RecvPacketFailed(t *testing.T) {
	ctl := gomock.NewController(t)
	defer ctl.Finish()
	setup := test.NewTestSetup(t, ctl)
	ctx := setup.Initializer.Ctx
	cdc := setup.Initializer.Marshaler
	routerModule := setup.RouterModule

	senderAccAddr := test.AccAddress()
	packet := transferPacket(t, "cosmos16plylpsgxechajltx9yeseqexzdzut9g8vla4k", nil)

	// Expected mocks
	gomock.InOrder(
		// We return a failed OnRecvPacket
		setup.Mocks.IBCModuleMock.EXPECT().OnRecvPacket(ctx, packet, senderAccAddr).
			Return(channeltypes.NewErrorAcknowledgement("test")),
	)

	ack := routerModule.OnRecvPacket(ctx, packet, senderAccAddr)
	require.False(t, ack.Success())

	expectedAck := &channeltypes.Acknowledgement{}
	err := cdc.UnmarshalJSON(ack.Acknowledgement(), expectedAck)
	require.NoError(t, err)
	require.Equal(t, "test", expectedAck.GetError())
}

func TestOnRecvPacket_ForwardNoFee(t *testing.T) {
	var err error
	ctl := gomock.NewController(t)
	defer ctl.Finish()
	setup := test.NewTestSetup(t, ctl)
	ctx := setup.Initializer.Ctx
	cdc := setup.Initializer.Marshaler
	routerModule := setup.RouterModule

	// Test data
	hostAddr := "cosmos1vzxkv3lxccnttr9rs0002s93sgw72h7ghukuhs"
	destAddr := "cosmos16plylpsgxechajltx9yeseqexzdzut9g8vla4k"
	port := "transfer"
	channel := "channel-0"
	denom := makeIBCDenom(testDestinationPort, testDestinationChannel, testDenom)
	senderAccAddr := test.AccAddress()
	testCoin := sdk.NewCoin(denom, sdk.NewInt(100))
	packetOrig := transferPacket(t, hostAddr, &keeper.PacketMetadata{
		Forward: &keeper.ForwardMetadata{
			Receiver: destAddr,
			Port:     port,
			Channel:  channel,
		},
	})
	packetFwd := transferPacket(t, destAddr, nil)

	acknowledgement := channeltypes.NewResultAcknowledgement([]byte("test"))
	successAck := cdc.MustMarshalJSON(&acknowledgement)

	// Expected mocks
	gomock.InOrder(
		setup.Mocks.IBCModuleMock.EXPECT().OnRecvPacket(ctx, packetOrig, senderAccAddr).
			Return(acknowledgement),

		setup.Mocks.TransferKeeperMock.EXPECT().Transfer(
			sdk.WrapSDKContext(ctx),
			transfertypes.NewMsgTransfer(
				port,
				channel,
				testCoin,
				hostAddr,
				destAddr,
				keeper.DefaultTransferPacketTimeoutHeight,
				uint64(ctx.BlockTime().UnixNano())+uint64(keeper.DefaultForwardTransferPacketTimeoutTimestamp.Nanoseconds()),
			),
		).Return(&apptypes.MsgTransferResponse{Sequence: 0}, nil),

		setup.Mocks.IBCModuleMock.EXPECT().OnAcknowledgementPacket(ctx, packetFwd, successAck, senderAccAddr).
			Return(nil),
	)

	// chain B with router module receives packet and forwards. ack should be nil so that it is not written yet.
	ack := routerModule.OnRecvPacket(ctx, packetOrig, senderAccAddr)
	require.Nil(t, ack)

	// ack returned from chain C
	err = routerModule.OnAcknowledgementPacket(ctx, packetFwd, successAck, senderAccAddr)
	require.NoError(t, err)
}

func TestOnRecvPacket_ForwardWithFee(t *testing.T) {
	var err error
	ctl := gomock.NewController(t)
	defer ctl.Finish()
	setup := test.NewTestSetup(t, ctl)
	ctx := setup.Initializer.Ctx
	cdc := setup.Initializer.Marshaler
	routerModule := setup.RouterModule

	// Set fee param to 10%
	setup.Keepers.RouterKeeper.SetParams(ctx, types.NewParams(sdk.NewDecWithPrec(10, 2)))

	// Test data
	hostAddr := "cosmos1vzxkv3lxccnttr9rs0002s93sgw72h7ghukuhs"
	destAddr := "cosmos16plylpsgxechajltx9yeseqexzdzut9g8vla4k"
	port := "transfer"
	channel := "channel-0"
	denom := makeIBCDenom(testDestinationPort, testDestinationChannel, testDenom)
	senderAccAddr := test.AccAddress()
	hostAccAddr := test.AccAddressFromBech32(t, hostAddr)
	testCoin := sdk.NewCoin(denom, sdk.NewInt(90))
	feeCoins := sdk.Coins{sdk.NewCoin(denom, sdk.NewInt(10))}
	packetOrig := transferPacket(t, hostAddr, &keeper.PacketMetadata{
		Forward: &keeper.ForwardMetadata{
			Receiver: destAddr,
			Port:     port,
			Channel:  channel,
		},
	})
	packetFwd := transferPacket(t, destAddr, nil)
	acknowledgement := channeltypes.NewResultAcknowledgement([]byte("test"))
	successAck := cdc.MustMarshalJSON(&acknowledgement)

	// Expected mocks
	gomock.InOrder(
		setup.Mocks.IBCModuleMock.EXPECT().OnRecvPacket(ctx, packetOrig, senderAccAddr).
			Return(acknowledgement),

		setup.Mocks.DistributionKeeperMock.EXPECT().FundCommunityPool(
			ctx,
			feeCoins,
			hostAccAddr,
		).Return(nil),

		setup.Mocks.TransferKeeperMock.EXPECT().Transfer(
			sdk.WrapSDKContext(ctx),
			transfertypes.NewMsgTransfer(
				port,
				channel,
				testCoin,
				hostAddr,
				destAddr,
				keeper.DefaultTransferPacketTimeoutHeight,
				uint64(ctx.BlockTime().UnixNano())+uint64(keeper.DefaultForwardTransferPacketTimeoutTimestamp.Nanoseconds()),
			),
		).Return(&apptypes.MsgTransferResponse{Sequence: 0}, nil),

		setup.Mocks.IBCModuleMock.EXPECT().OnAcknowledgementPacket(ctx, packetFwd, successAck, senderAccAddr).
			Return(nil),
	)

	// chain B with router module receives packet and forwards. ack should be nil so that it is not written yet.
	ack := routerModule.OnRecvPacket(ctx, packetOrig, senderAccAddr)
	require.Nil(t, ack)

	// ack returned from chain C
	err = routerModule.OnAcknowledgementPacket(ctx, packetFwd, successAck, senderAccAddr)
	require.NoError(t, err)
}

func TestOnRecvPacket_ForwardMultihop(t *testing.T) {
	var err error
	ctl := gomock.NewController(t)
	defer ctl.Finish()
	setup := test.NewTestSetup(t, ctl)
	ctx := setup.Initializer.Ctx
	cdc := setup.Initializer.Marshaler
	routerModule := setup.RouterModule

	// Test data
	hostAddr := "cosmos1vzxkv3lxccnttr9rs0002s93sgw72h7ghukuhs"
	hostAddr2 := "cosmos1q4p4gx889lfek5augdurrjclwtqvjhuntm6j4m"
	destAddr := "cosmos16plylpsgxechajltx9yeseqexzdzut9g8vla4k"
	port := "transfer"
	channel := "channel-0"
	channel2 := "channel-1"
	denom := makeIBCDenom(testDestinationPort, testDestinationChannel, testDenom)
	senderAccAddr := test.AccAddress()
	senderAccAddr2 := test.AccAddress()
	testCoin := sdk.NewCoin(denom, sdk.NewInt(100))
	nextMetadata := &keeper.PacketMetadata{
		Forward: &keeper.ForwardMetadata{
			Receiver: destAddr,
			Port:     port,
			Channel:  channel2,
		},
	}
	nextBz, err := json.Marshal(nextMetadata)
	require.NoError(t, err)

	next := string(nextBz)

	packetOrig := transferPacket(t, hostAddr, &keeper.PacketMetadata{
		Forward: &keeper.ForwardMetadata{
			Receiver: hostAddr2,
			Port:     port,
			Channel:  channel,
			Next:     &next,
		},
	})
	packet2 := transferPacket(t, hostAddr2, nextMetadata)
	packetFwd := transferPacket(t, destAddr, nil)

	msgTransfer1 := transfertypes.NewMsgTransfer(
		port,
		channel,
		testCoin,
		hostAddr,
		hostAddr2,
		keeper.DefaultTransferPacketTimeoutHeight,
		uint64(ctx.BlockTime().UnixNano())+uint64(keeper.DefaultForwardTransferPacketTimeoutTimestamp.Nanoseconds()),
	)
	memo1, err := json.Marshal(nextMetadata)
	require.NoError(t, err)
	msgTransfer1.Memo = string(memo1)

	msgTransfer2 := transfertypes.NewMsgTransfer(
		port,
		channel2,
		testCoin,
		hostAddr2,
		destAddr,
		keeper.DefaultTransferPacketTimeoutHeight,
		uint64(ctx.BlockTime().UnixNano())+uint64(keeper.DefaultForwardTransferPacketTimeoutTimestamp.Nanoseconds()),
	)
	// no memo on final forward

	acknowledgement := channeltypes.NewResultAcknowledgement([]byte("test"))
	successAck := cdc.MustMarshalJSON(&acknowledgement)

	// Expected mocks
	gomock.InOrder(
		setup.Mocks.IBCModuleMock.EXPECT().OnRecvPacket(ctx, packetOrig, senderAccAddr).
			Return(acknowledgement),

		setup.Mocks.TransferKeeperMock.EXPECT().Transfer(
			sdk.WrapSDKContext(ctx),
			msgTransfer1,
		).Return(&apptypes.MsgTransferResponse{Sequence: 0}, nil),

		setup.Mocks.IBCModuleMock.EXPECT().OnRecvPacket(ctx, packet2, senderAccAddr2).
			Return(acknowledgement),

		setup.Mocks.TransferKeeperMock.EXPECT().Transfer(
			sdk.WrapSDKContext(ctx),
			msgTransfer2,
		).Return(&apptypes.MsgTransferResponse{Sequence: 0}, nil),

		setup.Mocks.IBCModuleMock.EXPECT().OnAcknowledgementPacket(ctx, packetFwd, successAck, senderAccAddr2).
			Return(nil),

		setup.Mocks.IBCModuleMock.EXPECT().OnAcknowledgementPacket(ctx, packet2, successAck, senderAccAddr).
			Return(nil),
	)

	// chain B with router module receives packet and forwards. ack should be nil so that it is not written yet.
	ack := routerModule.OnRecvPacket(ctx, packetOrig, senderAccAddr)
	require.Nil(t, ack)

	// chain C with router module receives packet and forwards. ack should be nil so that it is not written yet.
	ack = routerModule.OnRecvPacket(ctx, packet2, senderAccAddr2)
	require.Nil(t, ack)

	// ack returned from chain D to chain C
	err = routerModule.OnAcknowledgementPacket(ctx, packetFwd, successAck, senderAccAddr2)
	require.NoError(t, err)

	// ack returned from chain C to chain B
	err = routerModule.OnAcknowledgementPacket(ctx, packet2, successAck, senderAccAddr)
	require.NoError(t, err)
}

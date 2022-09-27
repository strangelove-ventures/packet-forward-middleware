package router_test

import (
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

func transferPacket(t *testing.T, receiver string) channeltypes.Packet {
	transferPacket := transfertypes.FungibleTokenPacketData{
		Denom:    testDenom,
		Amount:   testAmount,
		Receiver: receiver,
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
	packet := transferPacket(t, "")

	ack := routerModule.OnRecvPacket(ctx, packet, senderAccAddr)
	require.False(t, ack.Success())

	expectedAck := &channeltypes.Acknowledgement{}
	err := cdc.UnmarshalJSON(ack.Acknowledgement(), expectedAck)
	require.NoError(t, err)
	require.Equal(t, "unparsable receiver field, need: '{address_on_this_chain}|{portid}/{channelid}(|{forward_timeout}(|{forward_retries})?)?:{final_dest_address}', got: ''", expectedAck.GetError())
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
	packet := transferPacket(t, "cosmos16plylpsgxechajltx9yeseqexzdzut9g8vla4k")

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
	packet := transferPacket(t, "cosmos16plylpsgxechajltx9yeseqexzdzut9g8vla4k")

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
	packetOrig := transferPacket(t, test.MakeForwardReceiver(hostAddr, port, channel, destAddr))
	packetFw := transferPacket(t, hostAddr)

	// Expected mocks
	gomock.InOrder(
		setup.Mocks.IBCModuleMock.EXPECT().OnRecvPacket(ctx, packetFw, senderAccAddr).
			Return(channeltypes.NewResultAcknowledgement([]byte("test"))),

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
		).Return(&apptypes.MsgTransferResponse{}, nil),

		setup.Mocks.ChannelKeeperMock.EXPECT().GetNextSequenceSend(ctx, port, channel).Return(uint64(0), true),
	)

	ack := routerModule.OnRecvPacket(ctx, packetOrig, senderAccAddr)
	require.True(t, ack.Success())

	expectedAck := &channeltypes.Acknowledgement{}
	err = cdc.UnmarshalJSON(ack.Acknowledgement(), expectedAck)
	require.NoError(t, err)
	require.Equal(t, "test", string(expectedAck.GetResult()))
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
	packetOrig := transferPacket(t, test.MakeForwardReceiver(hostAddr, port, channel, destAddr))
	packetFw := transferPacket(t, hostAddr)

	// Expected mocks
	gomock.InOrder(
		setup.Mocks.IBCModuleMock.EXPECT().OnRecvPacket(ctx, packetFw, senderAccAddr).
			Return(channeltypes.NewResultAcknowledgement([]byte("test"))),

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
		).Return(&apptypes.MsgTransferResponse{}, nil),

		setup.Mocks.ChannelKeeperMock.EXPECT().GetNextSequenceSend(ctx, port, channel).Return(uint64(0), true),
	)

	ack := routerModule.OnRecvPacket(ctx, packetOrig, senderAccAddr)
	require.True(t, ack.Success())

	expectedAck := &channeltypes.Acknowledgement{}
	err = cdc.UnmarshalJSON(ack.Acknowledgement(), expectedAck)
	require.NoError(t, err)
	require.Equal(t, "test", string(expectedAck.GetResult()))
}

package router_test

import (
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	transfertypes "github.com/cosmos/ibc-go/v3/modules/apps/transfer/types"
	clienttypes "github.com/cosmos/ibc-go/v3/modules/core/02-client/types"
	channeltypes "github.com/cosmos/ibc-go/v3/modules/core/04-channel/types"
	"github.com/golang/mock/gomock"
	"github.com/strangelove-ventures/packet-forward-middleware/v2/router/keeper"
	"github.com/strangelove-ventures/packet-forward-middleware/v2/test"
	"github.com/stretchr/testify/require"
)

var (
	testDenom  = "uatom"
	testAmount = "1000000"

	testSourcePort         = "transfer"
	testSourceChannel      = "channel-10"
	testDestinationPort    = "transfer"
	testDestinationChannel = "channel-11"
)

func ibcDenom(port, channel, denom string) string {
	prefixedDenom := transfertypes.GetDenomPrefix(port, channel) + denom
	return transfertypes.ParseDenomTrace(prefixedDenom).IBCDenom()
}

func testCoin(t *testing.T, denom string, amount string) sdk.Coin {
	amt, err := sdk.ParseUint(amount)
	require.NoError(t, err)

	return sdk.NewCoin(denom, sdk.NewIntFromUint64(amt.Uint64()))
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
	senderAddr := test.AccAddress()
	packet := emptyPacket()

	ack := routerModule.OnRecvPacket(ctx, packet, senderAddr)
	require.False(t, ack.Success())

	expectedAck := &channeltypes.Acknowledgement{}
	err := cdc.UnmarshalJSON(ack.Acknowledgement(), expectedAck)

	require.NoError(t, err)
	require.Equal(t, "cannot unmarshal ICS-20 transfer packet data", expectedAck.GetError())
}

func TestOnRecvPacket_InvalidReceiver(t *testing.T) {
	ctl := gomock.NewController(t)
	defer ctl.Finish()
	setup := test.NewTestSetup(t, ctl)
	ctx := setup.Initializer.Ctx
	cdc := setup.Initializer.Marshaler
	routerModule := setup.RouterModule
	senderAddr := test.AccAddress()
	packet := transferPacket(t, "")

	ack := routerModule.OnRecvPacket(ctx, packet, senderAddr)
	require.False(t, ack.Success())

	expectedAck := &channeltypes.Acknowledgement{}
	err := cdc.UnmarshalJSON(ack.Acknowledgement(), expectedAck)

	require.NoError(t, err)
	require.Equal(t, "cannot parse packet forwarding information", expectedAck.GetError())
}

func TestOnRecvPacket_NoForward(t *testing.T) {
	ctl := gomock.NewController(t)
	defer ctl.Finish()
	setup := test.NewTestSetup(t, ctl)
	ctx := setup.Initializer.Ctx
	cdc := setup.Initializer.Marshaler
	routerModule := setup.RouterModule
	senderAddr := test.AccAddress()
	packet := transferPacket(t, "cosmos16plylpsgxechajltx9yeseqexzdzut9g8vla4k")

	gomock.InOrder(
		setup.Mocks.IBCModuleMock.EXPECT().OnRecvPacket(ctx, packet, senderAddr).
			Return(channeltypes.NewResultAcknowledgement([]byte("test"))),
	)

	ack := routerModule.OnRecvPacket(ctx, packet, senderAddr)
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
	senderAddr := test.AccAddress()
	packet := transferPacket(t, "cosmos16plylpsgxechajltx9yeseqexzdzut9g8vla4k")

	gomock.InOrder(
		// We return a failed OnRecvPacket
		setup.Mocks.IBCModuleMock.EXPECT().OnRecvPacket(ctx, packet, senderAddr).
			Return(channeltypes.NewErrorAcknowledgement("test")),
	)

	ack := routerModule.OnRecvPacket(ctx, packet, senderAddr)
	require.False(t, ack.Success())

	expectedAck := &channeltypes.Acknowledgement{}
	err := cdc.UnmarshalJSON(ack.Acknowledgement(), expectedAck)
	require.NoError(t, err)
	require.Equal(t, "test", string(expectedAck.GetError()))
}

func TestOnRecvPacket_Forward0Fee(t *testing.T) {
	var err error
	ctl := gomock.NewController(t)
	defer ctl.Finish()
	setup := test.NewTestSetup(t, ctl)
	ctx := setup.Initializer.Ctx
	cdc := setup.Initializer.Marshaler
	routerModule := setup.RouterModule
	senderAddr := test.AccAddress()

	hostAddr := "cosmos1vzxkv3lxccnttr9rs0002s93sgw72h7ghukuhs"
	destAddr := "cosmos16plylpsgxechajltx9yeseqexzdzut9g8vla4k"
	port := "transfer"
	channel := "channel-0"

	testCoin := testCoin(t, ibcDenom(testDestinationPort, testDestinationChannel, testDenom), testAmount)
	packet0 := transferPacket(t, test.MakeForwardReceiver(hostAddr, port, channel, destAddr))
	packet1 := transferPacket(t, hostAddr)

	gomock.InOrder(
		setup.Mocks.IBCModuleMock.EXPECT().OnRecvPacket(ctx, packet1, senderAddr).
			Return(channeltypes.NewResultAcknowledgement([]byte("test"))),

		setup.Mocks.TransferKeeperMock.EXPECT().SendTransfer(
			ctx,
			port,
			channel,
			testCoin,
			test.AccAddressFromBech32(t, hostAddr),
			destAddr,
			clienttypes.Height{RevisionNumber: 0, RevisionHeight: 0},
			keeper.TransferDefaultTimeout(ctx),
		).Return(nil),
	)

	ack := routerModule.OnRecvPacket(ctx, packet0, senderAddr)
	require.True(t, ack.Success())

	expectedAck := &channeltypes.Acknowledgement{}
	err = cdc.UnmarshalJSON(ack.Acknowledgement(), expectedAck)
	require.NoError(t, err)
	require.Equal(t, "test", string(expectedAck.GetResult()))
}

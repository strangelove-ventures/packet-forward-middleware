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
)

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
		SourcePort:         "transfer",
		SourceChannel:      "channel-10",
		DestinationPort:    "transfer",
		DestinationChannel: "channel-11",
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
	testAddr := test.AccAddress()
	packet := emptyPacket()

	ack := routerModule.OnRecvPacket(ctx, packet, testAddr)
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
	testAddr := test.AccAddress()
	packet := transferPacket(t, "")

	ack := routerModule.OnRecvPacket(ctx, packet, testAddr)
	require.False(t, ack.Success())

	expectedAck := &channeltypes.Acknowledgement{}
	err := cdc.UnmarshalJSON(ack.Acknowledgement(), expectedAck)

	require.NoError(t, err)
	require.Equal(t, "cannot parse packet forwarding information", expectedAck.GetError())
}

func TestOnRecvPacket_NoTransfer(t *testing.T) {
	ctl := gomock.NewController(t)
	defer ctl.Finish()
	setup := test.NewTestSetup(t, ctl)
	ctx := setup.Initializer.Ctx
	cdc := setup.Initializer.Marshaler
	routerModule := setup.RouterModule
	testAddr := test.AccAddress()
	packet := transferPacket(t, "cosmos16plylpsgxechajltx9yeseqexzdzut9g8vla4k")

	gomock.InOrder(
		setup.Mocks.IBCModuleMock.EXPECT().OnRecvPacket(ctx, packet, testAddr).
			Return(channeltypes.NewResultAcknowledgement([]byte("test"))),
	)

	ack := routerModule.OnRecvPacket(ctx, packet, testAddr)
	require.True(t, ack.Success())

	expectedAck := &channeltypes.Acknowledgement{}
	err := cdc.UnmarshalJSON(ack.Acknowledgement(), expectedAck)
	require.NoError(t, err)
	require.Equal(t, "test", string(expectedAck.GetResult()))
}

func TestOnRecvPacket_Transfer0Fee(t *testing.T) {
	var err error
	ctl := gomock.NewController(t)
	defer ctl.Finish()
	setup := test.NewTestSetup(t, ctl)
	ctx := setup.Initializer.Ctx
	cdc := setup.Initializer.Marshaler
	routerModule := setup.RouterModule
	testCoin := testCoin(t, "ibc/5FEB332D2B121921C792F1A0DBF7C3163FF205337B4AFE6E14F69E8E49545F49", testAmount)
	testTransferTimeout := keeper.TransferDefaultTimeout(ctx)
	testTransferHeightTimeout := clienttypes.Height{RevisionNumber: 0, RevisionHeight: 0}
	senderAddr := test.AccAddress()
	packet0 := transferPacket(t, "cosmos1vzxkv3lxccnttr9rs0002s93sgw72h7ghukuhs|transfer/channel-0:cosmos16plylpsgxechajltx9yeseqexzdzut9g8vla4k")
	packet1 := transferPacket(t, "cosmos1vzxkv3lxccnttr9rs0002s93sgw72h7ghukuhs")
	receivingAddress := "cosmos16plylpsgxechajltx9yeseqexzdzut9g8vla4k"
	forwardingAddress, err := sdk.AccAddressFromBech32("cosmos1vzxkv3lxccnttr9rs0002s93sgw72h7ghukuhs")
	require.NoError(t, err)

	gomock.InOrder(
		setup.Mocks.IBCModuleMock.EXPECT().OnRecvPacket(ctx, packet1, senderAddr).
			Return(channeltypes.NewResultAcknowledgement([]byte("test"))),

		setup.Mocks.TransferKeeperMock.EXPECT().SendTransfer(
			ctx,
			"transfer",
			"channel-0",
			testCoin,
			forwardingAddress,
			receivingAddress,
			testTransferHeightTimeout,
			testTransferTimeout,
		).Return(nil),
	)

	ack := routerModule.OnRecvPacket(ctx, packet0, senderAddr)
	require.True(t, ack.Success())

	expectedAck := &channeltypes.Acknowledgement{}
	err = cdc.UnmarshalJSON(ack.Acknowledgement(), expectedAck)
	require.NoError(t, err)
	require.Equal(t, "test", string(expectedAck.GetResult()))
}

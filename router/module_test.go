package router_test

import (
	"testing"

	transfertypes "github.com/cosmos/ibc-go/v3/modules/apps/transfer/types"
	channeltypes "github.com/cosmos/ibc-go/v3/modules/core/04-channel/types"
	"github.com/golang/mock/gomock"
	"github.com/strangelove-ventures/packet-forward-middleware/v2/test"
	"github.com/stretchr/testify/require"
)

func TestOnRecvPacket_EmptyPacket(t *testing.T) {
	ctl := gomock.NewController(t)
	defer ctl.Finish()
	setup := test.NewTestSetup(ctl)
	ctx := setup.Initializer.Ctx
	cdc := setup.Initializer.Marshaler
	routerModule := setup.RouterModule

	var empty channeltypes.Packet

	ack := routerModule.OnRecvPacket(ctx, empty, test.AccAddress())
	require.False(t, ack.Success())

	expectedAck := &channeltypes.Acknowledgement{}
	err := cdc.UnmarshalJSON(ack.Acknowledgement(), expectedAck)

	require.NoError(t, err)
	require.Equal(t, "cannot unmarshal ICS-20 transfer packet data", expectedAck.GetError())
}

func TestOnRecvPacket_NoTransfer(t *testing.T) {
	ctl := gomock.NewController(t)
	defer ctl.Finish()
	setup := test.NewTestSetup(ctl)
	ctx := setup.Initializer.Ctx
	cdc := setup.Initializer.Marshaler
	routerModule := setup.RouterModule

	gomock.InOrder(
		setup.Mocks.IBCModuleMock.EXPECT().OnRecvPacket(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(channeltypes.NewResultAcknowledgement([]byte("test"))),
	)

	transferPacket := transfertypes.FungibleTokenPacketData{
		Receiver: "cosmos16plylpsgxechajltx9yeseqexzdzut9g8vla4k",
	}
	transferData, err := transfertypes.ModuleCdc.MarshalJSON(&transferPacket)
	require.NoError(t, err)

	packet := channeltypes.Packet{
		Data: transferData,
	}

	ack := routerModule.OnRecvPacket(ctx, packet, test.AccAddress())
	require.True(t, ack.Success())

	expectedAck := &channeltypes.Acknowledgement{}
	err = cdc.UnmarshalJSON(ack.Acknowledgement(), expectedAck)

	require.NoError(t, err)
	require.Equal(t, "test", string(expectedAck.GetResult()))
}

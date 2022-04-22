package router_test

import (
	"testing"

	channeltypes "github.com/cosmos/ibc-go/v3/modules/core/04-channel/types"
	"github.com/strangelove-ventures/packet-forward-middleware/v2/test"
	"github.com/stretchr/testify/require"
)

func TestOnRecvPacket_EmptyPacket(t *testing.T) {
	keepers := test.NewTestKeepers()
	routerModule := test.NewTestRouterModule(keepers)
	ctx := keepers.Initializer.Ctx

	var empty channeltypes.Packet

	ack := routerModule.OnRecvPacket(ctx, empty, test.AccAddress())
	require.False(t, ack.Success())

	expectedAck := &channeltypes.Acknowledgement{}
	err := keepers.Initializer.Marshaler.UnmarshalJSON(ack.Acknowledgement(), expectedAck)

	require.NoError(t, err)
	require.Equal(t, "cannot unmarshal ICS-20 transfer packet data", expectedAck.GetError())
}

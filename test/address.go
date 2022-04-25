package test

import (
	"fmt"
	"testing"

	"github.com/cosmos/cosmos-sdk/crypto/keys/ed25519"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
)

// AccAddress returns a random account address
func AccAddress() sdk.AccAddress {
	pk := ed25519.GenPrivKey().PubKey()
	addr := pk.Address()
	return sdk.AccAddress(addr)
}

func MakeForwardReceiver(hostAddr, port, channel, destAddr string) string {
	return fmt.Sprintf("%s|%s/%s:%s", hostAddr, port, channel, destAddr)
}

func AccAddressFromBech32(t *testing.T, addr string) sdk.AccAddress {
	a, err := sdk.AccAddressFromBech32(addr)
	require.NoError(t, err)

	return a
}

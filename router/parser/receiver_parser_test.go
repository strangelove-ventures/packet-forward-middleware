package parser_test

import (
	"testing"
	"time"

	"github.com/strangelove-ventures/packet-forward-middleware/v2/router/parser"
	"github.com/stretchr/testify/require"
)

func TestParseReceiverDataTransfer(t *testing.T) {
	data := "cosmos1vzxkv3lxccnttr9rs0002s93sgw72h7ghukuhs|transfer/channel-0:cosmos16plylpsgxechajltx9yeseqexzdzut9g8vla4k"
	pt, err := parser.ParseReceiverData(data)

	require.NoError(t, err)
	require.True(t, pt.ShouldForward)
	require.Equal(t, pt.HostAccAddr.String(), "cosmos1vzxkv3lxccnttr9rs0002s93sgw72h7ghukuhs")
	require.Equal(t, pt.Destination, "cosmos16plylpsgxechajltx9yeseqexzdzut9g8vla4k")
	require.Equal(t, pt.Port, "transfer")
	require.Equal(t, pt.Channel, "channel-0")
	require.Equal(t, pt.Timeout, 0*time.Nanosecond)
	require.Nil(t, pt.ForwardRetries)
}

func TestParseReceiverWithTimeout(t *testing.T) {
	data := "cosmos1vzxkv3lxccnttr9rs0002s93sgw72h7ghukuhs|transfer/channel-0|4s:cosmos16plylpsgxechajltx9yeseqexzdzut9g8vla4k"
	pt, err := parser.ParseReceiverData(data)

	require.NoError(t, err)
	require.True(t, pt.ShouldForward)
	require.Equal(t, pt.HostAccAddr.String(), "cosmos1vzxkv3lxccnttr9rs0002s93sgw72h7ghukuhs")
	require.Equal(t, pt.Destination, "cosmos16plylpsgxechajltx9yeseqexzdzut9g8vla4k")
	require.Equal(t, pt.Port, "transfer")
	require.Equal(t, pt.Channel, "channel-0")
	require.Equal(t, pt.Timeout, 4*time.Second)
	require.Nil(t, pt.ForwardRetries)

}

func TestParseReceiverWithRetriesAndTimeout(t *testing.T) {
	data := "cosmos1vzxkv3lxccnttr9rs0002s93sgw72h7ghukuhs|transfer/channel-0|10m|5:cosmos16plylpsgxechajltx9yeseqexzdzut9g8vla4k"
	pt, err := parser.ParseReceiverData(data)

	require.NoError(t, err)
	require.True(t, pt.ShouldForward)
	require.Equal(t, pt.HostAccAddr.String(), "cosmos1vzxkv3lxccnttr9rs0002s93sgw72h7ghukuhs")
	require.Equal(t, pt.Destination, "cosmos16plylpsgxechajltx9yeseqexzdzut9g8vla4k")
	require.Equal(t, pt.Port, "transfer")
	require.Equal(t, pt.Channel, "channel-0")
	require.Equal(t, pt.Timeout, 10*time.Minute)
	require.Equal(t, *pt.ForwardRetries, uint8(5))
}

func TestParseReceiverWithAnotherMultihop(t *testing.T) {
	data := "cosmos1vzxkv3lxccnttr9rs0002s93sgw72h7ghukuhs|transfer/channel-0|10m|5:cosmos1vzxkv3lxccnttr9rs0002s93sgw72h7ghukuhs|transfer/channel-0|10m|5:cosmos16plylpsgxechajltx9yeseqexzdzut9g8vla4k"
	pt, err := parser.ParseReceiverData(data)

	require.NoError(t, err)
	require.True(t, pt.ShouldForward)
	require.Equal(t, pt.HostAccAddr.String(), "cosmos1vzxkv3lxccnttr9rs0002s93sgw72h7ghukuhs")
	require.Equal(t, pt.Destination, "cosmos1vzxkv3lxccnttr9rs0002s93sgw72h7ghukuhs|transfer/channel-0|10m|5:cosmos16plylpsgxechajltx9yeseqexzdzut9g8vla4k")
	require.Equal(t, pt.Port, "transfer")
	require.Equal(t, pt.Channel, "channel-0")
	require.Equal(t, pt.Timeout, 10*time.Minute)
	require.Equal(t, *pt.ForwardRetries, uint8(5))
}

func TestParseReceiverDataNoTransfer(t *testing.T) {
	data := "cosmos16plylpsgxechajltx9yeseqexzdzut9g8vla4k"
	pt, err := parser.ParseReceiverData(data)

	require.NoError(t, err)
	require.False(t, pt.ShouldForward)
}

func TestParseReceiverDataErrors(t *testing.T) {
	testCases := []struct {
		name          string
		data          string
		errStartsWith string
	}{
		{
			"unparsable transfer field",
			"",
			"unparsable receiver",
		},
		{
			"unparsable transfer field",
			"abc:",
			"unparsable receiver",
		},
		{
			"missing pipe",
			"transfer/channel-0:cosmos16plylpsgxechajltx9yeseqexzdzut9g8vla4k",
			"formatting incorrect",
		},
		{
			"invalid this chain address",
			"somm16plylpsgxechajltx9yeseqexzdzut9g8vla4k|transfer/channel-0:cosmos16plylpsgxechajltx9yeseqexzdzut9g8vla4k",
			"decoding bech32 failed",
		},
		{
			"missing slash",
			"cosmos16plylpsgxechajltx9yeseqexzdzut9g8vla4k|transfer\\channel-0:cosmos16plylpsgxechajltx9yeseqexzdzut9g8vla4k",
			"formatting incorrect",
		},
		{
			"unparsable timeout",
			"cosmos16plylpsgxechajltx9yeseqexzdzut9g8vla4k|transfer\\channel-0|abc:cosmos16plylpsgxechajltx9yeseqexzdzut9g8vla4k",
			"unparsable timeout",
		},
		{
			"unparsable retries",
			"cosmos16plylpsgxechajltx9yeseqexzdzut9g8vla4k|transfer\\channel-0|10s|abc:cosmos16plylpsgxechajltx9yeseqexzdzut9g8vla4k",
			"unparsable retries",
		},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			_, err := parser.ParseReceiverData(tc.data)
			require.Error(t, err)
			require.Equal(t, err.Error()[:len(tc.errStartsWith)], tc.errStartsWith)
		})
	}
}

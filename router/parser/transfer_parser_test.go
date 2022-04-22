package parser_test

import (
	"testing"

	"github.com/strangelove-ventures/packet-forward-middleware/v2/router/parser"
	"github.com/stretchr/testify/require"
)

func TestParseIncomingTransferField(t *testing.T) {
	data := ""
	_, err := parser.ParseReceiverData(data)
	require.Error(t, err)
}

func TestParseIncomingTransferFieldErrors(t *testing.T) {
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
			"abc:def:",
			"unparsable receiver",
		},
		{
			"incorrect format for transfer field",
			"abc:def",
			"formatting incorrect",
		},
		{
			"missing pipe",
			"transfer/channel-0:cosmos16plylpsgxechajltx9yeseqexzdzut9g8vla4k",
			"formatting incorrect",
		},
		{
			"invalid receiver address",
			"somm16plylpsgxechajltx9yeseqexzdzut9g8vla4k",
			"decoding bech32 failed",
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

package types

import fmt "fmt"

const (
	// ModuleName defines the 29-fee name
	ModuleName = "packetforwardmiddleware"

	// StoreKey is the store key string for IBC transfer
	StoreKey = ModuleName

	// RouterKey is the message route for IBC transfer
	RouterKey = ModuleName

	// QuerierRoute is the querier route for IBC transfer
	QuerierRoute = ModuleName
)

type (
	NonrefundableKey           struct{}
	DisableDenomCompositionKey struct{}
	ProcessedKey               struct{}
)

func RefundPacketKey(channelID, portID string, sequence uint64) []byte {
	return []byte(fmt.Sprintf("%s/%s/%d", channelID, portID, sequence))
}

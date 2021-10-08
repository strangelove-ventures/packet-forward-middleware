package test

import (
	"github.com/strangelove-ventures/packet-forward-middleware/router"
)

func Fuzz(data []byte) int {
	if addr, dst, port, channel, err := router.ParseIncomingTransferField(string(data)); err != nil {
		if addr != nil || dst != "" || port != "" || channel != "" {
			panic("structs not nil on error")
		}
		return 0
	}
	return 1
}

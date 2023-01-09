package types

import (
	"fmt"
	"time"

	host "github.com/cosmos/ibc-go/v5/modules/core/24-host"
)

type PacketMetadata struct {
	Forward *ForwardMetadata `json:"forward"`
}

type ForwardMetadata struct {
	Receiver                string        `json:"receiver,omitempty"`
	Port                    string        `json:"port,omitempty"`
	Channel                 string        `json:"channel,omitempty"`
	Timeout                 time.Duration `json:"timeout,omitempty"`
	Retries                 *uint8        `json:"retries,omitempty"`
	Nonrefundable           bool          `json:"nonrefundable,omitempty"`
	Processed               bool          `json:"processed,omitempty"`
	DisableDenomComposition bool          `json:"disable_denom_composition,omitempty"`
	Next                    *string       `json:"next,omitempty"`
}

func (m *ForwardMetadata) Validate() error {
	if m.Receiver == "" {
		return fmt.Errorf("failed to validate forward metadata. receiver cannot be empty")
	}
	if err := host.PortIdentifierValidator(m.Port); err != nil {
		return fmt.Errorf("failed to validate forward metadata: %w", err)
	}
	if err := host.ChannelIdentifierValidator(m.Channel); err != nil {
		return fmt.Errorf("failed to validate forward metadata: %w", err)
	}

	return nil
}

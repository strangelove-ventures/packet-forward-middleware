package parser

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

type ParsedReceiver struct {
	ShouldForward bool

	HostAccAddr    sdk.AccAddress
	Destination    string
	Port           string
	Channel        string
	Timeout        time.Duration
	ForwardRetries *uint8
}

// For now this assumes one hop, should be better parsing
func ParseReceiverData(receiverData string) (*ParsedReceiver, error) {
	sep1 := strings.Split(receiverData, ":")

	receiver := &ParsedReceiver{
		ShouldForward: false,
	}

	// Standard address
	if len(sep1) == 1 && sep1[0] != "" {
		return receiver, nil
	}

	receiver.ShouldForward = true

	if len(sep1) < 2 || sep1[1] == "" {
		return nil, fmt.Errorf("unparsable receiver field, need: '{address_on_this_chain}|{portid}/{channelid}(|{forward_timeout}(|{forward_retries})?)?:{final_dest_address}', got: '%s'", receiverData)
	}

	// Final destination is the rest of the string after the first :, might be more multihops
	receiver.Destination = strings.Join(sep1[1:], ":")

	// Parse transfer fields
	sep2 := strings.Split(sep1[0], "|")
	if len(sep2) < 2 {
		return nil, fmt.Errorf("formatting incorrect, need: '{address_on_this_chain}|{portid}/{channelid}(|{forward_timeout}(|{forward_retries})?)?:{final_dest_address}', got: '%s'", receiverData)
	}

	if len(sep2) > 2 {
		timeout, err := time.ParseDuration(sep2[2])
		if err != nil {
			return nil, fmt.Errorf("unparsable timeout, need: '{address_on_this_chain}|{portid}/{channelid}(|{forward_timeout}(|{forward_retries})?)?:{final_dest_address}', got: '%s'", receiverData)
		}
		receiver.Timeout = timeout

		if len(sep2) > 3 {
			retries, err := strconv.ParseUint(sep2[3], 10, 8)
			if err != nil {
				return nil, fmt.Errorf("unparsable retries, need: '{address_on_this_chain}|{portid}/{channelid}(|{forward_timeout}(|{forward_retries})?)?:{final_dest_address}', got: '%s'", receiverData)
			}
			retriesUint8 := uint8(retries)
			receiver.ForwardRetries = &retriesUint8
		}
	}

	hostAccAddr, err := sdk.AccAddressFromBech32(sep2[0])
	if err != nil {
		return nil, err
	}
	receiver.HostAccAddr = hostAccAddr

	sep3 := strings.Split(sep2[1], "/")
	if len(sep3) != 2 {
		return nil, fmt.Errorf("formatting incorrect, need: '{address_on_this_chain}|{portid}/{channelid}(|{forward_timeout}(|{forward_retries})?)?:{final_dest_address}', got: '%s'", receiverData)
	}
	receiver.Port = sep3[0]
	receiver.Channel = sep3[1]

	return receiver, nil
}

// sending chain receiver field
// cosmos16plylpsgxechajltx9yeseqexzdzut9g8vla4k|transfer/channel-0:cosmos16plylpsgxechajltx9yeseqexzdzut9g8vla4k|transfer/channel-0: cosmos16plylpsgxechajltx9yeseqexzdzut9g8vla4k|transfer/channel-0:cosmos16plylpsgxechajltx9yeseqexzdzut9g8vla4k

// first proxy chain receiver field
// cosmos16plylpsgxechajltx9yeseqexzdzut9g8vla4k|transfer/channel-0:cosmos16plylpsgxechajltx9yeseqexzdzut9g8vla4k|transfer/channel-0:cosmos16plylpsgxechajltx9yeseqexzdzut9g8vla4k

// second proxy chain receiver field
// somm16plylpsgxechajltx9yeseqexzdzut9g8vla4k|transfer/channel-0:cosmos16plylpsgxechajltx9yeseqexzdzut9g8vla4k

// final proxy chain receiver field
// cosmos16plylpsgxechajltx9yeseqexzdzut9g8vla4k

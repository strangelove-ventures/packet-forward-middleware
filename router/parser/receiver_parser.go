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

	HostAccAddr sdk.AccAddress
	Destination string
	Port        string
	Channel     string
	MaxRetries  uint8
	Timeout     time.Duration
}

// For now this assumes one hop, should be better parsing
func ParseReceiverData(receiverData string) (*ParsedReceiver, error) {
	sep1 := strings.Split(receiverData, ":")

	// Standard address
	if len(sep1) == 1 && sep1[0] != "" {
		return &ParsedReceiver{
			ShouldForward: false,
		}, nil
	}

	if len(sep1) < 2 || sep1[1] == "" {
		return nil, fmt.Errorf("unparsable receiver field, need: '{address_on_this_chain}|{portid}/{channelid}:{final_dest_address}(:{max_retries}(:{timeout_duration})?)?', got: '%s'", receiverData)
	}

	// Final destination is the second element
	dest := sep1[1]

	var timeout time.Duration

	retries := uint8(0)
	if len(sep1) > 2 {
		retriesParsed, err := strconv.ParseUint(sep1[2], 10, 8)
		if err != nil {
			return nil, fmt.Errorf("unparsable retries, need: '{address_on_this_chain}|{portid}/{channelid}:{final_dest_address}(:{max_retries}(:{timeout_duration})?)?', got: '%s'", receiverData)
		}
		retries = uint8(retriesParsed)
		if len(sep1) > 3 {
			timeout, err = time.ParseDuration(sep1[3])
			if err != nil {
				return nil, fmt.Errorf("unparsable timeout, need: '{address_on_this_chain}|{portid}/{channelid}:{final_dest_address}(:{max_retries}(:{timeout_duration})?)?', got: '%s'", receiverData)
			}
		}
	}

	// Parse transfer fields
	sep2 := strings.Split(sep1[0], "|")
	if len(sep2) != 2 {
		return nil, fmt.Errorf("formatting incorrect, need: '{address_on_this_chain}|{portid}/{channelid}:{final_dest_address}(:{max_retries}(:{timeout_duration})?)?', got: '%s'", receiverData)
	}
	hostAccAddr, err := sdk.AccAddressFromBech32(sep2[0])
	if err != nil {
		return nil, err
	}

	sep3 := strings.Split(sep2[1], "/")
	if len(sep3) != 2 {
		return nil, fmt.Errorf("formatting incorrect, need: '{address_on_this_chain}|{portid}/{channelid}:{final_dest_address}(:{max_retries}(:{timeout_duration})?)?', got: '%s'", receiverData)

	}
	port := sep3[0]
	channel := sep3[1]

	return &ParsedReceiver{
		ShouldForward: true,

		HostAccAddr: hostAccAddr,
		Destination: dest,
		Port:        port,
		Channel:     channel,
		MaxRetries:  retries,
		Timeout:     timeout,
	}, nil
}

// sending chain receiver field
// cosmos16plylpsgxechajltx9yeseqexzdzut9g8vla4k|transfer/channel-0:cosmos16plylpsgxechajltx9yeseqexzdzut9g8vla4k|transfer/channel-0: cosmos16plylpsgxechajltx9yeseqexzdzut9g8vla4k|transfer/channel-0:cosmos16plylpsgxechajltx9yeseqexzdzut9g8vla4k

// first proxy chain receiver field
// cosmos16plylpsgxechajltx9yeseqexzdzut9g8vla4k|transfer/channel-0:cosmos16plylpsgxechajltx9yeseqexzdzut9g8vla4k|transfer/channel-0:cosmos16plylpsgxechajltx9yeseqexzdzut9g8vla4k

// second proxy chain receiver field
// somm16plylpsgxechajltx9yeseqexzdzut9g8vla4k|transfer/channel-0:cosmos16plylpsgxechajltx9yeseqexzdzut9g8vla4k

// final proxy chain receiver field
// cosmos16plylpsgxechajltx9yeseqexzdzut9g8vla4k

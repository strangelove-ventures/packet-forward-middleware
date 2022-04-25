package parser

import (
	"fmt"
	"strings"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

type ParsedReceiver struct {
	ShouldForward bool

	ReceiverAddress  sdk.AccAddress
	FinalDestination string
	Port             string
	Channel          string
}

// For now this assumes one hop, should be better parsing
func ParseReceiverData(receiverData string) (*ParsedReceiver, error) {
	sep1 := strings.Split(receiverData, ":")

	// Standard address
	if len(sep1) == 1 && sep1[0] != "" {
		thischainaddr, err := sdk.AccAddressFromBech32(receiverData)
		if err != nil {
			return nil, err
		}

		return &ParsedReceiver{
			ShouldForward:   false,
			ReceiverAddress: thischainaddr,
		}, nil
	}

	if len(sep1) < 2 || sep1[len(sep1)-1] == "" {
		return nil, fmt.Errorf("unparsable receiver field, need: '{address_on_this_chain}|{portid}/{channelid}:{final_dest_address}', got: '%s'", receiverData)
	}

	// Final destination is the most right element
	finalDestination := strings.Join(sep1[1:], ":")

	// Parse transfer fields
	sep2 := strings.Split(sep1[0], "|")
	if len(sep2) != 2 {
		return nil, fmt.Errorf("formatting incorrect, need: '{address_on_this_chain}|{portid}/{channelid}:{final_dest_address}', got: '%s'", receiverData)
	}
	thischainaddr, err := sdk.AccAddressFromBech32(sep2[0])
	if err != nil {
		return nil, err
	}

	sep3 := strings.Split(sep2[1], "/")
	if len(sep3) != 2 {
		return nil, fmt.Errorf("formatting incorrect, need: '{address_on_this_chain}|{portid}/{channelid}:{final_dest_address}', got: '%s'", receiverData)

	}
	port := sep3[0]
	channel := sep3[1]

	return &ParsedReceiver{
		ShouldForward: true,

		ReceiverAddress:  thischainaddr,
		FinalDestination: finalDestination,
		Port:             port,
		Channel:          channel,
	}, nil
}

// sending chain reviecer field
// cosmos16plylpsgxechajltx9yeseqexzdzut9g8vla4k|transfer/channel-0:cosmos16plylpsgxechajltx9yeseqexzdzut9g8vla4k|transfer/channel-0: cosmos16plylpsgxechajltx9yeseqexzdzut9g8vla4k|transfer/channel-0:cosmos16plylpsgxechajltx9yeseqexzdzut9g8vla4k

// first proxy chain reviever field
// cosmos16plylpsgxechajltx9yeseqexzdzut9g8vla4k|transfer/channel-0:cosmos16plylpsgxechajltx9yeseqexzdzut9g8vla4k|transfer/channel-0:cosmos16plylpsgxechajltx9yeseqexzdzut9g8vla4k

// second proxy chain reviever field
// somm16plylpsgxechajltx9yeseqexzdzut9g8vla4k|transfer/channel-0:cosmos16plylpsgxechajltx9yeseqexzdzut9g8vla4k

// final proxy chain reciever field
// cosmos16plylpsgxechajltx9yeseqexzdzut9g8vla4k

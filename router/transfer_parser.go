package router

import (
	"fmt"
	"strings"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// For now this assumes one hop, should be better parsing
func ParseIncomingTransferField(receiverData string) (thischainaddr sdk.AccAddress, finaldestination, port, channel string, err error) {
	sep1 := strings.Split(receiverData, ":")
	switch {
	case len(sep1) == 1 && sep1[0] != "":
		thischainaddr, err = sdk.AccAddressFromBech32(receiverData)
		return
	case len(sep1) >= 2 && sep1[len(sep1)-1] != "":
		finaldestination = strings.Join(sep1[1:], ":")
	default:
		return nil, "", "", "", fmt.Errorf("unparsable receiver field, need: '{address_on_this_chain}|{portid}/{channelid}:{final_dest_address}', got: '%s'", receiverData)
	}

	sep2 := strings.Split(sep1[0], "|")
	if len(sep2) != 2 {
		err = fmt.Errorf("formatting incorrect, need: '{address_on_this_chain}|{portid}/{channelid}:{final_dest_address}', got: '%s'", receiverData)
		return
	}
	thischainaddr, err = sdk.AccAddressFromBech32(sep2[0])
	if err != nil {
		return
	}

	sep3 := strings.Split(sep2[1], "/")
	if len(sep3) != 2 {
		err = fmt.Errorf("formatting incorrect, need: '{address_on_this_chain}|{portid}/{channelid}:{final_dest_address}', got: '%s'", receiverData)
		return
	}
	port = sep3[0]
	channel = sep3[1]

	return
}

// sending chain reviecer field
// cosmos16plylpsgxechajltx9yeseqexzdzut9g8vla4k|transfer/channel-0:cosmos16plylpsgxechajltx9yeseqexzdzut9g8vla4k|transfer/channel-0: cosmos16plylpsgxechajltx9yeseqexzdzut9g8vla4k|transfer/channel-0:cosmos16plylpsgxechajltx9yeseqexzdzut9g8vla4k

// first proxy chain reviever field
// cosmos16plylpsgxechajltx9yeseqexzdzut9g8vla4k|transfer/channel-0:cosmos16plylpsgxechajltx9yeseqexzdzut9g8vla4k|transfer/channel-0:cosmos16plylpsgxechajltx9yeseqexzdzut9g8vla4k

// second proxy chain reviever field
// somm16plylpsgxechajltx9yeseqexzdzut9g8vla4k|transfer/channel-0:cosmos16plylpsgxechajltx9yeseqexzdzut9g8vla4k

// final proxy chain reciever field
// cosmos16plylpsgxechajltx9yeseqexzdzut9g8vla4k

# packet-forward-middleware
Middleware for forwarding IBC packets

# About

The packet-forward-middleware is a middelware module built for Cosmos blockchains utilizing the IBC protocol. A chain which incorporates the 
packet-forward-middleware is able to route incoming IBC packets from a source chain to a destination chain. As the Cosmos SDK/IBC become commonplace in the 
blockchain space more and more zones will come online, these new zones joining are noticing a problem: they need to maintain a large amount of infrastructure 
(archive nodes and relayers for each counterparty chain) to connect with all the chains in the ecosystem, a number that is continuing to increase quickly. Luckly 
this problem has been anticipated and IBC has been architected to accomodate multi-hop transactions. However, a packet forwarding/routing feature was not in the 
initial IBC release. 

# Example 

By appending an intermediate address, and the port/channel identifiers for the final destination, clients will be able to outline more than one transfer at a time. 
The following example shows routing from Terra to Osmosis through the Hub:

```
// Packet sent from Terra to the hub, note the format of the forwaring info
// {intermediate_refund_address}|{foward_port}/{forward_channel}:{final_destination_address}
{
    "denom": "uluna",
    "amount": "100000000",
    "sender": "terra15gwkyepfc6xgca5t5zefzwy42uts8l2m4g40k6",
    "receiver": "cosmos1vzxkv3lxccnttr9rs0002s93sgw72h7ghukuhs|transfer/channel-141:osmo1vzxkv3lxccnttr9rs0002s93sgw72h7gl89vpz",
}

// When OnRecvPacket on the hub is called, this packet will be modified for fowarding to transfer/channel-141.
// Notice that all fields execept amount are modified as follows:
{
    "denom": "ibc/FEE3FB19682DAAAB02A0328A2B84A80E7DDFE5BA48F7D2C8C30AAC649B8DD519",
    "amount": "100000000",
    "sender": "cosmos1vzxkv3lxccnttr9rs0002s93sgw72h7ghukuhs",
    "receiver": "osmo1vzxkv3lxccnttr9rs0002s93sgw72h7gl89vpz",
}
```

# References

- https://www.mintscan.io/cosmos/proposals/56
- https://github.com/cosmos/ibc-go/v3/pull/373
- https://github.com/strangelove-ventures/governance/blob/master/proposals/2021-09-hub-ibc-router/README.md

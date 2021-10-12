go 1.15

module github.com/strangelove-ventures/packet-forward-middleware

replace github.com/gogo/protobuf => github.com/regen-network/protobuf v1.3.3-alpha.regen.1

require (
	github.com/armon/go-metrics v0.3.9
	github.com/cosmos/cosmos-sdk v0.44.0
	github.com/cosmos/ibc-go v1.2.0
	github.com/dvyukov/go-fuzz v0.0.0-20210914135545-4980593459a1 // indirect
	github.com/dvyukov/go-fuzz-corpus v0.0.0-20190920191254-c42c1b2914c7 // indirect
	github.com/elazarl/go-bindata-assetfs v1.0.1 // indirect
	github.com/gogo/protobuf v1.3.3
	github.com/golang/protobuf v1.5.2
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/gorilla/mux v1.8.0
	github.com/grpc-ecosystem/grpc-gateway v1.16.0
	github.com/spf13/cast v1.4.1 // indirect
	github.com/spf13/cobra v1.2.1
	github.com/stephens2424/writerset v1.0.2 // indirect
	github.com/tendermint/tendermint v0.34.13
	golang.org/x/mod v0.5.1 // indirect
	golang.org/x/sys v0.0.0-20211004093028-2c5d950f24ef // indirect
	golang.org/x/tools v0.1.7 // indirect
	google.golang.org/genproto v0.0.0-20210602131652-f16073e35f0c
	google.golang.org/grpc v1.40.0
	google.golang.org/protobuf v1.27.1 // indirect
)

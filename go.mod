go 1.15

module github.com/strangelove-ventures/packet-forward-middleware

replace github.com/gogo/protobuf => github.com/regen-network/protobuf v1.3.3-alpha.regen.1

require (
	github.com/armon/go-metrics v0.3.10
	github.com/cosmos/cosmos-sdk v0.45.0
	github.com/cosmos/ibc-go/v3 v3.0.0-alpha2
	github.com/gogo/protobuf v1.3.3
	github.com/golang/protobuf v1.5.2
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/gorilla/mux v1.8.0
	github.com/grpc-ecosystem/grpc-gateway v1.16.0
	github.com/spf13/cobra v1.3.0
	github.com/tendermint/tendermint v0.34.14
	google.golang.org/genproto v0.0.0-20211208223120-3a66f561d7aa
	google.golang.org/grpc v1.43.0
)

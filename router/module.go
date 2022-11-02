package router

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"time"

	"github.com/armon/go-metrics"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/cosmos/cosmos-sdk/types/module"
	simtypes "github.com/cosmos/cosmos-sdk/types/simulation"
	capabilitytypes "github.com/cosmos/cosmos-sdk/x/capability/types"
	transfertypes "github.com/cosmos/ibc-go/v3/modules/apps/transfer/types"
	channeltypes "github.com/cosmos/ibc-go/v3/modules/core/04-channel/types"
	porttypes "github.com/cosmos/ibc-go/v3/modules/core/05-port/types"
	ibcexported "github.com/cosmos/ibc-go/v3/modules/core/exported"
	"github.com/gorilla/mux"
	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/spf13/cobra"
	"github.com/strangelove-ventures/packet-forward-middleware/v3/router/client/cli"
	"github.com/strangelove-ventures/packet-forward-middleware/v3/router/keeper"
	"github.com/strangelove-ventures/packet-forward-middleware/v3/router/parser"
	"github.com/strangelove-ventures/packet-forward-middleware/v3/router/types"
	abci "github.com/tendermint/tendermint/abci/types"
)

var (
	_ module.AppModule      = AppModule{}
	_ module.AppModuleBasic = AppModuleBasic{}
	_ porttypes.IBCModule   = AppModule{}
)

// AppModuleBasic is the router AppModuleBasic
type AppModuleBasic struct{}

// Name implements AppModuleBasic interface
func (AppModuleBasic) Name() string {
	return types.ModuleName
}

// RegisterLegacyAminoCodec implements AppModuleBasic interface
func (AppModuleBasic) RegisterLegacyAminoCodec(cdc *codec.LegacyAmino) {}

// RegisterInterfaces registers module concrete types into protobuf Any.
func (AppModuleBasic) RegisterInterfaces(registry codectypes.InterfaceRegistry) {}

// DefaultGenesis returns default genesis state as raw bytes for the ibc
// router module.
func (AppModuleBasic) DefaultGenesis(cdc codec.JSONCodec) json.RawMessage {
	return cdc.MustMarshalJSON(types.DefaultGenesisState())
}

// ValidateGenesis performs genesis state validation for the router module.
func (AppModuleBasic) ValidateGenesis(cdc codec.JSONCodec, config client.TxEncodingConfig, bz json.RawMessage) error {
	var gs types.GenesisState
	if err := cdc.UnmarshalJSON(bz, &gs); err != nil {
		return fmt.Errorf("failed to unmarshal %s genesis state: %w", types.ModuleName, err)
	}

	return gs.Validate()
}

// RegisterRESTRoutes implements AppModuleBasic interface
func (AppModuleBasic) RegisterRESTRoutes(clientCtx client.Context, rtr *mux.Router) {}

// RegisterGRPCGatewayRoutes registers the gRPC Gateway routes for the ibc-router module.
func (AppModuleBasic) RegisterGRPCGatewayRoutes(clientCtx client.Context, mux *runtime.ServeMux) {
	types.RegisterQueryHandlerClient(context.Background(), mux, types.NewQueryClient(clientCtx))
}

// GetTxCmd implements AppModuleBasic interface
func (AppModuleBasic) GetTxCmd() *cobra.Command {
	return cli.NewTxCmd()
}

// GetQueryCmd implements AppModuleBasic interface
func (AppModuleBasic) GetQueryCmd() *cobra.Command {
	return cli.GetQueryCmd()
}

// AppModule represents the AppModule for this module
type AppModule struct {
	AppModuleBasic
	keeper keeper.Keeper
	app    porttypes.IBCModule

	retriesOnTimeout uint8
	forwardTimeout   time.Duration
	refundTimeout    time.Duration
}

// NewAppModule creates a new router module
func NewAppModule(
	k keeper.Keeper,
	app porttypes.IBCModule,
	retriesOnTimeout uint8,
	forwardTimeout time.Duration,
	refundTimeout time.Duration,
) AppModule {
	return AppModule{
		keeper:           k,
		app:              app,
		retriesOnTimeout: retriesOnTimeout,
		forwardTimeout:   forwardTimeout,
		refundTimeout:    refundTimeout,
	}
}

// RegisterInvariants implements the AppModule interface
func (AppModule) RegisterInvariants(ir sdk.InvariantRegistry) {}

// Route implements the AppModule interface
func (am AppModule) Route() sdk.Route {
	return sdk.Route{}
}

// QuerierRoute implements the AppModule interface
func (AppModule) QuerierRoute() string {
	return types.QuerierRoute
}

// LegacyQuerierHandler implements the AppModule interface
func (am AppModule) LegacyQuerierHandler(*codec.LegacyAmino) sdk.Querier {
	return nil
}

// RegisterServices registers module services.
func (am AppModule) RegisterServices(cfg module.Configurator) {
	types.RegisterQueryServer(cfg.QueryServer(), am.keeper)
}

// InitGenesis performs genesis initialization for the ibc-router module. It returns
// no validator updates.
func (am AppModule) InitGenesis(ctx sdk.Context, cdc codec.JSONCodec, data json.RawMessage) []abci.ValidatorUpdate {
	var genesisState types.GenesisState
	cdc.MustUnmarshalJSON(data, &genesisState)
	am.keeper.InitGenesis(ctx, genesisState)
	return []abci.ValidatorUpdate{}
}

// ExportGenesis returns the exported genesis state as raw bytes for the ibc-router
// module.
func (am AppModule) ExportGenesis(ctx sdk.Context, cdc codec.JSONCodec) json.RawMessage {
	gs := am.keeper.ExportGenesis(ctx)
	return cdc.MustMarshalJSON(gs)
}

// ConsensusVersion implements AppModule/ConsensusVersion.
func (AppModule) ConsensusVersion() uint64 { return 1 }

// BeginBlock implements the AppModule interface
func (am AppModule) BeginBlock(ctx sdk.Context, req abci.RequestBeginBlock) {}

// EndBlock implements the AppModule interface
func (am AppModule) EndBlock(ctx sdk.Context, req abci.RequestEndBlock) []abci.ValidatorUpdate {
	return []abci.ValidatorUpdate{}
}

// AppModuleSimulation functions

// GenerateGenesisState creates a randomized GenState of the router module.
func (AppModule) GenerateGenesisState(simState *module.SimulationState) {}

// ProposalContents doesn't return any content functions for governance proposals.
func (AppModule) ProposalContents(_ module.SimulationState) []simtypes.WeightedProposalContent {
	return nil
}

// RandomizedParams creates randomized ibc-router param changes for the simulator.
func (AppModule) RandomizedParams(r *rand.Rand) []simtypes.ParamChange {
	return nil
}

// RegisterStoreDecoder registers a decoder for router module's types
func (am AppModule) RegisterStoreDecoder(sdr sdk.StoreDecoderRegistry) {}

// WeightedOperations returns the all the router module operations with their respective weights.
func (am AppModule) WeightedOperations(_ module.SimulationState) []simtypes.WeightedOperation {
	return nil
}

// ICS 30 callbacks

// OnChanOpenInit implements the IBCModule interface
func (am AppModule) OnChanOpenInit(ctx sdk.Context, order channeltypes.Order, connectionHops []string, portID string, channelID string, chanCap *capabilitytypes.Capability, counterparty channeltypes.Counterparty, version string) error {
	// call underlying app's (transfer) callback
	return am.app.OnChanOpenInit(ctx, order, connectionHops, portID, channelID,
		chanCap, counterparty, version)
}

// OnChanOpenTry implements the IBCModule interface
func (am AppModule) OnChanOpenTry(ctx sdk.Context, order channeltypes.Order, connectionHops []string, portID, channelID string, chanCap *capabilitytypes.Capability, counterparty channeltypes.Counterparty, counterpartyVersion string,
) (version string, err error) {
	// call underlying app's (transfer) callback
	return am.app.OnChanOpenTry(ctx, order, connectionHops, portID, channelID,
		chanCap, counterparty, counterpartyVersion)
}

// OnChanOpenAck implements the IBCModule interface
func (am AppModule) OnChanOpenAck(ctx sdk.Context, portID, channelID string, counterpartyChannelID string, counterpartyVersion string) error {
	return am.app.OnChanOpenAck(ctx, portID, channelID, counterpartyChannelID, counterpartyVersion)
}

// OnChanOpenConfirm implements the IBCModule interface
func (am AppModule) OnChanOpenConfirm(ctx sdk.Context, portID, channelID string) error {
	// call underlying app's OnChanOpenConfirm callback.
	return am.app.OnChanOpenConfirm(ctx, portID, channelID)
}

// OnChanCloseInit implements the IBCModule interface
func (am AppModule) OnChanCloseInit(ctx sdk.Context, portID, channelID string) error {
	// TODO: Unescrow all remaining funds for unprocessed packets
	return am.app.OnChanCloseInit(ctx, portID, channelID)
}

// OnChanCloseConfirm implements the IBCModule interface
func (am AppModule) OnChanCloseConfirm(ctx sdk.Context, portID, channelID string) error {
	// TODO: Unescrow all remaining funds for unprocessed packets
	return am.app.OnChanCloseConfirm(ctx, portID, channelID)
}

// OnRecvPacket implements the IBCModule interface.
func (am AppModule) OnRecvPacket(ctx sdk.Context, packet channeltypes.Packet, relayer sdk.AccAddress) ibcexported.Acknowledgement {
	var data transfertypes.FungibleTokenPacketData
	if err := transfertypes.ModuleCdc.UnmarshalJSON(packet.GetData(), &data); err != nil {
		return channeltypes.NewErrorAcknowledgement(err.Error())
	}

	// parse out any forwarding info
	parsedReceiver, err := parser.ParseReceiverData(data.Receiver)
	if err != nil {
		return channeltypes.NewErrorAcknowledgement(err.Error())
	}

	if !parsedReceiver.ShouldForward {
		return am.app.OnRecvPacket(ctx, packet, relayer)
	}

	// Modify packet data to process packet transfer for this chain, omitting forwarding info
	newData := data
	newData.Receiver = parsedReceiver.HostAccAddr
	bz, err := transfertypes.ModuleCdc.MarshalJSON(&newData)
	if err != nil {
		return channeltypes.NewErrorAcknowledgement(err.Error())
	}
	newPacket := packet
	newPacket.Data = bz

	ack := am.app.OnRecvPacket(ctx, newPacket, relayer)
	if ack.Success() {
		// recalculate denom, skip checks that were already done in app.OnRecvPacket
		var err error
		// TODO put denom handling in separate function
		var denom string
		if transfertypes.ReceiverChainIsSource(packet.GetSourcePort(), packet.GetSourceChannel(), newData.Denom) {
			// remove prefix added by sender chain
			voucherPrefix := transfertypes.GetDenomPrefix(packet.GetSourcePort(), packet.GetSourceChannel())
			unprefixedDenom := newData.Denom[len(voucherPrefix):]

			// coin denomination used in sending from the escrow address
			denom = unprefixedDenom

			// The denomination used to send the coins is either the native denom or the hash of the path
			// if the denomination is not native.
			denomTrace := transfertypes.ParseDenomTrace(unprefixedDenom)
			if denomTrace.Path != "" {
				denom = denomTrace.IBCDenom()
			}
		} else {
			prefixedDenom := transfertypes.GetDenomPrefix(packet.GetDestPort(), packet.GetDestChannel()) + newData.Denom
			denom = transfertypes.ParseDenomTrace(prefixedDenom).IBCDenom()
		}
		unit, err := sdk.ParseUint(newData.Amount)
		if err != nil {
			channeltypes.NewErrorAcknowledgement(err.Error())
		}
		var token = sdk.NewCoin(denom, sdk.NewIntFromUint64(unit.Uint64()))

		var timeout time.Duration
		if parsedReceiver.Timeout.Nanoseconds() > 0 {
			timeout = parsedReceiver.Timeout
		} else {
			timeout = am.forwardTimeout
		}

		var retries uint8
		if parsedReceiver.ForwardRetries != nil {
			retries = *parsedReceiver.ForwardRetries
		} else {
			retries = am.retriesOnTimeout
		}

		err = am.keeper.ForwardTransferPacket(ctx, nil, packet, data.Sender, parsedReceiver, token, retries, timeout, []metrics.Label{})
		if err != nil {
			am.keeper.RefundForwardedPacket(ctx, packet, am.refundTimeout)
			ack = channeltypes.NewErrorAcknowledgement(err.Error())
		}
	}
	return ack
}

// OnAcknowledgementPacket implements the IBCModule interface
func (am AppModule) OnAcknowledgementPacket(ctx sdk.Context, packet channeltypes.Packet, acknowledgement []byte, relayer sdk.AccAddress) error {
	if err := am.app.OnAcknowledgementPacket(ctx, packet, acknowledgement, relayer); err != nil {
		return err
	}

	var ack channeltypes.Acknowledgement
	if err := json.Unmarshal(acknowledgement, &ack); err != nil {
		return sdkerrors.Wrapf(sdkerrors.ErrJSONUnmarshal, err.Error())
	}

	if !ack.Success() {
		// If acknowledgement indicates error, no retries should be attempted. Refund will be initiated now.
		am.keeper.RefundForwardedPacket(ctx, packet, am.refundTimeout)
		return nil
	}

	// For successful ack, we no longer need to track this packet.
	am.keeper.RemoveInFlightPacket(ctx, packet)
	return nil
}

// OnTimeoutPacket implements the IBCModule interface
func (am AppModule) OnTimeoutPacket(ctx sdk.Context, packet channeltypes.Packet, relayer sdk.AccAddress) error {
	if err := am.app.OnTimeoutPacket(ctx, packet, relayer); err != nil {
		return err
	}

	if err := am.keeper.HandleTimeout(ctx, packet); err != nil {
		am.keeper.RefundForwardedPacket(ctx, packet, am.refundTimeout)
	}

	return nil
}

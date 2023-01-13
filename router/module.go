package router

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"strings"
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
	transfertypes "github.com/cosmos/ibc-go/v6/modules/apps/transfer/types"
	clienttypes "github.com/cosmos/ibc-go/v6/modules/core/02-client/types"
	channeltypes "github.com/cosmos/ibc-go/v6/modules/core/04-channel/types"
	porttypes "github.com/cosmos/ibc-go/v6/modules/core/05-port/types"
	ibcexported "github.com/cosmos/ibc-go/v6/modules/core/exported"
	"github.com/gorilla/mux"
	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/spf13/cobra"
	"github.com/strangelove-ventures/packet-forward-middleware/v6/router/client/cli"
	"github.com/strangelove-ventures/packet-forward-middleware/v6/router/keeper"
	"github.com/strangelove-ventures/packet-forward-middleware/v6/router/types"
	abci "github.com/tendermint/tendermint/abci/types"
)

var (
	_ module.AppModule      = AppModule{}
	_ module.AppModuleBasic = AppModuleBasic{}
	_ porttypes.Middleware  = AppModule{}
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

// OnChanOpenInit implements the IBC Middleware interface
func (am AppModule) OnChanOpenInit(ctx sdk.Context, order channeltypes.Order, connectionHops []string, portID string, channelID string, chanCap *capabilitytypes.Capability, counterparty channeltypes.Counterparty, version string) (string, error) {
	// call underlying app's (transfer) callback
	return am.app.OnChanOpenInit(ctx, order, connectionHops, portID, channelID,
		chanCap, counterparty, version)
}

// OnChanOpenTry implements the IBC Middleware interface
func (am AppModule) OnChanOpenTry(ctx sdk.Context, order channeltypes.Order, connectionHops []string, portID, channelID string, chanCap *capabilitytypes.Capability, counterparty channeltypes.Counterparty, counterpartyVersion string,
) (version string, err error) {
	// call underlying app's (transfer) callback
	return am.app.OnChanOpenTry(ctx, order, connectionHops, portID, channelID,
		chanCap, counterparty, counterpartyVersion)
}

// OnChanOpenAck implements the IBC Middleware interface
func (am AppModule) OnChanOpenAck(ctx sdk.Context, portID, channelID string, counterpartyChannelID string, counterpartyVersion string) error {
	return am.app.OnChanOpenAck(ctx, portID, channelID, counterpartyChannelID, counterpartyVersion)
}

// OnChanOpenConfirm implements the IBC Middleware interface
func (am AppModule) OnChanOpenConfirm(ctx sdk.Context, portID, channelID string) error {
	// call underlying app's OnChanOpenConfirm callback.
	return am.app.OnChanOpenConfirm(ctx, portID, channelID)
}

// OnChanCloseInit implements the IBC Middleware interface
func (am AppModule) OnChanCloseInit(ctx sdk.Context, portID, channelID string) error {
	// TODO: Unescrow all remaining funds for unprocessed packets
	return am.app.OnChanCloseInit(ctx, portID, channelID)
}

// OnChanCloseConfirm implements the IBC Middleware interface
func (am AppModule) OnChanCloseConfirm(ctx sdk.Context, portID, channelID string) error {
	// TODO: Unescrow all remaining funds for unprocessed packets
	return am.app.OnChanCloseConfirm(ctx, portID, channelID)
}

func GetDenomForThisChain(port, channel, counterpartyPort, counterpartyChannel, denom string) string {
	counterpartyPrefix := transfertypes.GetDenomPrefix(counterpartyPort, counterpartyChannel)
	if strings.HasPrefix(denom, counterpartyPrefix) {
		// unwind denom
		unwoundDenom := denom[len(counterpartyPrefix):]
		denomTrace := transfertypes.ParseDenomTrace(unwoundDenom)
		if denomTrace.Path == "" {
			// denom is now unwound back to native denom
			return unwoundDenom
		}
		// denom is still IBC denom
		return denomTrace.IBCDenom()
	}
	// append port and channel from this chain to denom
	prefixedDenom := transfertypes.GetDenomPrefix(port, channel) + denom
	return transfertypes.ParseDenomTrace(prefixedDenom).IBCDenom()
}

// OnRecvPacket implements the IBC Middleware interface.
func (am AppModule) OnRecvPacket(ctx sdk.Context, packet channeltypes.Packet, relayer sdk.AccAddress) ibcexported.Acknowledgement {
	var data transfertypes.FungibleTokenPacketData
	if err := transfertypes.ModuleCdc.UnmarshalJSON(packet.GetData(), &data); err != nil {
		return channeltypes.NewErrorAcknowledgement(err)
	}

	am.keeper.Logger(ctx).Debug("packetForwardMiddleware OnRecvPacket",
		"sequence", packet.Sequence,
		"src-channel", packet.SourceChannel, "src-port", packet.SourcePort,
		"dst-channel", packet.DestinationChannel, "dst-port", packet.DestinationPort,
		"amount", data.Amount, "denom", data.Denom,
	)

	m := &keeper.PacketMetadata{}
	err := json.Unmarshal([]byte(data.Memo), m)
	if err != nil || m.Forward == nil {
		// not a packet that should be forwarded
		am.keeper.Logger(ctx).Debug("packetForwardMiddleware OnRecvPacket forward metadata does not exist")
		return am.app.OnRecvPacket(ctx, packet, relayer)
	}

	metadata := m.Forward

	if err := metadata.Validate(); err != nil {
		return channeltypes.NewErrorAcknowledgement(err)
	}

	ack := am.app.OnRecvPacket(ctx, packet, relayer)
	if ack == nil || !ack.Success() {
		return ack
	}

	denomOnThisChain := GetDenomForThisChain(
		packet.DestinationPort, packet.DestinationChannel,
		packet.SourcePort, packet.SourceChannel,
		data.Denom,
	)

	amountInt, ok := sdk.NewIntFromString(data.Amount)
	if !ok {
		return channeltypes.NewErrorAcknowledgement(fmt.Errorf("error parsing amount for forward: %s", data.Amount))
	}

	token := sdk.NewCoin(denomOnThisChain, amountInt)

	var timeout time.Duration
	if metadata.Timeout.Nanoseconds() > 0 {
		timeout = metadata.Timeout
	} else {
		timeout = am.forwardTimeout
	}

	var retries uint8
	if metadata.Retries != nil {
		retries = *metadata.Retries
	} else {
		retries = am.retriesOnTimeout
	}

	err = am.keeper.ForwardTransferPacket(ctx, nil, packet, data.Sender, data.Receiver, metadata, token, retries, timeout, []metrics.Label{})
	if err != nil {
		return channeltypes.NewErrorAcknowledgement(err)
	}

	// returning nil ack will prevent WriteAcknowledgement from occurring for forwarded packet.
	// This is intentional so that the acknowledgement will be written later based on the ack/timeout of the forwarded packet.
	return nil
}

// OnAcknowledgementPacket implements the IBC Middleware interface
func (am AppModule) OnAcknowledgementPacket(ctx sdk.Context, packet channeltypes.Packet, acknowledgement []byte, relayer sdk.AccAddress) error {
	var data transfertypes.FungibleTokenPacketData
	if err := transfertypes.ModuleCdc.UnmarshalJSON(packet.GetData(), &data); err != nil {
		am.keeper.Logger(ctx).Error("packetForwardMiddleware error parsing packet data from ack packet",
			"sequence", packet.Sequence,
			"src-channel", packet.SourceChannel, "src-port", packet.SourcePort,
			"dst-channel", packet.DestinationChannel, "dst-port", packet.DestinationPort,
			"error", err,
		)
		return am.app.OnAcknowledgementPacket(ctx, packet, acknowledgement, relayer)
	}

	am.keeper.Logger(ctx).Debug("packetForwardMiddleware OnAcknowledgementPacket",
		"sequence", packet.Sequence,
		"src-channel", packet.SourceChannel, "src-port", packet.SourcePort,
		"dst-channel", packet.DestinationChannel, "dst-port", packet.DestinationPort,
		"amount", data.Amount, "denom", data.Denom,
	)

	var ack channeltypes.Acknowledgement
	if err := channeltypes.SubModuleCdc.UnmarshalJSON(acknowledgement, &ack); err != nil {
		return sdkerrors.Wrapf(sdkerrors.ErrUnknownRequest, "cannot unmarshal ICS-20 transfer packet acknowledgement: %v", err)
	}

	inFlightPacket := am.keeper.GetAndClearInFlightPacket(ctx, packet.SourceChannel, packet.SourcePort, packet.Sequence)
	if inFlightPacket != nil {
		// this is a forwarded packet, so override handling to avoid refund from being processed.
		return am.keeper.WriteAcknowledgementForForwardedPacket(ctx, packet, data, inFlightPacket, ack)
	}

	return am.app.OnAcknowledgementPacket(ctx, packet, acknowledgement, relayer)
}

// OnTimeoutPacket implements the IBC Middleware interface
func (am AppModule) OnTimeoutPacket(ctx sdk.Context, packet channeltypes.Packet, relayer sdk.AccAddress) error {
	var data transfertypes.FungibleTokenPacketData
	if err := transfertypes.ModuleCdc.UnmarshalJSON(packet.GetData(), &data); err != nil {
		am.keeper.Logger(ctx).Error("packetForwardMiddleware error parsing packet data from timeout packet",
			"sequence", packet.Sequence,
			"src-channel", packet.SourceChannel, "src-port", packet.SourcePort,
			"dst-channel", packet.DestinationChannel, "dst-port", packet.DestinationPort,
			"error", err,
		)
		return am.app.OnTimeoutPacket(ctx, packet, relayer)
	}

	am.keeper.Logger(ctx).Debug("packetForwardMiddleware OnAcknowledgementPacket",
		"sequence", packet.Sequence,
		"src-channel", packet.SourceChannel, "src-port", packet.SourcePort,
		"dst-channel", packet.DestinationChannel, "dst-port", packet.DestinationPort,
		"amount", data.Amount, "denom", data.Denom,
	)

	inFlightPacket, err := am.keeper.TimeoutShouldRetry(ctx, packet)
	if inFlightPacket != nil {
		if err != nil {
			am.keeper.RemoveInFlightPacket(ctx, packet)
			// this is a forwarded packet, so override handling to avoid refund from being processed on this chain.
			// WriteAcknowledgement with proxied ack to return success/fail to previous chain.
			return am.keeper.WriteAcknowledgementForForwardedPacket(ctx, packet, data, inFlightPacket, channeltypes.NewErrorAcknowledgement(err))
		}
		// timeout should be retried. In order to do that, we need to handle this timeout to refund on this chain first.
		if err := am.app.OnTimeoutPacket(ctx, packet, relayer); err != nil {
			return err
		}
		return am.keeper.RetryTimeout(ctx, packet.SourceChannel, packet.SourcePort, data, inFlightPacket)
	}

	return am.app.OnTimeoutPacket(ctx, packet, relayer)
}

// SendPacket implements the IBC Middleware interface
func (am AppModule) SendPacket(
	ctx sdk.Context,
	chanCap *capabilitytypes.Capability,
	sourcePort string,
	sourceChannel string,
	timeoutHeight clienttypes.Height,
	timeoutTimestamp uint64,
	data []byte,
) (sequence uint64, err error) {
	return am.keeper.SendPacket(ctx, chanCap, sourcePort, sourceChannel, timeoutHeight, timeoutTimestamp, data)
}

// WriteAcknowledgement implements the IBC Middleware interface
func (am AppModule) WriteAcknowledgement(
	ctx sdk.Context,
	chanCap *capabilitytypes.Capability,
	packet ibcexported.PacketI,
	ack ibcexported.Acknowledgement,
) error {
	return am.keeper.WriteAcknowledgement(ctx, chanCap, packet, ack)
}

// GetAppVersion implements the IBC Middleware interface
func (am AppModule) GetAppVersion(
	ctx sdk.Context,
	portID,
	channelID string,
) (string, bool) {
	return am.keeper.GetAppVersion(ctx, portID, channelID)
}

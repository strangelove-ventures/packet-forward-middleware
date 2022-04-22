package test

import (
	"github.com/cosmos/cosmos-sdk/codec"
	cdctypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/store"
	storetypes "github.com/cosmos/cosmos-sdk/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	capabilitykeeper "github.com/cosmos/cosmos-sdk/x/capability/keeper"
	capabilitytypes "github.com/cosmos/cosmos-sdk/x/capability/types"
	paramskeeper "github.com/cosmos/cosmos-sdk/x/params/keeper"
	paramstypes "github.com/cosmos/cosmos-sdk/x/params/types"
	clienttypes "github.com/cosmos/ibc-go/v3/modules/core/02-client/types"
	"github.com/strangelove-ventures/packet-forward-middleware/router/keeper"
	"github.com/strangelove-ventures/packet-forward-middleware/router/types"
	"github.com/tendermint/tendermint/libs/log"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	tmdb "github.com/tendermint/tm-db"
)

func NewTestKeepers() *TestKeepers {
	initializer := newInitializer()

	paramsKeeper := initializer.paramsKeeper()
	capabilityKeeper := initializer.capabilityKeeper()
	transferKeeper := newTransferKeeperMock()
	distributionKeeper := newDistributionKeeperMock()

	routerKeeper := initializer.routerKeeper(paramsKeeper, transferKeeper, distributionKeeper)

	return &TestKeepers{
		Initializer:      initializer,
		ParamsKeeper:     paramsKeeper,
		CapabilityKeeper: capabilityKeeper,
		RouterKeeper:     routerKeeper,

		TransferKeeperMock:     transferKeeper,
		DistributionKeeperMock: distributionKeeper,
	}
}

type TestKeepers struct {
	Initializer initializer

	ParamsKeeper     paramskeeper.Keeper
	CapabilityKeeper capabilitykeeper.Keeper
	RouterKeeper     keeper.Keeper

	PortKeeperMock         PortKeeperMock
	TransferKeeperMock     TransferKeeperMock
	DistributionKeeperMock DistributionKeeperMock
}

type initializer struct {
	DB         *tmdb.MemDB
	StateStore store.CommitMultiStore
	Ctx        sdk.Context
	Marshaler  codec.Codec
	Amino      *codec.LegacyAmino
}

// Create an initializer with in memory database and default codecs
func newInitializer() initializer {
	logger := log.TestingLogger()
	logger.Debug("initializing test keepers")

	db := tmdb.NewMemDB()
	stateStore := store.NewCommitMultiStore(db)

	ctx := sdk.NewContext(stateStore, tmproto.Header{}, false, logger)
	interfaceRegistry := cdctypes.NewInterfaceRegistry()
	amino := codec.NewLegacyAmino()
	marshaler := codec.NewProtoCodec(interfaceRegistry)

	return initializer{
		DB:         db,
		StateStore: stateStore,
		Ctx:        ctx,
		Marshaler:  marshaler,
		Amino:      amino,
	}
}

func (i initializer) paramsKeeper() paramskeeper.Keeper {
	storeKey := sdk.NewKVStoreKey(paramstypes.StoreKey)
	transientStoreKey := sdk.NewTransientStoreKey(paramstypes.TStoreKey)
	paramsKeeper := paramskeeper.NewKeeper(i.Marshaler, i.Amino, storeKey, transientStoreKey)

	i.StateStore.MountStoreWithDB(storeKey, sdk.StoreTypeIAVL, i.DB)
	i.StateStore.MountStoreWithDB(transientStoreKey, sdk.StoreTypeTransient, i.DB)

	return paramsKeeper
}

func (i initializer) capabilityKeeper() capabilitykeeper.Keeper {
	storeKey := sdk.NewKVStoreKey(capabilitytypes.StoreKey)
	memStoreKey := storetypes.NewMemoryStoreKey(capabilitytypes.MemStoreKey)
	i.StateStore.MountStoreWithDB(storeKey, sdk.StoreTypeIAVL, i.DB)
	i.StateStore.MountStoreWithDB(memStoreKey, sdk.StoreTypeMemory, nil)

	capabilityKeeper := capabilitykeeper.NewKeeper(i.Marshaler, storeKey, memStoreKey)

	return *capabilityKeeper
}

func (i initializer) routerKeeper(paramsKeeper paramskeeper.Keeper, transferKeeper types.TransferKeeper, distributionKeeper types.DistributionKeeper) keeper.Keeper {
	storeKey := sdk.NewKVStoreKey(types.StoreKey)

	subspace := paramsKeeper.Subspace(types.ModuleName)
	routerKeeper := keeper.NewKeeper(i.Marshaler, storeKey, subspace, transferKeeper, distributionKeeper)

	return routerKeeper
}

// TODO replace below mocks with real mocks (to dynamically provide behavior on the fly)

type PortKeeperMock struct {
	Calls map[string]int
}

func newPortKeeperMock() PortKeeperMock {
	return PortKeeperMock{}
}

func (pk PortKeeperMock) BindPort(ctx sdk.Context, portID string) *capabilitytypes.Capability {
	pk.Calls["BindPort"]++
	return capabilitytypes.NewCapability(0)
}

func (pk PortKeeperMock) IsBound(ctx sdk.Context, portID string) bool {
	pk.Calls["IsBound"]++
	return true
}

type TransferKeeperMock struct {
	Calls map[string]int
}

func newTransferKeeperMock() TransferKeeperMock {
	return TransferKeeperMock{}
}

func (tk TransferKeeperMock) SendTransfer(ctx sdk.Context, sourcePort, sourceChannel string, token sdk.Coin, sender sdk.AccAddress, receiver string, timeoutHeight clienttypes.Height, timeoutTimestamp uint64) error {
	tk.Calls["SendTransfer"]++

	return nil
}

type DistributionKeeperMock struct {
	Calls map[string]int
}

func newDistributionKeeperMock() DistributionKeeperMock {
	return DistributionKeeperMock{}
}

func (dk DistributionKeeperMock) FundCommunityPool(ctx sdk.Context, amount sdk.Coins, sender sdk.AccAddress) error {
	dk.Calls["FundCommunityPool"]++

	return nil
}

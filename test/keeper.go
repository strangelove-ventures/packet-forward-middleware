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
	"github.com/golang/mock/gomock"
	"github.com/strangelove-ventures/packet-forward-middleware/v2/router/keeper"
	"github.com/strangelove-ventures/packet-forward-middleware/v2/router/types"
	"github.com/strangelove-ventures/packet-forward-middleware/v2/test/mock"
	"github.com/tendermint/tendermint/libs/log"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	tmdb "github.com/tendermint/tm-db"
)

func NewTestKeepers(ctl *gomock.Controller) *TestKeepers {
	initializer := newInitializer()

	paramsKeeper := initializer.paramsKeeper()
	capabilityKeeper := initializer.capabilityKeeper()

	portKeeper := mock.NewMockPortKeeper(ctl)
	transferKeeper := mock.NewMockTransferKeeper(ctl)
	distributionKeeper := mock.NewMockDistributionKeeper(ctl)

	routerKeeper := initializer.routerKeeper(paramsKeeper, transferKeeper, distributionKeeper)

	return &TestKeepers{
		Initializer:      initializer,
		ParamsKeeper:     paramsKeeper,
		CapabilityKeeper: capabilityKeeper,
		RouterKeeper:     routerKeeper,

		PortKeeperMock:         portKeeper,
		TransferKeeperMock:     transferKeeper,
		DistributionKeeperMock: distributionKeeper,
	}
}

type TestKeepers struct {
	Initializer initializer

	ParamsKeeper     paramskeeper.Keeper
	CapabilityKeeper capabilitykeeper.Keeper
	RouterKeeper     keeper.Keeper

	PortKeeperMock         *mock.MockPortKeeper
	TransferKeeperMock     *mock.MockTransferKeeper
	DistributionKeeperMock *mock.MockDistributionKeeper
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

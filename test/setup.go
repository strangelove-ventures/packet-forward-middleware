package test

import (
	"testing"

	"github.com/cosmos/cosmos-sdk/codec"
	cdctypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/store"
	storetypes "github.com/cosmos/cosmos-sdk/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	paramskeeper "github.com/cosmos/cosmos-sdk/x/params/keeper"
	paramstypes "github.com/cosmos/cosmos-sdk/x/params/types"
	porttypes "github.com/cosmos/ibc-go/v6/modules/core/05-port/types"
	"github.com/golang/mock/gomock"
	"github.com/strangelove-ventures/packet-forward-middleware/v6/router"
	"github.com/strangelove-ventures/packet-forward-middleware/v6/router/keeper"
	"github.com/strangelove-ventures/packet-forward-middleware/v6/router/types"
	"github.com/strangelove-ventures/packet-forward-middleware/v6/test/mock"
	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/libs/log"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	tmdb "github.com/tendermint/tm-db"
)

func NewTestSetup(t *testing.T, ctl *gomock.Controller) *TestSetup {
	initializer := newInitializer()

	transferKeeperMock := mock.NewMockTransferKeeper(ctl)
	distributionKeeperMock := mock.NewMockDistributionKeeper(ctl)
	ibcModuleMock := mock.NewMockIBCModule(ctl)

	paramsKeeper := initializer.paramsKeeper()
	routerKeeper := initializer.routerKeeper(paramsKeeper, transferKeeperMock, distributionKeeperMock)
	routerModule := initializer.routerModule(routerKeeper, ibcModuleMock)

	require.NoError(t, initializer.StateStore.LoadLatestVersion())

	routerKeeper.SetParams(initializer.Ctx, types.DefaultParams())

	return &TestSetup{
		Initializer: initializer,

		Keepers: &testKeepers{
			ParamsKeeper: &paramsKeeper,
			RouterKeeper: &routerKeeper,
		},

		Mocks: &testMocks{
			TransferKeeperMock:     transferKeeperMock,
			DistributionKeeperMock: distributionKeeperMock,
			IBCModuleMock:          ibcModuleMock,
		},

		RouterModule: &routerModule,
	}
}

type TestSetup struct {
	Initializer initializer

	Keepers *testKeepers
	Mocks   *testMocks

	RouterModule *router.AppModule
}

type testKeepers struct {
	ParamsKeeper *paramskeeper.Keeper
	RouterKeeper *keeper.Keeper
}

type testMocks struct {
	TransferKeeperMock     *mock.MockTransferKeeper
	DistributionKeeperMock *mock.MockDistributionKeeper
	IBCModuleMock          *mock.MockIBCModule
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
	logger.Debug("initializing test setup")

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
	i.StateStore.MountStoreWithDB(storeKey, storetypes.StoreTypeIAVL, i.DB)
	i.StateStore.MountStoreWithDB(transientStoreKey, storetypes.StoreTypeTransient, i.DB)

	paramsKeeper := paramskeeper.NewKeeper(i.Marshaler, i.Amino, storeKey, transientStoreKey)

	return paramsKeeper
}

func (i initializer) routerKeeper(paramsKeeper paramskeeper.Keeper, transferKeeper types.TransferKeeper, distributionKeeper types.DistributionKeeper) keeper.Keeper {
	storeKey := sdk.NewKVStoreKey(types.StoreKey)
	i.StateStore.MountStoreWithDB(storeKey, storetypes.StoreTypeIAVL, i.DB)

	subspace := paramsKeeper.Subspace(types.ModuleName)
	routerKeeper := keeper.NewKeeper(i.Marshaler, storeKey, subspace, transferKeeper, distributionKeeper)

	return routerKeeper
}

func (i initializer) routerModule(routerKeeper keeper.Keeper, ibcModule porttypes.IBCModule) router.AppModule {
	routerModule := router.NewAppModule(routerKeeper, ibcModule, 0,
		keeper.DefaultForwardTransferPacketTimeoutTimestamp, keeper.DefaultRefundTransferPacketTimeoutTimestamp)

	return routerModule
}

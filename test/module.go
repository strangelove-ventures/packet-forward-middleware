package test

import (
	ibcmock "github.com/cosmos/ibc-go/v3/testing/mock"
	"github.com/strangelove-ventures/packet-forward-middleware/v2/router"
	"github.com/strangelove-ventures/packet-forward-middleware/v2/router/types"
)

// AccAddress returns a random account address
func NewTestRouterModule(testkeepers *TestKeepers) router.AppModule {
	scopedKeeper := testkeepers.CapabilityKeeper.ScopeToModule(types.ModuleName)
	ibcApp := ibcmock.NewMockIBCApp("transfer", scopedKeeper)
	appModule := ibcmock.NewAppModule(testkeepers.PortKeeperMock)
	routerModule := router.NewAppModule(testkeepers.RouterKeeper, ibcmock.NewIBCModule(&appModule, ibcApp))

	return routerModule
}

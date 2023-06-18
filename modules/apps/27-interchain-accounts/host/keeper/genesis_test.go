package keeper_test

import (
	genesistypes "github.com/cosmos/ibc-go/v7/modules/apps/27-interchain-accounts/genesis/types"
	"github.com/cosmos/ibc-go/v7/modules/apps/27-interchain-accounts/host/keeper"
	"github.com/cosmos/ibc-go/v7/modules/apps/27-interchain-accounts/host/types"
	icatypes "github.com/cosmos/ibc-go/v7/modules/apps/27-interchain-accounts/types"
	ibctesting "github.com/cosmos/ibc-go/v7/testing"
)

func (s *KeeperTestSuite) TestInitGenesis() {
	s.SetupTest()

	interchainAccAddr := icatypes.GenerateAddress(s.chainB.GetContext(), ibctesting.FirstConnectionID, TestPortID)

	genesisState := genesistypes.HostGenesisState{
		ActiveChannels: []genesistypes.ActiveChannel{
			{
				ConnectionId: ibctesting.FirstConnectionID,
				PortId:       TestPortID,
				ChannelId:    ibctesting.FirstChannelID,
			},
		},
		InterchainAccounts: []genesistypes.RegisteredInterchainAccount{
			{
				ConnectionId:   ibctesting.FirstConnectionID,
				PortId:         TestPortID,
				AccountAddress: interchainAccAddr.String(),
			},
		},
		Port: icatypes.HostPortID,
	}

	keeper.InitGenesis(s.chainA.GetContext(), s.chainA.GetSimApp().ICAHostKeeper, genesisState)

	channelID, found := s.chainA.GetSimApp().ICAHostKeeper.GetActiveChannelID(s.chainA.GetContext(), ibctesting.FirstConnectionID, TestPortID)
	s.Require().True(found)
	s.Require().Equal(ibctesting.FirstChannelID, channelID)

	accountAdrr, found := s.chainA.GetSimApp().ICAHostKeeper.GetInterchainAccountAddress(s.chainA.GetContext(), ibctesting.FirstConnectionID, TestPortID)
	s.Require().True(found)
	s.Require().Equal(interchainAccAddr.String(), accountAdrr)

	expParams := genesisState.GetParams()
	params := s.chainA.GetSimApp().ICAHostKeeper.GetParams(s.chainA.GetContext())
	s.Require().Equal(expParams, params)
}

func (s *KeeperTestSuite) TestGenesisParams() {
	testCases := []struct {
		name    string
		input   types.Params
		expPass bool
	}{
		{"success: set default params", types.DefaultParams(), true},
		{"success: non-default params", types.NewParams(!types.DefaultHostEnabled, []string{"/cosmos.staking.v1beta1.MsgDelegate"}), true},
		{"success: set empty byte for allow messages", types.NewParams(true, nil), true},
		{"failure: set empty string for allow messages", types.NewParams(true, []string{""}), false},
		{"failure: set space string for allow messages", types.NewParams(true, []string{" "}), false},
	}

	for _, tc := range testCases {
		tc := tc

		s.Run(tc.name, func() {
			s.SetupTest() // reset
			interchainAccAddr := icatypes.GenerateAddress(s.chainB.GetContext(), ibctesting.FirstConnectionID, TestPortID)
			genesisState := genesistypes.HostGenesisState{
				ActiveChannels: []genesistypes.ActiveChannel{
					{
						ConnectionId: ibctesting.FirstConnectionID,
						PortId:       TestPortID,
						ChannelId:    ibctesting.FirstChannelID,
					},
				},
				InterchainAccounts: []genesistypes.RegisteredInterchainAccount{
					{
						ConnectionId:   ibctesting.FirstConnectionID,
						PortId:         TestPortID,
						AccountAddress: interchainAccAddr.String(),
					},
				},
				Port:   icatypes.HostPortID,
				Params: tc.input,
			}
			if tc.expPass {
				keeper.InitGenesis(s.chainA.GetContext(), s.chainA.GetSimApp().ICAHostKeeper, genesisState)

				channelID, found := s.chainA.GetSimApp().ICAHostKeeper.GetActiveChannelID(s.chainA.GetContext(), ibctesting.FirstConnectionID, TestPortID)
				s.Require().True(found)
				s.Require().Equal(ibctesting.FirstChannelID, channelID)

				accountAdrr, found := s.chainA.GetSimApp().ICAHostKeeper.GetInterchainAccountAddress(s.chainA.GetContext(), ibctesting.FirstConnectionID, TestPortID)
				s.Require().True(found)
				s.Require().Equal(interchainAccAddr.String(), accountAdrr)

				expParams := tc.input
				params := s.chainA.GetSimApp().ICAHostKeeper.GetParams(s.chainA.GetContext())
				s.Require().Equal(expParams, params)
			} else {
				s.Require().Panics(func() {
					keeper.InitGenesis(s.chainA.GetContext(), s.chainA.GetSimApp().ICAHostKeeper, genesisState)
				})
			}
		})
	}
}

func (s *KeeperTestSuite) TestExportGenesis() {
	s.SetupTest()

	path := NewICAPath(s.chainA, s.chainB)
	s.coordinator.SetupConnections(path)

	err := SetupICAPath(path, TestOwnerAddress)
	s.Require().NoError(err)

	interchainAccAddr, exists := s.chainB.GetSimApp().ICAHostKeeper.GetInterchainAccountAddress(s.chainB.GetContext(), path.EndpointB.ConnectionID, path.EndpointA.ChannelConfig.PortID)
	s.Require().True(exists)

	genesisState := keeper.ExportGenesis(s.chainB.GetContext(), s.chainB.GetSimApp().ICAHostKeeper)

	s.Require().Equal(path.EndpointB.ChannelID, genesisState.ActiveChannels[0].ChannelId)
	s.Require().Equal(path.EndpointA.ChannelConfig.PortID, genesisState.ActiveChannels[0].PortId)

	s.Require().Equal(interchainAccAddr, genesisState.InterchainAccounts[0].AccountAddress)
	s.Require().Equal(path.EndpointA.ChannelConfig.PortID, genesisState.InterchainAccounts[0].PortId)

	s.Require().Equal(icatypes.HostPortID, genesisState.GetPort())

	expParams := types.DefaultParams()
	s.Require().Equal(expParams, genesisState.GetParams())
}

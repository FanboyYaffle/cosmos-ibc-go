package keeper_test

import (
	"fmt"

	errorsmod "cosmossdk.io/errors"

	clienttypes "github.com/cosmos/ibc-go/v7/modules/core/02-client/types"
	connectiontypes "github.com/cosmos/ibc-go/v7/modules/core/03-connection/types"
	"github.com/cosmos/ibc-go/v7/modules/core/04-channel/types"
	commitmenttypes "github.com/cosmos/ibc-go/v7/modules/core/23-commitment/types"
	host "github.com/cosmos/ibc-go/v7/modules/core/24-host"
	"github.com/cosmos/ibc-go/v7/modules/core/exported"
	ibctesting "github.com/cosmos/ibc-go/v7/testing"
	"github.com/cosmos/ibc-go/v7/testing/mock"
)

func (suite *KeeperTestSuite) TestChanUpgradeInit() {
	var (
		path        *ibctesting.Path
		expSequence uint64
		upgrade     types.Upgrade
	)

	testCases := []struct {
		name     string
		malleate func()
		expPass  bool
	}{
		{
			"success",
			func() {},
			true,
		},
		{
			"success with later upgrade sequence",
			func() {
				channel := path.EndpointA.GetChannel()
				channel.UpgradeSequence = 4
				path.EndpointA.SetChannel(channel)
				expSequence = 5
			},
			true,
		},
		{
			"identical upgrade channel end",
			func() {
				channel := path.EndpointA.GetChannel()
				upgrade = types.NewUpgrade(
					types.NewUpgradeFields(
						channel.Ordering, channel.ConnectionHops, channel.Version,
					),
					types.NewTimeout(path.EndpointB.Chain.GetTimeoutHeight(), 0),
					0,
				)
			},
			false,
		},
		{
			"channel not found",
			func() {
				path.EndpointA.ChannelID = "invalid-channel"
				path.EndpointA.ChannelConfig.PortID = "invalid-port"
			},
			false,
		},
		{
			"channel state is not in OPEN state",
			func() {
				suite.Require().NoError(path.EndpointA.SetChannelState(types.CLOSED))
			},
			false,
		},
		{
			"proposed channel connection not found",
			func() {
				upgrade.Fields.ConnectionHops = []string{"connection-100"}
			},
			false,
		},
		{
			"invalid proposed channel connection state",
			func() {
				connectionEnd := path.EndpointA.GetConnection()
				connectionEnd.State = connectiontypes.UNINITIALIZED

				suite.chainA.GetSimApp().GetIBCKeeper().ConnectionKeeper.SetConnection(suite.chainA.GetContext(), "connection-100", connectionEnd)
				upgrade.Fields.ConnectionHops = []string{"connection-100"}
			},
			false,
		},
	}

	for _, tc := range testCases {
		tc := tc
		suite.Run(tc.name, func() {
			suite.SetupTest()

			path = ibctesting.NewPath(suite.chainA, suite.chainB)
			suite.coordinator.Setup(path)

			expSequence = 1

			upgrade = types.NewUpgrade(
				types.NewUpgradeFields(
					types.UNORDERED, []string{path.EndpointA.ConnectionID}, fmt.Sprintf("%s-v2", mock.Version),
				),
				types.NewTimeout(path.EndpointB.Chain.GetTimeoutHeight(), 0),
				0,
			)

			tc.malleate()

			proposedUpgrade, err := suite.chainA.GetSimApp().IBCKeeper.ChannelKeeper.ChanUpgradeInit(
				suite.chainA.GetContext(), path.EndpointA.ChannelConfig.PortID, path.EndpointA.ChannelID, upgrade.Fields, upgrade.Timeout,
			)

			if tc.expPass {
				suite.chainA.GetSimApp().IBCKeeper.ChannelKeeper.WriteUpgradeInitChannel(suite.chainA.GetContext(), path.EndpointA.ChannelConfig.PortID, path.EndpointA.ChannelID, proposedUpgrade)
				channel := path.EndpointA.GetChannel()

				suite.Require().NoError(err)
				suite.Require().Equal(expSequence, channel.UpgradeSequence)
				suite.Require().Equal(mock.Version, channel.Version)
				suite.Require().Equal(types.INITUPGRADE, channel.State)
			} else {
				suite.Require().Error(err)
			}
		})
	}
}

func (suite *KeeperTestSuite) TestChanUpgradeTry() {
	var (
		path                *ibctesting.Path
		proposedUpgrade     types.Upgrade
		counterpartyUpgrade types.Upgrade
	)

	testCases := []struct {
		name     string
		malleate func()
		expError error
	}{
		{
			"success",
			func() {},
			nil,
		},
		{
			"success: crossing hellos",
			func() {
				err := path.EndpointB.ChanUpgradeInit()
				suite.Require().NoError(err)
			},
			nil,
		},
		// {
		// 	"success: upgrade sequence is fast forwarded to counterparty upgrade sequence",
		// 	func() {
		// 		channel := path.EndpointA.GetChannel()
		// 		channel.UpgradeSequence = 5
		// 		path.EndpointA.SetChannel(channel)

		// 		expSequence = 5
		// 	},
		// 	true,
		// },
		// {
		{
			"channel not found",
			func() {
				path.EndpointB.ChannelID = ibctesting.InvalidID
			},
			types.ErrChannelNotFound,
		},
		{
			"channel state is not in OPEN or INITUPGRADE state",
			func() {
				suite.Require().NoError(path.EndpointB.SetChannelState(types.CLOSED))
			},
			types.ErrInvalidChannelState,
		},
		{
			"connection not found",
			func() {
				channel := path.EndpointB.GetChannel()
				channel.ConnectionHops = []string{"connection-100"}
				path.EndpointB.SetChannel(channel)
			},
			connectiontypes.ErrConnectionNotFound,
		},
		{
			"invalid connection state",
			func() {
				connectionEnd := path.EndpointB.GetConnection()
				connectionEnd.State = connectiontypes.UNINITIALIZED
				suite.chainB.GetSimApp().GetIBCKeeper().ConnectionKeeper.SetConnection(suite.chainB.GetContext(), path.EndpointB.ConnectionID, connectionEnd)
			},
			connectiontypes.ErrInvalidConnectionState,
		},
		{
			"timeout has passed",
			func() {
				counterpartyUpgrade.Timeout = types.NewTimeout(clienttypes.NewHeight(0, 1), 0)
			},
			types.ErrInvalidUpgrade,
		},
		{
			"initializing handshake fails, proposed connection hops do not exist",
			func() {
				proposedUpgrade.Fields.ConnectionHops = []string{ibctesting.InvalidID}
			},
			connectiontypes.ErrConnectionNotFound,
		},
		{
			"current upgrade not found even though channel is in INITUPGRADE",
			func() {
				// crossing hellos
				err := path.EndpointB.ChanUpgradeInit()
				suite.Require().NoError(err)

				suite.chainB.DeleteKey(host.ChannelUpgradeKey(path.EndpointB.ChannelConfig.PortID, path.EndpointB.ChannelID))
			},
			types.ErrUpgradeNotFound,
		},
		{
			"proposed upgrade fields must be equal to existing upgrade fields in crossing hellos",
			func() {
				// crossing hellos
				err := path.EndpointB.ChanUpgradeInit()
				suite.Require().NoError(err)

				proposedUpgrade.Fields.ConnectionHops = []string{ibctesting.InvalidID}
			},
			types.ErrInvalidUpgrade,
		},
		{
			"startFlushUpgradeHandshake fails due to proof verification failure, counterparty upgrade connection hops are tampered with",
			func() {
				counterpartyUpgrade.Fields.ConnectionHops = []string{ibctesting.InvalidID}
			},
			commitmenttypes.ErrInvalidProof,
		},
		{
			"startFlushUpgradeHandshake fails due to incompatible upgrades, chainB proposes a new connection hop that does not match counterparty",
			func() {
				// reuse existing connection to create a new connection in a non OPEN state
				connection := path.EndpointB.GetConnection()
				// ensure counterparty connectionID does not match connectionID set in counterparty proposed upgrade
				connection.Counterparty.ConnectionId = "connection-50"

				// set proposed connection in state
				proposedConnectionID := "connection-100" //nolint:goconst
				suite.chainB.GetSimApp().GetIBCKeeper().ConnectionKeeper.SetConnection(suite.chainB.GetContext(), proposedConnectionID, connection)
				proposedUpgrade.Fields.ConnectionHops[0] = proposedConnectionID
			},
			types.NewUpgradeError(1, types.ErrIncompatibleCounterpartyUpgrade),
		},
		{
			"startFlushUpgradeHandshake fails due to mismatch in upgrade sequences",
			func() {
				channel := path.EndpointB.GetChannel()
				channel.UpgradeSequence = 5
				path.EndpointB.SetChannel(channel)
			},
			types.NewUpgradeError(6, types.ErrIncompatibleCounterpartyUpgrade), // max sequence + 1 will be returned
		},
	}

	for _, tc := range testCases {
		tc := tc
		suite.Run(tc.name, func() {
			suite.SetupTest()
			expPass := tc.expError == nil

			path = ibctesting.NewPath(suite.chainA, suite.chainB)
			suite.coordinator.Setup(path)

			path.EndpointA.ChannelConfig.ProposedUpgrade.Fields.Version = mock.UpgradeVersion
			err := path.EndpointA.ChanUpgradeInit()
			suite.Require().NoError(err)

			path.EndpointB.ChannelConfig.ProposedUpgrade.Fields.Version = mock.UpgradeVersion
			proposedUpgrade = path.EndpointB.GetProposedUpgrade()

			var found bool
			counterpartyUpgrade, found = path.EndpointA.Chain.GetSimApp().IBCKeeper.ChannelKeeper.GetUpgrade(path.EndpointA.Chain.GetContext(), path.EndpointA.ChannelConfig.PortID, path.EndpointA.ChannelID)
			suite.Require().True(found)

			tc.malleate()

			// ensure clients are up to date to receive valid proofs
			suite.Require().NoError(path.EndpointB.UpdateClient())

			proofCounterpartyChannel, proofCounterpartyUpgrade, proofHeight := path.EndpointB.QueryChannelUpgradeProof()

			upgrade, err := suite.chainB.GetSimApp().IBCKeeper.ChannelKeeper.ChanUpgradeTry(
				suite.chainB.GetContext(),
				path.EndpointB.ChannelConfig.PortID,
				path.EndpointB.ChannelID,
				proposedUpgrade.Fields.ConnectionHops,
				proposedUpgrade.Timeout,
				counterpartyUpgrade,
				path.EndpointA.GetChannel().UpgradeSequence,
				proofCounterpartyChannel,
				proofCounterpartyUpgrade,
				proofHeight,
			)

			if expPass {
				suite.Require().NoError(err)
				suite.Require().NotEmpty(upgrade)
				suite.Require().Equal(proposedUpgrade.Fields, upgrade.Fields)
				suite.Require().Equal(proposedUpgrade.Timeout, upgrade.Timeout)

				latestSequenceSend, found := path.EndpointB.Chain.GetSimApp().IBCKeeper.ChannelKeeper.GetNextSequenceSend(path.EndpointB.Chain.GetContext(), path.EndpointB.ChannelConfig.PortID, path.EndpointB.ChannelID)
				suite.Require().True(found)
				suite.Require().Equal(latestSequenceSend-1, upgrade.LatestSequenceSend)
			} else {
				suite.assertUpgradeError(err, tc.expError)
				suite.Require().Empty(upgrade)
			}
		})
	}
}

// TestStartFlushUpgradeHandshake tests the startFlushUpgradeHandshake.
// UpgradeInit will be run on chainA and startFlushUpgradeHandshake
// will be called on chainB
func (suite *KeeperTestSuite) TestStartFlushUpgradeHandshake() {
	var (
		path                *ibctesting.Path
		upgrade             types.Upgrade
		counterpartyChannel types.Channel
		counterpartyUpgrade types.Upgrade
	)

	testCases := []struct {
		name     string
		malleate func()
		expError error
	}{
		{
			"success",
			func() {},
			nil,
		},
		{
			"channel not found",
			func() {
				path.EndpointB.ChannelID = "invalid-channel"
				path.EndpointB.ChannelConfig.PortID = "invalid-port"
			},
			types.ErrChannelNotFound,
		},
		{
			"connection not found",
			func() {
				channel := path.EndpointB.GetChannel()
				channel.ConnectionHops[0] = ibctesting.InvalidID
				path.EndpointB.SetChannel(channel)
			},
			connectiontypes.ErrConnectionNotFound,
		},
		{
			"connection state is not in OPEN state",
			func() {
				conn := path.EndpointB.GetConnection()
				conn.State = connectiontypes.INIT
				path.EndpointB.SetConnection(conn)
			},
			connectiontypes.ErrInvalidConnectionState,
		},
		{
			"failed verification for counterparty channel state due to incorrectly constructed counterparty channel",
			func() {
				counterpartyChannel.State = types.CLOSED
			},
			commitmenttypes.ErrInvalidProof,
		},
		{
			"failed verification for counterparty upgrade due to incorrectly constructed counterparty upgrade",
			func() {
				counterpartyUpgrade.LatestSequenceSend = 100
			},
			commitmenttypes.ErrInvalidProof,
		},
		{
			"upgrade sequence mismatch, endpointB channel upgrade sequence is ahead",
			func() {
				channel := path.EndpointB.GetChannel()
				channel.UpgradeSequence++
				path.EndpointB.SetChannel(channel)
			},
			types.NewUpgradeError(2, types.ErrIncompatibleCounterpartyUpgrade), // max sequence will be returned
		},
		{
			"upgrade ordering is not the same on both sides",
			func() {
				upgrade.Fields.Ordering = types.ORDERED
			},
			types.NewUpgradeError(1, types.ErrIncompatibleCounterpartyUpgrade),
		},
		{
			"proposed connection is not found",
			func() {
				upgrade.Fields.ConnectionHops[0] = ibctesting.InvalidID
			},
			types.NewUpgradeError(1, connectiontypes.ErrConnectionNotFound),
		},
		{
			"proposed connection is not in OPEN state",
			func() {
				// reuse existing connection to create a new connection in a non OPEN state
				connectionEnd := path.EndpointB.GetConnection()
				connectionEnd.State = connectiontypes.UNINITIALIZED
				connectionEnd.Counterparty.ConnectionId = counterpartyUpgrade.Fields.ConnectionHops[0] // both sides must be each other's counterparty

				// set proposed connection in state
				proposedConnectionID := "connection-100"
				suite.chainB.GetSimApp().GetIBCKeeper().ConnectionKeeper.SetConnection(suite.chainB.GetContext(), proposedConnectionID, connectionEnd)
				upgrade.Fields.ConnectionHops[0] = proposedConnectionID
			},
			types.NewUpgradeError(1, connectiontypes.ErrInvalidConnectionState),
		},
		{
			"proposed connection ends are not each other's counterparty",
			func() {
				// reuse existing connection to create a new connection in a non OPEN state
				connectionEnd := path.EndpointB.GetConnection()
				// ensure counterparty connectionID does not match connectionID set in counterparty proposed upgrade
				connectionEnd.Counterparty.ConnectionId = "connection-50"

				// set proposed connection in state
				proposedConnectionID := "connection-100"
				suite.chainB.GetSimApp().GetIBCKeeper().ConnectionKeeper.SetConnection(suite.chainB.GetContext(), proposedConnectionID, connectionEnd)
				upgrade.Fields.ConnectionHops[0] = proposedConnectionID
			},
			types.NewUpgradeError(1, types.ErrIncompatibleCounterpartyUpgrade),
		},
	}

	for _, tc := range testCases {
		tc := tc
		suite.Run(tc.name, func() {
			suite.SetupTest()

			path = ibctesting.NewPath(suite.chainA, suite.chainB)
			suite.coordinator.Setup(path)

			path.EndpointA.ChannelConfig.ProposedUpgrade.Fields.Version = mock.UpgradeVersion
			err := path.EndpointA.ChanUpgradeInit()
			suite.Require().NoError(err)

			// ensure proof verification succeeds
			err = path.EndpointB.UpdateClient()
			suite.Require().NoError(err)

			proofChannel, proofUpgrade, proofHeight := path.EndpointB.QueryChannelUpgradeProof()
			counterpartyChannel = path.EndpointA.GetChannel()

			var found bool
			counterpartyUpgrade, found = path.EndpointA.Chain.App.GetIBCKeeper().ChannelKeeper.GetUpgrade(path.EndpointA.Chain.GetContext(), path.EndpointA.ChannelConfig.PortID, path.EndpointA.ChannelID)
			suite.Require().True(found)

			// ensure that the channel has a valid upgrade sequence
			channel := path.EndpointB.GetChannel()
			channel.UpgradeSequence = 1
			path.EndpointB.SetChannel(channel)

			path.EndpointB.ChannelConfig.ProposedUpgrade.Fields.Version = mock.UpgradeVersion
			upgrade = path.EndpointB.GetProposedUpgrade()

			tc.malleate()

			err = suite.chainB.GetSimApp().IBCKeeper.ChannelKeeper.StartFlushUpgradeHandshake(
				suite.chainB.GetContext(), path.EndpointB.ChannelConfig.PortID, path.EndpointB.ChannelID, upgrade.Fields,
				counterpartyChannel, counterpartyUpgrade, proofChannel, proofUpgrade, proofHeight,
			)

			if tc.expError != nil {
				suite.assertUpgradeError(err, tc.expError)
			} else {
				suite.Require().NoError(err)
			}
		})
	}
}

func (suite *KeeperTestSuite) TestValidateUpgradeFields() {
	var (
		proposedUpgrade *types.UpgradeFields
		path            *ibctesting.Path
	)
	tests := []struct {
		name     string
		malleate func()
		expPass  bool
	}{
		{
			name: "change channel version",
			malleate: func() {
				proposedUpgrade.Version = mock.UpgradeVersion
			},
			expPass: true,
		},
		{
			name: "change connection hops",
			malleate: func() {
				path := ibctesting.NewPath(suite.chainA, suite.chainB)
				suite.coordinator.Setup(path)
				proposedUpgrade.ConnectionHops = []string{path.EndpointA.ConnectionID}
			},
			expPass: true,
		},
		{
			name:     "fails with unmodified fields",
			malleate: func() {},
			expPass:  false,
		},
		{
			name: "fails when connection is not set",
			malleate: func() {
				storeKey := suite.chainA.GetSimApp().GetKey(exported.StoreKey)
				kvStore := suite.chainA.GetContext().KVStore(storeKey)
				kvStore.Delete(host.ConnectionKey(ibctesting.FirstConnectionID))
			},
			expPass: false,
		},
		{
			name: "fails when connection is not open",
			malleate: func() {
				connection := path.EndpointA.GetConnection()
				connection.State = connectiontypes.UNINITIALIZED
				path.EndpointA.SetConnection(connection)
			},
			expPass: false,
		},
		{
			name: "fails when connection versions do not exist",
			malleate: func() {
				// update channel version first so that existing channel end is not identical to proposed upgrade
				proposedUpgrade.Version = mock.UpgradeVersion

				connection := path.EndpointA.GetConnection()
				connection.Versions = []*connectiontypes.Version{}
				path.EndpointA.SetConnection(connection)
			},
			expPass: false,
		},
		{
			name: "fails when connection version does not support the new ordering",
			malleate: func() {
				// update channel version first so that existing channel end is not identical to proposed upgrade
				proposedUpgrade.Version = mock.UpgradeVersion

				connection := path.EndpointA.GetConnection()
				connection.Versions = []*connectiontypes.Version{
					connectiontypes.NewVersion("1", []string{"ORDER_ORDERED"}),
				}
				path.EndpointA.SetConnection(connection)
			},
			expPass: false,
		},
	}

	for _, tc := range tests {
		tc := tc
		suite.Run(tc.name, func() {
			suite.SetupTest()
			path = ibctesting.NewPath(suite.chainA, suite.chainB)
			suite.coordinator.Setup(path)

			existingChannel := path.EndpointA.GetChannel()
			proposedUpgrade = &types.UpgradeFields{
				Ordering:       existingChannel.Ordering,
				ConnectionHops: existingChannel.ConnectionHops,
				Version:        existingChannel.Version,
			}

			tc.malleate()

			err := suite.chainA.GetSimApp().IBCKeeper.ChannelKeeper.ValidateUpgradeFields(suite.chainA.GetContext(), *proposedUpgrade, existingChannel)
			if tc.expPass {
				suite.Require().NoError(err)
			} else {
				suite.Require().Error(err)
			}
		})
	}
}

func (suite *KeeperTestSuite) assertUpgradeError(actualError, expError error) {
	suite.Require().Error(actualError)

	if expUpgradeError, ok := expError.(*types.UpgradeError); ok {
		upgradeError, ok := actualError.(*types.UpgradeError)
		suite.Require().True(ok)
		suite.Require().Equal(expUpgradeError.GetErrorReceipt(), upgradeError.GetErrorReceipt())
	}

	suite.Require().True(errorsmod.IsOf(actualError, expError), actualError)
}

package types_test

import (
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/cosmos/ibc-go/v3/modules/core/exported"
	"github.com/cosmos/ibc-go/v3/modules/light-clients/06-solomachine/types"
	ibctmtypes "github.com/cosmos/ibc-go/v3/modules/light-clients/07-tendermint/types"
	ibctesting "github.com/cosmos/ibc-go/v3/testing"
)

func (suite *SoloMachineTestSuite) TestCheckHeaderAndUpdateState() {
	var (
		clientState exported.ClientState
		header      exported.ClientMessage
	)

	// test singlesig and multisig public keys
	for _, solomachine := range []*ibctesting.Solomachine{suite.solomachine, suite.solomachineMulti} {

		testCases := []struct {
			name    string
			setup   func()
			expPass bool
		}{
			{
				"successful update",
				func() {
					clientState = solomachine.ClientState()
					header = solomachine.CreateHeader()
				},
				true,
			},
			{
				"wrong client state type",
				func() {
					clientState = &ibctmtypes.ClientState{}
					header = solomachine.CreateHeader()
				},
				false,
			},
			{
				"invalid header type",
				func() {
					clientState = solomachine.ClientState()
					header = &ibctmtypes.Header{}
				},
				false,
			},
			{
				"wrong sequence in header",
				func() {
					clientState = solomachine.ClientState()
					// store in temp before assigning to interface type
					h := solomachine.CreateHeader()
					h.Sequence++
					header = h
				},
				false,
			},
			{
				"invalid header Signature",
				func() {
					clientState = solomachine.ClientState()
					h := solomachine.CreateHeader()
					h.Signature = suite.GetInvalidProof()
					header = h
				}, false,
			},
			{
				"invalid timestamp in header",
				func() {
					clientState = solomachine.ClientState()
					h := solomachine.CreateHeader()
					h.Timestamp--
					header = h
				}, false,
			},
			{
				"signature uses wrong sequence",
				func() {
					clientState = solomachine.ClientState()
					solomachine.Sequence++
					header = solomachine.CreateHeader()
				},
				false,
			},
			{
				"signature uses new pubkey to sign",
				func() {
					// store in temp before assinging to interface type
					cs := solomachine.ClientState()
					h := solomachine.CreateHeader()

					publicKey, err := codectypes.NewAnyWithValue(solomachine.PublicKey)
					suite.NoError(err)

					data := &types.HeaderData{
						NewPubKey:      publicKey,
						NewDiversifier: h.NewDiversifier,
					}

					dataBz, err := suite.chainA.Codec.Marshal(data)
					suite.Require().NoError(err)

					// generate invalid signature
					signBytes := &types.SignBytes{
						Sequence:    cs.Sequence,
						Timestamp:   solomachine.Time,
						Diversifier: solomachine.Diversifier,
						DataType:    types.CLIENT,
						Data:        dataBz,
					}

					signBz, err := suite.chainA.Codec.Marshal(signBytes)
					suite.Require().NoError(err)

					sig := solomachine.GenerateSignature(signBz)
					suite.Require().NoError(err)
					h.Signature = sig

					clientState = cs
					header = h

				},
				false,
			},
			{
				"signature signs over old pubkey",
				func() {
					// store in temp before assinging to interface type
					cs := solomachine.ClientState()
					oldPubKey := solomachine.PublicKey
					h := solomachine.CreateHeader()

					// generate invalid signature
					data := append(sdk.Uint64ToBigEndian(cs.Sequence), oldPubKey.Bytes()...)
					sig := solomachine.GenerateSignature(data)
					h.Signature = sig

					clientState = cs
					header = h
				},
				false,
			},
			{
				"consensus state public key is nil",
				func() {
					cs := solomachine.ClientState()
					cs.ConsensusState.PublicKey = nil
					clientState = cs
					header = solomachine.CreateHeader()
				},
				false,
			},
		}

		for _, tc := range testCases {
			tc := tc

			suite.Run(tc.name, func() {
				// setup test
				tc.setup()

				clientState, consensusState, err := clientState.CheckHeaderAndUpdateState(suite.chainA.GetContext(), suite.chainA.Codec, suite.store, header)

				if tc.expPass {
					suite.Require().NoError(err)
					suite.Require().Equal(header.(*types.Header).NewPublicKey, clientState.(*types.ClientState).ConsensusState.PublicKey)
					suite.Require().Equal(false, clientState.(*types.ClientState).IsFrozen)
					suite.Require().Equal(header.(*types.Header).Sequence+1, clientState.(*types.ClientState).Sequence)
					suite.Require().Equal(consensusState, clientState.(*types.ClientState).ConsensusState)
				} else {
					suite.Require().Error(err)
					suite.Require().Nil(clientState)
					suite.Require().Nil(consensusState)
				}
			})
		}
	}
}

func (suite *SoloMachineTestSuite) TestVerifyClientMessageHeader() {
	var (
		clientMsg   exported.ClientMessage
		clientState *types.ClientState
	)

	// test singlesig and multisig public keys
	for _, solomachine := range []*ibctesting.Solomachine{suite.solomachine, suite.solomachineMulti} {

		testCases := []struct {
			name    string
			setup   func()
			expPass bool
		}{
			{
				"successful header",
				func() {
					clientMsg = solomachine.CreateHeader()
				},
				true,
			},
			{
				"successful misbehaviour",
				func() {
					clientMsg = solomachine.CreateMisbehaviour()
				},
				true,
			},
			{
				"invalid client message type",
				func() {
					clientMsg = &ibctmtypes.Header{}
				},
				false,
			},
			{
				"wrong sequence in header",
				func() {
					// store in temp before assigning to interface type
					h := solomachine.CreateHeader()
					h.Sequence++
					clientMsg = h
				},
				false,
			},
			{
				"invalid header Signature",
				func() {
					h := solomachine.CreateHeader()
					h.Signature = suite.GetInvalidProof()
					clientMsg = h
				}, false,
			},
			{
				"invalid timestamp in header",
				func() {
					h := solomachine.CreateHeader()
					h.Timestamp--
					clientMsg = h
				}, false,
			},
			{
				"signature uses wrong sequence",
				func() {

					solomachine.Sequence++
					clientMsg = solomachine.CreateHeader()
				},
				false,
			},
			{
				"signature uses new pubkey to sign",
				func() {
					// store in temp before assinging to interface type
					cs := solomachine.ClientState()
					h := solomachine.CreateHeader()

					publicKey, err := codectypes.NewAnyWithValue(solomachine.PublicKey)
					suite.NoError(err)

					data := &types.HeaderData{
						NewPubKey:      publicKey,
						NewDiversifier: h.NewDiversifier,
					}

					dataBz, err := suite.chainA.Codec.Marshal(data)
					suite.Require().NoError(err)

					// generate invalid signature
					signBytes := &types.SignBytes{
						Sequence:    cs.Sequence,
						Timestamp:   solomachine.Time,
						Diversifier: solomachine.Diversifier,
						DataType:    types.CLIENT,
						Data:        dataBz,
					}

					signBz, err := suite.chainA.Codec.Marshal(signBytes)
					suite.Require().NoError(err)

					sig := solomachine.GenerateSignature(signBz)
					suite.Require().NoError(err)
					h.Signature = sig

					clientState = cs
					clientMsg = h

				},
				false,
			},
			{
				"signature signs over old pubkey",
				func() {
					// store in temp before assinging to interface type
					cs := solomachine.ClientState()
					oldPubKey := solomachine.PublicKey
					h := solomachine.CreateHeader()

					// generate invalid signature
					data := append(sdk.Uint64ToBigEndian(cs.Sequence), oldPubKey.Bytes()...)
					sig := solomachine.GenerateSignature(data)
					h.Signature = sig

					clientState = cs
					clientMsg = h
				},
				false,
			},
			{
				"consensus state public key is nil - header",
				func() {
					clientState.ConsensusState.PublicKey = nil
					clientMsg = solomachine.CreateHeader()
				},
				false,
			},
		}

		for _, tc := range testCases {
			tc := tc

			suite.Run(tc.name, func() {
				clientState = solomachine.ClientState()

				// setup test
				tc.setup()

				err := clientState.VerifyClientMessage(suite.chainA.GetContext(), suite.chainA.Codec, suite.store, clientMsg)

				if tc.expPass {
					suite.Require().NoError(err)
				} else {
					suite.Require().Error(err)
				}
			})
		}
	}
}

func (suite *SoloMachineTestSuite) TestVerifyClientMessageMisbehaviour() {
	var (
		clientMsg   exported.ClientMessage
		clientState *types.ClientState
	)

	// test singlesig and multisig public keys
	for _, solomachine := range []*ibctesting.Solomachine{suite.solomachine, suite.solomachineMulti} {

		testCases := []struct {
			name    string
			setup   func()
			expPass bool
		}{
			{
				"successful misbehaviour",
				func() {
					clientMsg = solomachine.CreateMisbehaviour()
				},
				true,
			},
			{
				"old misbehaviour is successful (timestamp is less than current consensus state)",
				func() {
					clientState = solomachine.ClientState()
					solomachine.Time = solomachine.Time - 5
					clientMsg = solomachine.CreateMisbehaviour()
				}, true,
			},
			{
				"invalid client message type",
				func() {
					clientMsg = &ibctmtypes.Header{}
				},
				false,
			},
			{
				"consensus state pubkey is nil",
				func() {
					clientState.ConsensusState.PublicKey = nil
					clientMsg = solomachine.CreateMisbehaviour()
				},
				false,
			},
			{
				"invalid SignatureOne SignatureData",
				func() {
					m := solomachine.CreateMisbehaviour()

					m.SignatureOne.Signature = suite.GetInvalidProof()
					clientMsg = m
				}, false,
			},
			{
				"invalid SignatureTwo SignatureData",
				func() {
					m := solomachine.CreateMisbehaviour()

					m.SignatureTwo.Signature = suite.GetInvalidProof()
					clientMsg = m
				}, false,
			},
			{
				"invalid SignatureOne timestamp",
				func() {
					m := solomachine.CreateMisbehaviour()

					m.SignatureOne.Timestamp = 1000000000000
					clientMsg = m
				}, false,
			},
			{
				"invalid SignatureTwo timestamp",
				func() {
					m := solomachine.CreateMisbehaviour()

					m.SignatureTwo.Timestamp = 1000000000000
					clientMsg = m
				}, false,
			},
			{
				"invalid first signature data",
				func() {
					// store in temp before assigning to interface type
					m := solomachine.CreateMisbehaviour()

					msg := []byte("DATA ONE")
					signBytes := &types.SignBytes{
						Sequence:    solomachine.Sequence + 1,
						Timestamp:   solomachine.Time,
						Diversifier: solomachine.Diversifier,
						DataType:    types.CLIENT,
						Data:        msg,
					}

					data, err := suite.chainA.Codec.Marshal(signBytes)
					suite.Require().NoError(err)

					sig := solomachine.GenerateSignature(data)

					m.SignatureOne.Signature = sig
					m.SignatureOne.Data = msg
					clientMsg = m
				},
				false,
			},
			{
				"invalid second signature data",
				func() {
					// store in temp before assigning to interface type
					m := solomachine.CreateMisbehaviour()

					msg := []byte("DATA TWO")
					signBytes := &types.SignBytes{
						Sequence:    solomachine.Sequence + 1,
						Timestamp:   solomachine.Time,
						Diversifier: solomachine.Diversifier,
						DataType:    types.CLIENT,
						Data:        msg,
					}

					data, err := suite.chainA.Codec.Marshal(signBytes)
					suite.Require().NoError(err)

					sig := solomachine.GenerateSignature(data)

					m.SignatureTwo.Signature = sig
					m.SignatureTwo.Data = msg
					clientMsg = m
				},
				false,
			},
			{
				"wrong pubkey generates first signature",
				func() {
					badMisbehaviour := solomachine.CreateMisbehaviour()

					// update public key to a new one
					solomachine.CreateHeader()
					m := solomachine.CreateMisbehaviour()

					// set SignatureOne to use the wrong signature
					m.SignatureOne = badMisbehaviour.SignatureOne
					clientMsg = m
				}, false,
			},
			{
				"wrong pubkey generates second signature",
				func() {
					badMisbehaviour := solomachine.CreateMisbehaviour()

					// update public key to a new one
					solomachine.CreateHeader()
					m := solomachine.CreateMisbehaviour()

					// set SignatureTwo to use the wrong signature
					m.SignatureTwo = badMisbehaviour.SignatureTwo
					clientMsg = m
				}, false,
			},
			{
				"signatures sign over different sequence",
				func() {

					// store in temp before assigning to interface type
					m := solomachine.CreateMisbehaviour()

					// Signature One
					msg := []byte("DATA ONE")
					// sequence used is plus 1
					signBytes := &types.SignBytes{
						Sequence:    solomachine.Sequence + 1,
						Timestamp:   solomachine.Time,
						Diversifier: solomachine.Diversifier,
						DataType:    types.CLIENT,
						Data:        msg,
					}

					data, err := suite.chainA.Codec.Marshal(signBytes)
					suite.Require().NoError(err)

					sig := solomachine.GenerateSignature(data)

					m.SignatureOne.Signature = sig
					m.SignatureOne.Data = msg

					// Signature Two
					msg = []byte("DATA TWO")
					// sequence used is minus 1

					signBytes = &types.SignBytes{
						Sequence:    solomachine.Sequence - 1,
						Timestamp:   solomachine.Time,
						Diversifier: solomachine.Diversifier,
						DataType:    types.CLIENT,
						Data:        msg,
					}
					data, err = suite.chainA.Codec.Marshal(signBytes)
					suite.Require().NoError(err)

					sig = solomachine.GenerateSignature(data)

					m.SignatureTwo.Signature = sig
					m.SignatureTwo.Data = msg

					clientMsg = m
				},
				false,
			},
		}

		for _, tc := range testCases {
			tc := tc

			suite.Run(tc.name, func() {
				clientState = solomachine.ClientState()

				// setup test
				tc.setup()

				err := clientState.VerifyClientMessage(suite.chainA.GetContext(), suite.chainA.Codec, suite.store, clientMsg)

				if tc.expPass {
					suite.Require().NoError(err)
				} else {
					suite.Require().Error(err)
				}
			})
		}
	}
}

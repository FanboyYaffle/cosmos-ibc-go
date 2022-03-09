package types

import (
	"bytes"

	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"

	clienttypes "github.com/cosmos/ibc-go/v3/modules/core/02-client/types"
	host "github.com/cosmos/ibc-go/v3/modules/core/24-host"
	"github.com/cosmos/ibc-go/v3/modules/core/exported"
)

var _ exported.Misbehaviour = &DuplicateSignatureHeader{}

// ClientType is a Solo Machine light client.
func (header DuplicateSignatureHeader) ClientType() string {
	return exported.Solomachine
}

// GetClientID returns the ID of the client that committed a misbehaviour.
func (header DuplicateSignatureHeader) GetClientID() string {
	return header.ClientId
}

// Type implements Evidence interface.
func (header DuplicateSignatureHeader) Type() string {
	return exported.TypeClientMisbehaviour
}

// ValidateBasic implements Evidence interface.
func (header DuplicateSignatureHeader) ValidateBasic() error {
	if err := host.ClientIdentifierValidator(header.ClientId); err != nil {
		return sdkerrors.Wrap(err, "invalid client identifier for solo machine")
	}

	if header.Sequence == 0 {
		return sdkerrors.Wrap(clienttypes.ErrInvalidMisbehaviour, "sequence cannot be 0")
	}

	if err := header.SignatureOne.ValidateBasic(); err != nil {
		return sdkerrors.Wrap(err, "signature one failed basic validation")
	}

	if err := header.SignatureTwo.ValidateBasic(); err != nil {
		return sdkerrors.Wrap(err, "signature two failed basic validation")
	}

	// misbehaviour signatures cannot be identical
	if bytes.Equal(header.SignatureOne.Signature, header.SignatureTwo.Signature) {
		return sdkerrors.Wrap(clienttypes.ErrInvalidMisbehaviour, "misbehaviour signatures cannot be equal")
	}

	// message data signed cannot be identical
	if bytes.Equal(header.SignatureOne.Data, header.SignatureTwo.Data) {
		return sdkerrors.Wrap(clienttypes.ErrInvalidMisbehaviour, "misbehaviour signature data must be signed over different messages")
	}

	return nil
}

// ValidateBasic ensures that the signature and data fields are non-empty.
func (sd SignatureAndData) ValidateBasic() error {
	if len(sd.Signature) == 0 {
		return sdkerrors.Wrap(ErrInvalidSignatureAndData, "signature cannot be empty")
	}
	if len(sd.Data) == 0 {
		return sdkerrors.Wrap(ErrInvalidSignatureAndData, "data for signature cannot be empty")
	}
	if sd.DataType == UNSPECIFIED {
		return sdkerrors.Wrap(ErrInvalidSignatureAndData, "data type cannot be UNSPECIFIED")
	}
	if sd.Timestamp == 0 {
		return sdkerrors.Wrap(ErrInvalidSignatureAndData, "timestamp cannot be 0")
	}

	return nil
}

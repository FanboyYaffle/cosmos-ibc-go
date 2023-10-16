package types

/*
	This file is to allow for unexported functions and fields to be accessible to the testing package.
*/

// instantiateMessage is the message that is sent to the contract's instantiate entry point.
type InstantiateMessage struct {
	instantiateMessage
}

type StatusMessage statusMsg
type ExportMetadata exportMetadataMsg
type TimestampAtHeight timestampAtHeightMsg
type VerifyClientMessage verifyClientMessageMsg
type CheckForMisbehaviour checkForMisbehaviourMsg

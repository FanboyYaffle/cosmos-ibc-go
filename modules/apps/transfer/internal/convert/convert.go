package convert

import (
	"strings"

	"github.com/cosmos/ibc-go/v8/modules/apps/transfer/internal/denom"
	v1types "github.com/cosmos/ibc-go/v8/modules/apps/transfer/types"
	v3types "github.com/cosmos/ibc-go/v8/modules/apps/transfer/types/v3"
)

// ICS20V1ToV2 converts a ICS20-V1 packet data (FungibleTokenPacketDataV1) to a ICS20-V2 (FungibleTokenPacketDataV3) packet data.
func ICS20V1ToV2(packetData v1types.FungibleTokenPacketData) v3types.FungibleTokenPacketData {
	if err := packetData.ValidateBasic(); err != nil {
		panic(err)
	}

	v2Denom, trace := extractDenomAndTraceFromV1Denom(packetData.Denom)
	return v3types.FungibleTokenPacketData{
		Tokens: []*v3types.Token{
			{
				Denom:  v2Denom,
				Amount: packetData.Amount,
				Trace:  trace,
			},
		},
		Sender:   packetData.Sender,
		Receiver: packetData.Receiver,
		Memo:     packetData.Memo,
	}
}

// extractDenomAndTraceFromV1Denom extracts the base denom and remaining trace from a v1 IBC denom.
func extractDenomAndTraceFromV1Denom(v1Denom string) (string, []string) {
	v1DenomTrace := v1types.ParseDenomTrace(v1Denom)

	splitPath := strings.Split(v1Denom, "/")
	pathSlice, _ := denom.ExtractPathAndBaseFromFullDenom(splitPath)

	// if the path slice is empty, then the base denom is the full native denom.
	if len(pathSlice) == 0 {
		return v1DenomTrace.BaseDenom, nil
	}

	// this condition should never be reached.
	if len(pathSlice)%2 != 0 {
		panic("pathSlice length is not even")
	}

	// the path slices consists of entries of ports and channel ids separately,
	// we need to combine them to form the trace.
	var trace []string
	for i := 0; i < len(pathSlice); i += 2 {
		trace = append(trace, strings.Join(pathSlice[i:i+2], "/"))
	}

	return v1DenomTrace.BaseDenom, trace
}

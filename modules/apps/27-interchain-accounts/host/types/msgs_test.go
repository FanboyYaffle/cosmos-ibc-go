package types_test

import (
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"github.com/cosmos/ibc-go/v7/modules/apps/27-interchain-accounts/host/types"
	ibctesting "github.com/cosmos/ibc-go/v7/testing"
)

func TestMsgUpdateParamsValidateBasic(t *testing.T) {
	testCases := []struct {
		name    string
		msg     *types.MsgUpdateParams
		expPass bool
	}{
		{
			"success: valid authority address",
			types.NewMsgUpdateParams(sdk.AccAddress(ibctesting.TestAccAddress).String(), types.DefaultParams()),
			true,
		},
		{
			"failure: invalid authority address",
			types.NewMsgUpdateParams("authority", types.DefaultParams()),
			false,
		},
		{
			"failure: invalid allowed message",
			types.NewMsgUpdateParams("authority", types.Params{
				AllowMessages: []string{""},
			}),
			false,
		},
	}

	for _, tc := range testCases {
		err := tc.msg.ValidateBasic()
		if tc.expPass {
			require.NoError(t, err)
		} else {
			require.Error(t, err)
		}
	}
}

func TestMsgUpdateParamsGetSigners(t *testing.T) {
	testCases := []struct {
		name    string
		address sdk.AccAddress
		expPass bool
	}{
		{"success: valid address", sdk.AccAddress(ibctesting.TestAccAddress), true},
		{"failure: nil address", nil, false},
	}

	for _, tc := range testCases {
		msg := types.NewMsgUpdateParams(tc.address.String(), types.DefaultParams())
		if tc.expPass {
			require.Equal(t, []sdk.AccAddress{tc.address}, msg.GetSigners())
		} else {
			require.Panics(t, func() {
				msg.GetSigners()
			})
		}
	}
}

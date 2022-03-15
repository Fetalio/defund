package keeper

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/defundhub/defund/x/etf/types"
)

func (k msgServer) Invest(goCtx context.Context, msg *types.MsgInvest) (*types.MsgInvestResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	// TODO: Handling the message
	_ = ctx

	return &types.MsgInvestResponse{}, nil
}
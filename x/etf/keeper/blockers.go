package keeper

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	icatypes "github.com/cosmos/ibc-go/v4/modules/apps/27-interchain-accounts/types"
	channeltypes "github.com/cosmos/ibc-go/v4/modules/core/04-channel/types"
	brokertypes "github.com/defund-labs/defund/x/broker/types"
	etftypes "github.com/defund-labs/defund/x/etf/types"
)

func (k Keeper) EnsureICAChannelStaysOpen(ctx sdk.Context, brokerId string, fund etftypes.Fund) error {
	broker, found := k.brokerKeeper.GetBroker(ctx, brokerId)
	if !found {
		return sdkerrors.Wrap(brokertypes.ErrBrokerNotFound, fmt.Sprintf("broker %s not found", brokerId))
	}

	// get the ica account address port
	portID, err := icatypes.NewControllerPortID(fund.Address)
	if err != nil {
		return err
	}

	channels := k.channelKeeper.GetAllChannels(ctx)
	var nonClosedChannelFound bool = false

	// check if a channel with state other then closed exists (since we may be trying to open a new ICA channel, etc)
	for _, channel := range channels {
		if channel.PortId == portID {
			if channel.State != channeltypes.CLOSED {
				nonClosedChannelFound = true
			}
		}
	}

	// if we did not find a channel with state not closed, we must register broker account
	if !nonClosedChannelFound {
		ctx.Logger().Info(fmt.Sprintf("ICA channel for connection %s at port %s is closed. Attempting reopening of channel.", broker.ConnectionId, portID))
		err := k.RegisterBrokerAccount(ctx, broker.ConnectionId, fund.Address)
		if err != nil {
			return err
		}
	}

	return nil
}

// EndBlocker is the end blocker function for the etf module
func (k Keeper) EndBlocker(ctx sdk.Context) {
	funds := k.GetAllFund(ctx)

	for _, fund := range funds {
		// check if the channel for ica for each broker is open, if not re-open
		for i := range fund.Holdings {
			err := k.EnsureICAChannelStaysOpen(ctx, fund.Holdings[i].BrokerId, fund)
			if err != nil {
				ctx.Logger().Error(fmt.Sprintf("error while ensuring ICA channel is open for fund %s with broker %s (error = %s)", fund.Symbol, fund.Holdings[i].BrokerId, err.Error()))
			}
		}

		// only need to rebalance if there are balances/assets for this fund and if it isn't currently rebalancing
		if fund.Balances.HasBalance(fund.Symbol) && !fund.Rebalancing {
			// only have to run rebalance if this is rebalance period (aka no remainder)
			if ctx.BlockHeight()%fund.Rebalance == 0 {
				if fund.FundType == etftypes.FundType_ACTIVE {
					// if the fund is active run through the wasm keeper before you run rebalance
					contractSdkAddress, err := sdk.AccAddressFromBech32(fund.Contract)
					if err != nil {
						ctx.Logger().Error(fmt.Sprintf("error converting contract address %s to sdk address: %s", fund.Contract, err.Error()))
					}
					fundSdkAddress, err := sdk.AccAddressFromBech32(fund.Address)
					if err != nil {
						ctx.Logger().Error(fmt.Sprintf("error converting fund address %s to sdk address: %s", fund.Address, err.Error()))
					}
					_, err = k.wasmInternalKeeper.Execute(ctx, contractSdkAddress, fundSdkAddress, []byte(`{"runner": {}}`), sdk.NewCoins(sdk.Coin{Denom: "", Amount: sdk.NewInt(0)}))
					if err != nil {
						ctx.Logger().Error(fmt.Sprintf("error marshalling runner args on contract rebalance run for contract %s (error: %s)", fund.Contract, err.Error()))
					}
				}
				err := k.SendRebalanceTx(ctx, fund)
				if err != nil {
					ctx.Logger().Error(fmt.Sprintf("rebalance failed for fund %s with error: %s", fund.Symbol, err.Error()))
				}
			}
		}

		// create the balance queries we need for funds
		err := k.CreateBalances(ctx, fund)
		if err != nil {
			ctx.Logger().Error(fmt.Sprintf("error while creating account balance interqueries for fund %s... Error: %s", fund.Symbol, err.Error()))
		}
	}
}

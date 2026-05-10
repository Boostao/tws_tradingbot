import { GetWatchlist, GetCockpitState, UpdateCockpitState, GetStrategy, UpdateStrategy, AddWatchlistSymbol, RemoveWatchlistSymbol, ImportTradingViewWatchlist, GetRuntimeState } from '../../wailsjs/go/main/App';

export type BotState = {
	running: boolean;
};

// Map everything to Wails Bindings
export async function getHealth(): Promise<{ status: string }> {
	return { status: "ok" };
}

export async function getState(force = false): Promise<BotState> {
	return await GetRuntimeState();
}

export { 
    GetWatchlist as getWatchlist,
    GetCockpitState as getCockpitState,
    UpdateCockpitState as updateCockpitState,
    GetStrategy as getStrategy,
    UpdateStrategy as updateStrategy,
    AddWatchlistSymbol as addWatchlistSymbol,
    RemoveWatchlistSymbol as removeWatchlistSymbol,
    ImportTradingViewWatchlist as importTradingViewWatchlist,
    GetRuntimeState as getRuntimeState
};

const fs = require('fs');
const path = require('path');

const apiPath = path.resolve('frontend/src/lib/api.ts');
let content = fs.readFileSync(apiPath, 'utf8');

// Add import at the top
if (!content.includes('wailsjs')) {
    content = content.replace("export const API_BASE =", "import * as WailsApp from '../../wailsjs/go/main/App';\n\nexport const API_BASE =");
}

// Replace getWatchlist
content = content.replace(
    /export async function getWatchlist.*?\{[\s\S]*?\}/,
    `export async function getWatchlist(force = false): Promise<WatchlistResponse> {
        if (typeof window !== 'undefined' && (window as any).go) {
                return (await WailsApp.GetWatchlist()) as unknown as WatchlistResponse;
        }
        return fetchCachedJson<WatchlistResponse>('watchlist', cachePolicy.watchlist, 'watchlist.get', \`\${API_BASE}/api/v1/watchlist\`, 'Watchlist fetch', force);
}`
);

// Replace getCockpit
content = content.replace(
    /export async function getCockpit.*?\{[\s\S]*?\}/,
    `export async function getCockpit(force = false): Promise<CockpitState> {
        if (typeof window !== 'undefined' && (window as any).go) {
                return (await WailsApp.GetCockpitState()) as unknown as CockpitState;
        }
        return fetchCachedJson<CockpitState>('cockpit', cachePolicy.cockpit, 'cockpit.get', \`\${API_BASE}/api/v1/cockpit\`, 'Cockpit fetch', force);
}`
);

// Replace saveCockpit
content = content.replace(
    /export async function saveCockpit.*?\([\s\S]*?body: JSON.stringify\(payload\)\n\t\});\n\tif \(!response\.ok\) \{[\s\S]*?return \(await response\.json\(\)\) as CockpitState;\n\}/,
    `export async function saveCockpit(payload: {
        global_enabled: boolean;
        active_workspace_id?: string | null;
        workspaces: CockpitWorkspace[];
}): Promise<CockpitState> {
        if (typeof window !== 'undefined' && (window as any).go) {
                return (await WailsApp.UpdateCockpitState(payload as any)) as unknown as CockpitState;
        }
        const response = await timedFetch('cockpit.save', \`\${API_BASE}/api/v1/cockpit\`, {
                method: 'PUT',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(payload)
        });
        if (!response.ok) {
                throw await buildApiError(response, 'Cockpit save');
        }
        return (await response.json()) as CockpitState;
}`
);

// Replace getConfig
content = content.replace(
    /export async function getConfig.*?\{[\s\S]*?\}/,
    `export async function getConfig(force = false): Promise<ConfigResponse> {
        if (typeof window !== 'undefined' && (window as any).go) {
                return (await WailsApp.GetConfig()) as unknown as ConfigResponse;
        }
        return fetchCachedJson<ConfigResponse>('config', cachePolicy.config, 'config.get', \`\${API_BASE}/api/v1/config\`, 'Config fetch', force);
}`
);


// Replace updateConfig
content = content.replace(
    /export async function updateConfig.*?\) \{[\s\S]*?return \(await response\.json\(\)\) as ConfigResponse;\n\}/,
    `export async function updateConfig(updates: Record<string, Record<string, unknown>>): Promise<ConfigResponse> {
        if (typeof window !== 'undefined' && (window as any).go) {
                return (await WailsApp.UpdateConfig(updates as any)) as unknown as ConfigResponse;
        }
        const response = await timedFetch('config.update', \`\${API_BASE}/api/v1/config\`, {
                method: 'PUT',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ updates })
        });
        if (!response.ok) {
                throw await buildApiError(response, 'Config update');
        }
        return (await response.json()) as ConfigResponse;
}`
);

// Replace getStrategy
content = content.replace(
    /export async function getStrategy.*?\{[\s\S]*?\}/,
    `export async function getStrategy(force = false): Promise<Strategy> {
        if (typeof window !== 'undefined' && (window as any).go) {
                return (await WailsApp.GetStrategy()) as unknown as Strategy;
        }
        return fetchCachedJson<Strategy>('strategy', cachePolicy.strategy, 'strategy.get', \`\${API_BASE}/api/v1/strategy\`, 'Strategy fetch', force);
}`
);

fs.writeFileSync(apiPath, content);
console.log("api.ts patched");

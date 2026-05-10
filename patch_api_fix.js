const fs = require('fs');
const path = require('path');

const apiPath = path.resolve('frontend/src/lib/api.ts');
let content = fs.readFileSync(apiPath, 'utf8');

// Add import at the top
if (!content.includes('wailsjs')) {
    content = content.replace("export const API_BASE =", "import * as WailsApp from '../../wailsjs/go/main/App';\n\nexport const API_BASE =");
}

function processFunction(name, wailsCall, returnType) {
    const startIdx = content.indexOf(`export async function ${name}`);
    if (startIdx === -1) return;
    const endIdx = content.indexOf('}', startIdx) + 1;
    const body = content.substring(startIdx, endIdx);
    
    // Simplistic replace inside the body
    let newBody = body;
    if (name.includes('save') || name.includes('update')) {
        // e.g. updateConfig
        newBody = newBody.replace(/const response = await timedFetch/, `if (typeof window !== 'undefined' && (window as any).go) {\n                return (await ${wailsCall}) as unknown as ${returnType};\n        }\n        const response = await timedFetch`);
    } else {
        // e.g. getWatchlist
        newBody = newBody.replace(/return fetchCachedJson/, `if (typeof window !== 'undefined' && (window as any).go) {\n                return (await ${wailsCall}) as unknown as ${returnType};\n        }\n        return fetchCachedJson`);
    }
    
    content = content.replace(body, newBody);
}

processFunction('getWatchlist(force', 'WailsApp.GetWatchlist()', 'WatchlistResponse');
processFunction('getCockpit(force', 'WailsApp.GetCockpitState()', 'CockpitState');
processFunction('getConfig(force', 'WailsApp.GetConfig()', 'ConfigResponse');
processFunction('getStrategy(force', 'WailsApp.GetStrategy()', 'Strategy');

// Updates
processFunction('updateConfig', 'WailsApp.UpdateConfig(updates as any)', 'ConfigResponse');
processFunction('saveCockpit', 'WailsApp.UpdateCockpitState(payload as any)', 'CockpitState');

fs.writeFileSync(apiPath, content);
console.log("api.ts patched successfully");

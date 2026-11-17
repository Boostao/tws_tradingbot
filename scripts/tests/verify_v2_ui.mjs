#!/usr/bin/env node
/**
 * Opaque-box headless Chromium E2E UI verification suite for Cobalt V2 (R1–R9).
 * Zero-lockstep: resolves Playwright from PLAYWRIGHT_PATH, node_modules, or cached ~/.npm/_npx.
 * Intercepts Wails v3 POST /wails/runtime dynamically via frontend/bindings/tws_traderbot/app.js.
 */

import fs from 'node:fs';
import http from 'node:http';
import os from 'node:os';
import path from 'node:path';
import { createRequire } from 'node:module';
import { fileURLToPath } from 'node:url';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const REPO_ROOT = path.resolve(__dirname, '..', '..');

const args = process.argv.slice(2);
function getArg(flag, fallback) {
  const idx = args.indexOf(flag);
  if (idx !== -1 && args[idx + 1]) return args[idx + 1];
  return fallback;
}

const DIST_DIR = path.resolve(REPO_ROOT, getArg('--dist', 'frontend/dist'));
const BINDINGS_APP_JS = path.resolve(REPO_ROOT, 'frontend/bindings/tws_traderbot/app.js');

const MIME_TYPES = {
  '.html': 'text/html; charset=utf-8',
  '.js': 'application/javascript; charset=utf-8',
  '.mjs': 'application/javascript; charset=utf-8',
  '.css': 'text/css; charset=utf-8',
  '.svg': 'image/svg+xml',
  '.png': 'image/png',
  '.ico': 'image/x-icon',
  '.woff2': 'font/woff2',
  '.woff': 'font/woff',
  '.json': 'application/json; charset=utf-8',
};

const TOUR_SELECTOR = [
  '[data-testid="onboarding-tour"]',
  '[class*="tour-overlay"]',
  '[class*="onboarding-tour"]',
  '[class*="spotlight-tour"]',
  '[class*="onboarding-prompt"]',
  '[class*="tour-modal"]',
  '[class*="onboarding-modal"]',
  '[role="dialog"][class*="tour" i]',
  '[role="dialog"][class*="onboarding" i]',
  '[role="dialog"][class*="spotlight" i]',
  '[role="dialog"][aria-label*="tour" i]',
  '[role="dialog"][aria-label*="visite" i]',
  '[role="dialog"][aria-label*="onboarding" i]',
].join(', ');

const HELP_DRAWER_SELECTOR = [
  '[class*="help-drawer"]',
  '[data-testid="help-drawer"]',
  'aside[aria-label*="help" i]',
  'aside[aria-label*="aide" i]',
  '[role="dialog"][aria-label*="help" i]',
  '[role="dialog"][aria-label*="aide" i]',
].join(', ');

const LEAKED_I18N_KEY_RE =
  /\b(?:sidebar|tws|bot|nav|rule|action|scope|symbol|bracket|cockpit|monitoring|manual|watchlist|strategy|tour|help)_[a-z0-9_]{2,}\b/g;

// ---------------------------------------------------------------------------
// 1. Zero-Lockstep Playwright & Chromium Resolver
// ---------------------------------------------------------------------------
async function loadPlaywright() {
  const req = createRequire(import.meta.url);
  if (process.env.PLAYWRIGHT_PATH) {
    return req(process.env.PLAYWRIGHT_PATH);
  }
  for (const pkg of ['playwright', 'playwright-core']) {
    try {
      const mod = await import(pkg);
      return mod.default ?? mod;
    } catch {
      // continue to _npx fallback
    }
  }
  const npxRoot = path.join(os.homedir(), '.npm', '_npx');
  if (fs.existsSync(npxRoot)) {
    const candidates = [];
    for (const entry of fs.readdirSync(npxRoot)) {
      const pwEntry = path.join(npxRoot, entry, 'node_modules', 'playwright', 'index.js');
      if (fs.existsSync(pwEntry)) {
        candidates.push({ pwEntry, mtime: fs.statSync(pwEntry).mtimeMs });
      }
    }
    candidates.sort((a, b) => b.mtime - a.mtime);
    if (candidates.length > 0) {
      return req(candidates[0].pwEntry);
    }
  }
  throw new Error('Playwright not found. Set PLAYWRIGHT_PATH or install playwright in npm/_npx cache.');
}

function findChromiumExecutable() {
  const cacheDir = path.join(os.homedir(), '.cache', 'ms-playwright');
  if (fs.existsSync(cacheDir)) {
    const entries = fs.readdirSync(cacheDir).sort().reverse();
    for (const name of entries) {
      if (name.startsWith('chromium_headless_shell-')) {
        const bin = path.join(cacheDir, name, 'chrome-headless-shell-linux64', 'chrome-headless-shell');
        if (fs.existsSync(bin)) return bin;
      }
      if (name.startsWith('chromium-')) {
        const bin = path.join(cacheDir, name, 'chrome-linux64', 'chrome');
        if (fs.existsSync(bin)) return bin;
      }
    }
  }
  if (fs.existsSync('/usr/bin/chromium-browser')) return '/usr/bin/chromium-browser';
  if (fs.existsSync('/usr/bin/chromium')) return '/usr/bin/chromium';
  return undefined;
}

// ---------------------------------------------------------------------------
// 2. Dynamic Wails v3 Method ID Parser + Static File Server
// ---------------------------------------------------------------------------
function parseWailsMethodIDs(appJsPath) {
  if (!fs.existsSync(appJsPath)) {
    throw new Error(`Missing Wails bindings file: ${appJsPath}`);
  }
  const src = fs.readFileSync(appJsPath, 'utf8');
  const idToName = new Map();
  const re = /export function (\w+)\([^)]*\)\s*\{\s*return \$Call\.ByID\((\d+)/g;
  let m;
  while ((m = re.exec(src)) !== null) {
    idToName.set(Number(m[2]), m[1]);
  }
  if (idToName.size === 0) {
    throw new Error(`Parsed 0 Wails method IDs from ${appJsPath}`);
  }
  return idToName;
}

function startStaticServer(distDir) {
  return new Promise((resolve, reject) => {
    const server = http.createServer((req, res) => {
      const reqUrl = new URL(req.url || '/', 'http://127.0.0.1');
      let rel = decodeURIComponent(reqUrl.pathname).replace(/^\/+/, '');
      if (!rel) rel = 'index.html';

      let target = path.join(distDir, rel);
      if (!fs.existsSync(target) || fs.statSync(target).isDirectory()) {
        if (fs.existsSync(target + '.html')) {
          target = target + '.html';
        } else {
          target = path.join(distDir, 'index.html');
        }
      }

      const ext = path.extname(target).toLowerCase();
      const stream = fs.createReadStream(target);
      stream.on('error', (err) => {
        if (!res.headersSent) {
          res.writeHead(404, { 'Content-Type': 'text/plain; charset=utf-8' });
        }
        res.end(String(err.message || 'Not found'));
      });
      res.writeHead(200, { 'Content-Type': MIME_TYPES[ext] || 'application/octet-stream' });
      stream.pipe(res);
    });
    server.on('error', reject);
    server.listen(0, '127.0.0.1', () => {
      const addr = server.address();
      resolve({ server, baseUrl: `http://127.0.0.1:${addr.port}` });
    });
  });
}

// ---------------------------------------------------------------------------
// 3. Stateful Mock Wails IPC Fixtures & Route Installer
// ---------------------------------------------------------------------------
function deepMerge(target, src) {
  const out = { ...(target || {}) };
  for (const [k, v] of Object.entries(src || {})) {
    if (v && typeof v === 'object' && !Array.isArray(v) && out[k] && typeof out[k] === 'object' && !Array.isArray(out[k])) {
      out[k] = deepMerge(out[k], v);
    } else {
      out[k] = v;
    }
  }
  return out;
}

function makeTickerItems(count, heldSymbols = ['GOOGL', 'TSLA'], longNames = false) {
  const items = [];
  for (let i = 0; i < count; i++) {
    const sym = i === 0 ? 'GOOGL' : i === 1 ? 'TSLA' : `SYM${String(i).padStart(2, '0')}`;
    const venue = i % 5 === 0 ? 'TSX' : 'NASDAQ';
    items.push({
      id: i + 1,
      watchlist_id: 'cobalt-telegram',
      group_id: 'titres',
      group_name: 'Titres',
      symbol: sym,
      exchange: venue,
      venue,
      instrument_id: `${sym}.${venue}`,
      name: longNames ? `${sym} International Holdings & Capital Markets Corp` : `${sym} Corp`,
      enabled: i % 4 !== 3,
      tier: i % 3 === 0 ? '1-Blue Chip' : '2-Croissance',
      is_orderable: true,
      is_regime_reference: false,
      is_holding: heldSymbols.includes(sym),
      size_mode: i % 3 === 0 ? 'shares' : i % 3 === 1 ? 'dollar' : 'percent',
      size_value: i % 3 === 0 ? 10 : i % 3 === 1 ? 2500 : 5,
    });
  }
  return items;
}

function createMockBackend(overrides = {}) {
  const initialActiveWorkspaceId = overrides.activeWorkspaceId ?? 'achat';
  const initialActiveStrategyId = overrides.activeStrategyId ?? 'cours_weinstein_p2';

  const state = {
    twsConnected: overrides.twsConnected ?? true,
    running: overrides.running ?? false,
    accountId: overrides.accountId ?? 'DUO991122',
    equity: overrides.equity ?? 142850.5,
    dailyPnl: overrides.dailyPnl ?? 645.25,
    dailyPnlPct: overrides.dailyPnlPct ?? 0.45,
    dailyLossLimit: overrides.dailyLossLimit ?? 1500,
    circuitBreakerTripped: overrides.circuitBreakerTripped ?? false,
    globalEnabled: overrides.globalEnabled ?? true,
    cashUsd: overrides.cashUsd ?? 84200.5,
    cashCad: overrides.cashCad ?? 61400.25,
    buyingPower: overrides.buyingPower ?? 285000.0,
    activeWorkspaceId: initialActiveWorkspaceId,
    activeStrategyId: initialActiveStrategyId,
    activeWatchlistId: overrides.activeWatchlistId ?? 'cobalt-telegram',
    tickerCount: overrides.tickerCount ?? 12,
    longTickerNames: overrides.longTickerNames ?? false,
    applyDelayMs: overrides.applyDelayMs ?? 0,
    heldSymbols: overrides.heldSymbols ?? ['GOOGL', 'TSLA'],
    brokerAlerts: overrides.brokerAlerts ?? [
      { code: 10089, severity: 'warning', message: 'Delayed market data', hint: 'Enable real-time L1 subscription or switch to delayed quotes in TWS.', timestamp: '2026-11-17T14:00:00Z' },
      { code: 504, severity: 'error', message: 'Not connected', hint: 'Verify TWS API socket port 7497 is enabled and listening.', timestamp: '2026-11-17T13:55:00Z' },
    ],
    strategies: overrides.strategies ? overrides.strategies.map((s) => ({ ...s })) : [
      { id: 'cours_weinstein_p2', name: 'Cobalt Weinstein Phase 2', rule_count: 3, enabled_rule_count: 3, source: 'preset', pinned: true, sort_order: 0, workspace: 'swing' },
      { id: 'screener_10_30_haut', name: 'Croisement 10-30 Haut', rule_count: 2, enabled_rule_count: 2, source: 'preset', pinned: true, sort_order: 1, workspace: 'achat' },
      { id: 'screener_10_30_bas', name: 'Croisement 10-30 Bas (Short)', rule_count: 2, enabled_rule_count: 2, source: 'preset', pinned: false, sort_order: 2, workspace: 'short' },
      { id: 'style_01', name: 'Cobalt Modèle 1 — Volume', rule_count: 2, enabled_rule_count: 2, source: 'preset', pinned: false, sort_order: 3, workspace: 'achat' },
    ],
    workspaces: [
      {
        id: 'achat',
        name: 'Achat',
        kind: 'long',
        enabled: initialActiveWorkspaceId === 'achat',
        watchlist_id: 'cobalt-telegram',
        strategy_slots: [{ id: 'slot_1', label: 'Slot 1', strategy_id: initialActiveStrategyId, enabled: initialActiveWorkspaceId === 'achat' }],
      },
      {
        id: 'short',
        name: 'Short',
        kind: 'short',
        enabled: initialActiveWorkspaceId === 'short',
        watchlist_id: 'cobalt-telegram',
        strategy_slots: [{ id: 'slot_1', label: 'Slot 1', strategy_id: initialActiveStrategyId, enabled: initialActiveWorkspaceId === 'short' }],
      },
      {
        id: 'swing',
        name: 'Swing',
        kind: 'swing',
        enabled: initialActiveWorkspaceId === 'swing',
        watchlist_id: 'cobalt-telegram',
        strategy_slots: [{ id: 'slot_1', label: 'Slot 1', strategy_id: initialActiveStrategyId, enabled: initialActiveWorkspaceId === 'swing' }],
      },
    ],
  };

  function sortStrategies() {
    state.strategies.sort((a, b) => {
      if (Boolean(a.pinned) !== Boolean(b.pinned)) return a.pinned ? -1 : 1;
      if (a.sort_order !== b.sort_order) return a.sort_order - b.sort_order;
      return a.name.localeCompare(b.name);
    });
  }
  sortStrategies();

  function activeStrategyName() {
    const found = state.strategies.find((s) => s.id === state.activeStrategyId);
    return found ? found.name : '';
  }

  function workspaceKindForId(wsId) {
    const ws = state.workspaces.find((w) => w.id === wsId);
    if (ws?.kind) return ws.kind;
    if (wsId === 'short') return 'short';
    if (wsId === 'swing') return 'swing';
    return 'long';
  }

  function buildStrategyDetail(stratId) {
    const targetId = stratId || state.activeStrategyId;
    const summary = state.strategies.find((s) => s.id === targetId);
    return {
      id: summary ? summary.id : targetId,
      name: summary ? summary.name : activeStrategyName() || targetId,
      description: 'Cobalt Phase 2 Momentum',
      initial_capital: 100000,
      max_positions: 5,
      position_size_mode: 'shares',
      position_size_value: 100,
      rules: [
        {
          id: 'r_filter',
          name: 'VIX Macro Filter',
          scope: 'global',
          action: 'filter',
          enabled: true,
          priority: 10,
          condition: { type: 'less_than', indicator_a: { type: 'vix', timeframe: '1d' }, threshold: 25 },
        },
        {
          id: 'r_buy',
          name: 'EMA 10/30 Buy',
          scope: 'per_ticker',
          action: 'buy',
          enabled: true,
          priority: 50,
          condition: {
            type: 'crosses_above',
            indicator_a: { type: 'ema', period: 10, timeframe: '1d' },
            indicator_b: { type: 'ema', period: 30, timeframe: '1d' },
          },
        },
        {
          id: 'r_sell',
          name: 'RSI Overbought Exit',
          scope: 'per_ticker',
          action: 'sell',
          enabled: true,
          priority: 40,
          condition: { type: 'greater_than', indicator_a: { type: 'rsi', period: 14, timeframe: '1d' }, threshold: 75 },
        },
      ],
    };
  }

  function buildBotState() {
    const positions = state.twsConnected
      ? state.heldSymbols.map((sym) => ({
          symbol: sym,
          venue: 'NASDAQ',
          instrument_id: `${sym}.NASDAQ`,
          currency: 'USD',
          position: sym === 'GOOGL' ? 15 : 5,
          avg_cost: sym === 'GOOGL' ? 175.25 : 240.0,
          market_price: sym === 'GOOGL' ? 182.0 : 246.0,
          market_value: sym === 'GOOGL' ? 2730.0 : 1230.0,
          unrealized_pnl: sym === 'GOOGL' ? 101.25 : 30.0,
          realized_pnl: 0,
        }))
      : [];
    const orders = state.twsConnected
      ? [
          {
            order_id: 42,
            symbol: 'NVDA',
            venue: 'NASDAQ',
            instrument_id: 'NVDA.NASDAQ',
            action: 'BUY',
            order_type: 'LMT',
            total_qty: 10,
            filled_qty: 0,
            remaining_qty: 10,
            limit_price: 135.5,
            avg_fill_price: 0,
            status: 'Submitted',
            outside_rth: false,
          },
        ]
      : [];

    let status = 'STOPPED';
    if (state.circuitBreakerTripped) status = 'CIRCUIT_BREAKER';
    else if (state.running) status = 'RUNNING';
    else if (state.globalEnabled) status = 'ARMED';

    return {
      status,
      running: state.running,
      runner_active: state.running,
      tws_connected: state.twsConnected,
      account_id: state.twsConnected ? state.accountId : '',
      dry_run: true,
      equity: state.twsConnected ? state.equity : 0,
      buying_power: state.twsConnected ? state.buyingPower : 0,
      cash_usd: state.twsConnected ? state.cashUsd : 0,
      cash_cad: state.twsConnected ? state.cashCad : 0,
      cash_base: state.twsConnected ? state.cashUsd + state.cashCad * 0.72 : 0,
      daily_pnl: state.twsConnected ? state.dailyPnl : 0,
      daily_pnl_percent: state.twsConnected ? state.dailyPnlPct : 0,
      total_pnl: state.twsConnected ? state.dailyPnl : 0,
      realized_pnl: state.twsConnected ? 420.0 : 0,
      unrealized_pnl: state.twsConnected ? state.dailyPnl - 420.0 : 0,
      daily_loss_limit: state.dailyLossLimit,
      circuit_breaker_tripped: state.circuitBreakerTripped,
      active_workspace_id: state.activeWorkspaceId,
      active_strategy_id: state.activeStrategyId,
      active_strategy: activeStrategyName(),
      session_status: 'NY: RTH | TSX: RTH',
      open_positions_count: positions.length,
      pending_orders_count: orders.length,
      open_positions: positions,
      open_orders: orders,
      broker_alerts: state.brokerAlerts,
      trades_today: 2,
      win_rate_today: 100,
      last_heartbeat: '2026-11-17T14:05:00Z',
      last_update: '2026-11-17T14:05:00Z',
    };
  }

  function buildCockpitState() {
    return {
      global_enabled: state.globalEnabled,
      active_workspace_id: state.activeWorkspaceId,
      workspaces: state.workspaces.map((ws) => ({
        ...ws,
        enabled: ws.id === state.activeWorkspaceId,
        strategy_slots: (ws.strategy_slots || []).map((slot) => ({
          ...slot,
          strategy_id: ws.id === state.activeWorkspaceId ? state.activeStrategyId : slot.strategy_id,
          enabled: ws.id === state.activeWorkspaceId ? Boolean(slot.enabled) : false,
        })),
      })),
      strategy_library: state.strategies,
      updated_at: '2026-11-17T14:05:00Z',
    };
  }

  function buildWatchlist(id) {
    if (id === 'positions-detenues') {
      const heldItems = state.twsConnected ? makeTickerItems(state.heldSymbols.length, state.heldSymbols) : [];
      return {
        id: 'positions-detenues',
        name: 'Positions détenues ($$$$)',
        description: 'Live held positions',
        is_default: false,
        symbols: heldItems.map((it) => it.instrument_id),
        groups: [{ id: 'holding', name: 'Positions Détenues', source: 'manual', items: heldItems }],
        updated_at: '2026-11-17T14:05:00Z',
      };
    }
    if (id === 'cobalt-home-run') {
      const items = makeTickerItems(4, state.heldSymbols).map((it) => ({ ...it, watchlist_id: 'cobalt-home-run' }));
      return {
        id: 'cobalt-home-run',
        name: '⚾️@ Cobalt- Home Run',
        description: 'Growth universe',
        is_default: false,
        symbols: items.map((it) => it.instrument_id),
        groups: [{ id: 'titres-croissance', name: 'Croissance', source: 'tradingview', items }],
        feed: { provider: 'tradingview', url: 'https://www.tradingview.com/watchlists/111426240/', title: 'Cobalt Home Run', last_refreshed_at: '2026-11-17T14:00:00Z' },
        updated_at: '2026-11-17T14:05:00Z',
      };
    }
    const items = makeTickerItems(state.tickerCount, state.heldSymbols, state.longTickerNames);
    return {
      id: 'cobalt-telegram',
      name: '✝️@ Cobalt- TELEGRAM',
      description: 'Master universe',
      is_default: true,
      symbols: items.map((it) => it.instrument_id),
      groups: items.length > 0 ? [{ id: 'titres', name: 'Titres', source: 'tradingview', items }] : [],
      feed: { provider: 'tradingview', url: 'https://www.tradingview.com/watchlists/209453826/', title: 'Cobalt TELEGRAM', last_refreshed_at: '2026-11-17T14:00:00Z' },
      updated_at: '2026-11-17T14:05:00Z',
    };
  }

  let configState = {
    ib: {
      host: '127.0.0.1',
      port: 7497,
      client_id: 1,
      account: state.twsConnected ? state.accountId : '',
      trading_mode: 'paper',
    },
    runtime: {
      fixed_notional: 10000,
      bracket_enabled: true,
      stop_loss_pct: 2,
      take_profit_pct: 4,
    },
  };

  const ipcCalls = [];

  async function installOnPage(page, idToName) {
    await page.route('**/wails/runtime', async (route) => {
      const postRaw = route.request().postData() || '{}';
      let payload;
      try {
        payload = JSON.parse(postRaw);
      } catch {
        payload = {};
      }

      if (payload.object !== 0) {
        await route.fulfill({ status: 200, contentType: 'application/json', body: '{}' });
        return;
      }

      const methodID = payload.args?.methodID;
      const callArgs = payload.args?.args ?? [];
      const methodName = idToName.get(methodID) || `UNKNOWN_${methodID}`;
      ipcCalls.push({ methodName, args: callArgs });

      let result = {};
      switch (methodName) {
        case 'GetConfig':
          result = configState;
          break;
        case 'UpdateConfig':
          configState = deepMerge(configState, callArgs[0] || {});
          result = configState;
          break;
        case 'GetTWSConnection':
          result = configState.ib;
          break;
        case 'UpdateTWSConnection': {
          const [host, port, clientID] = callArgs;
          if (host) configState.ib.host = host;
          if (typeof port === 'number') configState.ib.port = port;
          if (typeof clientID === 'number') configState.ib.client_id = clientID;
          result = null;
          break;
        }
        case 'ConnectTws': {
          const [host, port, clientID] = callArgs;
          if (host) configState.ib.host = host;
          if (typeof port === 'number') configState.ib.port = port;
          if (typeof clientID === 'number') configState.ib.client_id = clientID;
          state.twsConnected = true;
          configState.ib.account = state.accountId;
          result = { connected: true, status: 'CONNECTED', account: state.accountId };
          break;
        }
        case 'DisconnectTws':
          state.twsConnected = false;
          state.running = false;
          result = { connected: false, status: 'DISCONNECTED' };
          break;
        case 'StartBot':
          state.running = true;
          result = { running: true, status: 'RUNNING' };
          break;
        case 'StopBot':
          state.running = false;
          result = { running: false, status: state.globalEnabled ? 'ARMED' : 'STOPPED' };
          break;
        case 'GetRuntimeState':
          result = buildBotState();
          break;
        case 'GetCockpitState':
          result = buildCockpitState();
          break;
        case 'UpdateCockpitState': {
          const upd = callArgs[0] || {};
          if (typeof upd.global_enabled === 'boolean') state.globalEnabled = upd.global_enabled;
          if (upd.active_workspace_id) state.activeWorkspaceId = upd.active_workspace_id;
          if (Array.isArray(upd.workspaces) && upd.workspaces.length > 0) {
            state.workspaces = upd.workspaces.map((ws) => ({ ...ws }));
            const activeWs =
              state.workspaces.find((w) => w.id === state.activeWorkspaceId) ||
              state.workspaces.find((w) => w.enabled);
            if (activeWs) {
              state.activeWorkspaceId = activeWs.id;
              if (activeWs.watchlist_id) state.activeWatchlistId = activeWs.watchlist_id;
              const activeSlot =
                (activeWs.strategy_slots || []).find((s) => s.enabled && s.strategy_id) ||
                (activeWs.strategy_slots || []).find((s) => s.strategy_id);
              if (activeSlot?.strategy_id) {
                state.activeStrategyId = activeSlot.strategy_id;
              }
            }
          }
          result = buildCockpitState();
          break;
        }
        case 'ApplyStrategyPreset': {
          if (state.applyDelayMs > 0) {
            await new Promise((r) => setTimeout(r, state.applyDelayMs));
          }
          const [workspaceId, slotId, strategyId] = callArgs;
          if (strategyId) {
            state.activeStrategyId = strategyId;
            const ws = state.workspaces.find((w) => w.id === (workspaceId || state.activeWorkspaceId));
            if (ws) {
              ws.strategy_slots = [{ id: slotId || 'slot_1', label: 'Slot 1', strategy_id: strategyId, enabled: true }];
            }
          }
          result = null;
          break;
        }
        case 'ToggleStrategyPin': {
          const [id] = callArgs;
          const target = state.strategies.find((s) => s.id === id);
          if (target) target.pinned = !target.pinned;
          sortStrategies();
          result = state.strategies;
          break;
        }
        case 'MoveStrategyOrder': {
          const [id, dir] = callArgs;
          const idx = state.strategies.findIndex((s) => s.id === id);
          if (idx !== -1) {
            const swapIdx = dir === 'up' ? idx - 1 : idx + 1;
            if (swapIdx >= 0 && swapIdx < state.strategies.length) {
              const tmpOrder = state.strategies[idx].sort_order;
              state.strategies[idx].sort_order = state.strategies[swapIdx].sort_order;
              state.strategies[swapIdx].sort_order = tmpOrder;
              sortStrategies();
            }
          }
          result = state.strategies;
          break;
        }
        case 'GetWatchlists':
          result = [
            { id: 'cobalt-telegram', name: '✝️@ Cobalt- TELEGRAM', is_default: true, source: 'tradingview', item_count: state.tickerCount },
            { id: 'cobalt-home-run', name: '⚾️@ Cobalt- Home Run', is_default: false, source: 'tradingview', item_count: 4 },
            { id: 'positions-detenues', name: 'Positions détenues ($$$$)', is_default: false, source: 'manual', item_count: state.twsConnected ? state.heldSymbols.length : 0 },
          ];
          break;
        case 'GetWatchlist':
          result = buildWatchlist('cobalt-telegram');
          break;
        case 'GetWatchlistById':
          result = buildWatchlist(callArgs[0] || 'cobalt-telegram');
          break;
        case 'RefreshWatchlistFeed':
          result = buildWatchlist(callArgs[0] || state.activeWatchlistId);
          break;
        case 'ReplaceWatchlist':
          result = buildWatchlist(callArgs[0]?.watchlist_id || state.activeWatchlistId);
          break;
        case 'GetSymbols':
          result = {
            symbols: makeTickerItems(state.tickerCount, state.heldSymbols),
            source: 'ibkr_contract_resolver',
            total: state.tickerCount,
          };
          break;
        case 'GetStrategyLibrary':
          result = state.strategies;
          break;
        case 'GetStrategyPreset':
        case 'GetStrategy':
          result = buildStrategyDetail(callArgs[0]);
          break;
        case 'UpdateStrategy':
          result = { ...buildStrategyDetail(state.activeStrategyId), ...(callArgs[0] || {}) };
          break;
        case 'ValidateStrategy':
          result = { valid: true, errors: [] };
          break;
        case 'SaveStrategyPreset': {
          const strat = callArgs[0] || {};
          const id = strat.id || state.activeStrategyId || `custom_${Date.now()}`;
          const existing = state.strategies.find((s) => s.id === id);
          const rules = Array.isArray(strat.rules) ? strat.rules : [];
          const enabledCount = rules.filter((r) => r.enabled).length;
          if (existing) {
            if (strat.name) existing.name = strat.name;
            existing.rule_count = rules.length || existing.rule_count;
            existing.enabled_rule_count = enabledCount || existing.enabled_rule_count;
            result = existing;
          } else {
            const added = {
              id,
              name: strat.name || id,
              rule_count: rules.length,
              enabled_rule_count: enabledCount,
              source: 'custom',
              pinned: false,
              sort_order: state.strategies.length + 1,
              workspace: 'achat',
            };
            state.strategies.push(added);
            sortStrategies();
            result = added;
          }
          break;
        }
        case 'DeleteStrategyPreset': {
          const [id] = callArgs;
          state.strategies = state.strategies.filter((s) => s.id !== id);
          result = null;
          break;
        }
        case 'GetDiagnostics':
          result = {
            startup: {
              environment: 'live_workstation',
              trading_mode: 'paper',
              ib_host: '127.0.0.1',
              ib_port: 7497,
              client_id: 1,
              account: state.twsConnected ? state.accountId : '',
              log_level: 'WARN',
              log_file: '',
              watchlist_path: '',
              strategy_path: '',
              symbol_cache_path: '',
            },
            runtime: { runner_active: state.running, last_runtime_reload_at: '2026-11-17T14:05:00Z' },
            symbols: { source: 'ibkr_contract_resolver', last_checked_at: '2026-11-17T14:05:00Z' },
            session_status: 'NY: RTH | TSX: RTH',
            account: state.twsConnected ? state.accountId : '',
            dry_run: true,
            active_workspace_id: state.activeWorkspaceId,
            active_strategy_id: state.activeStrategyId,
            active_strategy_name: activeStrategyName(),
            broker_alerts: state.brokerAlerts,
          };
          break;
        case 'DryRunBot':
          result = {
            strategy: activeStrategyName(),
            workspace_kind: workspaceKindForId(state.activeWorkspaceId),
            subscriptions: [
              { symbol: 'GOOGL.NASDAQ', timeframe: '1d' },
              { symbol: 'TSLA.NASDAQ', timeframe: '1d' },
            ],
            positions: {},
            open_orders: [],
            planned_orders: [{ symbol: 'NVDA.NASDAQ', action: 'BUY', qty: 10, price: 135.5 }],
            state: buildBotState(),
            generated_at: '2026-11-17T14:05:00Z',
          };
          break;
        case 'GetRecentTrades':
          result = [
            { id: 1, order_id: 101, symbol: 'GOOGL', action: 'BUY', quantity: 15, price: 175.25, status: 'SIMULATED_FILL', mode: 'dry_run', created_at: '2026-11-17T14:00:00Z' },
          ];
          break;
        case 'ResetCircuitBreaker':
          state.circuitBreakerTripped = false;
          state.globalEnabled = true;
          result = null;
          break;
        default:
          result = {};
      }

      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(result),
      });
    });
  }

  return { state, ipcCalls, installOnPage, buildBotState };
}

// ---------------------------------------------------------------------------
// 4. Test Runner & Visibility Helpers
// ---------------------------------------------------------------------------
function assert(cond, msg) {
  if (!cond) throw new Error(msg);
}

async function isVisibleInViewport(page, selector) {
  return page.evaluate((sel) => {
    const els = Array.from(document.querySelectorAll(sel));
    return els.some((el) => {
      const cs = getComputedStyle(el);
      if (cs.display === 'none' || cs.visibility === 'hidden' || parseFloat(cs.opacity) === 0) return false;
      const r = el.getBoundingClientRect();
      if (r.width <= 0 || r.height <= 0) return false;
      return r.right > 4 && r.left < window.innerWidth - 4 && r.bottom > 4 && r.top < window.innerHeight - 4;
    });
  }, selector);
}

async function dismissOpenTourIfPresent(page) {
  const open = await isVisibleInViewport(page, TOUR_SELECTOR);
  if (!open) return;
  const tour = page.locator(TOUR_SELECTOR).first();
  const skipOrClose = tour
    .locator('button')
    .filter({ hasText: /skip|passer|fermer|close|terminer|finish|ignorer|plus tard|later/i })
    .first();
  if ((await skipOrClose.count()) > 0) {
    await skipOrClose.click().catch(() => {});
  } else {
    await page.keyboard.press('Escape').catch(() => {});
  }
  await page.waitForTimeout(100);
}

async function withIsolatedPage(browser, baseUrl, idToName, mockOverrides, viewport, fn, options = {}) {
  const seedTourDismissed = options.seedTourDismissed !== false;
  const context = await browser.newContext({ viewport });

  if (seedTourDismissed) {
    await context.addInitScript(() => {
      try {
        const doneKeys = [
          'cobalt_tour_completed',
          'cobalt_onboarding_completed',
          'cobalt_tour_dismissed',
          'cobalt_onboarding_dismissed',
          'cobalt_tour_state',
          'cobalt_onboarding_state',
          'onboarding_completed',
          'tour_completed',
          'tour_dismissed',
          'tws_traderbot_tour',
        ];
        for (const k of doneKeys) {
          if (!localStorage.getItem(k)) {
            localStorage.setItem(k, JSON.stringify({ completed: true, dismissed: true, track: 1, step: 5 }));
          }
        }
      } catch {
        // ignore storage errors
      }
    });
  }

  const page = await context.newPage();
  const pageErrors = [];
  const consoleErrors = [];
  page.on('pageerror', (err) => pageErrors.push(err.message));
  page.on('console', (msg) => {
    if (msg.type() === 'error') consoleErrors.push(msg.text());
  });

  const mock = createMockBackend(mockOverrides);
  await mock.installOnPage(page, idToName);
  try {
    await fn(page, mock, baseUrl);
    assert(pageErrors.length === 0, `Uncaught pageerror(s): ${pageErrors.join(' | ')}`);
    assert(consoleErrors.length === 0, `Console error(s): ${consoleErrors.join(' | ')}`);
  } finally {
    await context.close();
  }
}

async function run() {
  if (!fs.existsSync(DIST_DIR)) {
    throw new Error(`frontend/dist not found at ${DIST_DIR}`);
  }
  const idToName = parseWailsMethodIDs(BINDINGS_APP_JS);
  const pw = await loadPlaywright();
  const executablePath = findChromiumExecutable();
  const browser = await pw.chromium.launch({ headless: true, executablePath });
  const { server, baseUrl } = await startStaticServer(DIST_DIR);

  const results = [];
  async function testCase(id, fn) {
    try {
      await fn();
      results.push({ id, passed: true });
      console.log(`PASS  ${id}`);
    } catch (err) {
      results.push({ id, passed: false, error: err.message });
      console.log(`FAIL  ${id}: ${err.message}`);
    }
  }

  try {
    // -----------------------------------------------------------------------
    // R1: Cockpit Layout, HUD Height Lock (52 vs 2 vs 0), Zero Outer/Main Overflow, No Bottom Banner
    // -----------------------------------------------------------------------
    await testCase('R1_CockpitLayout_52vs2Tickers_And_NoBottomBanner', async () => {
      let h52 = 0;
      let h2 = 0;
      for (const vp of [
        { width: 1440, height: 900 },
        { width: 1280, height: 768 },
      ]) {
        await withIsolatedPage(browser, baseUrl, idToName, { tickerCount: 52 }, vp, async (page) => {
          await page.goto(`${baseUrl}/cockpit`, { waitUntil: 'networkidle' });
          await dismissOpenTourIfPresent(page);
          await page.waitForSelector('.cockpit-hud-strip');

          const geom52 = await page.evaluate(() => {
            const shell = document.querySelector('.cockpit-shell');
            const hud = document.querySelector('.cockpit-hud-strip');
            const grid = document.querySelector('.cockpit-grid');
            const mainEl = document.querySelector('.app-main, main');
            const safetyCount = document.querySelectorAll('.cockpit-safety').length;
            const shellRows = shell ? getComputedStyle(shell).gridTemplateRows.trim().split(/\s+/).length : 0;
            return {
              shellRows,
              hudRect: hud?.getBoundingClientRect().toJSON(),
              gridRect: grid?.getBoundingClientRect().toJSON(),
              safetyCount,
              docScrollH: document.documentElement.scrollHeight,
              innerH: window.innerHeight,
              mainScrollH: mainEl ? mainEl.scrollHeight : 0,
              mainClientH: mainEl ? mainEl.clientHeight : 0,
              hasMain: Boolean(mainEl),
            };
          });

          assert(geom52.shellRows === 3, `.cockpit-shell must have 3 grid rows (auto auto minmax(0,1fr)), got ${geom52.shellRows}`);
          assert(geom52.safetyCount === 0, `Duplicate bottom .cockpit-safety banner must be removed (found ${geom52.safetyCount})`);
          assert(geom52.hudRect && geom52.gridRect, 'Missing .cockpit-hud-strip or .cockpit-grid');
          assert(geom52.hudRect.height <= 68, `HUD strip height ${geom52.hudRect.height}px exceeds compact single-bar <= 68px at ${vp.width}x${vp.height}`);
          assert(geom52.hudRect.bottom <= geom52.gridRect.top + 1, `HUD strip bottom (${geom52.hudRect.bottom}) overlaps .cockpit-grid top (${geom52.gridRect.top})`);
          assert(geom52.docScrollH <= geom52.innerH + 4, `Outer page overflow at ${vp.width}x${vp.height}: scrollHeight=${geom52.docScrollH} > innerHeight=${geom52.innerH}`);
          if (geom52.hasMain) {
            assert(
              geom52.mainScrollH <= geom52.mainClientH + 4,
              `.app-main/main vertical overflow at ${vp.width}x${vp.height}: scrollHeight=${geom52.mainScrollH} > clientHeight=${geom52.mainClientH}`,
            );
          }
          if (vp.width === 1440) h52 = geom52.hudRect.height;
        });
      }

      await withIsolatedPage(browser, baseUrl, idToName, { tickerCount: 2 }, { width: 1440, height: 900 }, async (page) => {
        await page.goto(`${baseUrl}/cockpit`, { waitUntil: 'networkidle' });
        await dismissOpenTourIfPresent(page);
        await page.waitForSelector('.cockpit-hud-strip');
        h2 = await page.evaluate(() => document.querySelector('.cockpit-hud-strip').getBoundingClientRect().height);
        assert(Math.abs(h52 - h2) <= 4, `HUD strip height drifted between 52 tickers (${h52}px) and 2 tickers (${h2}px)`);
      });

      await withIsolatedPage(browser, baseUrl, idToName, { tickerCount: 0 }, { width: 1440, height: 900 }, async (page) => {
        await page.goto(`${baseUrl}/cockpit`, { waitUntil: 'networkidle' });
        await dismissOpenTourIfPresent(page);
        await page.waitForSelector('.cockpit-hud-strip');
        const h0 = await page.evaluate(() => document.querySelector('.cockpit-hud-strip').getBoundingClientRect().height);
        assert(h0 <= 68, `HUD strip stretched on 0 tickers (${h0}px)`);
      });

      // GAP-2 & GAP-3: Tripped circuit breaker HUD geometry + .cockpit-group-list zero horizontal overflow (EN & FR at 1440x900 & 1280x768)
      for (const vp of [
        { width: 1440, height: 900 },
        { width: 1280, height: 768 },
      ]) {
        await withIsolatedPage(
          browser,
          baseUrl,
          idToName,
          {
            tickerCount: 12,
            longTickerNames: true,
            circuitBreakerTripped: true,
            globalEnabled: false,
            dailyPnl: -1450,
            dailyLossLimit: 1500,
          },
          vp,
          async (page) => {
            for (const lang of ['en', 'fr']) {
              await page.goto(`${baseUrl}/cockpit`, { waitUntil: 'networkidle' });
              await dismissOpenTourIfPresent(page);
              const langSelect = page.locator('.sidebar select:has(option[value="fr"])').first();
              await langSelect.selectOption(lang);
              await page.waitForTimeout(150);

              const overflowCheck = await page.evaluate(() => {
                const hud = document.querySelector('.cockpit-hud-strip');
                const resetBtn = document.querySelector('.hud-reset-btn');
                const groupList = document.querySelector('.cockpit-group-list');
                const hudRect = hud?.getBoundingClientRect();
                const resetRect = resetBtn?.getBoundingClientRect();
                return {
                  hudClientW: hud?.clientWidth || 0,
                  hudScrollW: hud?.scrollWidth || 0,
                  hudH: hudRect?.height || 0,
                  hudRight: hudRect?.right || 0,
                  resetRight: resetRect?.right || 0,
                  resetW: resetRect?.width || 0,
                  groupClientW: groupList?.clientWidth || 0,
                  groupScrollW: groupList?.scrollWidth || 0,
                };
              });

              assert(overflowCheck.hudH <= 68, `Tripped HUD height ${overflowCheck.hudH}px > 68 at ${vp.width}x${vp.height} (${lang})`);
              assert(
                overflowCheck.hudScrollW <= overflowCheck.hudClientW + 2,
                `Tripped HUD horizontal overflow at ${vp.width}x${vp.height} (${lang}): scrollW=${overflowCheck.hudScrollW} > clientW=${overflowCheck.hudClientW}`,
              );
              assert(
                overflowCheck.resetW > 0 && overflowCheck.resetRight <= overflowCheck.hudRight + 1,
                `HUD Reset button clipped off-screen at ${vp.width}x${vp.height} (${lang}): resetRight=${overflowCheck.resetRight} > hudRight=${overflowCheck.hudRight}`,
              );
              assert(
                overflowCheck.groupScrollW <= overflowCheck.groupClientW + 2,
                `.cockpit-group-list horizontal overflow at ${vp.width}x${vp.height} (${lang}): scrollW=${overflowCheck.groupScrollW} > clientW=${overflowCheck.groupClientW}`,
              );
            }
          },
        );
      }
    });

    // -----------------------------------------------------------------------
    // R2-ui: Disconnected $0.00 (No $100,000 Stub), State Update Push, $$$$, watchlist_id
    // -----------------------------------------------------------------------
    await testCase('R2_UI_ZeroEquityDisconnected_LivePush_Holdings_WatchlistID', async () => {
      await withIsolatedPage(browser, baseUrl, idToName, { twsConnected: false, equity: 0 }, { width: 1440, height: 900 }, async (page, mock) => {
        await page.goto(`${baseUrl}/cockpit`, { waitUntil: 'networkidle' });
        await dismissOpenTourIfPresent(page);
        await page.waitForSelector('.hud-equity .hud-value');
        const equityDisconnected = (await page.locator('.hud-equity .hud-value').innerText()).trim();
        assert(equityDisconnected === '$0.00', `Disconnected equity must render $0.00 (no $100,000 stub), got ${equityDisconnected}`);

        // Push live state_update event via Wails runtime event bus
        mock.state.twsConnected = true;
        mock.state.equity = 142850.5;
        await page.evaluate((next) => window._wails?.dispatchWailsEvent?.({ name: 'state_update', data: next }), mock.buildBotState());
        await page.waitForTimeout(100);
        const equityLive = (await page.locator('.hud-equity .hud-value').innerText()).trim();
        assert(equityLive.includes('142,850.50'), `Expected live pushed equity $142,850.50, got ${equityLive}`);

        // Assert $$$$ holding badge renders strictly on held rows (GOOGL, TSLA) and NOT on non-held rows
        const holdingCheck = await page.evaluate(() => {
          const rows = Array.from(document.querySelectorAll('.cockpit-ticker-row'));
          const heldHasBadge = [];
          const nonHeldHasBadge = [];
          for (const row of rows) {
            const text = (row.innerText || '').toUpperCase();
            const badges = Array.from(row.querySelectorAll('.holding-badge')).filter((b) => (b.innerText || '').includes('$$$$'));
            const hasBadge = badges.length > 0;
            if (text.includes('GOOGL') || text.includes('TSLA')) {
              heldHasBadge.push({ text: text.slice(0, 40), hasBadge });
            } else if (text.includes('SYM')) {
              nonHeldHasBadge.push({ text: text.slice(0, 40), hasBadge });
            }
          }
          return { rowCount: rows.length, heldHasBadge, nonHeldHasBadge };
        });
        assert(holdingCheck.rowCount >= 4, `Expected ticker rows on /cockpit, got ${holdingCheck.rowCount}`);
        assert(
          holdingCheck.heldHasBadge.length >= 2 && holdingCheck.heldHasBadge.every((r) => r.hasBadge),
          `Held rows (GOOGL/TSLA) must display .holding-badge $$$$: ${JSON.stringify(holdingCheck)}`,
        );
        assert(
          holdingCheck.nonHeldHasBadge.length > 0 && holdingCheck.nonHeldHasBadge.every((r) => !r.hasBadge),
          `Non-held rows must NOT display .holding-badge $$$$: ${JSON.stringify(holdingCheck)}`,
        );

        // Switch to cobalt-home-run pill and toggle first ticker -> ReplaceWatchlist must pass watchlist_id='cobalt-home-run'
        await page.getByRole('button', { name: /Home Run/i }).click();
        await page.waitForTimeout(100);
        await page.locator('.cockpit-ticker-row .ticker-toggle label.toggle').first().click();
        await page.waitForTimeout(100);
        const replaceCall = mock.ipcCalls.find((c) => c.methodName === 'ReplaceWatchlist');
        assert(replaceCall, 'Expected ReplaceWatchlist IPC call on ticker toggle');
        assert(replaceCall.args?.[0]?.watchlist_id === 'cobalt-home-run', `Expected watchlist_id 'cobalt-home-run', got ${JSON.stringify(replaceCall.args?.[0])}`);
      });
    });

    // -----------------------------------------------------------------------
    // R3: Global <select> Styling Across Routes, Single Status Badge, Collapsible Sidebar Chips
    // -----------------------------------------------------------------------
    await testCase('R3_GlobalSelectStyling_And_CollapsibleSidebar', async () => {
      // Check dark <select> styling across /cockpit, /watchlist, and /strategy
      await withIsolatedPage(browser, baseUrl, idToName, { twsConnected: true }, { width: 1440, height: 900 }, async (page) => {
        for (const route of ['/cockpit', '/watchlist', '/strategy']) {
          await page.goto(`${baseUrl}${route}`, { waitUntil: 'networkidle' });
          await dismissOpenTourIfPresent(page);
          await page.waitForSelector('select');

          const selectCheck = await page.evaluate(() => {
            const shellDisplay = getComputedStyle(document.querySelector('.app-shell')).display;
            const selects = Array.from(document.querySelectorAll('select'));
            return {
              shellDisplay,
              selectInfo: selects.map((s) => {
                const cs = getComputedStyle(s);
                return {
                  appearance: cs.appearance || cs.webkitAppearance,
                  bg: cs.backgroundColor,
                  bgImg: cs.backgroundImage,
                };
              }),
            };
          });
          assert(selectCheck.shellDisplay === 'grid', `.app-shell display on ${route} must be grid, got ${selectCheck.shellDisplay}`);
          assert(selectCheck.selectInfo.length > 0, `Expected <select> elements on ${route}`);
          for (const info of selectCheck.selectInfo) {
            assert(info.appearance === 'none', `<select> on ${route} must have appearance: none, got ${info.appearance}`);
            assert(info.bgImg && info.bgImg !== 'none', `<select> on ${route} must include custom SVG chevron background-image, got ${info.bgImg}`);
            const rgb = info.bg.match(/\d+/g)?.map(Number) || [255, 255, 255];
            assert(rgb[0] < 50 && rgb[1] < 50 && rgb[2] < 60, `<select> background on ${route} must be dark Uni Noir, got ${info.bg}`);
          }
        }

        // Check auto-collapse + single authoritative status badge + interactive expand/collapse on /cockpit
        await page.goto(`${baseUrl}/cockpit`, { waitUntil: 'networkidle' });
        await dismissOpenTourIfPresent(page);
        await page.waitForSelector('.sidebar select');

        const sidebarState = await page.evaluate(() => {
          function isEffectivelyVisible(el) {
            let cur = el;
            while (cur && cur !== document.body) {
              const cs = getComputedStyle(cur);
              if (cs.display === 'none' || cs.visibility === 'hidden' || parseFloat(cs.opacity) === 0) return false;
              const r = cur.getBoundingClientRect();
              if (r.width <= 0 || r.height <= 0) return false;
              if ((cs.overflow === 'hidden' || cs.overflowY === 'hidden') && (r.height <= 2 || cs.maxHeight === '0px')) {
                return false;
              }
              cur = cur.parentElement;
            }
            const elRect = el.getBoundingClientRect();
            let parent = el.parentElement;
            while (parent && parent !== document.body) {
              const pcs = getComputedStyle(parent);
              if (pcs.overflow === 'hidden' || pcs.overflowY === 'hidden') {
                const pr = parent.getBoundingClientRect();
                if (elRect.bottom <= pr.top + 1 || elRect.top >= pr.bottom - 1 || pr.height <= 2) {
                  return false;
                }
              }
              parent = parent.parentElement;
            }
            return true;
          }

          const sidebarText = document.querySelector('.sidebar')?.innerText || '';
          const visibleInputs = Array.from(document.querySelectorAll('.sidebar input[type="text"], .sidebar input[type="number"]'))
            .filter((el) => isEffectivelyVisible(el)).length;

          // V2-T1-F04-03: Single authoritative TWS/Bot status badge (no duplicate TWS Connected / Running clutter)
          const sidebarStatusRows = document.querySelectorAll('.sidebar .status-row').length;
          const statusBadges = document.querySelectorAll(
            '[data-testid="status-badge"], .status-badge, .auth-status-badge, .sidebar-status-badge',
          ).length;
          const dupTwsTextCount = Array.from(document.querySelectorAll('.sidebar *, .cockpit-shell *')).filter((el) => {
            if (el.children.length > 0) return false;
            if (el.closest('select, option, button')) return false;
            const txt = (el.textContent || '').trim();
            return /^(?:●\s*)?(?:TWS\s*)?(?:Connected|Connecté|Disconnected|Déconnecté|Broker connected|TWS linked)$/i.test(txt);
          }).length;

          return { sidebarText, visibleInputs, sidebarStatusRows, statusBadges, dupTwsTextCount };
        });

        assert(
          sidebarState.visibleInputs === 0,
          `Sidebar Broker & Risk sections must auto-collapse when tws_connected=true (found ${sidebarState.visibleInputs} effectively visible inputs)`,
        );
        assert(
          sidebarState.sidebarText.includes('7497') && sidebarState.sidebarText.includes('DUO991122'),
          `Collapsed Sidebar summary chips must show port/account (7497 & DUO991122), got: ${sidebarState.sidebarText}`,
        );
        assert(
          sidebarState.sidebarStatusRows <= 1 && sidebarState.dupTwsTextCount <= 1,
          `Duplicate TWS/Bot status indicators detected (sidebarStatusRows=${sidebarState.sidebarStatusRows}, dupTwsTextCount=${sidebarState.dupTwsTextCount}, statusBadges=${sidebarState.statusBadges})`,
        );

        // Interactive collapse/expand toggle click on Sidebar Broker or Risk section
        const toggleBtn = page
          .locator('.sidebar button, .sidebar [role="button"], .sidebar summary')
          .filter({ hasText: /connexion|connection|broker|portefeuille|portfolio|risque|risk|7497/i })
          .first();
        assert((await toggleBtn.count()) > 0, 'Missing interactive collapsible section toggle in Sidebar');
        await toggleBtn.click();
        await page.waitForTimeout(150);
        const expandedInputs = await page.evaluate(() => {
          return Array.from(document.querySelectorAll('.sidebar input[type="text"], .sidebar input[type="number"]')).filter((el) => {
            const cs = getComputedStyle(el);
            const r = el.getBoundingClientRect();
            return cs.display !== 'none' && cs.visibility !== 'hidden' && r.height > 0;
          }).length;
        });
        assert(expandedInputs > 0, 'Clicking collapsed Sidebar Broker/Risk toggle must expand its inputs');

        await toggleBtn.click();
        await page.waitForTimeout(150);
      });

      // Disconnected Sidebar (tws_connected: false) must keep Broker/Risk expanded
      await withIsolatedPage(browser, baseUrl, idToName, { twsConnected: false, equity: 0 }, { width: 1440, height: 900 }, async (page) => {
        await page.goto(`${baseUrl}/cockpit`, { waitUntil: 'networkidle' });
        await dismissOpenTourIfPresent(page);
        await page.waitForSelector('.sidebar');
        const visibleInputsDisconnected = await page.evaluate(() => {
          return Array.from(document.querySelectorAll('.sidebar input[type="text"], .sidebar input[type="number"]')).filter((el) => {
            const cs = getComputedStyle(el);
            const r = el.getBoundingClientRect();
            return cs.display !== 'none' && cs.visibility !== 'hidden' && r.height > 0;
          }).length;
        });
        assert(visibleInputsDisconnected > 0, 'Disconnected Sidebar (tws_connected: false) must keep Broker/Risk inputs expanded');
      });

      // GAP-3: Sidebar must not overflow horizontally with a 62-char active strategy name
      await withIsolatedPage(
        browser,
        baseUrl,
        idToName,
        {
          twsConnected: true,
          strategies: [
            {
              id: 'cours_weinstein_p2',
              name: 'Cobalt Weinstein Phase 2 Ultra Long Strategy Name For Layout Stress Test',
              rule_count: 3,
              enabled_rule_count: 3,
              source: 'preset',
              pinned: true,
              sort_order: 0,
              workspace: 'swing',
            },
          ],
        },
        { width: 1280, height: 768 },
        async (page) => {
          await page.goto(`${baseUrl}/cockpit`, { waitUntil: 'networkidle' });
          await dismissOpenTourIfPresent(page);
          await page.waitForSelector('.sidebar-status-panel');
          const sb = await page.evaluate(() => {
            const sidebar = document.querySelector('.sidebar');
            const pill = document.querySelector('.bot-state-pill');
            return {
              clientW: sidebar?.clientWidth || 0,
              scrollW: sidebar?.scrollWidth || 0,
              sidebarRight: sidebar?.getBoundingClientRect().right || 0,
              pillRight: pill?.getBoundingClientRect().right || 0,
            };
          });
          assert(sb.scrollW <= sb.clientW + 2, `Sidebar horizontal overflow on long strategy name: scrollW=${sb.scrollW} > clientW=${sb.clientW}`);
          assert(sb.pillRight <= sb.sidebarRight + 1, `.bot-state-pill clipped past sidebar right (${sb.pillRight} > ${sb.sidebarRight})`);
        },
      );
    });

    // -----------------------------------------------------------------------
    // R4-fe: Strategy Stack (No <select>), Pin/Reorder DOM Order, <78% vs >=78% Slide-to-Arm DOM Swap
    // -----------------------------------------------------------------------
    await testCase('R4_StrategyStack_PinReorder_And_SlideToArmThreshold', async () => {
      await withIsolatedPage(browser, baseUrl, idToName, { twsConnected: true }, { width: 1440, height: 900 }, async (page, mock) => {
        await page.goto(`${baseUrl}/cockpit`, { waitUntil: 'networkidle' });
        await dismissOpenTourIfPresent(page);
        await page.waitForSelector('.cockpit-slot-list');

        const selectCount = await page.locator('.cockpit-slot-list select').count();
        assert(selectCount === 0, `Cockpit Strategy column must NOT use <select> dropdown (found ${selectCount})`);

        const activeCardSelector =
          '[data-testid="active-strategy-card"], .active-strategy-card, .cockpit-active-strategy, .cockpit-slot-list > *:first-child';
        const beforeActiveText = await page.locator(activeCardSelector).first().innerText();
        assert(
          beforeActiveText.includes('Cobalt Weinstein Phase 2'),
          `Expected initial 1/1 Active Strategy Card to show 'Cobalt Weinstein Phase 2', got: ${beforeActiveText}`,
        );

        // Locate Slide-to-Arm track on a non-active queued strategy
        const rawSliderTracks = page.locator(
          [
            '[data-testid="slide-to-arm"]:not([aria-disabled="true"]):not(.disabled):not(.armed)',
            '.slide-to-arm-track:not([aria-disabled="true"]):not(.disabled):not(.armed)',
            '.slide-arm-track:not([aria-disabled="true"]):not(.disabled):not(.armed)',
            '.slide-track:not([aria-disabled="true"]):not(.disabled):not(.armed)',
            '[class*="slide"][class*="track"]:not([aria-disabled="true"]):not(.disabled):not(.armed)',
            '[role="slider"]:not([aria-disabled="true"]):not(.disabled):not(.armed)',
          ].join(', '),
        );
        const trackCount = await rawSliderTracks.count();
        assert(trackCount > 0, 'Missing Slide-to-Arm drag track in Cockpit Strategy Stack');
        let sliderTrack = rawSliderTracks.first();
        for (let idx = 0; idx < trackCount; idx++) {
          const cand = rawSliderTracks.nth(idx);
          const inActiveRow = await cand.evaluate((el) => {
            if (el.closest('[data-testid="active-strategy-card"], .active-strategy-card, .cockpit-active-strategy, [data-active="true"], .is-active')) {
              return true;
            }
            const row = el.closest('[data-strategy-id], .strategy-stack-item, .slot-row, li');
            return Boolean(row && (row.classList.contains('active') || row.getAttribute('data-strategy-id') === 'cours_weinstein_p2'));
          });
          if (!inActiveRow) {
            sliderTrack = cand;
            break;
          }
        }

        const box = await sliderTrack.boundingBox();
        assert(box && box.width >= 60, `Slide-to-Arm track boundingBox invalid: ${JSON.stringify(box)}`);

        const beforeArmCalls = mock.ipcCalls.filter((c) => c.methodName === 'ApplyStrategyPreset' || c.methodName === 'UpdateCockpitState').length;
        const beforeActiveId = mock.state.activeStrategyId;

        // Drag < 78% (50%) -> must snap back without arming
        const startX = box.x + 12;
        const midY = box.y + box.height / 2;
        await page.mouse.move(startX, midY);
        await page.mouse.down();
        await page.mouse.move(box.x + box.width * 0.5, midY, { steps: 8 });
        await page.mouse.up();
        await page.waitForTimeout(100);

        const afterSnapCalls = mock.ipcCalls.filter((c) => c.methodName === 'ApplyStrategyPreset' || c.methodName === 'UpdateCockpitState').length;
        assert(afterSnapCalls === beforeArmCalls, 'Slide-to-Arm drag at 50% (< 78%) must snap back WITHOUT arming strategy');
        assert(mock.state.activeStrategyId === beforeActiveId, '50% snap-back must not mutate activeStrategyId');

        // GAP-4 (1): Click at 90% without dragging >= 16px -> must NOT arm
        await page.mouse.click(box.x + box.width * 0.9, midY);
        await page.waitForTimeout(80);
        const afterShortClickCalls = mock.ipcCalls.filter((c) => c.methodName === 'ApplyStrategyPreset' || c.methodName === 'UpdateCockpitState').length;
        assert(afterShortClickCalls === beforeArmCalls, 'Slide-to-Arm click at 90% without >=16px drag must NOT arm');

        // GAP-4 (2): pointercancel after dragging to 90% -> must abort WITHOUT arming
        await sliderTrack.evaluate((el) => {
          const r = el.getBoundingClientRect();
          const y = r.top + r.height / 2;
          el.dispatchEvent(new PointerEvent('pointerdown', { bubbles: true, clientX: r.left + 10, clientY: y, button: 0, pointerType: 'touch' }));
          window.dispatchEvent(new PointerEvent('pointermove', { bubbles: true, clientX: r.left + r.width * 0.9, clientY: y, pointerType: 'touch' }));
          window.dispatchEvent(new PointerEvent('pointercancel', { bubbles: true, clientX: r.left + r.width * 0.9, clientY: y, pointerType: 'touch' }));
        });
        await page.waitForTimeout(100);
        const afterCancelCalls = mock.ipcCalls.filter((c) => c.methodName === 'ApplyStrategyPreset' || c.methodName === 'UpdateCockpitState').length;
        assert(afterCancelCalls === beforeArmCalls, 'Slide-to-Arm pointercancel at 90% must abort WITHOUT arming');

        // Drag >= 78% (90%) -> must arm & hot-swap strategy in mock state AND 1/1 DOM card
        await page.mouse.move(startX, midY);
        await page.mouse.down();
        await page.mouse.move(box.x + box.width * 0.9, midY, { steps: 10 });
        await page.mouse.up();
        await page.waitForTimeout(150);

        const afterArmCalls = mock.ipcCalls.filter((c) => c.methodName === 'ApplyStrategyPreset' || c.methodName === 'UpdateCockpitState').length;
        assert(afterArmCalls > beforeArmCalls, 'Slide-to-Arm drag at 90% (>= 78%) must invoke ApplyStrategyPreset/UpdateCockpitState');
        assert(mock.state.activeStrategyId !== beforeActiveId, 'Slide-to-Arm >= 78% must update mock.state.activeStrategyId');
        const armedObj = mock.state.strategies.find((s) => s.id === mock.state.activeStrategyId);
        const afterActiveText = await page.locator(activeCardSelector).first().innerText();
        assert(
          armedObj && afterActiveText.includes(armedObj.name),
          `Active Strategy Card DOM did not update to newly armed strategy ${armedObj?.name}; got: ${afterActiveText}`,
        );

        // Snapshot stack order in DOM before Pin & Move
        const getStackNames = () =>
          page.evaluate(() => {
            const rows = Array.from(
              document.querySelectorAll('.cockpit-slot-list [data-strategy-id], .cockpit-slot-list .strategy-stack-item, .cockpit-slot-list .slot-row, .cockpit-slot-list li, .cockpit-slot-list > div'),
            );
            return rows.map((r) => (r.getAttribute('data-strategy-id') || r.innerText || '').trim()).filter(Boolean);
          });

        const orderBeforePin = await getStackNames();
        const pinBtn = page
          .locator(
            'button[aria-label*="pin" i], button[title*="pin" i], button[aria-label*="épingl" i], button[title*="épingl" i], .strategy-pin-btn, button:has-text("📌")',
          )
          .last();
        assert((await pinBtn.count()) > 0, 'Missing Pin button on Strategy Stack');
        await pinBtn.click();
        await page.waitForTimeout(150);
        assert(mock.ipcCalls.some((c) => c.methodName === 'ToggleStrategyPin'), 'Pin click must invoke ToggleStrategyPin');
        const orderAfterPin = await getStackNames();
        assert(
          JSON.stringify(orderAfterPin) !== JSON.stringify(orderBeforePin),
          `Pin click must reorder DOM Strategy Stack (${JSON.stringify(orderBeforePin)} -> ${JSON.stringify(orderAfterPin)})`,
        );

        const moveSelector = [
          'button[aria-label*="move" i]',
          'button[title*="move" i]',
          'button[aria-label*="monter" i]',
          'button[title*="monter" i]',
          'button[aria-label*="descendre" i]',
          'button[title*="descendre" i]',
          'button[aria-label*="haut" i]',
          'button[title*="haut" i]',
          'button[aria-label*="bas" i]',
          'button[title*="bas" i]',
          'button[aria-label*="déplacer" i]',
          'button[title*="déplacer" i]',
          'button[aria-label*="up" i]',
          'button[title*="up" i]',
          'button[aria-label*="down" i]',
          'button[title*="down" i]',
          '.strategy-move-btn',
          'button:has-text("↑")',
          'button:has-text("↓")',
        ].join(', ');
        assert((await page.locator(moveSelector).count()) > 0, 'Missing ↑/↓ reorder button on Strategy Stack');
        // Click a down button on an earlier row (or the 2nd move button) so it's not a boundary no-op
        const downBtn = page
          .locator(
            'button[aria-label*="down" i], button[title*="down" i], button[aria-label*="descendre" i], button[title*="descendre" i], button[aria-label*="bas" i], button[title*="bas" i], button:has-text("↓")',
          )
          .first();
        const moveBtnToClick = (await downBtn.count()) > 0 ? downBtn : page.locator(moveSelector).nth(1);
        await moveBtnToClick.click();
        await page.waitForTimeout(150);
        assert(mock.ipcCalls.some((c) => c.methodName === 'MoveStrategyOrder'), 'Reorder click must invoke MoveStrategyOrder');
        const orderAfterMove = await getStackNames();
        assert(
          JSON.stringify(orderAfterMove) !== JSON.stringify(orderAfterPin),
          `Reorder click must update DOM Strategy Stack order (${JSON.stringify(orderAfterPin)} -> ${JSON.stringify(orderAfterMove)})`,
        );
      });

      // GAP-4 (3): While ApplyStrategyPreset is in-flight, sibling SlideToArm tracks must have aria-disabled="true"
      await withIsolatedPage(browser, baseUrl, idToName, { twsConnected: true, applyDelayMs: 350 }, { width: 1440, height: 900 }, async (page) => {
        await page.goto(`${baseUrl}/cockpit`, { waitUntil: 'networkidle' });
        await dismissOpenTourIfPresent(page);
        await page.waitForSelector('.slide-to-arm-track');
        const track = page.locator('.slide-to-arm-track:not([aria-disabled="true"])').first();
        const box = await track.boundingBox();
        await page.mouse.move(box.x + 12, box.y + box.height / 2);
        await page.mouse.down();
        await page.mouse.move(box.x + box.width * 0.9, box.y + box.height / 2, { steps: 6 });
        await page.mouse.up();
        await page.waitForTimeout(80);
        const disabledDuringSave = await page.evaluate(() => {
          const tracks = Array.from(document.querySelectorAll('.slide-to-arm-track'));
          return tracks.length > 0 && tracks.every((el) => el.getAttribute('aria-disabled') === 'true' && el.classList.contains('is-disabled'));
        });
        assert(disabledDuringSave, 'Sibling SlideToArm tracks must be disabled (aria-disabled="true") while ApplyStrategyPreset is saving');
        await page.waitForTimeout(400);
      });
    });

    // -----------------------------------------------------------------------
    // R5-fe: 4-Panel Monitoring Overhaul (Scoped to Main, Zero Dev Noise, Stopped Strategy, DryRun NVDA)
    // -----------------------------------------------------------------------
    await testCase('R5_Monitoring_4Panels_ZeroDevNoise_StoppedStrategy_DryRun', async () => {
      await withIsolatedPage(browser, baseUrl, idToName, { twsConnected: true, running: false }, { width: 1440, height: 900 }, async (page, mock) => {
        await page.goto(`${baseUrl}/monitoring`, { waitUntil: 'networkidle' });
        await dismissOpenTourIfPresent(page);
        await page.waitForTimeout(200);

        const monCheck = await page.evaluate(() => {
          const mainEl = document.querySelector('.monitoring-page, main, .app-main');
          const text = mainEl ? mainEl.innerText : '';
          const panelNodes = mainEl
            ? Array.from(mainEl.querySelectorAll('[data-testid^="monitoring-panel"], .monitoring-panel, .panel, .card, section')).filter(
                (el) => {
                  const r = el.getBoundingClientRect();
                  return r.width > 120 && r.height > 60;
                },
              )
            : [];
          return { text, panelCount: panelNodes.length };
        });

        assert(monCheck.panelCount >= 4, `Monitoring must render >= 4 distinct trader panels/cards in main area, found ${monCheck.panelCount}`);

        const lowerText = monCheck.text.toLowerCase();
        const forbidden = ['tws_traderbot.sqlite', 'local_sqlite', 'desktop_local', 'tws_traderbot.log', 'logs: info'];
        for (const bad of forbidden) {
          assert(!lowerText.includes(bad), `Monitoring page leaks legacy dev noise "${bad}"`);
        }
        assert(monCheck.text.includes('Cobalt Weinstein Phase 2'), 'Monitoring main view must display armed Cockpit strategy even when STOPPED/paused');
        assert(monCheck.text.includes('USD') && monCheck.text.includes('CAD'), 'Monitoring Panel 1 must display USD & CAD currency readiness');
        assert(monCheck.text.includes('GOOGL') && monCheck.text.includes('TSLA'), 'Monitoring Panel 2 must display live open positions');
        assert(monCheck.text.includes('10089') && monCheck.text.includes('RTH'), 'Monitoring Panel 4 must display RTH/ETH session status & 10089 alert');

        const dryRunBtn = page.getByRole('button', { name: /dry.?run|simul|scan/i }).first();
        assert((await dryRunBtn.count()) > 0, 'Monitoring Panel 3 missing Dry-Run scanner button');
        await dryRunBtn.click();
        await page.waitForTimeout(200);
        assert(mock.ipcCalls.some((c) => c.methodName === 'DryRunBot'), 'Clicking Dry-Run scanner must invoke DryRunBot');

        const mainAfterDryRun = await page.evaluate(() => {
          const mainEl = document.querySelector('.monitoring-page, main, .app-main');
          return mainEl ? mainEl.innerText : '';
        });
        assert(mainAfterDryRun.includes('NVDA'), 'Monitoring Panel 3 must display NVDA planned order from DryRunBot output');
      });
    });

    // -----------------------------------------------------------------------
    // R6-fe: Cobalt Emblem in .sidebar-brand (inline <svg> OR rendered <img>)
    // -----------------------------------------------------------------------
    await testCase('R6_Sidebar_CobaltSVGEmblem', async () => {
      await withIsolatedPage(browser, baseUrl, idToName, {}, { width: 1440, height: 900 }, async (page) => {
        await page.goto(`${baseUrl}/cockpit`, { waitUntil: 'networkidle' });
        await dismissOpenTourIfPresent(page);
        const emblemInfo = await page.evaluate(() => {
          const svg = document.querySelector('.sidebar-brand svg');
          if (svg) {
            const r = svg.getBoundingClientRect();
            if (r.width > 0 && r.height > 0) {
              return { kind: 'svg', width: r.width, height: r.height, html: svg.outerHTML.toLowerCase() };
            }
          }
          const img = document.querySelector('.sidebar-brand img');
          if (img) {
            const r = img.getBoundingClientRect();
            return {
              kind: 'img',
              width: r.width,
              height: r.height,
              complete: img.complete,
              naturalWidth: img.naturalWidth,
              src: img.getAttribute('src') || '',
            };
          }
          return null;
        });

        assert(emblemInfo, 'Sidebar .sidebar-brand missing rendered Cobalt <svg> or <img> emblem');
        if (emblemInfo.kind === 'svg') {
          assert(emblemInfo.width > 0 && emblemInfo.height > 0, 'Sidebar .sidebar-brand <svg> has zero dimensions');
          assert(!emblemInfo.html.includes('#ff3e00'), 'Sidebar emblem must not contain Svelte stock #ff3e00');
        } else {
          assert(
            emblemInfo.complete && emblemInfo.naturalWidth > 0 && emblemInfo.src.length > 0,
            `Sidebar .sidebar-brand <img> failed to load: ${JSON.stringify(emblemInfo)}`,
          );
        }
      });
    });

    // -----------------------------------------------------------------------
    // R7-fe: Zero-Scroll 3-Pane Strategy Builder Workbench (Search, Filter Pills, JSON Drawer, 22 indicators, 9 timeframes)
    // -----------------------------------------------------------------------
    await testCase('R7_Strategy_3PaneZeroScroll_22Indicators_9Timeframes', async () => {
      for (const vp of [
        { width: 1440, height: 900 },
        { width: 1280, height: 768 },
      ]) {
        await withIsolatedPage(browser, baseUrl, idToName, {}, vp, async (page, mock) => {
          await page.goto(`${baseUrl}/strategy`, { waitUntil: 'networkidle' });
          await dismissOpenTourIfPresent(page);
          await page.waitForTimeout(200);

          const layout = await page.evaluate(() => {
            const mainEl = document.querySelector('.app-main, main');
            const docScrollH = document.documentElement.scrollHeight;
            const innerH = window.innerHeight;
            const mainScrollH = mainEl ? mainEl.scrollHeight : 0;
            const mainClientH = mainEl ? mainEl.clientHeight : 0;
            let panes = Array.from(document.querySelectorAll('.strategy-workbench > *, [data-testid^="strategy-pane"]'))
              .map((el) => el.getBoundingClientRect())
              .filter((r) => r.width > 120 && r.height > 200);
            if (panes.length < 3) {
              const containers = Array.from(document.querySelectorAll('main div, main section, .app-content > *'));
              for (const c of containers) {
                const kids = Array.from(c.children)
                  .map((el) => el.getBoundingClientRect())
                  .filter((r) => r.width > 120 && r.height > 250)
                  .sort((a, b) => a.left - b.left);
                if (kids.length >= 3 && kids[0].right <= kids[1].left + 16 && kids[1].right <= kids[2].left + 16) {
                  panes = kids;
                  break;
                }
              }
            }

            const optionValues = Array.from(document.querySelectorAll('select option')).map((o) => o.value);
            return { docScrollH, innerH, mainScrollH, mainClientH, hasMain: Boolean(mainEl), panes, optionValues };
          });

          assert(layout.docScrollH <= layout.innerH + 4, `/strategy has outer scroll overflow at ${vp.width}x${vp.height}: ${layout.docScrollH} > ${layout.innerH}`);
          if (layout.hasMain) {
            assert(
              layout.mainScrollH <= layout.mainClientH + 4,
              `/strategy .app-main/main vertical overflow at ${vp.width}x${vp.height}: ${layout.mainScrollH} > ${layout.mainClientH}`,
            );
          }
          assert(layout.panes.length >= 3, `/strategy must render 3-pane workbench (.strategy-workbench > 3 panes), found ${layout.panes.length}`);
          const [p1, p2, p3] = layout.panes;
          assert(p1.right <= p2.left + 8 && p2.right <= p3.left + 8, `/strategy panes must be side-by-side horizontally`);

          const expectedTimeframes = ['1m', '5m', '15m', '30m', '1h', '4h', '1d', '1w', '1M'];
          for (const tf of expectedTimeframes) {
            assert(layout.optionValues.includes(tf), `/strategy Rule Inspector missing timeframe ${tf}`);
          }
          const expectedIndicators = [
            'price', 'volume', 'ema', 'sma', 'rsi', 'macd', 'vix', 'time', 'bollinger', 'stochastic', 'obv',
            'alligator', 'heikin_ashi', 'rvol', 'guppy', 'atr', 'relative_strength', 'high_52w',
            'dividend_yield', 'pe_ratio', 'relative_performance', 'ml_signal',
          ];
          for (const ind of expectedIndicators) {
            assert(layout.optionValues.includes(ind), `/strategy Rule Inspector missing indicator ${ind}`);
          }

          if (vp.width === 1440) {
            // Test Preset Stack search & filter pills
            const searchInput = page
              .locator('main input[type="search"], main input[placeholder*="recherch" i], main input[placeholder*="search" i], main input[placeholder*="filtr" i]')
              .first();
            assert((await searchInput.count()) > 0, '/strategy Left Preset Stack missing search input');
            await searchInput.fill('Weinstein');
            await page.waitForTimeout(100);
            const filteredText = await page.locator('main').innerText();
            assert(filteredText.includes('Cobalt Weinstein Phase 2'), '/strategy search for Weinstein must show Cobalt Weinstein Phase 2');
            await searchInput.fill('');
            await page.waitForTimeout(100);

            const pillSelectors = [/^(tous|all)$/i, /achat/i, /short/i, /swing/i, /📌/];
            for (const pillRe of pillSelectors) {
              const pill = page.locator('main button').filter({ hasText: pillRe }).first();
              assert((await pill.count()) > 0, `/strategy Preset Stack missing filter pill matching ${pillRe}`);
            }
            const swingPill = page.locator('main button').filter({ hasText: /swing/i }).first();
            await swingPill.click();
            await page.waitForTimeout(100);
            const allPill = page.locator('main button').filter({ hasText: /^(tous|all)$/i }).first();
            await allPill.click();
            await page.waitForTimeout(100);

            // Test JSON Drawer open/display
            const jsonBtn = page.locator('main button').filter({ hasText: /json/i }).first();
            assert((await jsonBtn.count()) > 0, '/strategy missing JSON Drawer toggle button');
            await jsonBtn.click();
            await page.waitForTimeout(150);
            const jsonVisible = await page.evaluate(() => {
              const els = Array.from(document.querySelectorAll('pre, textarea, code, [class*="json"]'));
              return els.some((el) => {
                const cs = getComputedStyle(el);
                const r = el.getBoundingClientRect();
                const txt = (el.value ?? el.textContent ?? '');
                return cs.display !== 'none' && cs.visibility !== 'hidden' && r.width > 50 && r.height > 12 && txt.includes('"rules"');
              });
            });
            assert(jsonVisible, '/strategy JSON Drawer must open and render strategy JSON ("rules")');
            await jsonBtn.click().catch(() => {});
            await page.waitForTimeout(100);
          }

          const validateBtn = page.getByRole('button', { name: /valid/i }).first();
          await validateBtn.click();
          await page.waitForTimeout(100);
          assert(mock.ipcCalls.some((c) => c.methodName === 'ValidateStrategy'), 'Validate button must invoke ValidateStrategy');

          // GAP-5: 1280x768 FR zero-scroll + JSON drawer invalid JSON error visibility + Escape precedence
          if (vp.width === 1280) {
            const langSelect = page.locator('.sidebar select:has(option[value="fr"])').first();
            await langSelect.selectOption('fr');
            await page.waitForTimeout(120);
            const frOverflow = await page.evaluate(() => ({
              docScrollH: document.documentElement.scrollHeight,
              innerH: window.innerHeight,
              docScrollW: document.documentElement.scrollWidth,
              innerW: window.innerWidth,
            }));
            assert(frOverflow.docScrollH <= frOverflow.innerH + 4, `/strategy FR 1280x768 vertical overflow: ${frOverflow.docScrollH} > ${frOverflow.innerH}`);
            assert(frOverflow.docScrollW <= frOverflow.innerW + 2, `/strategy FR 1280x768 horizontal overflow: ${frOverflow.docScrollW} > ${frOverflow.innerW}`);

            const jsonBtn = page.locator('main button').filter({ hasText: /json/i }).first();
            await jsonBtn.click();
            await page.waitForTimeout(120);
            await page.locator('.json-drawer-textarea').fill('{bad');
            await page.locator('.json-drawer button').filter({ hasText: /appliquer|apply/i }).first().click();
            await page.waitForTimeout(100);

            const alertVisible = await page.evaluate(() => {
              const alert = document.querySelector('.json-drawer .json-drawer-alert');
              if (!alert) return false;
              const r = alert.getBoundingClientRect();
              return r.width > 50 && r.height > 10 && (alert.textContent || '').trim().length > 0;
            });
            assert(alertVisible, 'Invalid JSON error alert must be visibly rendered inside .json-drawer (not occluded)');

            // Open HelpDrawer over JSON drawer -> first Escape must close ONLY HelpDrawer, second Escape closes JSON drawer
            await page.keyboard.press('F1');
            await page.waitForTimeout(120);
            assert(await isVisibleInViewport(page, HELP_DRAWER_SELECTOR), 'F1 should open HelpDrawer over JSON drawer');
            await page.keyboard.press('Escape');
            await page.waitForTimeout(120);
            assert(!(await isVisibleInViewport(page, HELP_DRAWER_SELECTOR)), 'First Escape must close HelpDrawer');
            assert(await isVisibleInViewport(page, '.json-drawer'), 'First Escape must keep .json-drawer open under HelpDrawer');
            await page.keyboard.press('Escape');
            await page.waitForTimeout(120);
            assert(!(await isVisibleInViewport(page, '.json-drawer')), 'Second Escape must close .json-drawer');
          }
        });
      }
    });

    // -----------------------------------------------------------------------
    // R8: F1/? Help Drawer, 3-Track Spotlight Tour (5 Stops + localStorage), /manual Launchers
    // -----------------------------------------------------------------------
    await testCase('R8_HelpDrawer_3TrackTour_And_ManualLaunchers', async () => {
      // Strictly fresh context (seedTourDismissed: false) for first-launch tour & localStorage persistence
      await withIsolatedPage(
        browser,
        baseUrl,
        idToName,
        {},
        { width: 1440, height: 900 },
        async (page) => {
          await page.goto(`${baseUrl}/cockpit`, { waitUntil: 'networkidle' });
          await page.waitForTimeout(250);

          // First launch must show Spotlight Onboarding Tour visibly in viewport
          assert(await isVisibleInViewport(page, TOUR_SELECTOR), 'Fresh launch (empty localStorage) must offer/open Spotlight Onboarding Tour');

          // GAP-6 (2): F1 while OnboardingTour is open must NOT open HelpDrawer behind the tour
          await page.keyboard.press('F1');
          await page.waitForTimeout(120);
          assert(!(await isVisibleInViewport(page, HELP_DRAWER_SELECTOR)), 'F1 must not open HelpDrawer behind active OnboardingTour');

          // V2-T1-F09-04: Step through Track 1's 5 Spotlight stops
          const tourLoc = page.locator(TOUR_SELECTOR).first();
          const startTrack1Btn = tourLoc.locator('button').filter({ hasText: /60|fast|en piste|start|démarr|track\s*1|piste\s*1/i }).first();
          if ((await startTrack1Btn.count()) > 0) {
            await startTrack1Btn.click();
            await page.waitForTimeout(150);
          }

          const seenStops = new Set();
          seenStops.add((await tourLoc.innerText()).trim());
          for (let step = 0; step < 4; step++) {
            const nextBtn = tourLoc.locator('button').filter({ hasText: /next|suivant|continue|étape/i }).first();
            if ((await nextBtn.count()) === 0) break;
            await nextBtn.click();
            await page.waitForTimeout(150);
            if (await isVisibleInViewport(page, TOUR_SELECTOR)) {
              seenStops.add((await tourLoc.innerText()).trim());
            }
          }
          assert(seenStops.size >= 5, `Track 1 Spotlight Tour must step through 5 distinct stops, observed ${seenStops.size}`);

          // Complete or dismiss tour and verify localStorage persistence across reload
          const finishOrSkip = tourLoc.locator('button').filter({ hasText: /terminer|finish|done|skip|passer|fermer|close/i }).first();
          if ((await finishOrSkip.count()) > 0) {
            await finishOrSkip.click();
          } else {
            await page.keyboard.press('Escape');
          }
          await page.waitForTimeout(150);

          const lsKeys = await page.evaluate(() => Object.keys(localStorage));
          assert(lsKeys.length > 0, 'Tour completion/dismissal must persist state in localStorage');

          await page.reload({ waitUntil: 'networkidle' });
          await page.waitForTimeout(200);
          const reopenedVisible = await isVisibleInViewport(page, TOUR_SELECTOR);
          assert(!reopenedVisible, 'Reloading after tour completion/dismissal must NOT auto-reopen Spotlight Tour');

          // F1 key opens right-hand Slide-Over Help Drawer on /cockpit
          await page.keyboard.press('F1');
          await page.waitForTimeout(150);
          assert(await isVisibleInViewport(page, HELP_DRAWER_SELECTOR), 'Pressing F1 must open right-hand Context Help Drawer');
          const drawerBox = await page.evaluate((sel) => {
            const els = Array.from(document.querySelectorAll(sel));
            const el = els.find((cand) => {
              const r = cand.getBoundingClientRect();
              return r.width > 0 && r.height > 0;
            });
            return el ? { rect: el.getBoundingClientRect().toJSON(), text: el.innerText } : null;
          }, HELP_DRAWER_SELECTOR);
          assert(drawerBox && drawerBox.rect.width > 200, 'Help Drawer bounding rect invalid after F1');
          assert(drawerBox.rect.x > 1440 * 0.4, `Help Drawer must be anchored on the right side (x=${drawerBox.rect.x})`);
          assert(drawerBox.text.includes('10089') && drawerBox.text.includes('FXCONV'), 'Help Drawer must explain 10089 and FXCONV quick fixes');

          await page.keyboard.press('Escape');
          await page.waitForTimeout(150);
          assert(!(await isVisibleInViewport(page, HELP_DRAWER_SELECTOR)), 'Pressing Escape must close Help Drawer (not remain visible)');

          // Cockpit '?' button opens Help Drawer (same selector union)
          const helpBtn = page.locator('button[aria-label*="help" i], button[aria-label*="aide" i], button:has-text("?")').first();
          assert((await helpBtn.count()) > 0, 'Cockpit header missing ? Help Drawer button');
          await helpBtn.click();
          await page.waitForTimeout(150);
          assert(await isVisibleInViewport(page, HELP_DRAWER_SELECTOR), 'Clicking ? button must open Help Drawer');
          await page.keyboard.press('Escape');
          await page.waitForTimeout(100);

          // GAP-6 (1 & 2b): Corrupt localStorage recovery + F1 inside focused <input>
          await page.evaluate(() => localStorage.setItem('cobalt_onboarding_state', '{corrupt'));
          await page.reload({ waitUntil: 'networkidle' });
          await page.waitForTimeout(200);
          assert(await isVisibleInViewport(page, TOUR_SELECTOR), 'Corrupt localStorage must recover cleanly to welcome Spotlight Tour');
          await dismissOpenTourIfPresent(page);
          const filterInput = page.locator('.cockpit-filter-input input').first();
          await filterInput.focus();
          await page.keyboard.press('F1');
          await page.waitForTimeout(120);
          assert(await isVisibleInViewport(page, HELP_DRAWER_SELECTOR), 'Pressing F1 while an <input> is focused must open HelpDrawer');
          await page.keyboard.press('Escape');
          await page.waitForTimeout(100);

          // /manual 1-click launchers for Track 1, 2, 3 and active launch verification
          await page.goto(`${baseUrl}/manual`, { waitUntil: 'networkidle' });
          await page.waitForTimeout(200);
          const launcherSelector = 'main button';
          const launcherFilter = /tour|visite|piste|track|lancer|launch|en piste/i;
          const launcherBtns = page.locator(launcherSelector).filter({ hasText: launcherFilter });
          const launcherCount = await launcherBtns.count();
          assert(launcherCount >= 3, `/manual must provide 1-click launchers for Tracks 1, 2, and 3 (found ${launcherCount})`);

          for (let trackIdx = 0; trackIdx < 3; trackIdx++) {
            const btn = page.locator(launcherSelector).filter({ hasText: launcherFilter }).nth(trackIdx);
            assert((await btn.count()) > 0, `/manual missing Track ${trackIdx + 1} launcher button`);
            await btn.click();
            await page.waitForTimeout(250);
            const tourLaunchedFromManual = await isVisibleInViewport(page, TOUR_SELECTOR);
            assert(tourLaunchedFromManual, `Clicking Track ${trackIdx + 1} launcher on /manual must open Spotlight Tour cleanly`);
            await dismissOpenTourIfPresent(page);
            if (trackIdx < 2) {
              await page.goto(`${baseUrl}/manual`, { waitUntil: 'networkidle' });
              await page.waitForTimeout(150);
            }
          }
        },
        { seedTourDismissed: false },
      );
    });

    // -----------------------------------------------------------------------
    // R9-fe: Full EN <-> FR Runtime i18n Sweep (Actual Delta + Zero Leaked Keys Across 5 Routes, HelpDrawer & Tour)
    // -----------------------------------------------------------------------
    await testCase('R9_Bilingual_EN_FR_Sweep_ZeroLeakedKeys', async () => {
      const captured = { en: {}, fr: {} };

      await withIsolatedPage(browser, baseUrl, idToName, {}, { width: 1440, height: 900 }, async (page) => {
        for (const lang of ['en', 'fr']) {
          for (const route of ['/cockpit', '/monitoring', '/watchlist', '/strategy', '/manual']) {
            await page.goto(`${baseUrl}${route}`, { waitUntil: 'networkidle' });
            await dismissOpenTourIfPresent(page);
            const langSelect = page.locator('.sidebar select:has(option[value="fr"])').first();
            assert((await langSelect.count()) > 0, 'Sidebar missing language <select> with option[value="fr"]');
            await langSelect.selectOption(lang);
            await page.waitForTimeout(150);

            const bodyText = await page.evaluate(() => document.body.innerText.trim());
            const leaked = bodyText.match(LEAKED_I18N_KEY_RE);
            assert(!leaked, `Untranslated i18n key(s) leaked on ${route} (${lang}): ${leaked?.join(', ')}`);
            const mainText = await page.evaluate(
              () => document.querySelector('.app-main, main')?.innerText.trim() || document.body.innerText.trim(),
            );
            captured[lang][route] = mainText;

            if (route === '/cockpit') {
              await page.keyboard.press('F1');
              await page.waitForTimeout(150);
              const drawerText = await page.evaluate((sel) => {
                const el = Array.from(document.querySelectorAll(sel)).find((c) => {
                  const r = c.getBoundingClientRect();
                  return r.width > 0 && r.height > 0;
                });
                return el ? el.innerText.trim() : '';
              }, HELP_DRAWER_SELECTOR);
              if (drawerText) {
                const drawerLeaked = drawerText.match(LEAKED_I18N_KEY_RE);
                assert(!drawerLeaked, `Untranslated i18n key(s) leaked in HelpDrawer (${lang}): ${drawerLeaked?.join(', ')}`);
                captured[lang].helpDrawer = drawerText;
              }
              await page.keyboard.press('Escape');
              await page.waitForTimeout(100);
            }
          }
        }
      });

      // Sweep OnboardingTour in both EN and FR in a fresh context
      await withIsolatedPage(
        browser,
        baseUrl,
        idToName,
        {},
        { width: 1440, height: 900 },
        async (page) => {
          await page.goto(`${baseUrl}/cockpit`, { waitUntil: 'networkidle' });
          await page.waitForTimeout(200);
          for (const lang of ['en', 'fr']) {
            const langSelect = page.locator('.sidebar select:has(option[value="fr"])').first();
            if ((await langSelect.count()) > 0) {
              await langSelect.selectOption(lang, { force: true });
              await page.waitForTimeout(150);
            }
            const tourText = await page.evaluate((sel) => {
              const el = Array.from(document.querySelectorAll(sel)).find((c) => {
                const r = c.getBoundingClientRect();
                return r.width > 0 && r.height > 0;
              });
              return el ? el.innerText.trim() : '';
            }, TOUR_SELECTOR);
            if (tourText) {
              const tourLeaked = tourText.match(LEAKED_I18N_KEY_RE);
              assert(!tourLeaked, `Untranslated i18n key(s) leaked in OnboardingTour (${lang}): ${tourLeaked?.join(', ')}`);
              captured[lang].tour = tourText;
            }
          }
        },
        { seedTourDismissed: false },
      );

      for (const route of ['/cockpit', '/monitoring', '/watchlist', '/strategy', '/manual']) {
        assert(
          captured.en[route] && captured.fr[route] && captured.en[route] !== captured.fr[route],
          `EN vs FR rendered text did not change on ${route}`,
        );
      }
      if (captured.en.helpDrawer || captured.fr.helpDrawer) {
        assert(
          Boolean(captured.en.helpDrawer) && Boolean(captured.fr.helpDrawer),
          `HelpDrawer must render non-empty text in both EN and FR (en=${Boolean(captured.en.helpDrawer)}, fr=${Boolean(captured.fr.helpDrawer)})`,
        );
        assert(
          captured.en.helpDrawer !== captured.fr.helpDrawer,
          'EN vs FR rendered text did not change inside HelpDrawer',
        );
      }
      if (captured.en.tour || captured.fr.tour) {
        assert(
          Boolean(captured.en.tour) && Boolean(captured.fr.tour),
          `OnboardingTour must render non-empty text in both EN and FR (en=${Boolean(captured.en.tour)}, fr=${Boolean(captured.fr.tour)})`,
        );
        assert(
          captured.en.tour !== captured.fr.tour,
          'EN vs FR rendered text did not change inside OnboardingTour',
        );
      }

      // GAP-6 (3): Switching EN <-> FR must preserve in-memory route state (no {#key $language} unmount)
      await withIsolatedPage(browser, baseUrl, idToName, {}, { width: 1440, height: 900 }, async (page) => {
        await page.goto(`${baseUrl}/cockpit`, { waitUntil: 'networkidle' });
        await dismissOpenTourIfPresent(page);
        await page.getByRole('button', { name: /Home Run/i }).click();
        await page.waitForTimeout(100);
        const langSelect = page.locator('.sidebar select:has(option[value="fr"])').first();
        await langSelect.selectOption('fr');
        await page.waitForTimeout(150);
        const homeRunStillActive = await page.locator('.cockpit-pill.active-pill').innerText();
        assert(homeRunStillActive.includes('Home Run'), `Switching EN->FR reset selected watchlist on /cockpit: got ${homeRunStillActive}`);

        await page.goto(`${baseUrl}/strategy`, { waitUntil: 'networkidle' });
        await dismissOpenTourIfPresent(page);
        await page.waitForSelector('.strategy-name-input');
        await page.locator('.strategy-name-input').fill('Unsaved Cobalt Draft');
        await langSelect.selectOption('en');
        await page.waitForTimeout(150);
        const preservedName = await page.locator('.strategy-name-input').inputValue();
        assert(preservedName === 'Unsaved Cobalt Draft', `Switching FR->EN wiped unsaved strategy name edit: got ${preservedName}`);
      });
    });
  } finally {
    await browser.close();
    server.close();
  }

  const failed = results.filter((r) => !r.passed);
  console.log(`\nSummary: ${results.length - failed.length}/${results.length} UI checks passed.`);
  if (failed.length > 0) {
    process.exitCode = 1;
  }
}

run().catch((err) => {
  console.error('Fatal UI verification error:', err);
  process.exit(1);
});

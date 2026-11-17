# Cobalt Brand Identity, Design System & Desktop UI Guide (`02_present_state`)

> **Navigation:** [Master Index](../00_index.md) · [System Architecture](./system_architecture.md) · [Engineering Playbook](../05_engineering_and_agents/engineering_playbook.md)  
> **Status:** `Active`  
> **Canonical Brand Reference:** `https://www.cobaltinvestissements.com/` harmonized with `tws_traderbot` `Uni Noir` workstation ergonomics.

---

## 1. Canonical Brand Alignment (`cobaltinvestissements.com` × `Uni Noir`)

`tws_traderbot` unifies the institutional identity of **Cobalt Investissements** (`https://www.cobaltinvestissements.com/`) with a low-glare, high-density **`Uni Noir`** desktop trading workstation palette:

| Token Role | `cobaltinvestissements.com` Canonical | Harmonized `tws_traderbot` (`Uni Noir`) | Operational Usage |
|---|---|---|---|
| **Royal Cobalt Primary** | `--cobalt-blue: #263c8a` (`#20409A` crest) | `--cobalt-brand: #263c8a` / `--cobalt-electric: #1D63ED` | `#263c8a` for brand borders, emblem rims, and institutional headers; `#1D63ED` for primary interactive CTAs and active tabs |
| **Midnight Navy Anchor** | `--cobalt-blue-dark: #0e0c28` | `--bg-navy-deep: #0e0c28` / `--bg-surface: #0B111E` | Sidebar navigation anchor (`#0e0c28` / `#0B111E`) and brand squircle gradient mid-stop |
| **Uni Noir Workstation Void** | `--content-bg: #1a1a2e` | `--bg-void: #05070B` | Main ultra-dense HUD strip, telemetry rows, and darkest canvas depth |
| **Card & Panel Surface** | `--content-card-bg: #1c1c1c` | `--bg-card: #0F1726` / `--bg-card-elevated: #141E30` | Panel containers (`1px solid #1E293B` subtle border; `#263c8a` or `#38BDF8` active focus border) |
| **Muted Slate-Cobalt** | `--cobalt-blue-light: #7a80b4` | `--text-slate-cobalt: #7A80B4` / `--text-secondary: #94A3B8` | Inactive icons, metadata captions, secondary labels |
| **Cobalt Electric Cyan** | `--link-color: #8ab4f8` | `--cobalt-cyan: #38BDF8` / `--cobalt-link: #8AB4F8` | Active focus outlines, telemetry highlights, `1-Blue Chip` tier badges |
| **Cobalt Orange Accent** | `--cobalt-orange: #faa031` (`#fdcd92` hover, `#d06e1e` active) | `--cobalt-orange: #FAA031` / `--cobalt-orange-light: #FDCD92` / `--cobalt-amber-dark: #D06E1E` | Active section dividers, `📌` pinned strategy state, Slide-to-Arm accent, emblem diamond, `3-Spéculatif` tier |
| **Gold Pastille** | `--cobalt-pastille: #edc44e` (`#e9c6678e` border) | `--cobalt-gold: #EDC44E` | Starred presets, key status callouts |
| **Telemetry Positive** | `--metric-positive: #4caf50` | `--sem-bull: #10B981` (`#4CAF50` / `#34D399` text) | Positive P&L, TWS Connected, Armed Breaker, `$$$$` live holdings badge |
| **Telemetry Negative** | `--metric-negative: #ef5350` | `--sem-bear: #EF4444` (`#EF5350` / `#F87171` text) | Negative P&L, Tripped Loss Breaker, Disconnected, Danger CTAs |
| **Telemetry Warning** | `--metric-warning: #ffb74d` | `--sem-warn: #FAA031` (`#FFB74D` / `#FBBF24` text) | Extended Trading Hours (`ETH`), `10089`/`504` broker alerts, near-breach gauge |

---

## 2. Canonical `:root` CSS Custom Properties Contract (`frontend/src/app.css`)

All styling in [`frontend/src/app.css`](../../frontend/src/app.css) must derive from these `:root` tokens rather than ad-hoc hex values:

```css
:root {
	color-scheme: dark;

	/* Brand Identity (cobaltinvestissements.com) */
	--cobalt-brand: #263c8a;
	--cobalt-navy: #0e0c28;
	--cobalt-slate: #7a80b4;
	--cobalt-orange: #faa031;
	--cobalt-orange-light: #fdcd92;
	--cobalt-amber-dark: #d06e1e;
	--cobalt-gold: #edc44e;
	--cobalt-gold-border: #e9c6678e;

	/* Uni Noir Workstation Surfaces & Borders */
	--bg-void: #05070b;
	--bg-surface: #0b111e;
	--bg-card: #0f1726;
	--bg-card-elevated: #141e30;
	--bg-input: #0b1320;
	--border-subtle: #1f2937;
	--border-highlight: #2f3b52;

	/* Interactive & Text */
	--cobalt-electric: #1d63ed;
	--cobalt-cyan: #38bdf8;
	--cobalt-link: #8ab4f8;
	--text-primary: #e6edf3;
	--text-secondary: #94a3b8;
	--text-muted: #64748b;

	/* Semantic Telemetry & Method 8.8 Tiers */
	--sem-bull: #10b981;
	--sem-bull-text: #4caf50;
	--sem-bear: #ef4444;
	--sem-bear-text: #ef5350;
	--sem-warn: #faa031;
	--sem-warn-text: #ffb74d;
	--tier-1-bluechip: #38bdf8;
	--tier-2-growth: #c084fc;
	--tier-3-speculative: #faa031;

	/* Typography (Offline-Safe) */
	--font-sans: 'Inter', 'Nunito Sans', system-ui, -apple-system, sans-serif;
	--font-mono: 'IoskeleyMono', 'JetBrains Mono', 'SFMono-Regular', Menlo, Consolas, monospace;
}
```

---

## 3. Typography Scale & Offline Font Invariant

1. **Zero External Font CDN Calls:** The desktop WebView runs locally and offline-first. Never reference `fonts.googleapis.com` or remote font CDNs.
2. **Local Monospace Face:** `frontend/static/IoskeleyMono-Regular.woff2` is loaded locally via `@font-face` as `'IoskeleyMono'`.
3. **System-Safe Sans Stack:** Headings, navigation, modals, `HelpDrawer.svelte`, `OnboardingTour.svelte`, and `/manual` use `--font-sans` (`'Inter', 'Nunito Sans', system-ui, -apple-system, sans-serif`), falling back cleanly to OS sans-serif when `Inter`/`Nunito Sans` `.woff2` files are not bundled.
4. **Mandatory `tabular-nums`:** All numeric telemetry—equity, `NetLiquidation`, `BuyingPower`, `USD`/`CAD` cash, P&L, prices, timestamps, and `#` / `$` / `%` sizing inputs—must use `font-family: var(--font-mono); font-variant-numeric: tabular-nums;`.
5. **Workstation Scale (`11px`–`22px`):**
   - `11px`: Uppercase section kickers, status badges, tier pills (`letter-spacing: 0.08em`–`0.14em`, `font-weight: 700`).
   - `12px`: Metadata strips, table column headers, summary chips, compact timestamps.
   - `13px`: Dense `.cockpit-ticker-row` symbols/venues, sidebar nav items, strategy stack rows.
   - `14px`: Body copy, form inputs, `<select>` controls, buttons.
   - `18px`–`22px`: View headers (`h1`/`h2`, `font-weight: 700`, `letter-spacing: -0.02em`).

---

## 4. Global Dark `<select>` & `<option>` Rule

Linux GTK4 / WebKitGTK renders unstyled `<select>` popups in warm-grey light OS colors unless `color-scheme: dark` and explicit dropdown CSS are applied globally in [`frontend/src/app.css`](../../frontend/src/app.css):

```css
select {
	color-scheme: dark;
	appearance: none;
	-webkit-appearance: none;
	background-color: var(--bg-input, #0b1320);
	color: var(--text-primary, #e6edf3);
	border: 1px solid var(--border-subtle, #1f2937);
	border-radius: 8px;
	padding: 8px 32px 8px 10px;
	font-size: 14px;
	background-image: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='12' height='12' viewBox='0 0 12 12'%3E%3Cpath fill='%2394a3b8' d='M2.5 4.5L6 8l3.5-3.5z'/%3E%3C/svg%3E");
	background-repeat: no-repeat;
	background-position: right 10px center;
	background-size: 12px 12px;
	cursor: pointer;
}

select:focus {
	outline: none;
	border-color: var(--cobalt-cyan, #38bdf8);
	box-shadow: 0 0 0 2px rgba(56, 189, 248, 0.22);
}

select option {
	background-color: #0b1320;
	color: #e6edf3;
}
```

Component-scoped `<style>` blocks (such as [`Sidebar.svelte`](../../frontend/src/lib/components/Sidebar.svelte)) must not override `select` background, `color`, or `appearance` back to browser defaults.

---

## 5. Layout, Density & Component Contracts

1. **`+layout.svelte` Shell Reconciliation:** [`frontend/src/routes/+layout.svelte`](../../frontend/src/routes/+layout.svelte) must not override `.app-shell` with `#121212` or conflicting flex rules against `app.css`. The root layout owns the `280px` sidebar + fluid `minmax(0, 1fr)` main viewport (`100dvh`, `overflow: hidden` shell with controlled inner scroll).
2. **`.cockpit-shell` & `.cockpit-hud-strip`:**
   - `.cockpit-shell` uses `display: grid; grid-template-rows: auto auto minmax(0, 1fr); gap: 12px; min-height: 0;` (Header → Single-Bar HUD Strip → Main Workspace Grid).
   - `.cockpit-hud-strip` is locked to a compact single-bar height (`min-height: 44px`, `align-items: center`) regardless of watchlist length—preventing overlap on 52+ ticker lists and vertical stretching on 2-ticker lists.
   - Remove duplicate bottom `.cockpit-safety` banners from the Cockpit view.
3. **`.cockpit-ticker-row` Density:** Fixed single-line row (`height: 38px; min-height: 38px; white-space: nowrap; flex-wrap: nowrap;`), pairing normalized `SYMBOL.VENUE` identifiers, `$$$$` holding badges, `#` / `$` / `%` sizing inputs (`tabular-nums`), and non-destructive suspend toggles.
4. **Sidebar Brand Header & Collapsible Sections ([`Sidebar.svelte`](../../frontend/src/lib/components/Sidebar.svelte)):**
   - Anchor background in `--cobalt-navy` (`#0e0c28`) / `--bg-surface` (`#0B111E`).
   - `.sidebar-brand` displays the Cobalt geometric emblem (`32×32`–`40×40` Spec B 24-shard crest or `$lib/assets/favicon.svg`) beside the crisp `"COBALT"` title and subtitle.
   - Expose **one** authoritative TWS/Bot status badge across Sidebar, Cockpit, and Monitoring (eliminating duplicate status pills).
   - Broker Connection (`Connexion TWS`) and Portfolio & Risk Management (`Gestion du portefeuille & Risque`) are collapsible sections that auto-collapse when connected, rendering compact 1-line summary chips.
5. **Cockpit Strategy Stack, Monitoring, Builder & Help Surfaces:**
   - Cockpit Strategy column displays the `1/1` Active Strategy Card above a `(pinned DESC, sort_order ASC)` Strategy Stack with `📌` pin, `↑`/`↓` reorder, and a `>= 78%` drag-threshold Slide-to-Arm switch.
   - `/monitoring` renders 4 trader-focused panels free of SQLite/file-path/`desktop_local` developer noise.
   - `/strategy` uses a fixed-viewport zero-scroll 3-pane workbench (`Preset Stack` · `Filtres Macro → Entrées → Sorties` · `Rule Inspector` + JSON drawer).
   - `HelpDrawer.svelte` (`F1` / `?`) and `OnboardingTour.svelte` (3-track interactive spotlight tour) inherit `:root` tokens (`--bg-card`, `--cobalt-brand`, `--cobalt-orange`, `--cobalt-cyan`).

---

## 6. Two-Tier Cobalt Geometric Emblem Specification

Because fine 24-shard rays sub-pixel collapse below `24×24` while institutional desktop surfaces (`>= 32×32`) showcase the full Cobalt crest traced from `cobaltinvestissements.com`'s `logo_blanc.png`, `tws_traderbot` standardizes a **Two-Tier Cobalt Emblem**:

| Tier | SVG Spec | Target Scales | Applied Assets |
|---|---|---|---|
| **Tier 1: Spec A — Unified Multi-Scale Emblem** | `viewBox="0 0 100 100"`, `<title>Cobalt</title>`, Midnight-to-Uni-Noir squircle (`#1D367A` → `#0e0c28` → `#05070B`, `rx="22"`, `#263c8a` stroke), `#38BDF8` → `#1D63ED` crystal wing facets, concentric `#FFFFFF` Cobalt `C` core (`r_out=20.5`, `r_in=10`), `#FAA031` Cobalt Orange crown diamond | `16×16` – `1024×1024` | [`frontend/src/lib/assets/favicon.svg`](../../frontend/src/lib/assets/favicon.svg), [`site/favicon.svg`](../../site/favicon.svg), and `16×16` / `24×24` layers of [`build/windows/icon.ico`](../../build/windows/icon.ico) |
| **Tier 2: Spec B — Canonical 24-Shard Faceted Crest** | `viewBox="0 0 100 100"` identical `#1D367A` → `#0e0c28` → `#05070B` squircle + `#263c8a` stroke enclosing the 24-shard crest (`scale(0.90)` centered) in `#FFFFFF` with `#FAA031` Cobalt Orange tail diamond accent (`#18`) | `>= 32×32` | [`build/appicon.png`](../../build/appicon.png) (`1024×1024` 8-bit `srgba` `PNG32`), `32×32`–`256×256` layers of [`build/windows/icon.ico`](../../build/windows/icon.ico), `~/.local/share/icons/hicolor/256x256/apps/ca.boostao.tws_traderbot.png`, and [`Sidebar.svelte`](../../frontend/src/lib/components/Sidebar.svelte) brand mark |

---

## 7. Desktop Asset Inventory & Wayland Packaging (`ca.boostao.tws_traderbot`)

KDE Plasma Wayland ignores GTK4 programmatic per-window icon setters and resolves taskbar/titlebar icons strictly by matching the surface `xdg_toplevel.app_id` against an installed `.desktop` entry and `hicolor` icon:

1. **Canonical Application ID:** `ca.boostao.tws_traderbot` (matching `productIdentifier` in [`build/config.yml`](../../build/config.yml)).
2. **Go / Wails v3 Entry Point ([`main.go`](../../main.go)):**
   - Embeds `//go:embed build/appicon.png` into `appIcon`.
   - Sets `application.Options.Icon: appIcon` and `Linux: application.LinuxOptions{ApplicationID: "ca.boostao.tws_traderbot", ProgramName: "ca.boostao.tws_traderbot"}`.
   - Sets `application.WebviewWindowOptions.Linux: application.LinuxWindow{Icon: appIcon}`.
3. **Linux Desktop Entry ([`build/linux/ca.boostao.tws_traderbot.desktop`](../../build/linux/ca.boostao.tws_traderbot.desktop)):**
   - Uses `Icon=ca.boostao.tws_traderbot`, `StartupWMClass=ca.boostao.tws_traderbot`, `Categories=Office;Finance;`, `Terminal=false`, and omits `TryExec` so pre-build `task dev` registration stays visible to `kbuildsycoca6`.
4. **Automated Installation ([`Taskfile.yml`](../../Taskfile.yml)):**
   - Task `linux:install-desktop` (`platforms: [linux]`, CI-guarded via `if [ -n "${CI:-}" ]; then exit 0; fi`) installs a double-quoted `Exec="..."` `.desktop` file into `~/.local/share/applications/ca.boostao.tws_traderbot.desktop`, resizes [`build/appicon.png`](../../build/appicon.png) to `256×256` at `~/.local/share/icons/hicolor/256x256/apps/ca.boostao.tws_traderbot.png`, and refreshes `gtk-update-icon-cache`, `update-desktop-database`, and `kbuildsycoca6` when present.
   - Invoked automatically by both `linux:build` and `dev`.
5. **Release Portal (`site/`):** [`site/favicon.svg`](../../site/favicon.svg) is copied verbatim to `site/dist/favicon.svg` by `node site/build.mjs`.

# Evidence & Citations Index (`04_evidence_and_sources`)

> **Navigation:** [Master Index](../00_index.md) | **Related:** [Cobalt Trading Universe](../02_present_state/cobalt_trading_universe.md) · [Indicators & Courses](../02_present_state/cobalt_indicators_and_courses.md) · [System Architecture](../02_present_state/system_architecture.md) · [Decisions & Gaps](./decisions_and_gaps.md)  
> **Status:** `Active`

---

## 1. Purpose & Privacy Redaction Standard

This index catalogs all local project artifacts, multimedia recordings, Telegram export records, authenticated Cobalt portal extractions, course video transcripts/keyframes, repository provenance, and web evidence inspected for the `tws_traderbot` Operational Knowledge Framework.

**Privacy & Hygiene Guardrail:** All citations strip personal identities, member/instructor names, phone numbers, email addresses, login credentials, IBKR paper/live account identifiers, meeting join tokens (`Tid`/`Oid`), and non-technical chat banter. Participants are referenced exclusively by functional role (`Trader / Client`, `Developer`, `Advisor / Ops`, `Legal / Strategy Participant`).

---

## 2. Local Multimedia & Client Communication Artifacts

*(Located under the parent `cobalt/` project workspace outside the `tws_traderbot/` git repository root.)*

| ID | Artifact Path | Date / Size | Modality | Technical Evidence Extracted |
|---|---|---|---|---|
| **L-01** | `ChatExport_2026-11-16/messages.html` | `2026-03-12` – `2026-07-09` (3,288 lines) | HTML Export | Chronology of PineScript -> VPS/webhook -> local TWS API pivot; TradingView list links (`#75`, `#107`); UI readability request (`#84`); per-ticker sizing requirement `# shares / $ / %` (`#108`, `#110`, `#111`); OBV/tick-volume hint (`#94`, `#95`); real-account readiness (`#91`); Go desktop rewrite (`#99`). |
| **L-02** | `ChatExport_2026-11-16/files/2026-04-28_Meeting-COBALT-AI-Algo-API_resume.pdf` | `2026-04-28` (2 pages) | PDF Summary | Settled local TWS API execution (no VPS for v1); single-page cockpit requirement; rule weighting concept (e.g., VIX intensity/duration); human-trigger / compliance guardrail vs. autonomous unattended trading; action items for paper sketch and 4–5 baseline strategies (e.g., 9/21/55 MA). |
| **L-03** | `ChatExport_2026-11-16/video_files/bruno 1.mp4` & `bruno 1.m4a` | `2026-05-02` (`12:16`) | Video / Audio | Primary behavioral walkthrough by `Trader / Client`: 3-click cockpit workflow (`Achat` / `Short` / `Swing` / day-trading), strategy dropdown slots, suspend-without-deleting toggles (`100–500+` symbols -> `1–2` trades/day), TradingView share-link distribution over Telegram, buy-first / manual-exit v1 scope, and Main Breaker safety switch. |
| **L-04** | `video_frames/bruno_1/t000001.png` – `t001157.png` | `2026-05-02` (37 PNGs) | Key Frames | Visual capture of `photo_3` sketch walkthrough (`t000001`–`t000536`, `t000646`–`t001157`), Trader's TradingView `CBLT - TODAY` watchlist (`t000549`: `TITRES` + `SECTION 1`, 18 tickers on `LMND` 1D chart), TradingView `@ Cobalt- TELEGRAM` capture workflow (`t000617`), and Telegram pasting (`t000631`). |
| **L-05** | `ChatExport_2026-11-16/voice_messages/audio_1@27-04-2026_14-49-02.ogg` | `2026-04-27` (`00:03`) | OGG Audio | Non-technical meeting scheduling check only; no technical specification content. |

### 2.1 Telegram Photo Evidence (`ChatExport_2026-11-16/photos/`)

*(Full-resolution `.jpg` artifacts inspected; `_thumb.jpg` files excluded from primary analysis.)*

| ID | Filename | Date | Content & Technical Relevance |
|---|---|---|---|
| **P-01** | `photo_1@13-03-2026_02-03-08.jpg` | `2026-03-13` | Group avatar graphic; non-technical (no system/trading content). |
| **P-02** | `photo_2@28-04-2026_21-35-58.jpg` | `2026-04-28` | Matrix UI sketch (`Advisor / Ops`, `#68`/`#69`): rows = tickers (`NVDA`, `MSFT`, `AAPL`, `GE`), columns = strategies (`Short`, `Ema100`, `10-30 up`, `GoldenX`), Master ON/OFF (`#1`), right-click cherry-pick locking (`#4`). |
| **P-03** | `photo_3@28-04-2026_23-07-02.jpg` | `2026-04-28` | `Trader / Client` 2-column UI sketch (`#71`): top tabs (`menu page`, `achat`, `short`), left `stratégies` dropdowns (`EMA 100 5 min`, `Ema 9 croise 21 5 min`) + toggles, right `titres` (`aapl`, `msft`) + list-level/per-ticker toggles + scrollbar (`page déroulante`). |
| **P-04** | `photo_4@02-05-2026_12-27-54.jpg` | `2026-05-02` | Cropped TradingView `CRYPTO ETF` watchlist section (`BITX`, `ETHA`, `BSOL`, `SOLT`, `XRPC`, `ETHU`, `UXRP`, `MSBT`, `SOLZ`, `YBTC`, `XRPR`, `YETH`) shared prior to `bruno 1.mp4`. |
| **P-05** | `photo_5@02-05-2026_19-44-32.jpg` | `2026-05-02` | Early web prototype `Cockpit` screenshot (EN): `Achat` / `Short` / `Swing`, `Main breaker`, `4` strategy slots (`1/4`, `EMA Crossover with VIX Filter`), `532/532` tickers from `@ Cobalt- TELEGRAM` (`VIX`, `NQ1!`, `ES1!`, `BTC` 3/3, `CRYPTO` 5/5, `CRYPTO ETF` 15/15, `TITRES` 506/506). |
| **P-06** | `photo_6@02-05-2026_19-47-33.jpg` | `2026-05-02` | Prototype `Strategy Builder` screenshot: `Quick Start` (with `Generate PineScript` button from earlier design), `Strategy Library` preset CRUD, and 4 rules (`VIX Slope Filter`, `Market Hours Filter`, `EMA 9/21 Crossover Buy/Sell`). |
| **P-07** | `photo_7@02-05-2026_19-47-33.jpg` | `2026-05-02` | Prototype `Watchlist` screenshot: imported `https://www.tradingview.com/watchlists/209453826/`, `532 active / 532 total`, last refreshed `2 May 2026, 19:36`, showing `Manual`, `@ Cobalt- TELEGRAM`, `BTC`, `CRYPTO`, `CRYPTO ETF` groups. |
| **P-08** | `photo_8@03-05-2026_12-23-08.jpg` | `2026-05-03` | French-localized hosted prototype (`cazpe.com/cockpit`) `Swing` tab screenshot with `Coupe-circuit principal`, `532 / 532`, `Filter tickers` bar; validated by `Trader / Client` (`#88`). |
| **P-09** | `photo_9@06-05-2026_09-55-57.jpg` | `2026-05-06` | Paper-trading `Monitoring` run (`127.0.0.1:7497`, Client ID `1`, `Fixed Notional ($): 10000`, `Stop-loss %: 2`, `Take-profit %: 4`, `config/watchlist.txt`, `config/active_strategy.json`, `TWS LINKED`). *(Account ID omitted per privacy guardrail.)* |
| **P-10** | `photo_10@11-05-2026_13-12-49.jpg` | `2026-05-11` | Native desktop `tws_traderbot` (`Cobalt v0.1.0`) screenshot post Go/Wails rewrite: `534 active / 534 total` from `209453826`, `User Manual` nav item added, TWS connection sidebar (`Disconnected`). |

---

## 3. Local Project Documentation, Configs & SQLite Artifacts

| ID | Artifact Path | Scope & Evidence Summary |
|---|---|---|
| **D-01** | `cobalt/NOTES.md` | Synthesizes client feedback from `bruno 1.mp4`, confirms TradingView shared watchlists as canonical source (Telegram = transport), documents 3-surface architecture (`Cockpit`, `Watchlist`, `Strategy Builder`), buy-first / manual-exit v1 decision, and transitional gaps. |
| **D-02** | `cobalt/MIGRATION_PLAN_GO_WAILS.md` | Original Python/FastAPI -> Go + Wails migration plan (`2026-05-09`): single static binary, `modernc.org/sqlite`, Go IBAPI port, indicator/ML/PineScript evaluation. |
| **D-03** | `cobalt/TWS_SETUP_GUIDE.md` & `cobalt/install_tws.sh` | Local IBKR TWS installation and API socket configuration (`127.0.0.1:7497`, `Enable ActiveX and Socket Clients`, uncheck `Read-Only API`). |
| **D-04** | `cobalt/tws_traderbot.sqlite` & `tws_traderbot/tws_traderbot.sqlite` | Local SQLite DBs (`watchlist`, `system_config` tables). Parent copy holds `tws_connection` (`127.0.0.1:7497`, `client_id: 1`), `cockpit_state` (single `Main Trading` workspace, `enabled: false`), and `strategy` (`{}`). |
| **D-05** | `tws_traderbot/README.md`, `AGENTS.md`, `okf/AGENTS.md`, `.agents/skills/okf-curator/SKILL.md` | Wails v3 architecture guide, Go/Svelte build commands, OKF allowlist protocol, and mechanical hygiene rules (`scripts/check-okf-hygiene.py`). |
| **D-06** | `tws_traderbot` Core Source Files (`wails3` `HEAD`: `22beb38`) | Inspected for trading-universe and sizing verification: `app.go`, `app_config.go`, `app_watchlist.go`, `backend/models/schemas.go`, `backend/models/strategy.go`, `backend/bot/ibkr.go`, `backend/bot/engine.go`, `backend/bot/runner.go`, `backend/db/cockpit.go`, `backend/db/watchlist.go`, `frontend/src/lib/components/Sidebar.svelte`, `frontend/src/routes/cockpit/+page.svelte`, `frontend/src/routes/watchlist/+page.svelte`, `frontend/src/lib/api.ts`, `frontend/src/lib/i18n/translations.ts`. |
| **D-07** | Git Branches & History (`tws_traderbot/.git`) | Branches `pinescript` (`bfe5d48`), `alpha` (`482968b`, Python/FastAPI era with `config/default.yaml`, `config/watchlist.txt`, `strategies/example_strategy.json`, `docs/strategy_guide.md`, `docs/project_ideas.md`), `main` (`fc6c4f9`), `wails3` (`22beb38`), and tag `v0.0.1` (`961f467`). |

---

## 4. Authenticated Cobalt Portal, Method PDF, Briefs & Video Extractions

*(Extracted cleanly into sanitized scratch dumps during the `2026-11-16` authenticated portal & Vimeo caption pass; personal credentials and member/instructor names excluded.)*

### 4.1 Portal Lessons, PDFs & `/mon-compte/analyse` Daily Briefs

| ID | Artifact / Source | Scope & Verified Contents |
|---|---|---|
| **C-01** | `2025-11-17_Methode_de_travail_8_8.pdf` | Official Cobalt Investissements **`Méthode de travail` — `8e Edition (Révision 8)`** (1 page). Defines the `A -> A -> A` (`Analyse – Alarme – Action`) workflow, `12` analysis steps (`A–L`) including 4-tier categorization (`1-Blue Chip / 2-Croissance / 3-Spéculatif / 4-Rejeté`), and `5` execution steps (`A–E`) including `$$$$` owned-positions tracking. |
| **C-02** | `analyse_daily_briefs_clean.md` (`step_124` + `step_91.json`) | **`/mon-compte/analyse` Historical Index (`473` entries, `2025-09-09` ID `9864` to `2026-11-13` ID `11641`):** `228` `REGARD MATINAL`, `152` `VIDÉO`, `63` `EN REVUE`, and `30` macro/event/ticker spotlights. Includes `15` full recent daily entries (`11641` down to `11586`, `2026-11-03`–`2026-11-13`) capturing the `Économie -> Stocks -> Prémarkets – À regarder` (`3–4` focus tickers) cadence and `/static/videos/2026-11-*.mp4` links. |
| **C-03** | `cobalt_lessons_verified.md` & `browser_sandbox/outputs/step_28.json` | Sanitized DOM extractions (`2,781` lines) covering `trading_view`, `Lesson 213` (13 chart styles), `Lesson 212` (`MODÈLE ANALYSE` setup tables), `214`, `215`, `216`, `221`, `224`, `229`, `230`, `361`, `386`, `498`, `Cours-03` (`195`–`198`, `313`), `Cours-04` (`225`, `301`), `Cours-05` (`280`, `292`), `Cours-06` (`293`, `318`, `381` -> `t.me/CobaltAdmin_bot`), `351`, `378`, `59`. |
| **C-04** | `extra_lessons_verified.md` (`step_79.json`) | `22` additional unlocked course lessons (`1,467` lines): `Cours-01` (`185`, `291`, `112`, `113`, `52`), `Cours-02` (`299`, `290`, `189`, `366`), `TradingView` (`206`, `208`, `209`, `210`, `211`, `217` tier limits, `220`, `490`), `Bitcoin` (`485`, `486`, `487`, `488`, `492`, linking `/static/files/2026-03-26_Lexique_formation_adoptop_2026.pdf`). |

### 4.2 Course Video Transcripts (`vimeo_manifest.json` — `36` Vimeo Videos)

*(Composed of `34` Vimeo `fr-x-autogen.vtt` auto-captions in `vimeo_transcripts_clean.md` plus `2` local Whisper `small` transcripts in `vimeo_whisper_fallback.md`.)*

| Batch Artifact | Videos & Lesson Mapping | Technical Topics Covered |
|---|---|---|
| **V-01** `transcripts_tv_onboarding_and_courses.md` (`10` videos) | `1073484378` (`208`), `918792383` & `918792518` (`209`), `1073484407` (`210`), `1073484472` (`211`), `918791543` (`220`), `1085482134` & `1085482351` (`366`), `1085485413` (`318`), `1085485683` (`381`) | Account/brokerage boundaries (TradingView vs. IBKR/banks), toolbar/desktop sync, `Cours-02` Weinstein Phase 1–4 & SRT top-down analysis (`Monthly` -> `Weekly` -> `Daily`), Cobalt ecosystem overview, and `CANAL-Cobalt` / `CobaltAdmin_bot` Telegram onboarding & Stripe/token mechanics. |
| **V-02** `transcripts_tv_indicators_and_charts.md` (`8` videos) | `1073484488`, `1073484506`, `1073484529` (`212`), `1073484572` (`213`), `1073484629` (`214`), `919858213` (`221`), `919879538` (`229`), `925433660` (`230`) | `MODÈLE ANALYSE` dark theme, 4 price display modes, RTH/ETH rules, all 13 Cobalt Chart Styles (`1`–`9`, `3G`, `3T`, `4E`, `F`), drawing/Fibonacci tools, SPDR multi-screen templates, object tree, data window, and Bar Replay. |
| **V-03** `transcripts_tv_watchlists_alerts_screeners_ratios.md` (`11` videos) | `1073484454` (`490`), `1073486969` (`215`), `1073486991` (`206`/`215`), `1073487011` (`216`), `1091932848` (`361`), `1091932920` (`498`), `1091949058` (`224`), `862597406`, `862597412`, `879186103`, `1227017565` (`386`) | Watchlist creation, sections, color-flagged buckets (`rouges`), `.txt` import/dedup cleanup, price/indicator/watchlist alerts + pause/resume, Screener 2.0 (`52W High`, `Golden Cross`, `10/30 EMA`), Paper Trading DOM/SL/TP, and `COBALT Ratios | BASIC` & `PRO` (`v1.23` -> `v1.33`). |
| **V-04** `transcripts_bitcoin.md` (`5` videos) | `1214365536` (`485`), `1214365534` (`486`), `1214365537` (`487`), `1214365535` (`488`), `923161763` (`492`) | Monetary history, Bitcoin protocol fundamentals, hardware-wallet custody, and auxiliary Interac crypto chatbot (`StrongHodl`, separate from `tws_traderbot`). |
| **V-05** `vimeo_whisper_fallback.md` (`2` videos) | `512370024` (`185`), `521723754` (`378`) | Whisper `small` (`lang=fr`) fallback transcripts covering email whitelisting (`185`) and TWS API/IBKR setup context (`378`). |

### 4.3 Targeted Video Keyframes (`keyframes/` — `14` JPEGs)

| Lesson | Keyframe Files | Visual Evidence Captured |
|---|---|---|
| **`212`** | `212_modele_1_vid1073484488_21s.jpg`, `212_modele_3_vid1073484529_355s.jpg` | `MODÈLE ANALYSE` chart appearance, candlestick/bar modes, and layout controls. |
| **`213`** | `213_indicateurs_vid1073484572_195s.jpg`, `213_indicateurs_vid1073484572_972s.jpg` | Cobalt indicator stack configuration and multi-pane styling. |
| **`214`** | `214_outils_vid1073484629_361s.jpg` | Drawing tools toolbar and trendline/Fibonacci workspace layout. |
| **`215`** | `215_watchlists_vid1073486969_808s.jpg` | Color-flagged watchlist (`rouges` containing `AAPL`, `MSFT`, `COST`, `UNH`), `SMA 30 close` + `SMA 200 close` on `MAG` `1D`, and `1..8` template toolbar. |
| **`216`** | `216_alertes_vid1073487011_234s.jpg` | TradingView Alerts configuration panel (`Prix` / `Données techniques` / `Liste de surveillance`). |
| **`221`** | `221_multi_ecrans_vid919858213_71s.jpg` | Multi-screen sector layout setup (`SPDR S&P` / synchronized panes). |
| **`361`** | `361_modeles_screener_vid1091932848_255s.jpg`, `361_modeles_screener_vid1091932848_410s.jpg` | Saved Screener 2.0 presets (`52W High`, `Golden Cross`, `10/30 EMA` cross up/down). |
| **`386`** | `386_ratios_pro_vid879186103_79s.jpg`, `386_ratios_pro_vid879186103_236s.jpg` | `COBALT Ratios | PRO` panel metrics layout and `@ # % 0..9 B E F` template toolbar. |
| **`498`** | `498_screener2_vid1091932920_21s.jpg`, `498_screener2_vid1091932920_174s.jpg` | Screener 2.0 filter pills, primary-listing toggle, and watchlist-scoped scanning. |

---

## 5. Web & External Evidence Sources

| ID | URL / Source | Verification Status (`2026-11-16`) | Technical Evidence & Role |
|---|---|---|---|
| **W-01** | `https://www.tradingview.com/watchlists/209453826/` | Verified (`HTTP 200`, `Shared: True`) | **`✝️@ Cobalt- TELEGRAM`** shared watchlist. Embedded `application/prs.init-data+json` payload contains `691` entries (`687` symbols across `###BTC`, `###CRYPTO`, `###CRYPTO ETF`, `###TITRES`, plus `3` ungrouped macro/futures `TVC:VIX`, `CME_MINI:ES1!`, `CME_MINI:NQ1!`). |
| **W-02** | `https://www.tradingview.com/watchlists/111426240/` | Verified (`HTTP 200`, `Shared: True`) | **`⚾️@ Cobalt- Home Run`** shared watchlist (`messages.html` `#107`). Embedded JSON payload contains `124` entries (`123` symbols: `10` ungrouped ETFs/BTC/dual-class + `113` in `###TITRES` across `NYSE`, `NASDAQ`, `AMEX`, `CBOE`, `TSX`, `TSXV`, `COINBASE`). |
| **W-03** | `https://www.tradingview.com/watchlists/40576813/` | Verified via `tv_watchlist_40576813.json` (`Shared: True`) | **`CBLT - MARCHÉS`** shared watchlist (`Lesson 197`, `Created: 2021-02-20`, `Modified: 2026-11-02`). Contains `54` macro, index, yield, commodity, FX, and crypto regime benchmarks across `12` TradingView prefixes. *(Author username omitted per privacy guardrail.)* |
| **W-04** | `https://www.cobaltinvestissements.com/` | Verified (`HTTP 200`, Authenticated Crawl) | Cobalt Investissements web portal (`Accueil`, `/mon-compte/cours/*`, `/mon-compte/analyse`). Source of course lessons, Vimeo video embeds, `Méthode de travail 8.8`, and `473` daily market briefs. |
| **W-05** | `https://www.tradingview.com/script/CmlZsOCK-COBALT-Ratios-BASIC/` | Referenced in `Lesson 386` | Published TradingView PineScript indicator (`COBALT Ratios | BASIC` public script page). |
| **W-06** | `https://github.com/Boostao/tws_tradingbot` | Verified (`HTTP 200`) | Upstream GitHub repository (`origin` remote in `.git`), releases and branches (`main`, `alpha`, `pinescript`, `wails3`). |
| **W-07** | `cazpe.com` (Historical Prototype Reference) | Referenced in `messages.html` `#83` & `photo_8` | Ephemeral DigitalOcean-hosted web prototype deployed `2026-05-02` to validate French/English Cockpit, Watchlist, and Strategy Builder UX before the native Go/Wails rewrite. |
| **W-08** | `https://capitalise.ai` | Referenced in `messages.html` `#23` | Conceptual benchmark cited by `Developer` (`2026-03-29`) illustrating natural-language strategy rule authoring vs. PineScript webhooks. |
| **W-09** | `http://t.me/CobaltAdmin_bot` | Referenced in `Lesson 381` (`cobalt_lessons_verified.md`) | Telegram subscription-management bot provisioning paid/trial access to the one-way `CANAL-Cobalt` signal channel. |

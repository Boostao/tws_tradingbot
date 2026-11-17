# Cobalt Indicators, TradingView Setups & Course Curriculum (`02_present_state`)

> **Navigation:** [Master Index](../00_index.md) | **Related:** [Cobalt Trading Universe](./cobalt_trading_universe.md) · [System Architecture](./system_architecture.md) · [Capability Matrix](./capability_matrix.md) · [Citations Index](../04_evidence_and_sources/citations_index.md)  
> **Status:** `Active`

---

## 1. Scope & Sources

This document specifies the technical indicator configurations, TradingView chart templates, `Méthode de travail 8.8` workflow, portal curriculum, and trader behavioral patterns extracted verbatim from authenticated pages of `https://www.cobaltinvestissements.com/mon-compte/`, `35` course video transcripts (`34` Vimeo `fr-x-autogen.vtt` captions plus `512370024` via local Whisper), `14` video keyframes, and `2025-11-17_Methode_de_travail_8_8.pdf` (`2026-11-16` snapshot).

**Scope & Compliance Boundary:** Everything herein documents software settings, indicator parameters, chart configurations, and curriculum mechanics strictly as technical specifications for `tws_traderbot`. All personal names, emails, credentials, account identifiers, and financial advice are excluded; course sections distinguish between **unlocked lesson/transcript extractions** and **locked syllabus titles** gated behind paid tiers.

---

## 2. Member Portal Navigation & Access Tiers

### 2.1 Portal Structure (`/mon-compte/`)

The Cobalt `ZONE PRIVÉE` shell renders a top TradingView `ticker-tape` widget (`FOREXCOM:SPXUSD`, `IG:NASDAQ`, `SPREADEX:DJI`, `IG:RUSSELL`, `CRYPTO:BTCUSD`, `CRYPTO:ETHUSD`, `TVC:GOLD`, `TVC:SILVER`), a live Bank of Canada `USD/CAD` exchange bar, a theme switcher (`Clair` / `Fonce` / `Systeme`), and the following sidebar navigation:

| Group | Label | Route | Notes |
|---|---|---|---|
| **Core** | `Tableau de bord` | `/mon-compte` | Dashboard widgets (`Les indispensables`, `Événements à venir`, `Avis`, `Analyses`, `Opportunités`, TradingView `Calendrier Économique`, `Fermetures de marché` for `NYSE`/`TSX`, TradingView `Heatmap` for `Stock`/`Crypto`/`Etf`) |
| **Core** | `Compte & Support` | `#submenu-support` | `/mon-compte/avis`, `/mon-compte/faq`, `/support`, `/mon-compte/preference` |
| **Préparation** | `Abonnements \| Formations` | `/mon-compte/achats` | Subscription & standalone course catalog |
| **Préparation** | `Calendrier \| Événements` | `/mon-compte/formations` | Live sessions / monthly follow-up calendar (`Suivi Mensuel` & `Day Trading` webinar registration via account tokens) |
| **Préparation** | `Analyses \| Marchés & Secteurs` | `/mon-compte/analyse` | Daily `Regard Matinal` briefs & `VIDÉO` market walkthroughs (`/mon-compte/analyse?favoris=1`) |
| **Préparation** | `Opportunités` | `/mon-compte/opportunites` | Masked ticker alerts (`BT**`, `EA**`, `VI**`, `NV**`, `IO**`, `MU**`, `IB**`, `OS**` — gated with `*** Abonnement requis ***`) |
| **Votre parcours** | `Formation Initiale En Cours` | `/mon-compte/cours/formation_en_cours` | Live cohort recordings (retained 3 months until the next cohort starts; `0` items in current snapshot) |
| **Votre parcours** | `Suivi Mensuel` / `Day Trading` | *(sidebar token counters `0`)* | Post-cohort monthly follow-up (1/month) & intraday workshop token tracks |
| **Formations en ligne** | `TradingView` | `/mon-compte/cours/trading_view` | 20 unlocked lessons + 2 locked modules (`1`-year inclusion, then subscription) |
| **Formations en ligne** | `Cours-01` .. `Cours-06` | `/mon-compte/cours/cours_01` .. `cours_06` | 6-week core technical/fundamental syllabus (`1`-year inclusion, then subscription) |
| **Formations en ligne** | `Formations avancées` | `/mon-compte/cours/advanced` | `0` accessible lessons in current account snapshot |
| **Formations spécifiques** | `IBKR Trader Workstation (TWS)` | `/mon-compte/cours/trader_ibkr` | 1 unlocked intro (`view/378`) + 56 locked modules |
| **Formations spécifiques** | `Bitcoin` | `/mon-compte/cours/bitcoin` | 5 unlocked intro modules (`488`, `487`, `486`, `485`, `492`, plus `2026-03-26_Lexique_formation_adoptop_2026.pdf`) + 17 locked modules |
| **Formations spécifiques** | `Ethereum, cryptos et DeFi` | `/mon-compte/cours/cryptos` | `0` accessible lessons in current account snapshot |

### 2.2 Subscription Tiers & Ecosystem Connection (`/mon-compte/achats`, `318`, `381`)

- **Gated Memberships:** `Abonnement Opportunités` (`50$ / mois`, `600$ / 13 mois`), `Abonnement CANAL-Cobalt` (`35$ / semaine`, `90$ / mois`, `1080$ / 13 mois` via `/mon-compte/achats`; historical video `1085485683` also references `$30/wk`, `$40/mo`, `$960/13mo`), `Forfait « PRO+ »` (`2085$ / Année`), `Forfait « PREMIUM »` (`2899$ / Année`).
- **Standalone Specific Courses:** `IBKR Trader Workstation (TWS)` (`99$ / 1 an`), `Tout sur le Bitcoin - Formation avancée` (`129$ / 1 an`), `Ethereum, altcoins, DeFi et trading des cryptos - Formation avancée` (`129$ / 1 an`).
- **Locked TradingView Modules (`#abonnement-opportunites`):**
  - `TradingView | Listes de surveillance "COBALT" à télécharger` (`/mon-compte/cours/view/294`, linked from Lesson `206`, redirects to `/mon-compte/achats` when unsubscribed; unlinked on `/mon-compte/cours/trading_view`).
  - `TradingView | Daytrading - Modèles d'indicateurs Intraséance` (unlinked below `Obtenir cette formation` on `/mon-compte/cours/trading_view`).
- **`CANAL-Cobalt` Telegram Transport (`Lessons 318` & `381`):** Activated via `http://t.me/CobaltAdmin_bot` (Stripe payment or free-trial token link emitted after week 7). Operates as a one-way alert broadcast (no peer-to-peer chat) active **`09:00–13:30`** (excluding seminar/day-trading event days), streaming pre-market prep, sector/ETF moves, watchlists, and strategy signals.

---

## 3. Official Methodology (`Méthode de travail 8.8`, `Lesson 351`)

Extracted from `2025-11-17_Methode_de_travail_8_8.pdf` (`8e Edition (Révision 8)` — *"Analysez et gagnez stratégiquement"*):

### 3.1 Core Cycle: `A -> A -> A (Analyse – Alarme – Action)`

1. **Daily Pre-Check (`1–4`):** Review `Cobalt / Opportunités / CANAL-Cobalt` $\rightarrow$ inspect `MARCHÉS/SECTEURS` trends & phases $\rightarrow$ place/adjust alarms on personal lists $\rightarrow$ monitor trend pivots using the 12-step filter below.
2. **12 Analysis & Stock Selection Steps (`ÉTAPES D'ANALYSE ET DE SÉLECTION DE TITRES`):**
   - **A. Recherches:** Cobalt platform, TradingView, company website, CNBC, Investing.com, Finviz, Earnings Whispers, MSN Money, Etf.com, Morningstar.
   - **B. Catégorisation:** Evaluate fundamentals & `COBALT-Ratios` table; classify or reject into **`1-Blue Chip`**, **`2-Croissance`**, **`3-Spéculatif`**, or **`4-Rejeté`**.
   - **C. Caractéristiques:** Check volume, volatility, smooth chart behavior (`graphique constant`), sector correlation, and `float vs. shares outstanding`.
   - **D. Phase & Pattern:** Identify current Weinstein market phase and dominant chart pattern.
   - **E. Niveaux (`M & W`):** Trace major Supports, Resistances, and Trends (`SRT`) on Monthly and Weekly charts.
   - **F. Niveaux (`D`):** Trace major `SRT` levels on Daily charts.
   - **G. Analyse & Filtre:** Analyze and filter using **Cobalt Indicator Templates `1` to `9`**.
   - **H. Intra séance:** Inspect intraday direction changes stepping down **`30m` to `1m`** across Cobalt templates.
   - **I. Outils d'analyse:** Apply chart patterns (`Figures Boursières`) and **Fibonacci retracements**.
   - **J. Analyse de risque:** Compute gain/loss ratio, **Take Profit**, and **Stop-Loss** relative to nearby `S/R` levels.
   - **K. Alertes:** Program alerts on `Supports`, `Resistances`, `Tendances`, and **`EARNINGS`**.
   - **L. Organisation:** File tickers into personal watchlists and categories.
3. **5 Action / Execution Steps (`PRENDRE ACTION / EXÉCUTION`):**
   - **A. `ORDRE D'ACHAT`:** Set entry using `Market` (`Au marché`), `Limit` (`Limite`), or `Buy Stop` (`Achat Stop`).
   - **B. `ORDRE DE VENTE`:** Immediately define and place exit `Limit` and/or `Stop-Loss` orders.
   - **C. `ALARMES DE SUIVI DE VOS ORDRES`:** Place tracking alerts at buy, sell, and stop-loss prices.
   - **D. `NOTES SUR LE GRAPHIQUE`:** Annotate chart with date, share count (`nombre de parts`), entry price, exit price, and next `earnings` date.
   - **E. `CLASSER`:** Move purchased tickers into the dedicated owned-positions watchlist **`($$$$)`**.

---

## 4. TradingView Chart Styles (`Lesson 213`: 13 Standardized Models)

Lesson `213` (`/mon-compte/cours/view/213`, Vimeo `1073484572`, ~51m) defines **13 core TradingView indicator templates** pinned to the favorites toolbar (visible in keyframes `386_..._236s.jpg` and `361_..._410s.jpg` alongside optional instructor toolbar pills `# % @ $ 0..9 B E F`).

### 4.1 Setup Constraints, Tier Minimums & Scale Rules

1. **Minimum TradingView Subscription (`Video 918792518`):** `Plus` (`10` indicators/chart, `100` price + `100` technical alerts) is the practical minimum because `Essential` caps at `5` indicators while **Style `7` (Guppy) requires `10` EMAs**. Instructors run **`Premium`** (`25` indicators/chart, `8` charts/tab, `400` price + `400` tech alerts + `2` watchlist alerts with no expiry).
2. **Equity Ticker Prerequisite:** Templates must be built on a **single equity** (e.g., `AAPL`), **not** an index (`CAC`, `SPX`), ETF (`SPY`, `QQQ`), or forex pair, because indices/ETFs/forex lack fundamental data (`P/E Ratio`, `Rendement du dividende %`) and TradingView blocks adding those indicators.
3. **Volume + MACD Scale Incompatibility (Style `4E`):** Inserting `MACD` into the same `Bas` pane as `Volume (Vol)` distorts the plot. Via MACD `...` $\rightarrow$ `Épingler à l'échelle`: Option `A` puts 2 scales right, **Option `B` (active standard)** pins `Volume` right and `MACD` left, while Option `C` (hide MACD scale) fails to persist on save. In Style `4E`, `Volume` opacity is also lowered to **`50%`** (`Video 1073484572`) so overlaid MACD lines remain clear.
4. **Default Interval Binding (`Se souvenir de l'intervalle`):**
   - **Daily (`D`):** Styles `1`, `2`, `3`, `3G`, `3T`, `4`, `4E`, `5`, `6`, `8`, `F` (11 templates).
   - **Weekly (`W`):** Styles `7` (`Méthode de Guppy`) and `9` (`SMA 13-26 semaines`) (2 templates).
5. **Shared Moving-Average & MACD Defaults:** All `SMA`/`EMA` series use `Source: Clôture`, `Décalage: 0`, `Adoucissement: Rien`, `Plage temporelle: Graphique`, `Attendre la fermeture de la plage temporelle: Activé`. All `MACD` instances (`3G`, `3T`, `4E`) use `Fast: 12`, `Slow: 26`, `Signal: 9`, `Source: Clôture`, `Oscillator MA: MME`, `Signal MA: MME`, `Histogramme: Désactivé`, `MACD: Rouge 2px`, `Signal: Lime 2px`, `Zero: Gris 1px`, scale labels & status values `Désactivé`.
6. **Indicator Update Caution (`Video 1073484572`):** Clicking TradingView's purple `"Update available"` spiral resets custom line colors and does not auto-save to templates; minor TradingView script updates are left unapplied unless required.

### 4.2 Complete Specification of the 13 Indicator Templates

| Style ID & Name | Interval | Upper Pane (`Haut`) Indicators & Settings | Lower / Middle Pane (`Milieu` / `Bas`) Indicators & Settings |
|---|---|---|---|
| **`1` (`Mouvement de prix`)** | `D` | *(Price candles only; starting view for `M`/`W` SRT analysis)* | `Bas`: **`Volume (Vol)`** — `Longueur MA: 100`, `Couleur basée sur la clôture précédente: Désactivé`; `Volume` Histogramme (En croissance: Blanc `2px`; En chute: Rouge `2px`); `Volume MA: Activé` (Rouge `1px`, Droite). |
| **`2` (`SMA 30-200`)** | `D` | **`SMA(30)`** (Jaune `2px`, Droite)<br>**`SMA(200)`** (Mauve `2px`, Droite) | `Bas`: **`Volume (Vol)`** — identical to Style `1` (`Longueur MA: 100`, growing Blanc `2px`, falling Rouge `2px`, `Volume MA` Rouge `1px`). |
| **`3` (`Croisement EMA 10-30`)** | `D` | **Price hidden** manually via `"Droite"` mode (`0%` opacity).<br>**`EMA(10)`** (Rouge `2px`, Droite; custom bright green/red swapped per transcript vs. DOM table)<br>**`EMA(30)`** (Lime `2px`, Droite) | `Bas`: **`Volume d'équilibre (OBV)`** — `Adoucissement: Rien`, `Plage: Graphique`, `Attendre fermeture: Activé`; `Volume OnBalance`: Jaune pâle `2px` Droite; scale labels & status values `Désactivé`. |
| **`3G` (`Alligator`)** | `D` | **`Alligator Williams`** — `Jaw: 13` (Offset `8`, Bleu `2px`), `Teeth: 8` (Offset `5`, Magenta `2px`), `Lips: 5` (Offset `3`, Vert `2px`); `Source: Clôture`, `Décalage: 0`. | `Bas`: **`MACD (12, 26, 9, MME/MME)`** — standard Cobalt MACD. |
| **`3T` (`Triple Croisement EMA 9-21-55`)** | `D` | **`EMA(9)`** (Rouge `2px`, Droite)<br>**`EMA(21)`** (Jaune `2px`, Droite)<br>**`EMA(55)`** (Lilas `2px`, Droite) | `Bas`: **`MACD (12, 26, 9, MME/MME)`** — standard Cobalt MACD. |
| **`4` (`SMA 100-150-PE`)** | `D` | **`SMA(100)`** (Orange `2px`, Droite)<br>**`SMA(150)`** (Turquoise `2px`, Droite) | `Bas`: **`Ratio cours/bénéfice (P/E Ratio)`** — `Symbole principal du graphique`; `P/E ratio: Activé` (Magenta `2px`, `Ligne Interrompue`). |
| **`4E` (`EMA 100-VOL-MACD`)** | `D` | **`EMA(100)`** (Lime `2px`, Droite) | `Bas` *(dual scale Option B)*: **`Volume (Vol)`** (`Longueur MA: 100`, growing Blanc `2px`, falling Blanc `2px`, **`50%` opacity** in video, **`Volume MA: Désactivé`**) on right scale **+** **`MACD (12, 26, 9, MME/MME)`** on left scale. |
| **`5` (`Golden Cross / Death Cross`)** | `D` | **`SMA(50)`** (Bleu pâle `2px`, Droite)<br>**`SMA(200)`** (Mauve `2px`, Droite) | `Bas`: **`Relative Performance between Stock and SPX (Ticker/SPX)`** — `Longueur: 100`, `EMA?: Désactivé`; Plot 1: Jaune pâle `2px` Droite; Plot 2: Rouge `2px` Droite; scale labels & status values `Désactivé`. |
| **`6` (`Suracheté / Survendu`)** | `D` | **`Bandes de bollinger (BB)`** — `Longueur: 20`, `Type MM de base: SMA`, `Source: Clôture`, `StdDev: 2`, `Décalage: 0`; Base: Jaune `2px`, Supérieur & Inférieur: Bleu pâle `2px`, Arrière-plan: Vert (`10%` table / `20%` spoken). | `Milieu`: **`RSI`** — `Longueur RSI: 14`, `Clôture`, `Calculer la divergence: Désactivé`, `Adoucissement: Type SMA / Longueur 14 / BB StdDev 2`; RSI: Bleu pâle `2px`; `RSI-Based MA: Désactivé`; Bands `70` / `30` (gris `1px` `Traitirée`); Fill: Mauve `20%`.<br>`Bas`: **`Stochastique (Stoch)`** — `%K Longueur: 15`, `%K Lissage: 5`, `%D Lissage: 5`; `%K`: Bleu pâle `2px`; `%D`: Orange `2px`; Bands `80` / `20` (gris `1px` `Traitirée`); Fill: Mauve `20%`; scale labels & status values `Désactivé`. |
| **`7` (`Méthode de Guppy`)** | `W` | **5 fast EMAs:** `EMA(3)`, `EMA(5)`, `EMA(8)`, `EMA(12)`, `EMA(15)` (Lime `1px`, Droite)<br>**4 slow EMAs:** `EMA(30)`, `EMA(40)`, `EMA(50)`, `EMA(60)` (Rouge `1px`, Droite)<br>**Anchor EMA:** `EMA(200)` (Mauve `2px`, Droite) | *(None)* |
| **`8`** *(Long-term EMA + Dividend Yield)* | `D` | **`EMA(150)`** (Bleu pâle `2px`, Droite)<br>**`EMA(250)`** (Rose pâle `2px`, Droite)<br>**`EMA(350)`** (Rose `2px`, Droite) | `Bas`: **`Rendement du dividende %`** — `Symbole principal du graphique`, `Show Change: Désactivé`, `Show Date: Activé`; `Dividend yield %`: Turquoise `2px`, `Ligne à degrés avec losanges`; `Étiquettes des volets: Activé`. |
| **`9` (`SMA 13-26 semaines`)** | `W` | **`SMA(13)`** (Bleu `2px`, Droite)<br>**`SMA(26)`** (Gris `2px`, Droite) | `Bas`: **`Relative Performance between Stock and SPX (Ticker/SPX)`** — identical to Style `5` (`Longueur: 100`, `EMA?: Désactivé`, Jaune pâle `2px` & Rouge `2px`). |
| **`F`** *(Dual: `F` + `F sans intervalle`)* | `D` *(or unlocked)* | **`COBALT Ratios \| PRO`** *(or `EXPERT` / `BASIC`)*<br>**`SMA(200)`** (Mauve `2px`, Droite) — *`F sans intervalle` uses Style `1` (`Volume`) with `Remember interval` OFF for intraday inspection* | `Bas`: **`Relative Performance between Stock and SPX (Ticker/SPX)`** — `Longueur: 100`, `EMA?: Désactivé`; Plot 1: Jaune `2px`; Plot 2: Rouge `2px` (omitted in `F sans intervalle`). |

---

## 5. `MODÈLE ANALYSE` Chart & Workspace Rules (`Lessons 212`, `490`, `229`)

Lesson `212` (`/mon-compte/cours/view/212`, Vimeo `1073484488`, `1073484506`, `1073484529`) and Lesson `490` (`1073484454`) standardize **TradingView Desktop** under layout **`ANALYSE`** (stored in workspace theme `COBALT`).

### 5.1 Favorite Intervals & Four Price Display Modes

- **9 Favorite Intervals (`Lesson 490`):** `1m` (smallest standard Cobalt interval), `5m`, `15m`, `30m`, `1h`, `4h`, `D`, `W`, `M` (keyboard entry e.g. `15 + Enter`, plus custom intervals such as `14m`).
- **Four Standardized Price Modes (`Lesson 212` + Keyframes):**
  1. **`Chandelles japonaises`:** White bullish candles and Red bearish candles (body/border/wick) on a solid black (`Uni Noir`) canvas.
  2. **`Droite` (`0%` Opacity):** Hides the price line completely to isolate indicator crossovers cleanly in **Style `3` (`EMA 10-30`)**; must be toggled manually because bar style is not stored in indicator templates.
  3. **`Région` (`Area`):** Solid **`BLANC (100%)`** line — used in Lesson `292` to spot peaks, troughs, and chart figures rapidly.
  4. **`Heikin-Ashi`:** Kept green to distinguish from Japanese candles, with **`Prix réels sur l'échelle de prix`** checked so scale labels track real market prices.

### 5.2 Core Settings, `RTH`/`ETH` & Layout Persistence Hazards

- **Timezone & Session (`RTH` vs. `ETH`):** Timezone locked to **`Toronto`** (North American exchange alignment). Intraday intervals (`< 1D`: `1m`..`4h`) expose the **`RTH` / `ETH`** toggle (`Pré / post marché` labels `Valeur, Droite / Orange / Bleu`), whereas `>= 1D` (`D`, `W`, `M`) bars aggregate full sessions and hide intraday session controls.
- **Status Line, Scales & Canvas (`step_28.json`):** Logo, Title (`Ticker et description`), OHLC values, Bar change, and Indicator Titles/Inputs/Values (`50%` background) ON; Symbol Volume, Last-day change, Bar countdown, Prev Close, High/Low, and Bid/Ask hidden; `Auto` price scale (`No overlapping labels` ON); `24h` time scale (`lun. 29 sep '97`, weekday ON); `Uni Noir` background, `Aucun` grid, `Blanc (50%)` dashed crosshair, Watermark ON, margins `10%` top / `8%` bottom / `10 bars` right; Trading overlay shows `Buy / Sell` buttons, `Positions` / `Brackets` (Silver), `Execution marks` ON, `One-click trading` OFF (or explicitly enabled in paper trading).
- **Drawing Sync & Ticker-Migration Data-Loss Hazard (`Lessons 212`, `229`, `214`):**
  - Drawings belong to the **layout** (`ANALYSE`), not the ticker globally — Cobalt sets drawing sync to **`Synchroniser dans la mise en page`** (not global across `Marchés` / `Secteurs`), managed via the **Object Tree (`Arborescence des objets`)**.
  - If a symbol migrates exchanges (e.g., `TSXV` $\rightarrow$ `TSX` or `NASDAQ` $\rightarrow$ `NYSE`), or undergoes a stock split, TradingView recreates history under the new symbol/scale and **loses or misaligns all chart drawings and position notes (`"achat... 10 à 174.94"`)**. Chart notes serve solely as visual context, never as an accounting ledger.

---

## 6. Specialized TradingView Modules (`386`, `221`, `216`, `498`, `361`, `214`–`230`)

### 6.1 `COBALT Ratios` Panel (`PRO`, `EXPERT`, `BASIC` — `Lesson 386`)

Keyframe `213_indicateurs_vid1073484572_195s.jpg` confirms two invite-only scripts authored by Cobalt (`COBALT Ratios | EXPERT` and `COBALT Ratios | PRO`) alongside the public script [`COBALT Ratios | BASIC`](https://www.tradingview.com/script/CmlZsOCK-COBALT-Ratios-BASIC/) (`10` real-time metrics).

**Exact 19-Row `COBALT Ratios | PRO` Layout (`386_ratios_pro_vid879186103_236s.jpg`, v1.23) & Color Thresholds (`Video 879186103`):**

| Row | Metric Label | Sample (`BMY`) | Cobalt Threshold / Color Rule (`Green` / `Yellow` / `Red`) |
|---|---|---|---|
| **0** | Sector / Industry Header | `Health Tech / Pharma: Major` | ETFs/Indices show `Miscellaneous` & `-` on all fundamental rows (`79s.jpg`) |
| **1** | `FLOAT` | `2.09B` | Higher float preferred; flags volatile low-float news movers |
| **2** | `P/E` | `12.94` | **Green `<= 15`** (sector-dependent) |
| **3** | `EPS` | `$3.94` | **Green when positive** |
| **4** | `P/CF` | `8.27` | Lower preferred; **warning/amber starting around `8`** |
| **5** | `BVPS` | `$14.25` | Liquidation book value per share (safety margin) |
| **6** | `P/B` | `3.58` | **Green `<= 1`**, **Yellow/acceptable up to `2`** (Graham/Buffett rule) |
| **7** | `GRAHAM FACT` | `46.33` | `P/E * P/B`; **Target `< 22.5`** (`15 * 1.5`) |
| **8** | `GRAHAM NUM` | `$35.56` | $\sqrt{22.5 \times \text{EPS} \times \text{BVPS}}$ intrinsic price reference |
| **9** | `DIVIDEND` | `3.93 %` | Dividend yield % (`Green` when paying) |
| **10–12** | `ROA`, `ROE`, `ROI` | `8.75%` / `26.87%` / `12.45%` | **Green `>= 10%`**, yellow below `10%`, red if negative/weak |
| **13** | `ATR` | `3.04 %` | 14-session (~2-week) volatility %; **`> 5%` flags high-volatility ticker** (speaker equates `ATR`/`ADR` role here) |
| **14** | `R&D/REVENUE` | `20.39 %` | Future R&D reinvestment ratio |
| **15–16** | `GR. MARGIN`, `NET MARGIN` | `56.72%` / `17.58%` | Gross & net profitability margins |
| **17** | `CURR. RATIO` | `1.18` | Short-term (`1y`) debt coverage; low triggers warning, **`> 3.5`** flags idle liquidity |
| **18** | `DEBT/EQUITY` | `1.35` | **Buffett benchmark `<= 0.5`** (`50%`; growth companies often exceed) |

- **Version `1.33` Additions (`Video 1227017565`):** Added `EPS Growth (1Y)`, `Forward P/E Ratio`, `Cash Operating Activity`, `Cash Investing Activity`, `Cash Financing Activity` (expanding toward the `30` metrics advertised on Lesson `386`), plus `Auto` light/dark theme colors, Industry header toggle, Category grouping toggle, **2-column mode** with L-shape `décalage de coupure`, and hover tooltip (`infobulle`) toggle.
- **Z-Order & Market-Data Warnings:** Requires `Visual Order -> Bring to Front` so candles stay behind the table. Free `CBOE-BZX` TradingView data drops quotes/candles (and delays `TSX`/`TSXV` by `15m` per `Video 918792518`), corrupting **`ADR`/`ATR`** calculations; live US (`$119.40/yr` bundle or `$36/yr` NASDAQ) and Canadian (`$132/yr` TSX/TSXV) exchange data feeds are recommended for intraday work.

### 6.2 Multi-Monitor Layouts (`Lesson 221`) & Screener 2.0 (`Lessons 498`, `361`)

- **Multi-Monitor Workspaces (`Video 919858213`):**
  - Requires **TradingView Desktop** so detached monitor windows persist across restarts.
  - **`Marché` Layout (`8`-pane horizontal):** Loads `CBLT - MARCHÉS` (`SPX`, `DJI`, `QQQ`, `VIX`, `TSX`, `GOLD`, `SILVER`, etc.) with `SMA(100)` (`0` template without Volume/MACD).
  - **`Secteur 1 à 6` + `Secteur 7 à 12` (Dual `6`-pane workaround for 12 US SPDR sectors beyond Premium's 8-pane limit):** Synchronizes `Intervals`, `Crosshair`, `Time`, and `Date Range` (`MODÈLES MULTI-ÉCRANS`: `SCTR1: 1 à 8`, `SCTR2: 9 à 16`, `Industry 1-8`, `Industry 9-16`). **`Alt + Click`** maximizes/restores any single pane.
- **Screener 2.0 (`Videos 1091932920`, `1091932848` & Keyframes `498_..._174s.jpg`, `361_..._255s.jpg`):**
  - **Universe Hygiene:** Disable **`Liste primaire` (`Primary listing`)** under `Marché` (`13,805` US listings vs. `8,120` primary-only) so dual-class shares like `BRK.B` appear; filter by index (`S&P 500`, `503` symbols) or inside a Cobalt watchlist (e.g., `Cobalt titres composantes ETF`, `710` US tickers).
  - **Observed Screener Filters & Columns:** Filters include `Prix`, `Variation %`, `Capitalisation boursière`, `P/E`, `Croissance dil du BPA`, `Rendement du Dividende %`, `Secteur`, `Note des analystes`, `Perf %`, `Croissance des revenus`, `PEG`, `ROE`, `Beta`, `Date de gains récents`, `Date de gains à venir`; columns include `Symbole`, `Prix`, `Variation %`, `Volume`, `Volume Rel`, `Capitalisation boursière`, `Gross Profit FY`, `P/E`, `BPA dil TTM`, `Croissance dil du BPA YoY TTM`, `Rendement du Dividende %`, `Secteur`, `Note des analystes`.
  - **4 Official Cobalt Screener Presets (`Video 1091932848`):**
    1. **`US 52W High`:** `Prix >= Haut (52 semaines)`.
    2. **`Golden Cross`:** `SMA(50, 1J)` `croise au-dessus` `SMA(200, 1J)` *(must verify on `1D` chart, not `1W`!)*.
    3. **`Croisement 10-30 Haut`:** `EMA(10, 1J)` `croise au-dessus` `EMA(30, 1J)`.
    4. **`Croisement 10-30 Bas`:** `EMA(10, 1J)` `croise en dessous` `EMA(30, 1J)`.

### 6.3 Alerts (`Lesson 216`), Drawing Tools (`214`), Watchlists (`215`) & Replay/Paper (`224`, `230`)

- **3 Alert Tiers (`Video 1073487011`, `216_..._234s.jpg`):** `Price` (`400` Premium), `Technical` on lines, parallel/flat-top/disjoint channels, or indicators (`400` Premium — rectangles & regression channels do not support alerts), and **Watchlist Alerts** (`2` Premium — runs e.g. `EMA(10)` crosses `EMA(30)` `1D` across an entire 500-stock list).
- **Alert Trigger & Notification Rules:** `Once Only` (`une fois seulement`) for price/S-R levels (triggered alerts move to `Inactive` without consuming quota and are triaged end-of-day via `Inactive only` + `Most recent first`); `Once Per Bar` for MA crossovers; avoid `Once Per Bar Close` on `1D` unless 16:00 close-only alerts are wanted. Extended-hours (`ETH`) alerts must be placed from an intraday chart with `ETH` enabled. **Email-to-SMS (`avtech.com`) is noted as deprecated** across most Canadian carriers (Videotron/Rogers dropped; Telus cited as residual) — Cobalt recommends TradingView **mobile push + desktop toast** alerts and CSV log export.
- **Shortcuts, Tools & Watchlists (`214`, `215`, `206`):** `Alt + T` (Trendline), `Alt + H` (Horizontal Line — templates `Coût moyen` green, `Stop loss` red), `Alt + J` (Horizontal Ray), `Alt + V` (Vertical Line — used for **Earnings alerts** 1–2 days prior), `Alt + W` (add ticker to active watchlist), `Shift + W` (open watchlist manager), `Ctrl + Drag` (clone). Colored flag lists (`Listes marquées` e.g. `Rouges` in `215_..._808s.jpg`) pair with Screener 2.0 triage; re-importing `.txt` lists creates duplicates requiring manual cleanup (`Lesson 206`).
- **Bar Replay (`Lesson 230`, `925433660`) & Paper Trading (`Lesson 224`, `1091949058`):** Replay simulates bar-by-bar entries/exits (`1x`/`10x`, e.g., `$5,000` trade on `$100k` account $\rightarrow$ `16 shares`) and computes win-rate/P&L; Paper Trading requires setting **`1:1` leverage** for stocks and crypto at account creation, supports drag-and-drop `TP`/`SL` brackets + `Reverse Position`, and warns that `MKT` orders fail outside RTH.

---

## 7. Course Curriculum (`Cours-01`..`Cours-06`) & Signal Rules

### 7.1 Unlocked Analytical Workflows & Transcript Rules

| Course / Lesson | Verified Scope & Technical Rules |
|---|---|
| **`Cours-01`** (`185`, `291`, `112`, `113`, `52`) | Email whitelist setup (`512370024`), companion accounts (`Investing.com`, `Webull.ca`, `113`), Weekly (`W`) Style `1` charts tracing **Supports, Resistances, and Trends (`SRT`)** (`52`: S $\leftrightarrow$ R role reversal; oblique trendlines act as dynamic S/R). |
| **`Cours-02`** (`299`, `290`, `189`, `366`) | IBKR onboarding (`299`, `$2,000 USD` funding note for live market data), Weekly (`W`) Style `1` analysis combining **SRT** with **Weinstein Market Phases** (`189`, `366` Videos `1085482134` & `1085482351`):<br>- Always begin on **Style `1` (`M` then `W`)** before stepping down to `D` (`30m -> 1m` for timing).<br>- **Phase 1:** Basing; **Phase 2:** Uptrend (primary long target; `Phase 3` sideways consolidation inside long-term Phase 2 often resolves upward if fundamentals hold); **Phase 4:** Downtrend (strictly avoid buying).<br>- **Recent pivots outweigh ~8-year-old pivots**; projected targets measure prior box height (`grandeur du palier`) added above breakout resistance. |
| **`Cours-03`** (`195`, `197`, `196`, `198`, `313`) | Top-down market/sector lists (`40576813` `CBLT - MARCHÉS`, US/CA sector `we.tl` lists); Daily (`D`) multi-template sweep across **Styles `1–5`** (`198`); crossover & momentum examples (`313`) combining `SMA 30/200`, `EMA 10/30` (Style `3` hidden price), `EMA 9-21-55` (Style `3T`), `OBV`, `MACD`, and `SMA 50/200` (`Golden/Death Cross`) on `TXN`, `FIVE`, `ON`, `IONQ`, `ARM`, `NKE`, `GOOGL`, `MSFT`, `JOET`. |
| **`Cours-04`** (`225`, `301`) | Fundamental due diligence per ticker (`301`): fundamentals, news, products, and price history alongside technical setup. |
| **`Cours-05`** (`351`, `280`, `292`, `59`) | **`Méthode de travail 8.8`** (`351`); Daily (`D`) **`Area` (`Région`)** chart inspection (`292`) for 5 examples each of Head & Shoulders, Triangles, `Bull Flag`, Wedge (`Biseau`), `W`, and Cup & Handle, **Fibonacci retracements** (`5–10` titles), and watchlist bucketing (`Blue chips`, `Croissances`, `Spéculatifs`); short-selling preparation (`59`) isolating `5–10` **Weinstein Phase 4** tickers with fundamental checks (`P/E`, `Dividendes`, `Earnings`) + alerts. |
| **`Cours-06`** (`293`, `318`, `381`) | Post-cohort ecosystem progression (`318`, `381`): 6-week core $\rightarrow$ DayTrading $\rightarrow$ `CANAL-Cobalt` (`http://t.me/CobaltAdmin_bot`) + `Suivi Mensuel`. |

### 7.2 Locked Syllabus Titles (`Cours-01`..`Cours-06` & `trader_ibkr`)

- **Locked `Cours-01..06` Signal & Risk Taxonomy (`step_36.json`):** `Volume de transactions`, `Block Trade`, `Différent États de marché possible` (`Cours-01`); `Les phases de Weinstein`, `Stabilisation Phase 3 et continuité de Phase 2` (`Cours-02`); `La volatilité: Le VIX`, `Point de pivot / Tipping point`, `2e cycle haussier`, `Technique EMA 10-30`, `Exemples #1: Croisement EMA 10/30, VOB et MACD`, `Faux break out`, `Webull LEVEL 2` (`Cours-03`); `Blue Chips`/`Croissance`/`Spéculatifs`, `Dividendes`, `P/E ratio`, **`ATR`**, `Earnings USA/Canada`, `FINVIZ.COM`, `Changement majeur de tendance`, `5 Patterns à trader`, `Pattern #3 - Début de phase 2` (`Cours-04`); `Vente à découvert`, `short interest`, 12 chart patterns (`Tête Épaule`, `Tête Épaule inversé`, Triangles, `Bull/Bear Flag`, `Biseau`, `W`, `Cup/handle`, `Diamant`), `3 et 4 barres`, `trois soldats blancs`, `HEIKIN ASHI`, `Fibonacci` (`Cours-05`); `Tulipomanie`, `mouvement de masse`, `ETF Spider (SPDR)` / `iShares`, `Ordres d'achat et de vente`, **`Pattern du revirement graphique 30 min`**, **`Achat le jour suivant un break out`**, US dividend tax, `Gestion du risque` (`Sélection des titres`, `Gestion du capital`, `Momentum`, `Stratégie de sortie`, `% de gain pour reprendre une perte`) (`Cours-06`).
- **`IBKR Trader Workstation (TWS)` (`57` Items: `378` + `56` Locked):** Covers Paper Demo reset, `REER`/`CELI`/`CELIAPP`, `USD/CAD` (`FXCONV` pitfall FAQ), Market Data, TradingView-IBKR bridge, Mosaic/Charts/Watchlists/Scanner, Order Types (`Market`, `Limit`, `Stop`, `Stop-Limit`, `Bracket`, `Trailing-Stop`, `Trailing-Limit`, `OCA`, exit strategies), fast manual execution (**Hot Buttons** `Limit` & `Profit/Stop Loss`, custom button bar, **Hotkeys** 1–2, **Pre-Market/After-Hours**, **BookTrader**), and **`Autorisations de connexion API (Automation and Software Disclosure)`**.

---

## 8. "Vibe Trader" Behavioral Workflow & Codebase Gap Mapping

### 8.1 Operational Behavioral Patterns

1. **Top-Down Funnel (`13,805` $\rightarrow$ `1–3` Trades):** Macro/pre-market triage (`Regard Matinal` + `CBLT - MARCHÉS` + `SPDR` multi-screen `SCTR1/2`) $\rightarrow$ `Screener 2.0` (`Primary listing` OFF, `52W High` / `Golden Cross` / `EMA 10-30`) $\rightarrow$ `Méthode 8.8` 12-step filtering (`Blue Chip` / `Croissance` / `Spéculatif` / `Rejeté`) $\rightarrow$ `5–10` alert-armed candidates $\rightarrow$ `1–3` active positions moved to `($$$$)` (aligning with [Cobalt Trading Universe](./cobalt_trading_universe.md#L16-L23)).
2. **Noise-Stripped Visual Inspection:** Cycles `Styles 1..9` & `F` via single-letter toolbar pills (`0%` `Droite` in Style `3` to see pure `EMA 10/30` crosses, `100%` white `Area` for patterns, dual `F` / `F sans intervalle` to inspect `COBALT Ratios` without losing intraday timeframe).
3. **Alert-First Semi-Discretionary Execution:** Relies on `Once Only` S/R/Earnings alerts and `Once Per Bar` MA crossover alerts to stay patient across wide watchlists, then executes via rapid broker triggers (TWS Hot Buttons, Hotkeys, Brackets, BookTrader, or `tws_traderbot` armed gating).

### 8.2 Strategy Implementation Mapping (`backend/strategy/` vs. Cobalt Rules)

Comparing Cobalt's verified templates and `Méthode 8.8` against [`backend/models/strategy.go`](../../backend/models/strategy.go), [`backend/strategy/math.go`](../../backend/strategy/math.go), and [`backend/strategy/evaluator.go`](../../backend/strategy/evaluator.go) highlights exact technical gaps:

| Cobalt Indicator / Rule | Required Parameters & Timeframes | Current `tws_traderbot` Support (`HEAD`) |
|---|---|---|
| **`SMA`** (`Styles 2, 4, 5, 9, F` + Screener) | Lengths `13, 26, 30, 50, 100, 150, 200` across all 9 Cobalt intervals | **Implemented & Tested** in `backend/strategy/math.go` and `indicators.go` across all 9 timeframes (`1m`..`1M`). |
| **`EMA`** (`Styles 3, 3T, 4E, 7, 8` + Screener) | Lengths `3, 5, 8, 9, 10, 12, 15, 21, 30, 40, 50, 55, 60, 100, 150, 200, 250, 350` across all 9 intervals | **Implemented & Tested** in `backend/strategy/math.go` and `indicators.go` with multi-timeframe BarFrame support. |
| **Crossover & Slope Rules** | `CrossAbove`, `CrossBelow`, `GreaterThan`, `LessThan`, `SlopeAbove`, `SlopeBelow`, `WithinRange` | **Implemented & Tested** in `backend/strategy/evaluator.go` covering all 7 condition types. |
| **`MACD (12, 26, 9)`** (`Styles 3G, 3T, 4E`) | Fast `12`, Slow `26`, Signal `9` (`EMA`/`EMA`) | **Implemented & Tested** in `backend/strategy/indicators.go` (`ComputeMACD`). |
| **`RSI (14)`** (`Style 6`) | Length `14` (`Close`), `SMA(14)` smoothing, bands `70` / `30` | **Implemented & Tested** in `backend/strategy/indicators.go` (`ComputeRSI`). |
| **`Volume` + `Volume MA(100)` & `OBV`** (`Styles 1, 2, 3, 4E`) | Bar volume histogram + `SMA(Volume, 100)` + cumulative `OBV` | **Implemented & Tested** in `backend/strategy/indicators.go` (`ComputeOBV`, `ComputeRVOL`). |
| **`BB (20, 2)`, `Stoch (15, 5, 5)`, `Alligator`** (`Styles 3G, 6`) | `SMA(20) ± 2σ`, `%K(15,5)/%D(5)` (`80/20`), Alligator `13/8/5` (offsets `8/5/3`) | **Implemented & Tested** in `backend/strategy/indicators.go` (`ComputeBollingerBands`, `ComputeStochastic`, `ComputeWilliamsAlligator` / `CalculateAlligator`). |
| **`Guppy MMA` & `Heikin-Ashi`** (`Style 3G`, `Cours-05`) | Weekly fast EMAs (3..15), slow EMAs (30..60), anchor 200; smoothed HA bars | **Implemented & Tested** in `backend/strategy/indicators.go` (`ComputeGuppyMMA`, `ComputeHeikinAshi`). |
| **`Ticker / SPX` Relative Perf (`100`)** (`Styles 5, 9, F`) | Cross-symbol ratio vs. `SPX` (`100` lookback) | **Implemented & Tested** in `backend/strategy/indicators.go` (`ComputeRelativeStrength`). |
| **Volatility (`ATR 14` & Volatility %)** | 14-period ATR and >5% volatility guard | **Implemented & Tested** in `backend/strategy/indicators.go` (`ComputeATR`). |
| **20 Presets & 22 `IndicatorType` Constants** | All 13 chart styles, 4 Screener 2.0 presets, 3 Cours setups | **Implemented & Tested** pre-seeded into SQLite in `backend/strategy/catalog.go`; `backend/models/strategy.go` declares all 22 `IndicatorType` constants (`ml_signal` retained only as a legacy schema fallback to `close` series, unseeded in presets). |

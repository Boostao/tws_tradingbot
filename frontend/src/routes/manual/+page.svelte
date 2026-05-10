<script lang="ts">
    import { BookOpen, Plug, LayoutGrid, Activity, List, GitBranch, ExternalLink } from 'lucide-svelte';
    import { language, t } from '$lib/i18n';
    
    $: lang = $language;
</script>

<h1 class="heading">
    <span class="heading-icon"><BookOpen size={20} strokeWidth={1.6} /></span>
    {lang && t('user_manual') || 'User Manual'}
</h1>

<div class="card" style="margin-bottom: 2rem;">
    <h2>
        <span style="display:inline-flex; align-items:center; gap: 0.5rem; color: var(--accent);">
            <Plug size={20} strokeWidth={1.6}/> TWS Connection Configuration
        </span>
    </h2>
    <p>
        Before the bot can execute any trades or read market data, it must be securely connected to Interactive Brokers' Trader Workstation (TWS) or IB Gateway.
        You can configure this connection directly from the bottom-left corner of the sidebar under the <strong>"Connection"</strong> tab.
    </p>
    <ul style="line-height: 1.6; margin-bottom: 1rem; margin-top: 0.5rem; padding-left: 1.5rem;">
        <li><strong>Host:</strong> Usually <code>127.0.0.1</code> locally.</li>
        <li><strong>Port:</strong> By default, this is <code>7497</code> for Paper Trading and <code>7496</code> for Live Trading in TWS.</li>
        <li><strong>Client ID:</strong> An arbitrary ID (e.g. <code>1</code>). Make sure it doesn't conflict with other plugins.</li>
        <li><strong>Trading Mode:</strong> Important! Ensure this matches your TWS session (Paper vs. Live) to prevent accidental real-money orders.</li>
    </ul>

    <p>
        <strong>Important TWS Settings:</strong> For the bot to connect, TWS must be configured to accept API connections.
        Inside TWS, go to <strong>File &rarr; Global Configuration &rarr; API &rarr; Settings</strong>, and ensure <em>"Enable ActiveX and Socket Clients"</em> is checked.
    </p>
    <p>
        <a href="https://www.interactivebrokers.com/campus/ibkr-api-page/twsapi-doc/#tws-config-api" target="_blank" rel="noopener noreferrer" style="display: inline-flex; align-items: center; gap: 0.3rem; color: var(--accent); text-decoration: underline;">
            Read the official IBKR TWS API Configuration Guide <ExternalLink size={14} />
        </a>
    </p>
</div>

<div class="card" style="margin-bottom: 2rem;">
    <h2>
        <span style="display:inline-flex; align-items:center; gap: 0.5rem; color: var(--accent);">
            <LayoutGrid size={20} strokeWidth={1.6}/> Cockpit
        </span>
    </h2>
    <p>
        The <strong>Cockpit</strong> is your main control center. It dictates <em>what</em> the bot is going to trade. You will spend most of your time here assigning strategies to symbols.
    </p>
    <ul style="line-height: 1.6; padding-left: 1.5rem; padding-top: 0.5rem;">
        <li><strong>Workspaces:</strong> You can create multiple workspaces (like tabs) to group different categories of symbols or different trading approaches.</li>
        <li><strong>Slots (Symbol Rows):</strong> Each row represents a specific market symbol (e.g., AAPL). For each symbol, you can manually assign a strategy that the bot will use.</li>
        <li><strong>Strategy Activation:</strong> The small "play/pause" toggle next to each strategy allows you to enable or suspend the strategy for that specific symbol. It gives you fine-grained control if you want to sit out of a market temporarily.</li>
        <li><strong>Global System Toggle:</strong> The large toggle at the top right acts as a master "kill switch." If it's disabled, no trades will go through anywhere. Turn this on when you are ready to let the bot operate freely based on your assigned strategies.</li>
    </ul>
</div>

<div class="card" style="margin-bottom: 2rem;">
    <h2>
        <span style="display:inline-flex; align-items:center; gap: 0.5rem; color: var(--accent);">
            <Activity size={20} strokeWidth={1.6}/> Monitoring
        </span>
    </h2>
    <p>
        The <strong>Monitoring</strong> page provides a bird's-eye view of your account health and the bot's real-time actions.
    </p>
    <ul style="line-height: 1.6; padding-left: 1.5rem; padding-top: 0.5rem;">
        <li><strong>Top Dashboard:</strong> This presents a summary of your account balance, day's profits and losses, and the number of trades the bot has executed today.</li>
        <li><strong>System Status:</strong> Shows whether the internal engine is catching data anomalies, and if the overall rule computation is running smoothly.</li>
        <li><strong>Activity Logs:</strong> Instead of rummaging through technical text files, important bot decisions (like order submissions, filled trades, or errors) will appear in a neat list here.</li>
        <li><strong>When to use:</strong> Keep this page open on a second monitor while the bot is running to passively ensure everything is ticking along nicely.</li>
    </ul>
</div>

<div class="card" style="margin-bottom: 2rem;">
    <h2>
        <span style="display:inline-flex; align-items:center; gap: 0.5rem; color: var(--accent);">
            <List size={20} strokeWidth={1.6}/> Watchlist
        </span>
    </h2>
    <p>
        The <strong>Watchlist</strong> acts as an address book of market symbols that the bot is actively tracking. The bot needs to load data for symbols before it can trade them.
    </p>
    <ul style="line-height: 1.6; padding-left: 1.5rem; padding-top: 0.5rem;">
        <li><strong>Manual Entries:</strong> You can add symbols one-by-one safely into custom groups to keep them organized (e.g., Tech Stocks, Commodities).</li>
        <li><strong>Feeds and Imports:</strong> You can paste an external URL (such as a TradingView screener link) or upload a CSV file to automatically populate your watchlist with hundreds of tickers instantly.</li>
        <li><strong>Visibility:</strong> Symbols added here will become available to select inside your Cockpit.</li>
    </ul>
</div>

<div class="card">
    <h2>
        <span style="display:inline-flex; align-items:center; gap: 0.5rem; color: var(--accent);">
            <GitBranch size={20} strokeWidth={1.6}/> Strategy Builder
        </span>
    </h2>
    <p>
        The <strong>Strategy Builder</strong> allows you to craft the actual intelligence and rules that the bot uses to make decisions.
    </p>
    <ul style="line-height: 1.6; padding-left: 1.5rem; padding-top: 0.5rem;">
        <li><strong>Indicators:</strong> Define the mathematical tools (like Moving Averages or RSI) you want the strategy to be aware of.</li>
        <li><strong>Entry Rules:</strong> Define the exact conditions that must be met in for the bot to buy an asset (e.g., 'When the price crosses over the Moving Average, buy').</li>
        <li><strong>Exit Rules:</strong> Define when the bot should sell an asset to take profit or cut its losses.</li>
        <li><strong>Validation:</strong> The button at the top right allows you to test your strategy's logic to make sure there are no typos or impossible conditions before deploying it to real money.</li>
    </ul>
</div>

<style>
    h2 {
        margin-top: 0;
        margin-bottom: 1rem;
        font-size: 1.25rem;
    }
    p {
        line-height: 1.6;
        margin-bottom: 1rem;
        color: var(--text-2, #d1d5db);
    }
    ul li {
        color: var(--text-2, #d1d5db);
        margin-bottom: 0.5rem;
    }
</style>

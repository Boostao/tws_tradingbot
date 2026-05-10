<script lang="ts">
    import { onMount } from 'svelte';
    import { getWatchlist } from '$lib/api';
    
    let watchlist = null;

    onMount(async () => {
        watchlist = await getWatchlist();
    });
</script>

<div class="dashboard">
    <div class="panel left-panel pulse-feed">
        <h2>The Pulse (Dynamic Watchlist)</h2>
        {#if watchlist && watchlist.groups}
            {#each watchlist.groups as group}
                <div class="watchlist-group">
                    <h3>{group.name}</h3>
                    {#each group.items as item}
                        <div class="watchlist-item">
                            <span class="symbol">{item.symbol}</span>
                            <span class="exchange">{item.exchange}</span>
                            <label class="switch">
                                <input type="checkbox" checked={item.enabled}>
                                <span class="slider"></span>
                            </label>
                        </div>
                    {/each}
                </div>
            {/each}
        {/if}
    </div>

    <div class="panel center-panel cockpit-matrix">
        <h2>The Cockpit Matrix</h2>
        <p>Heatmap overlaying Active Strategies × Rule Conditions</p>
        <div class="matrix-grid">
            <!-- Grid representation -->
        </div>
    </div>

    <div class="panel bottom-panel execution-terminal">
        <h2>Terminal / Execution Ledger</h2>
        <div class="ledger-scroller">
            <p>[10:02:45] BOUGHT 100 TSLA @ 145.20</p>
            <p>[10:03:12] SOLD 50 MSFT @ 180.30</p>
        </div>
    </div>
</div>

<style>
.dashboard {
    display: grid;
    grid-template-columns: 300px 1fr;
    grid-template-rows: 1fr 200px;
    gap: 10px;
    height: 100%;
}
.panel {
    background: #1e1e1e;
    border: 1px solid #333;
    border-radius: 8px;
    padding: 15px;
    overflow-y: auto;
}
.left-panel {
    grid-column: 1 / 2;
    grid-row: 1 / 3;
}
.center-panel {
    grid-column: 2 / 3;
    grid-row: 1 / 2;
}
.bottom-panel {
    grid-column: 2 / 3;
    grid-row: 2 / 3;
}

h2 {
    margin-top: 0;
    font-size: 1.1rem;
    border-bottom: 1px solid #444;
    padding-bottom: 5px;
}

/* Switches */
.switch {
  position: relative;
  display: inline-block;
  width: 40px;
  height: 20px;
}
.switch input { 
  opacity: 0;
  width: 0;
  height: 0;
}
.slider {
  position: absolute;
  cursor: pointer;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background-color: #ccc;
  transition: .4s;
  border-radius: 20px;
}
.slider:before {
  position: absolute;
  content: "";
  height: 16px;
  width: 16px;
  left: 2px;
  bottom: 2px;
  background-color: white;
  transition: .4s;
  border-radius: 50%;
}
input:checked + .slider {
  background-color: #2196F3;
}
input:checked + .slider:before {
  transform: translateX(20px);
}
.watchlist-item {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 8px 0;
    border-bottom: 1px solid #2a2a2a;
}
.matrix-grid {
    height: 100%;
    border: 1px dashed #444;
}
.ledger-scroller {
    font-family: monospace;
    font-size: 0.9rem;
    color: #4CAF50;
}
</style>

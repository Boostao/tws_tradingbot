<script lang="ts">
	import { Compass, HelpCircle, ShieldCheck, SlidersHorizontal, Terminal, X } from 'lucide-svelte';
	import { language, t } from '$lib/i18n';
	import { closeHelpDrawer, helpDrawerOpen, launchTour } from '$lib/stores/onboarding';

	$: lang = $language;
</script>

{#if $helpDrawerOpen}
	<div class="help-scrim" on:click={closeHelpDrawer} aria-hidden="true"></div>
	<div
		class="help-drawer"
		data-testid="help-drawer"
		role="dialog"
		aria-modal="true"
		aria-label={lang && t('help_drawer_title')}
	>
		<header class="help-drawer-header">
			<div class="help-drawer-heading">
				<span class="help-kicker font-mono">F1 · COBALT V2</span>
				<h2>
					<HelpCircle size={18} strokeWidth={1.8} />
					<span>{lang && t('help_drawer_title')}</span>
				</h2>
				<p class="help-subtitle">{lang && t('help_drawer_subtitle')}</p>
			</div>
			<button
				type="button"
				class="secondary help-close-icon"
				aria-label={lang && t('help_close_aria')}
				on:click={closeHelpDrawer}
			>
				<X size={16} strokeWidth={2} />
			</button>
		</header>

		<div class="help-drawer-body">
			<!-- 1. Glyph & Semantic Color Legend -->
			<section class="help-section">
				<div class="help-section-title">
					<span class="help-section-dot dot-cyan"></span>
					<h3>{lang && t('help_sec_glyphs_title')}</h3>
				</div>
				<div class="help-legend-grid">
					<div class="help-legend-row">
						<span class="help-glyph glyph-bull font-mono">$$$$</span>
						<span class="help-legend-copy">{lang && t('help_glyph_holding_desc')}</span>
					</div>
					<div class="help-legend-row">
						<span class="help-glyph glyph-orange font-mono">📌</span>
						<span class="help-legend-copy">{lang && t('help_glyph_pin_desc')}</span>
					</div>
					<div class="help-legend-row">
						<span class="help-glyph glyph-cyan font-mono"># / $ / %</span>
						<span class="help-legend-copy">{lang && t('help_glyph_sizing_desc')}</span>
					</div>
					<div class="help-legend-row">
						<span class="help-glyph glyph-warn font-mono">RTH / ETH</span>
						<span class="help-legend-copy">{lang && t('help_glyph_session_desc')}</span>
					</div>
					<div class="help-legend-row">
						<span class="help-glyph glyph-brand font-mono">1/1</span>
						<span class="help-legend-copy">{lang && t('help_glyph_exclusive_desc')}</span>
					</div>
				</div>
				<div class="help-tier-strip">
					<span class="help-tier-label">{lang && t('help_tiers_label')}</span>
					<div class="help-tier-pills">
						<span class="help-tier-pill tier-1">1-Blue Chip</span>
						<span class="help-tier-pill tier-2">2-Croissance</span>
						<span class="help-tier-pill tier-3">3-Spéculatif</span>
					</div>
				</div>
			</section>

			<!-- 2. 4-Level Safety Gate & Daily Loss Breaker -->
			<section class="help-section">
				<div class="help-section-title">
					<ShieldCheck size={15} strokeWidth={1.8} />
					<h3>{lang && t('help_sec_gates_title')}</h3>
				</div>
				<ol class="help-gate-list">
					<li>
						<strong class="font-mono">1.</strong>
						<span>{lang && t('help_gate_level1')}</span>
					</li>
					<li>
						<strong class="font-mono">2.</strong>
						<span>{lang && t('help_gate_level2')}</span>
					</li>
					<li>
						<strong class="font-mono">3.</strong>
						<span>{lang && t('help_gate_level3')}</span>
					</li>
					<li>
						<strong class="font-mono">4.</strong>
						<span>{lang && t('help_gate_level4')}</span>
					</li>
				</ol>
				<p class="help-callout">{lang && t('help_breaker_note')}</p>
			</section>

			<!-- 3. Slide-to-Arm (>= 78%) & Sizing Tips -->
			<section class="help-section">
				<div class="help-section-title">
					<SlidersHorizontal size={15} strokeWidth={1.8} />
					<h3>{lang && t('help_sec_arming_title')}</h3>
				</div>
				<ul class="help-bullet-list">
					<li>{lang && t('help_arming_slide_tip')}</li>
					<li>{lang && t('help_arming_sizing_tip')}</li>
				</ul>
			</section>

			<!-- 4. TWS Quick Fixes (10089, FXCONV, 504) -->
			<section class="help-section">
				<div class="help-section-title">
					<Terminal size={15} strokeWidth={1.8} />
					<h3>{lang && t('help_sec_tws_title')}</h3>
				</div>
				<div class="help-fix-list">
					<div class="help-fix-card">
						<span class="help-fix-code font-mono">10089</span>
						<p>{lang && t('help_tws_10089_desc')}</p>
					</div>
					<div class="help-fix-card">
						<span class="help-fix-code font-mono">FXCONV</span>
						<p>{lang && t('help_tws_fxconv_desc')}</p>
					</div>
					<div class="help-fix-card">
						<span class="help-fix-code font-mono">504 · 7497 / 7496</span>
						<p>{lang && t('help_tws_504_desc')}</p>
					</div>
				</div>
			</section>
		</div>

		<footer class="help-drawer-footer">
			<div class="help-tour-row-label">
				<Compass size={14} strokeWidth={1.8} />
				<span>{lang && t('help_tours_heading')}</span>
			</div>
			<div class="help-tour-actions">
				<button type="button" class="secondary help-tour-btn" on:click={() => launchTour(1)}>
					{lang && t('help_launch_track1')}
				</button>
				<button type="button" class="secondary help-tour-btn" on:click={() => launchTour(2)}>
					{lang && t('help_launch_track2')}
				</button>
				<button type="button" class="secondary help-tour-btn" on:click={() => launchTour(3)}>
					{lang && t('help_launch_track3')}
				</button>
			</div>
			<div class="help-footer-close-row">
				<span class="help-shortcut-hint font-mono">{lang && t('help_shortcut_hint')}</span>
				<button type="button" class="secondary" on:click={closeHelpDrawer}>
					{lang && t('help_close_btn')}
				</button>
			</div>
		</footer>
	</div>
{/if}

<style>
	.help-scrim {
		position: fixed;
		inset: 0;
		background: rgba(5, 7, 11, 0.62);
		backdrop-filter: blur(2px);
		z-index: 199;
	}

	.help-drawer {
		position: fixed;
		inset: 0 0 0 auto;
		width: min(440px, 92vw);
		z-index: 200;
		background: var(--bg-surface, #0b111e);
		border-left: 1px solid var(--cobalt-brand, #263c8a);
		box-shadow: -16px 0 40px rgba(0, 0, 0, 0.55);
		display: flex;
		flex-direction: column;
		font-family: var(--font-sans);
		color: var(--text-primary, #e6edf3);
	}

	.help-drawer-header {
		display: flex;
		align-items: flex-start;
		justify-content: space-between;
		gap: 12px;
		padding: 16px 18px 14px;
		border-bottom: 1px solid var(--border-subtle, #1f2937);
		background: linear-gradient(180deg, rgba(38, 60, 138, 0.22), rgba(11, 17, 30, 0.96));
	}

	.help-drawer-heading {
		display: grid;
		gap: 4px;
	}

	.help-kicker {
		font-size: 11px;
		font-weight: 700;
		letter-spacing: 0.1em;
		color: var(--cobalt-orange, #faa031);
		text-transform: uppercase;
	}

	.help-drawer-heading h2 {
		margin: 0;
		display: inline-flex;
		align-items: center;
		gap: 8px;
		font-size: 18px;
		font-weight: 700;
		color: var(--text-primary, #e6edf3);
	}

	.help-subtitle {
		margin: 0;
		font-size: 12px;
		line-height: 1.45;
		color: var(--text-secondary, #94a3b8);
	}

	.help-close-icon {
		padding: 6px;
		display: inline-flex;
		align-items: center;
		justify-content: center;
		flex-shrink: 0;
	}

	.help-drawer-body {
		flex: 1 1 auto;
		min-height: 0;
		overflow-y: auto;
		padding: 14px 18px;
		display: grid;
		gap: 12px;
		align-content: start;
	}

	.help-section {
		background: var(--bg-card, #0f1726);
		border: 1px solid var(--border-subtle, #1f2937);
		border-radius: 10px;
		padding: 12px 13px;
		display: grid;
		gap: 9px;
	}

	.help-section-title {
		display: flex;
		align-items: center;
		gap: 8px;
		color: var(--cobalt-cyan, #38bdf8);
	}

	.help-section-title h3 {
		margin: 0;
		font-size: 13px;
		font-weight: 700;
		letter-spacing: 0.02em;
		color: var(--text-primary, #e6edf3);
	}

	.help-section-dot {
		width: 8px;
		height: 8px;
		border-radius: 999px;
		background: var(--cobalt-cyan, #38bdf8);
		flex-shrink: 0;
	}

	.help-legend-grid {
		display: grid;
		gap: 7px;
	}

	.help-legend-row {
		display: grid;
		grid-template-columns: 78px minmax(0, 1fr);
		align-items: center;
		gap: 8px;
		font-size: 12px;
		line-height: 1.4;
		color: var(--text-secondary, #94a3b8);
	}

	.help-glyph {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		padding: 2px 6px;
		border-radius: 6px;
		font-size: 11px;
		font-weight: 700;
		border: 1px solid var(--border-highlight, #2f3b52);
		background: var(--bg-void, #05070b);
		white-space: nowrap;
	}

	.glyph-bull {
		color: var(--sem-bull-text, #4caf50);
		border-color: rgba(16, 185, 129, 0.4);
	}

	.glyph-orange {
		color: var(--cobalt-orange, #faa031);
		border-color: rgba(250, 160, 49, 0.45);
	}

	.glyph-cyan {
		color: var(--cobalt-cyan, #38bdf8);
		border-color: rgba(56, 189, 248, 0.4);
	}

	.glyph-warn {
		color: var(--sem-warn-text, #ffb74d);
		border-color: rgba(250, 160, 49, 0.4);
	}

	.glyph-brand {
		color: var(--cobalt-link, #8ab4f8);
		border-color: var(--cobalt-brand, #263c8a);
	}

	.help-tier-strip {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 8px;
		flex-wrap: wrap;
		padding-top: 6px;
		border-top: 1px solid var(--border-subtle, #1f2937);
	}

	.help-tier-label {
		font-size: 11px;
		color: var(--text-muted, #64748b);
		font-weight: 600;
	}

	.help-tier-pills {
		display: inline-flex;
		gap: 6px;
		flex-wrap: wrap;
	}

	.help-tier-pill {
		font-size: 11px;
		font-weight: 700;
		padding: 2px 7px;
		border-radius: 999px;
		background: var(--bg-void, #05070b);
		border: 1px solid var(--border-subtle, #1f2937);
	}

	.help-tier-pill.tier-1 {
		color: var(--tier-1-bluechip, #38bdf8);
		border-color: rgba(56, 189, 248, 0.4);
	}

	.help-tier-pill.tier-2 {
		color: var(--tier-2-growth, #c084fc);
		border-color: rgba(192, 132, 252, 0.4);
	}

	.help-tier-pill.tier-3 {
		color: var(--tier-3-speculative, #faa031);
		border-color: rgba(250, 160, 49, 0.4);
	}

	.help-gate-list {
		margin: 0;
		padding: 0;
		list-style: none;
		display: grid;
		gap: 6px;
	}

	.help-gate-list li {
		display: flex;
		gap: 8px;
		font-size: 12px;
		line-height: 1.45;
		color: var(--text-secondary, #94a3b8);
	}

	.help-gate-list strong {
		color: var(--cobalt-cyan, #38bdf8);
		flex-shrink: 0;
	}

	.help-callout {
		margin: 0;
		padding: 8px 10px;
		border-radius: 8px;
		background: rgba(250, 160, 49, 0.08);
		border: 1px solid rgba(250, 160, 49, 0.35);
		font-size: 12px;
		line-height: 1.45;
		color: var(--sem-warn-text, #ffb74d);
	}

	.help-bullet-list {
		margin: 0;
		padding-left: 16px;
		display: grid;
		gap: 6px;
		font-size: 12px;
		line-height: 1.45;
		color: var(--text-secondary, #94a3b8);
	}

	.help-fix-list {
		display: grid;
		gap: 8px;
	}

	.help-fix-card {
		background: var(--bg-void, #05070b);
		border: 1px solid var(--border-subtle, #1f2937);
		border-radius: 8px;
		padding: 8px 10px;
		display: grid;
		gap: 4px;
	}

	.help-fix-code {
		font-size: 11px;
		font-weight: 700;
		color: var(--cobalt-orange, #faa031);
	}

	.help-fix-card p {
		margin: 0;
		font-size: 12px;
		line-height: 1.45;
		color: var(--text-secondary, #94a3b8);
	}

	.help-drawer-footer {
		padding: 12px 18px 14px;
		border-top: 1px solid var(--border-subtle, #1f2937);
		background: var(--bg-card, #0f1726);
		display: grid;
		gap: 10px;
	}

	.help-tour-row-label {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		font-size: 12px;
		font-weight: 700;
		color: var(--text-primary, #e6edf3);
	}

	.help-tour-actions {
		display: grid;
		grid-template-columns: repeat(3, minmax(0, 1fr));
		gap: 6px;
	}

	.help-tour-btn {
		padding: 7px 8px;
		font-size: 11px;
		line-height: 1.25;
		text-align: center;
	}

	.help-footer-close-row {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 10px;
	}

	.help-shortcut-hint {
		font-size: 11px;
		color: var(--text-muted, #64748b);
	}
</style>

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
            <Plug size={20} strokeWidth={1.6}/> {lang && t('manual_tws_config_title')}
        </span>
    </h2>
    <p>{lang && t('manual_tws_config_p1')}</p>
    <ul style="line-height: 1.6; margin-bottom: 1rem; margin-top: 0.5rem; padding-left: 1.5rem;">
        <li><strong>{lang && t('manual_tws_config_host')}</strong> {lang && t('manual_tws_config_host_desc')}</li>
        <li><strong>{lang && t('manual_tws_config_port')}</strong> {lang && t('manual_tws_config_port_desc')}</li>
        <li><strong>{lang && t('manual_tws_config_client_id')}</strong> {lang && t('manual_tws_config_client_id_desc')}</li>
        <li><strong>{lang && t('manual_tws_config_trading_mode')}</strong> {lang && t('manual_tws_config_trading_mode_desc')}</li>
    </ul>

    <p>
        <strong>{lang && t('manual_tws_config_settings_title')}</strong> {lang && t('manual_tws_config_settings_desc')}
    </p>
    <p>
        <a href="https://www.interactivebrokers.com/campus/ibkr-api-page/twsapi-doc/#tws-config-api" target="_blank" rel="noopener noreferrer" style="display: inline-flex; align-items: center; gap: 0.3rem; color: var(--accent); text-decoration: underline;">
            {lang && t('manual_tws_config_read_docs')} <ExternalLink size={14} />
        </a>
    </p>
</div>

<div class="card" style="margin-bottom: 2rem;">
    <h2>
        <span style="display:inline-flex; align-items:center; gap: 0.5rem; color: var(--accent);">
            <LayoutGrid size={20} strokeWidth={1.6}/> {lang && t('manual_cockpit_title')}
        </span>
    </h2>
    <p>{lang && t('manual_cockpit_p1')}</p>
    <ul style="line-height: 1.6; padding-left: 1.5rem; padding-top: 0.5rem;">
        <li><strong>{lang && t('manual_cockpit_workspaces')}</strong> {lang && t('manual_cockpit_workspaces_desc')}</li>
        <li><strong>{lang && t('manual_cockpit_slots')}</strong> {lang && t('manual_cockpit_slots_desc')}</li>
        <li><strong>{lang && t('manual_cockpit_strategy_act')}</strong> {lang && t('manual_cockpit_strategy_act_desc')}</li>
        <li><strong>{lang && t('manual_cockpit_global')}</strong> {lang && t('manual_cockpit_global_desc')}</li>
    </ul>
</div>

<div class="card" style="margin-bottom: 2rem;">
    <h2>
        <span style="display:inline-flex; align-items:center; gap: 0.5rem; color: var(--accent);">
            <Activity size={20} strokeWidth={1.6}/> {lang && t('manual_monitoring_title')}
        </span>
    </h2>
    <p>{lang && t('manual_monitoring_p1')}</p>
    <ul style="line-height: 1.6; padding-left: 1.5rem; padding-top: 0.5rem;">
        <li><strong>{lang && t('manual_monitoring_top')}</strong> {lang && t('manual_monitoring_top_desc')}</li>
        <li><strong>{lang && t('manual_monitoring_status')}</strong> {lang && t('manual_monitoring_status_desc')}</li>
        <li><strong>{lang && t('manual_monitoring_logs')}</strong> {lang && t('manual_monitoring_logs_desc')}</li>
        <li><strong>{lang && t('manual_monitoring_when')}</strong> {lang && t('manual_monitoring_when_desc')}</li>
    </ul>
</div>

<div class="card" style="margin-bottom: 2rem;">
    <h2>
        <span style="display:inline-flex; align-items:center; gap: 0.5rem; color: var(--accent);">
            <List size={20} strokeWidth={1.6}/> {lang && t('manual_watchlist_title')}
        </span>
    </h2>
    <p>{lang && t('manual_watchlist_p1')}</p>
    <ul style="line-height: 1.6; padding-left: 1.5rem; padding-top: 0.5rem;">
        <li><strong>{lang && t('manual_watchlist_manual')}</strong> {lang && t('manual_watchlist_manual_desc')}</li>
        <li><strong>{lang && t('manual_watchlist_feeds')}</strong> {lang && t('manual_watchlist_feeds_desc')}</li>
        <li><strong>{lang && t('manual_watchlist_visibility')}</strong> {lang && t('manual_watchlist_visibility_desc')}</li>
    </ul>
</div>

<div class="card">
    <h2>
        <span style="display:inline-flex; align-items:center; gap: 0.5rem; color: var(--accent);">
            <GitBranch size={20} strokeWidth={1.6}/> {lang && t('manual_strategy_title')}
        </span>
    </h2>
    <p>{lang && t('manual_strategy_p1')}</p>
    <ul style="line-height: 1.6; padding-left: 1.5rem; padding-top: 0.5rem;">
        <li><strong>{lang && t('manual_strategy_indicators')}</strong> {lang && t('manual_strategy_indicators_desc')}</li>
        <li><strong>{lang && t('manual_strategy_entry')}</strong> {lang && t('manual_strategy_entry_desc')}</li>
        <li><strong>{lang && t('manual_strategy_exit')}</strong> {lang && t('manual_strategy_exit_desc')}</li>
        <li><strong>{lang && t('manual_strategy_validation')}</strong> {lang && t('manual_strategy_validation_desc')}</li>
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

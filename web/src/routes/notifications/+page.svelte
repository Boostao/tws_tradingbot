<script lang="ts">
	import { onDestroy, onMount } from 'svelte';
	import { createReconnectingWebSocket } from '$lib/ws';
	import {
		clearCache,
		formatApiError,
		getConfig,
		getNotifications,
		getRestMetric,
		testNotification,
		updateConfig
	} from '$lib/api';
	import { t, language } from '$lib/i18n';
	import { Bell, Settings, Send, MessageCircle, BellRing, Inbox } from 'lucide-svelte';

	type TelegramConfig = {
		enabled: boolean;
		bot_token: string;
		chat_id: string;
		commands_enabled: boolean;
		poll_interval: number;
	};

	type DiscordConfig = {
		enabled: boolean;
		webhook_url: string;
	};

	type NotificationsConfig = {
		enabled: boolean;
		telegram: TelegramConfig;
		discord: DiscordConfig;
	};

	let notificationsEnabled = false;
	let telegramEnabled = false;
	let telegramBotToken = '';
	let telegramChatId = '';
	let telegramCommandsEnabled = false;
	let telegramPollInterval = 5;
	let discordEnabled = false;
	let discordWebhookUrl = '';
	let testMessage = 'Test notification from Cobalt Traderbot';
	let testChannel: '' | 'telegram' | 'discord' = '';
	let message = '';
	let status = '';
	let loading = true;
	let events: Array<{ id: string; message: string; level?: string; channel?: string; created_at?: string }> = [];
	let total = 0;
	let page = 1;
	let pageSize = 25;
	const pageSizes = [10, 25, 50, 100, 200];
	let unseenCount = 0;
	let connected = false;
	let reconnects = 0;
	let lastEventAt: number | null = null;
	let lastCloseInfo: string | null = null;
	let lastErrorAt: number | null = null;
	let streamError = '';
	let restLastMs: number | null = null;
	let restLastAt: number | null = null;
	let connection: { close: () => void } | null = null;
	$: _lang = $language;

	function normalizeConfig(config: Record<string, unknown>) {
		const notifications = (config.notifications ?? {}) as NotificationsConfig;
		notificationsEnabled = !!notifications.enabled;
		telegramEnabled = !!notifications.telegram?.enabled;
		telegramBotToken = notifications.telegram?.bot_token === '***' ? '' : notifications.telegram?.bot_token || '';
		telegramChatId = notifications.telegram?.chat_id || '';
		telegramCommandsEnabled = !!notifications.telegram?.commands_enabled;
		telegramPollInterval = Number(notifications.telegram?.poll_interval ?? 5);
		discordEnabled = !!notifications.discord?.enabled;
		discordWebhookUrl = notifications.discord?.webhook_url === '***' ? '' : notifications.discord?.webhook_url || '';
	}

	async function loadConfig(force = false) {
		loading = true;
		message = '';
		try {
			const config = await getConfig(force);
			normalizeConfig(config);
			captureRestMetric('config.get');
			await loadPage(page);
		} catch (err) {
			message = formatApiError(err);
		} finally {
			loading = false;
		}
	}

	async function loadPage(nextPage = page) {
		message = '';
		try {
			const existing = await getNotifications(nextPage, pageSize);
			events = (existing.events as typeof events) ?? [];
			total = existing.total ?? events.length;
			page = existing.page ?? nextPage;
			captureRestMetric('notifications.list');
			if (page === 1) {
				unseenCount = 0;
			}
			if (page > Math.max(1, Math.ceil(total / pageSize))) {
				page = Math.max(1, Math.ceil(total / pageSize));
				if (page !== nextPage) {
					const fallback = await getNotifications(page, pageSize);
					events = (fallback.events as typeof events) ?? [];
					total = fallback.total ?? events.length;
					captureRestMetric('notifications.list');
				}
			}
		} catch (err) {
			message = formatApiError(err);
		}
	}

	function connect() {
		connection = createReconnectingWebSocket('/ws/notifications', {
			onMessage: (event) => {
				const payload = JSON.parse(event.data) as {
					events?: typeof events;
					event?: (typeof events)[number];
				};
				if (payload.events && page === 1) {
					events = payload.events.slice(0, pageSize);
					total = Math.max(total, payload.events.length);
					unseenCount = 0;
				} else if (payload.event) {
					total += 1;
					if (page === 1) {
						events = [payload.event, ...events].slice(0, pageSize);
					} else {
						unseenCount += 1;
					}
				}
				lastEventAt = Date.now();
			}
			,
			onOpen: () => {
				connected = true;
				streamError = '';
			},
			onClose: (event) => {
				connected = false;
				reconnects += 1;
				lastCloseInfo = formatClose(event);
			},
			onError: () => {
				streamError = 'Notifications stream error';
				lastErrorAt = Date.now();
			}
		});
	}

	onMount(() => {
		loadConfig();
		connect();
	});

	onDestroy(() => {
		connection?.close();
	});

	async function handleSave() {
		message = '';
		status = '';
		try {
			await updateConfig({
				notifications: {
					enabled: notificationsEnabled,
					telegram: {
						enabled: telegramEnabled,
						bot_token: telegramBotToken || undefined,
						chat_id: telegramChatId || undefined,
						commands_enabled: telegramCommandsEnabled,
						poll_interval: telegramPollInterval
					},
					discord: {
						enabled: discordEnabled,
						webhook_url: discordWebhookUrl || undefined
					}
				}
			});
			status = t('saved');
			captureRestMetric('config.update');
			await loadConfig();
		} catch (err) {
			message = formatApiError(err);
		}
	}

	async function handleTest() {
		message = '';
		status = '';
		try {
			const result = await testNotification(testMessage, testChannel || undefined);
			status = result.status;
			captureRestMetric('notifications.test');
		} catch (err) {
			message = formatApiError(err);
		}
	}

	$: totalPages = Math.max(1, Math.ceil(total / pageSize));
	$: canPrev = page > 1;
	$: canNext = page < totalPages;

	function handlePageSizeChange(size: number) {
		pageSize = size;
		page = 1;
		loadPage(1);
	}

	const formatLatency = (timestamp: number | null) => {
		if (!timestamp) return '--';
		const delta = Math.max(0, Date.now() - timestamp);
		return `${Math.round(delta / 1000)}s`;
	};

	const formatMs = (value: number | null) =>
		value === null || Number.isNaN(value) ? '--' : `${Math.round(value)}ms`;

	const captureRestMetric = (key: string) => {
		const metric = getRestMetric(key);
		if (!metric) return;
		restLastMs = metric.lastMs;
		restLastAt = metric.lastAt;
	};

	const formatClose = (event?: CloseEvent) => {
		if (!event) return '--';
		const reason = event.reason ? ` - ${event.reason}` : '';
		return `${event.code}${reason}`;
	};
</script>

<h1 class="heading"><span class="heading-icon"><Bell size={20} strokeWidth={1.6} /></span>{t('notifications')}</h1>

{#if message}
	<p style="color: #f87171;">{message}</p>
{/if}

{#if streamError}
	<p style="color: #f87171;">{streamError}</p>
{/if}

{#if loading}
	<p class="muted">Loading notifications config...</p>
{:else}
	<div class="card">
		<h2 class="heading"><span class="heading-icon"><Settings size={18} strokeWidth={1.6} /></span>{t('global_settings')}</h2>
		<label style="display: flex; gap: 10px; align-items: center;">
			<input type="checkbox" bind:checked={notificationsEnabled} />
			Enable notifications
		</label>
	</div>

	<div class="card">
		<h2 class="heading"><span class="heading-icon"><Send size={18} strokeWidth={1.6} /></span>{t('telegram')}</h2>
		<label style="display: flex; gap: 10px; align-items: center;">
			<input type="checkbox" bind:checked={telegramEnabled} />
			Enable Telegram
		</label>
		<div style="display: grid; gap: 12px; grid-template-columns: repeat(auto-fit, minmax(220px, 1fr)); margin-top: 12px;">
			<label>
				Bot Token
				<input type="password" bind:value={telegramBotToken} placeholder="Set token" />
			</label>
			<label>
				Chat ID
				<input type="text" bind:value={telegramChatId} placeholder="123456789" />
			</label>
			<label style="display: flex; gap: 10px; align-items: center;">
				<input type="checkbox" bind:checked={telegramCommandsEnabled} />
				Commands Enabled
			</label>
			<label>
				Poll Interval (seconds)
				<input type="number" min="1" bind:value={telegramPollInterval} />
			</label>
		</div>
	</div>

	<div class="card">
		<h2 class="heading"><span class="heading-icon"><MessageCircle size={18} strokeWidth={1.6} /></span>{t('discord')}</h2>
		<label style="display: flex; gap: 10px; align-items: center;">
			<input type="checkbox" bind:checked={discordEnabled} />
			Enable Discord
		</label>
		<div style="display: grid; gap: 12px; grid-template-columns: repeat(auto-fit, minmax(220px, 1fr)); margin-top: 12px;">
			<label>
				Webhook URL
				<input type="password" bind:value={discordWebhookUrl} placeholder="https://discord.com/api/webhooks/..." />
			</label>
		</div>
	</div>

	<div class="card">
		<h2 class="heading"><span class="heading-icon"><BellRing size={18} strokeWidth={1.6} /></span>{t('test_notification')}</h2>
		<div style="display: grid; gap: 12px; grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));">
			<label>
				Message
				<input type="text" bind:value={testMessage} />
			</label>
			<label>
				Channel
				<select bind:value={testChannel}>
					<option value="">All</option>
					<option value="telegram">Telegram</option>
					<option value="discord">Discord</option>
				</select>
			</label>
		</div>
		<div style="margin-top: 12px; display: flex; gap: 8px;">
			<button on:click={handleTest}>Send Test</button>
			<button class="secondary" on:click={() => loadConfig(true)}>Reload (bypass cache)</button>
			<button class="secondary" on:click={handleSave}>{t('save')}</button>
			<button
				class="secondary"
				on:click={() => {
					clearCache('config');
					message = 'Cache cleared';
				}}
			>
				Clear cache
			</button>
		</div>
		{#if status}
			<p class="muted" style="margin-top: 8px;">{status}</p>
		{/if}
	</div>

	<div class="card">
		<h2 class="heading"><span class="heading-icon"><Inbox size={18} strokeWidth={1.6} /></span>{t('recent_alerts')}</h2>
		<div style="display: flex; gap: 8px; flex-wrap: wrap; margin-bottom: 10px;">
			<span class="badge">
				<span class={`status-dot ${connected ? '' : 'offline'}`}></span>
				Stream {connected ? 'connected' : 'disconnected'}
			</span>
			<span class="badge">Lag: {formatLatency(lastEventAt)}</span>
			<span class="badge">Reconnects: {reconnects}</span>
			<span class="badge">Last close: {lastCloseInfo ?? '--'}</span>
			<span class="badge">Last error: {formatLatency(lastErrorAt)}</span>
			<span class="badge">REST last: {formatMs(restLastMs)}</span>
			<span class="badge">REST age: {formatLatency(restLastAt)}</span>
		</div>
		<div style="display: flex; flex-wrap: wrap; gap: 8px; align-items: center; margin-bottom: 12px;">
			<button class="secondary" on:click={() => canPrev && loadPage(page - 1)} disabled={!canPrev}>
				Prev
			</button>
			<button class="secondary" on:click={() => canNext && loadPage(page + 1)} disabled={!canNext}>
				Next
			</button>
			<span class="muted">Page {page} of {totalPages}</span>
			{#if page !== 1 && unseenCount > 0}
				<span class="badge">{unseenCount} new</span>
			{/if}
			<label style="display: flex; gap: 6px; align-items: center;">
				<span class="muted">Per page</span>
				<select bind:value={pageSize} on:change={(e) => handlePageSizeChange(Number((e.target as HTMLSelectElement).value))}>
					{#each pageSizes as size}
						<option value={size}>{size}</option>
					{/each}
				</select>
			</label>
		</div>
		{#if events.length === 0}
			<p class="muted">No alerts yet.</p>
		{:else}
			<table class="table">
				<thead>
					<tr>
						<th>Time</th>
						<th>Channel</th>
						<th>Level</th>
						<th>Message</th>
					</tr>
				</thead>
				<tbody>
					{#each events as event}
						<tr>
							<td>{event.created_at ?? '--'}</td>
							<td>{event.channel ?? 'all'}</td>
							<td>{event.level ?? 'info'}</td>
							<td>{event.message}</td>
						</tr>
					{/each}
				</tbody>
			</table>
		{/if}
	</div>
{/if}

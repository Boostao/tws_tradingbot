<script lang="ts">
	import { t, language } from '$lib/i18n';
	import type { DocRecord } from '$lib/docs/types';

	export let data: { docs: DocRecord[] };

	$: _lang = $language;
</script>

<h1 class="heading">{t('documentation')}</h1>
<p class="muted" style="margin-top: 6px;">{t('docs_subtitle')}</p>

<div class="doc-grid" style="margin-top: 16px;">
	{#each data.docs as doc}
		<a class="doc-card" href={`/docs/${doc.slug}`}>
			<h3>{t(doc.titleKey)}</h3>
			<p class="doc-meta">{t(doc.descriptionKey)}</p>
		</a>
	{/each}
</div>

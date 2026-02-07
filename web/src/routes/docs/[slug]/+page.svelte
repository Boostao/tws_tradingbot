<script lang="ts">
	import { afterUpdate, onMount, tick } from 'svelte';
	import { t, language } from '$lib/i18n';
	import type { DocRecord, DocSection } from '$lib/docs/types';

	export let data: { doc: DocRecord; docs: DocRecord[] };

	let contentEl: HTMLDivElement | null = null;
	let lastRenderedSlug = '';

	$: _lang = $language;

	const renderSection = (section: DocSection) => section;

	const renderMermaid = async () => {
		await tick();
		if (!contentEl) return;
		const mermaid = (await import('mermaid')).default;
		mermaid.initialize({ startOnLoad: false, theme: 'dark' });
		await mermaid.run({ nodes: contentEl.querySelectorAll('.mermaid') });
		lastRenderedSlug = data.doc.slug;
	};

	onMount(renderMermaid);

	afterUpdate(() => {
		if (data.doc.slug !== lastRenderedSlug) {
			void renderMermaid();
		}
	});
</script>

<div class="doc-layout">
	<aside class="doc-nav">
		{#each data.docs as doc}
			<a href={`/docs/${doc.slug}`} class:active={doc.slug === data.doc.slug}>{t(doc.titleKey)}</a>
		{/each}
	</aside>
	<section class="doc-content" bind:this={contentEl}>
		{#each data.doc.sections as section}
			{#if renderSection(section).type === 'heading'}
				<h2>{t(section.key)}</h2>
			{:else if renderSection(section).type === 'paragraph'}
				<p>{t(section.key)}</p>
			{:else if renderSection(section).type === 'list'}
				<ul>
					{#each section.keys as key}
						<li>{t(key)}</li>
					{/each}
				</ul>
			{:else if renderSection(section).type === 'code'}
				<pre><code class={`language-${section.language ?? 'text'}`}>{section.code}</code></pre>
			{:else if renderSection(section).type === 'mermaid'}
				<div class="mermaid">{section.code}</div>
			{/if}
		{/each}
	</section>
</div>

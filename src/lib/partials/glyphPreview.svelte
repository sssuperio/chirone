<script lang="ts">
	import FontDisplayMetrics from '$lib/components/fontDisplayMetrics.svelte';
	import { syntaxes, selectedGlyph, glyphs } from '$lib/stores';
	import type { GlyphInput } from '$lib/types';
	import { getUnicodeNumber } from '$lib/GTL/unicode';
	import FontGenerator from './fontGenerator.svelte';

	//

	export let showTitle = true;
	export let title = 'Anteprima';
	export let compact = false;
	export let canvasWidth = 300;
	export let canvasHeight = 150;
	export let showLegend = true;
	export let debug = false;

	let currentGlyph: GlyphInput | undefined;
	$: if ($selectedGlyph) currentGlyph = $glyphs.find((g) => g.id === $selectedGlyph);

	let currentGlyphText: string;
	$: if (currentGlyph) currentGlyphText = getCharFromGlyph(currentGlyph);

	function getCharFromGlyph(glyph: GlyphInput) {
		try {
			return String.fromCharCode(getUnicodeNumber(glyph.name));
		} catch {
			return '';
		}
	}
</script>

<div class={compact ? 'space-y-3' : 'space-y-8'}>
	{#if showTitle}
		<p class="text-small font-mono text-slate-900 text-sm">{title}</p>
	{/if}

	{#if !currentGlyph}
		<p>No glyph selected</p>
	{:else if !$syntaxes.length}
		<p class="text-sm font-mono text-slate-500">Nessuna sintassi disponibile per l'anteprima.</p>
	{:else}
		{#each $syntaxes as syntax, i}
			<FontGenerator {syntax} glyphs={[currentGlyph]} let:font>
				{#if font}
					<div class={compact ? 'space-y-1' : 'space-y-2'}>
						<p class="text-small font-mono text-slate-900 text-sm">
							{font.names.fontSubfamily.en}
						</p>
						<FontDisplayMetrics
							{canvasWidth}
							{canvasHeight}
							{font}
							text={currentGlyphText}
							showLegend={showLegend && i === 0}
							{debug}
						/>
					</div>
				{/if}
			</FontGenerator>
		{/each}
	{/if}
</div>

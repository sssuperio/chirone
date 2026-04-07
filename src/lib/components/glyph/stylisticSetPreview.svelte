<script lang="ts">
	import type { GlyphInput } from '$lib/types';
	import { syntaxes, glyphs } from '$lib/stores';
	import { parseGlyphStructure, replaceGlyphStructureComponents, type GlyphComponentRef } from '$lib/GTL/structure';
	import FontGenerator from '$lib/partials/fontGenerator.svelte';
	import FontDisplayMetrics from '$lib/components/fontDisplayMetrics.svelte';

	export let glyph: GlyphInput;
	export let canvasWidth = 200;
	export let canvasHeight = 100;

	function getComponentStem(name: string): string {
		return name.replace(/\.ss\d\d\.component$/i, '').replace(/\.component$/i, '');
	}

	function getStylisticSetIndex(name: string): number | null {
		const match = name.match(/\.ss(\d\d)\.component$/i);
		if (!match) return null;
		return parseInt(match[1], 10);
	}

	function getComponentVariants(componentName: string): Array<string> {
		const stem = getComponentStem(componentName);
		const stemLower = stem.toLowerCase();
		const variants = new Set<string>();

		for (const g of $glyphs) {
			if (!g.name.toLowerCase().endsWith('.component')) continue;
			if (getComponentStem(g.name).toLowerCase() !== stemLower) continue;
			variants.add(g.name);
		}

		return Array.from(variants).sort((a, b) => {
			const idxA = getStylisticSetIndex(a) ?? 0;
			const idxB = getStylisticSetIndex(b) ?? 0;
			return idxA - idxB;
		});
	}

	function getGlyphComponents(glyph: GlyphInput): Array<GlyphComponentRef> {
		return parseGlyphStructure(glyph.structure).components;
	}

	function generateCombinations<T>(options: Array<Array<T>>): Array<Array<T>> {
		if (options.length === 0) return [[]];
		if (options.length === 1) return options[0].map(item => [item]);

		const result: Array<Array<T>> = [];
		const rest = generateCombinations(options.slice(1));
		for (const first of options[0]) {
			for (const combo of rest) {
				result.push([first, ...combo]);
			}
		}
		return result;
	}

	function swapComponentVariant(glyph: GlyphInput, symbol: string, newVariant: string): GlyphInput {
		const parsed = parseGlyphStructure(glyph.structure);
		const newComponents = parsed.components.map(c =>
			c.symbol === symbol ? { ...c, name: newVariant } : c
		);
		return {
			...glyph,
			structure: replaceGlyphStructureComponents(glyph.structure, newComponents)
		};
	}

	function getVariantLabel(name: string): string {
		const idx = getStylisticSetIndex(name);
		return idx !== null ? `ss${String(idx).padStart(2, '0')}` : 'base';
	}

	$: components = getGlyphComponents(glyph);
	$: componentVariantOptions = components.map(c => getComponentVariants(c.name));
	$: allCombinations = generateCombinations(componentVariantOptions);
</script>

{#if allCombinations.length > 1}
	<div class="mt-4 space-y-2">
		<p class="text-small font-mono text-slate-900 text-sm">
			Stylistic Sets ({allCombinations.length} combinations)
		</p>
		<div class="flex flex-wrap gap-2">
			{#each allCombinations as combination, comboIndex}
				{@const previewGlyph = combination.reduce(
					(g, variant, i) => swapComponentVariant(g, components[i].symbol, variant),
					glyph
				)}
				{@const comboLabel = combination.map(v => getVariantLabel(v)).join(' + ')}
				<div class="border border-slate-300 p-2 bg-white">
					<p class="text-xs font-mono text-slate-500 mb-1">{comboLabel}</p>
					{#each $syntaxes as syntax}
						<FontGenerator {syntax} glyphs={[previewGlyph]} let:font>
							{#if font}
								<FontDisplayMetrics
									{canvasWidth}
									{canvasHeight}
									{font}
									glyphName={glyph.name}
									text="▀"
									showLegend={false}
								/>
							{/if}
						</FontGenerator>
					{/each}
				</div>
			{/each}
		</div>
	</div>
{/if}

<script lang="ts">
import type { GlyphInput, Rule, Syntax } from '$lib/types';
import { glyphs, metrics, selectedGlyph, syntaxes } from '$lib/stores';
	import { nanoid } from 'nanoid';
	import _ from 'lodash';

	import Sidebar from '$lib/ui/sidebar.svelte';
	import SidebarTile from '$lib/ui/sidebarTile.svelte';
	import Button from '$lib/ui/button.svelte';
	import GlyphPainter from '$lib/components/glyph/glyphPainter.svelte';
	import GlyphPreview from '$lib/partials/glyphPreview.svelte';

	import { ShapeKind, createEmptyRule } from '$lib/types';
	import DeleteButton from '$lib/ui/deleteButton.svelte';
	import AddGlyphModal from './AddGlyphModal.svelte';
	import { UNICODE } from '$lib/GTL/unicode';
	import { resolveUnicodeNumber } from '$lib/GTL/glyphName';

	//

	function getUniqueSymbols(glyphs: Array<GlyphInput>): Array<string> {
		const symbols = [];
		for (const g of glyphs) {
			const txt = g.structure.replace(/(\r\n|\n|\r)/gm, '');
			if (txt) {
				symbols.push(...txt.split(''));
			}
		}

		// Unique symbols
		return _.uniq(symbols);
	}

	function getRuleBySymbol(syntax: Syntax, s: string): Rule {
		for (let rule of syntax.rules) {
			if (rule.symbol == s) {
				return rule;
			}
		}
		throw new Error('missingSymbol');
	}

	function updateSyntaxSymbols(syntax: Syntax, symbols: Array<string>): boolean {
		let changed = false;

		// Getting all symbols in syntax
		const syntaxSymbols = syntax.rules.map((r) => r.symbol);
		// Checking for additions
		for (let symbol of symbols) {
			if (!syntaxSymbols.includes(symbol)) {
				syntax.rules.push(createEmptyRule(symbol));
				changed = true;
			}
		}
		// Removing if a symbol goes away
		for (let symbol of syntaxSymbols) {
			if (!getUniqueSymbols($glyphs).includes(symbol)) {
				const extraRule = getRuleBySymbol(syntax, symbol);
				const index = syntax.rules.indexOf(extraRule);
				syntax.rules.splice(index, 1);
				changed = true;
			}
		}

		return changed;
	}

	$: {
		const uniqueSymbols = getUniqueSymbols($glyphs);
		let changed = false;
		for (let syntax of $syntaxes) {
			if (updateSyntaxSymbols(syntax, uniqueSymbols)) {
				changed = true;
			}
		}
		if (changed) {
			$syntaxes = [...$syntaxes];
		}
	}

	function handleDelete() {
		$glyphs = $glyphs.filter((g) => g.id != $selectedGlyph);
		$selectedGlyph = $glyphs[0].id;
	}

	function sortGlyphs(glyphs: GlyphInput[]): GlyphInput[] {
		return [...glyphs].sort((a, b) => {
			const aUnicode = resolveUnicodeNumber(a.name);
			const bUnicode = resolveUnicodeNumber(b.name);

			if (aUnicode !== undefined && bUnicode !== undefined) {
				return aUnicode - bUnicode;
			}
			if (aUnicode !== undefined) return -1;
			if (bUnicode !== undefined) return 1;

			return a.name.localeCompare(b.name);
		});
	}

	function getGlyphString(glyphName: string): string | undefined {
		const unicode = resolveUnicodeNumber(glyphName);
		if (unicode === undefined) return undefined;
		try {
			return String.fromCodePoint(unicode);
		} catch {
			return undefined;
		}
	}

	function getSyntaxSymbols(syntaxes: Array<Syntax>): Array<string> {
		const symbols = new Set<string>();
		for (const syntax of syntaxes) {
			for (const rule of syntax.rules) {
				if (rule.symbol?.length === 1) {
					symbols.add(rule.symbol);
				}
			}
		}
		return Array.from(symbols);
	}

	function getRulesBySymbol(syntaxes: Array<Syntax>): Record<string, Rule> {
		const map: Record<string, Rule> = {};
		for (const syntax of syntaxes) {
			for (const rule of syntax.rules) {
				if (rule.symbol?.length === 1 && !map[rule.symbol]) {
					map[rule.symbol] = rule;
				}
			}
		}
		return map;
	}

	function getVoidFillSymbol(rules: Record<string, Rule>): string {
		// Prefer a visible symbol mapped as Void (not blank space).
		for (const [symbol, rule] of Object.entries(rules)) {
			if (rule.shape.kind === ShapeKind.Void && symbol !== ' ') {
				return symbol;
			}
		}

		// If only blank-space Void exists, use a visible fallback.
		return '.';
	}

	function fillVoidInStructure(structure: string, fillSymbol: string, targetHeight: number): string {
		const sourceRows = structure ? structure.split(/\r?\n/) : [];
		const width = Math.max(1, ...sourceRows.map((row) => row.length));
		const height = Math.max(1, targetHeight, sourceRows.length);
		const rows = Array.from({ length: height }, (_, index) => sourceRows[index] ?? '');

		if (width <= 0) return structure;

		return rows
			.map((row) =>
				row
					.padEnd(width, ' ')
					.split('')
					.map((char) => (char === ' ' ? fillSymbol : char))
					.join('')
			)
			.join('\n');
	}

	//

	let isAddGlyphModalOpen = false;
	let designTopRatio = 0.45;
	let isResizingDesignPanels = false;
	let designPanelsEl: HTMLDivElement | undefined;

	$: brushSymbols = getSyntaxSymbols($syntaxes);
	$: rulesBySymbol = getRulesBySymbol($syntaxes);
	$: voidFillSymbol = getVoidFillSymbol(rulesBySymbol);
	$: fillTargetHeight = Math.max(1, Math.round($metrics.height || 1));

	function updateDesignPanelRatio(event: PointerEvent) {
		if (!isResizingDesignPanels || !designPanelsEl) return;
		const rect = designPanelsEl.getBoundingClientRect();
		if (!rect.height) return;
		const ratio = (event.clientY - rect.top) / rect.height;
		designTopRatio = Math.min(0.8, Math.max(0.2, ratio));
	}

	function startDesignPanelResize(event: PointerEvent) {
		isResizingDesignPanels = true;
		updateDesignPanelRatio(event);
	}

	function stopDesignPanelResize() {
		isResizingDesignPanels = false;
	}
</script>

<!--  -->

<svelte:window
	on:pointermove={updateDesignPanelRatio}
	on:pointerup={stopDesignPanelResize}
	on:pointercancel={stopDesignPanelResize}
/>

<div class="flex flex-row flex-nowrap items-stretch overflow-hidden grow">
	<div class="shrink-0 flex items-stretch">
		<Sidebar>
			<svelte:fragment slot="topArea">
				<Button
					on:click={() => {
						isAddGlyphModalOpen = true;
					}}>+ Aggiungi glifo</Button
				>
			</svelte:fragment>
			<svelte:fragment slot="listTitle">Lista glifi</svelte:fragment>
			<svelte:fragment slot="items">
				{#each sortGlyphs($glyphs) as g (g.id)}
					{@const glyphString = getGlyphString(g.name)}
					<SidebarTile selection={selectedGlyph} id={g.id}>
						{#if glyphString}
							{glyphString}
						{/if}
						<span class="opacity-25">
							– {g.name}
						</span>
					</SidebarTile>
				{/each}
			</svelte:fragment>
		</Sidebar>
	</div>

	<!-- Glyph area -->
	<div class="p-8 space-y-8 grow flex flex-col items-stretch">
		{#each $glyphs as g}
			{#if g.id == $selectedGlyph}
				{@const glyphString = getGlyphString(g.name)}
				{@const glyphName = UNICODE[g.name]}
				<div class="shrink-0 flex justify-between items-center">
					<div class="flex gap-4">
						{#if glyphString}
							<div class="w-12 h-12 flex items-center justify-center border-black text-xl border">
								<p>{glyphString}</p>
							</div>
						{/if}
						<div class="text-gray-400">
							<p>{g.name}</p>
							{#if glyphName}
								<p>
									{glyphName}
								</p>
							{/if}
						</div>
					</div>
					<DeleteButton on:delete={handleDelete} />
				</div>
				<hr />
				<div class="h-0 grow min-h-0 flex flex-col lg:flex-row gap-4">
					<div class="min-h-0 flex-1 flex flex-col gap-2">
						<div
							bind:this={designPanelsEl}
							class="h-0 grow min-h-0 grid gap-0"
							style={`grid-template-rows: ${Math.round(designTopRatio * 100)}% 0.75rem ${Math.round((1 - designTopRatio) * 100)}%;`}
						>
							<div class="min-h-0 flex flex-col">
								<div class="mb-2 flex items-center justify-between gap-3">
									<p class="text-small font-mono text-slate-900 text-sm">Struttura glifo</p>
									<Button
										on:click={() => {
											g.structure = fillVoidInStructure(
												g.structure,
												voidFillSymbol,
												fillTargetHeight
											);
										}}
									>
										Fill void ({voidFillSymbol} / h={fillTargetHeight})
									</Button>
								</div>
								<textarea
									class="h-0 grow min-h-0 p-2 bg-slate-200 tracking-[0.75em] hover:bg-slate-300 font-mono focus:ring-4"
									bind:value={g.structure}
								/>
							</div>

							<div
								class="cursor-row-resize bg-slate-200 hover:bg-slate-300 flex items-center justify-center"
								on:pointerdown|preventDefault={startDesignPanelResize}
							>
								<div class="w-12 h-1 bg-slate-500/70" />
							</div>

							<div class="min-h-0 flex flex-col">
								<p class="text-small font-mono text-slate-900 mb-2 text-sm">Visual designer</p>
								<div class="h-0 grow min-h-0">
									<GlyphPainter bind:structure={g.structure} brushes={brushSymbols} {rulesBySymbol} />
								</div>
							</div>
						</div>
					</div>

					<div class="min-h-0 flex-1 flex flex-col lg:border-l lg:border-slate-300 lg:pl-4 bg-slate-50">
						<p class="text-small font-mono text-slate-900 mb-2 text-sm">Anteprima e metriche</p>
						<div class="h-0 grow min-h-0 overflow-y-auto">
							<GlyphPreview canvasHeight={300} debug />
						</div>
					</div>
				</div>
				{/if}
			{/each}
	</div>
</div>

<!--  -->

<AddGlyphModal bind:open={isAddGlyphModalOpen} />

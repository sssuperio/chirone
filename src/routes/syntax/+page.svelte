<script lang="ts" context="module">
	import { writable } from 'svelte/store';
	export const currentSyntaxId = writable<string>('');
</script>

<script lang="ts">
	import { createEmptySyntax, createEmptyRule, ShapeKind, type GlyphInput } from '$lib/types';
	import type { Syntax } from '$lib/types';
	import { syntaxes, glyphs } from '$lib/stores';
	import { nanoid } from 'nanoid';
	import {
		getUniqueSymbolsFromGlyphs,
		parseGlyphStructure,
		replaceGlyphStructureBody
	} from '$lib/GTL/structure';

	import SyntaxEditor from '$lib/components/syntax/syntaxEditor.svelte';
	import InputText from '$lib/ui/inputText.svelte';
	import Sidebar from '$lib/ui/sidebar.svelte';
	import SidebarTile from '$lib/ui/sidebarTile.svelte';
	import Button from '$lib/ui/button.svelte';
	import DeleteButton from '$lib/ui/deleteButton.svelte';
	import InputNumber from '$lib/ui/inputNumber.svelte';

	//

	/**
	 * Updating all the syntaxes if there are new symbols
	 */

	function updateSyntaxSymbols(syntax: Syntax, uniqueSymbols: Array<string>): boolean {
		let changed = false;
		const usedSymbols = new Set(uniqueSymbols);
		const syntaxSymbols = syntax.rules.map((r) => r.symbol);

		for (let symbol of uniqueSymbols) {
			if (!syntaxSymbols.includes(symbol)) {
				const newRule = createEmptyRule(symbol);
				newRule.unused = false;
				syntax.rules.push(newRule);
				changed = true;
			}
		}

		for (const rule of syntax.rules) {
			const nextUnused = !usedSymbols.has(rule.symbol);
			if ((rule.unused ?? false) !== nextUnused) {
				rule.unused = nextUnused;
				changed = true;
			}
		}

		return changed;
	}

	$: {
		const uniqueSymbols = getUniqueSymbols($glyphs);
		let changed = false;
		for (const syntax of $syntaxes) {
			if (updateSyntaxSymbols(syntax, uniqueSymbols)) {
				changed = true;
			}
		}
		if (changed) {
			$syntaxes = [...$syntaxes];
		}
	}

	/**
	 * Adding syntaxes
	 */

	function getUniqueSymbols(glyphs: Array<GlyphInput>): Array<string> {
		return getUniqueSymbolsFromGlyphs(glyphs);
	}

	function addSyntax(name: string | null = null) {
		const newSyntax = createEmptySyntax(
			name ? name : nanoid(5),
			nanoid(5),
			getUniqueSymbols($glyphs)
		);

		$syntaxes = [...$syntaxes, newSyntax];
		$currentSyntaxId = newSyntax.id;
	}

	// Shorthand function for the button
	function addSyntaxBtn() {
		addSyntax();
	}

	let currentSyntax: Syntax | undefined;
	$: currentSyntax = $syntaxes.find((s) => s.id == $currentSyntaxId);

	let currentSyntaxIndex: number | undefined;
	$: if (currentSyntax) currentSyntaxIndex = $syntaxes.indexOf(currentSyntax);
	$: currentUnusedRulesCount = currentSyntax
		? currentSyntax.rules.filter((rule) => rule.unused ?? false).length
		: 0;

	function handleDelete() {
		$syntaxes = $syntaxes.filter((s) => s.id != $currentSyntaxId);
		if ($syntaxes[0]) $currentSyntaxId = $syntaxes[0].id;
	}

	function cleanupUnusedRules() {
		if (currentSyntaxIndex === undefined) return;
		const syntax = $syntaxes[currentSyntaxIndex];
		const nextRules = syntax.rules.filter((rule) => !(rule.unused ?? false));
		if (nextRules.length === syntax.rules.length) return;
		syntax.rules = nextRules;
		$syntaxes = [...$syntaxes];
	}

	function handleSyntaxEditorChanged(event: CustomEvent<{ glyphsChanged: boolean }>) {
		$syntaxes = [...$syntaxes];
		if (event.detail.glyphsChanged) {
			$glyphs = [...$glyphs];
		}
	}

	function getVoidFillSymbolForSyntax(syntax: Syntax): string {
		for (const rule of syntax.rules) {
			if (rule.shape.kind !== ShapeKind.Void) continue;
			if (!rule.symbol) continue;
			if (rule.symbol === ' ') continue;
			return rule.symbol;
		}
		return '.';
	}

	function fillVoidAllGlyphsForCurrentStyle() {
		if (currentSyntaxIndex === undefined) return;
		const syntax = $syntaxes[currentSyntaxIndex];
		const fillSymbol = getVoidFillSymbolForSyntax(syntax);
		const targetHeight = Math.max(1, Math.trunc(syntax.grid.rows || 1));
		let changed = false;

		for (const glyph of $glyphs) {
			const parsed = parseGlyphStructure(glyph.structure);
			const sourceRows = parsed.body ? parsed.body.split(/\r?\n/) : [];
			const width = Math.max(1, ...sourceRows.map((row) => row.length));
			const height = Math.max(1, targetHeight, sourceRows.length);
			const rows = Array.from({ length: height }, (_, index) => sourceRows[index] ?? '');

			const nextBody = rows
				.map((row) =>
					row
						.padEnd(width, ' ')
						.split('')
						.map((char) => (char === ' ' ? fillSymbol : char))
						.join('')
				)
				.join('\n');

			const nextStructure = replaceGlyphStructureBody(glyph.structure, nextBody);
			if (nextStructure !== glyph.structure) {
				glyph.structure = nextStructure;
				changed = true;
			}
		}

		if (changed) {
			$glyphs = [...$glyphs];
		}
	}
</script>

<!--  -->

<div class="h-full flex flex-row flex-nowrap items-stretch">
	<!-- sidebar -->
	<Sidebar>
		<svelte:fragment slot="topArea">
			<Button on:click={addSyntaxBtn}>+ Aggiungi stile</Button>
		</svelte:fragment>
		<svelte:fragment slot="listTitle">Lista stili</svelte:fragment>
		<svelte:fragment slot="items">
			{#each $syntaxes as s (s.id)}
				<SidebarTile selection={currentSyntaxId} id={s.id}>
					{s.name}
				</SidebarTile>
			{/each}
		</svelte:fragment>
	</Sidebar>

	<!-- syntax editor -->
	<div class="grow min-w-0 p-8 space-y-8 overflow-y-auto">
		{#key currentSyntaxIndex}
			{#if currentSyntaxIndex !== undefined}
				<div class="space-y-4">
					<div class="flex flex-col">
						<p class="text-small font-mono text-slate-900 mb-2 text-sm">Nome stile</p>
						<InputText name="styleName" bind:value={$syntaxes[currentSyntaxIndex].name} />
					</div>
						<div class="flex gap-4">
							<div>
								<p class="text-small font-mono text-slate-900 mb-2 text-sm">Colonne</p>
								<InputNumber bind:value={$syntaxes[currentSyntaxIndex].grid.columns} />
							</div>
							<div>
								<p class="text-small font-mono text-slate-900 mb-2 text-sm">Righe</p>
								<InputNumber bind:value={$syntaxes[currentSyntaxIndex].grid.rows} />
							</div>
						</div>
						<div class="flex gap-2 items-center">
							<Button disabled={!currentUnusedRulesCount} on:click={cleanupUnusedRules}>
								Pulisci simboli inutilizzati ({currentUnusedRulesCount})
							</Button>
							<Button on:click={fillVoidAllGlyphsForCurrentStyle}>
								Fill void su tutti i glifi
							</Button>
						</div>
						<DeleteButton on:delete={handleDelete} />
					</div>
				<hr />
				<SyntaxEditor
					bind:syntax={$syntaxes[currentSyntaxIndex]}
					glyphs={$glyphs}
					on:changed={handleSyntaxEditorChanged}
				/>
			{/if}
		{/key}
	</div>
</div>

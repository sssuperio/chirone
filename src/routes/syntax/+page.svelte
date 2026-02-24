<script lang="ts" context="module">
	import { writable } from 'svelte/store';
	export const currentSyntaxId = writable<string>('');
</script>

<script lang="ts">
	import { createEmptySyntax, createEmptyRule, type GlyphInput } from '$lib/types';
	import type { Syntax } from '$lib/types';
	import { syntaxes, glyphs } from '$lib/stores';
	import { nanoid } from 'nanoid';
	import { getUniqueSymbolsFromGlyphs } from '$lib/GTL/structure';

	import SyntaxEditor from '$lib/components/syntax/syntaxEditor.svelte';
	import InputText from '$lib/ui/inputText.svelte';
	import Sidebar from '$lib/ui/sidebar.svelte';
	import SidebarTile from '$lib/ui/sidebarTile.svelte';
	import Button from '$lib/ui/button.svelte';
	import SyntaxPreview from '$lib/partials/syntaxPreview.svelte';
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
	<div class="p-8 space-y-8 overflow-y-auto">
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
					</div>
					<DeleteButton on:delete={handleDelete} />
				</div>
				<hr />
				<SyntaxEditor bind:syntax={$syntaxes[currentSyntaxIndex]} />
			{/if}
		{/key}
	</div>

	<div class="p-8 border border-l-gray-300 overflow-y-scroll">
		{#key currentSyntaxIndex}
			{#if currentSyntaxIndex !== undefined}
				<SyntaxPreview syntax={$syntaxes[currentSyntaxIndex]} />
			{/if}
		{/key}
	</div>
</div>

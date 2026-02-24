<script lang="ts">
	import { Modal } from 'flowbite-svelte';
	import { nanoid } from 'nanoid';
	import { glyphs, selectedGlyph } from '$lib/stores';
	import type { GlyphInput } from '$lib/types';
	import {
		getGeneratableGlyphSetDefinitions,
		getGlyphNamesForSet,
		type GlyphSetID
	} from '$lib/GTL/glyphSets';
	import Button from '$lib/ui/button.svelte';

	export let open = false;

	const setDefinitions = getGeneratableGlyphSetDefinitions();
	let selectedSetID: GlyphSetID = setDefinitions[0]?.id ?? 'latin-base';

	$: selectedDefinition =
		setDefinitions.find((definition) => definition.id === selectedSetID) ?? setDefinitions[0];
	$: existingNames = new Set($glyphs.map((glyph) => glyph.name));
	$: allSetNames = selectedDefinition ? getGlyphNamesForSet(selectedDefinition.id) : [];
	$: missingNames = allSetNames.filter((name) => !existingNames.has(name));

	function addSetGlyphs() {
		if (!selectedDefinition || !missingNames.length) {
			open = false;
			return;
		}

		const createdGlyphs: Array<GlyphInput> = missingNames.map((name) => ({
			id: nanoid(5),
			name,
			structure: '',
			set: selectedDefinition.id
		}));

		$glyphs = [...$glyphs, ...createdGlyphs];
		if (createdGlyphs.length) {
			$selectedGlyph = createdGlyphs[0].id;
		}
		open = false;
	}
</script>

<Modal outsideclose class="!rounded-none-none !font-mono" bind:open title="Aggiungi set di glifi">
	<div class="space-y-3 font-mono">
		<div class="space-y-1">
			<p class="text-xs text-slate-600">Set</p>
			<select class="w-full border hover:border-blue-600 px-4 py-2" bind:value={selectedSetID}>
				{#each setDefinitions as definition (definition.id)}
					<option value={definition.id}>{definition.label}</option>
				{/each}
			</select>
		</div>

		{#if selectedDefinition}
			<p class="text-sm text-slate-700">{selectedDefinition.description}</p>
		{/if}

		<div class="bg-slate-100 p-3 text-sm">
			<p>
				Presenti: {allSetNames.length - missingNames.length}/{allSetNames.length}
			</p>
			<p class="text-rose-700">Da aggiungere: {missingNames.length}</p>
		</div>

		{#if missingNames.length}
			<div class="max-h-40 overflow-y-auto border border-slate-200 bg-white p-2 text-xs">
				<p class="text-slate-500 mb-1">Glifi mancanti</p>
				<p class="break-words">{missingNames.join(', ')}</p>
			</div>
		{/if}

		<div class="flex gap-2">
			<Button on:click={() => (open = false)}>Annulla</Button>
			<Button disabled={!missingNames.length} on:click={addSetGlyphs}>+ Aggiungi set</Button>
		</div>
	</div>
</Modal>

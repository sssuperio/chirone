<script lang="ts">
	import {
		getAlternateBaseName,
		getLigatureComponentNames,
		isValidGlyphName,
		normalizeGlyphNameInput
	} from '$lib/GTL/glyphName';
	import { findCharInUnicodeList } from '$lib/GTL/unicode';
	import { glyphs, selectedGlyph } from '$lib/stores';
	import type { GlyphInput } from '$lib/types';
	import { Modal } from 'flowbite-svelte';
	import { tick } from 'svelte';
	import { nanoid } from 'nanoid';
	import Button from '$lib/ui/button.svelte';

	export let open = false;

	//

	let glyphNameInput = '';
	let glyphNameInputEl: HTMLInputElement | undefined;

	$: normalizedGlyphName = normalizeGlyphNameInput(glyphNameInput);
	$: unicodeMatch =
		glyphNameInput.trim().length === 1 ? findCharInUnicodeList(glyphNameInput.trim()) : undefined;
	$: glyphAlreadyExists = doesGlyphAlreadyExist(normalizedGlyphName);
	$: ligatureComponents = getLigatureComponentNames(normalizedGlyphName);
	$: alternateBase = getAlternateBaseName(normalizedGlyphName);
	$: autoStructure = getAutoStructure(normalizedGlyphName);
	$: canAdd = Boolean(normalizedGlyphName && isValidGlyphName(normalizedGlyphName) && !glyphAlreadyExists);
	$: missingLigatureComponents =
		ligatureComponents.length > 1
			? ligatureComponents.filter((name) => !findGlyphByName(name))
			: [];

	//

	function doesGlyphAlreadyExist(glyphName: string): boolean {
		return Boolean($glyphs.find((g) => g.name === glyphName));
	}

	function findGlyphByName(name: string): GlyphInput | undefined {
		return $glyphs.find((glyph) => glyph.name === name);
	}

	function splitRows(structure: string): Array<string> {
		return structure ? structure.split(/\r?\n/) : [];
	}

	function trimRight(text: string): string {
		return text.replace(/\s+$/g, '');
	}

	function mergeStructuresHorizontally(structures: Array<string>): string {
		const rowsByGlyph = structures.map((structure) => splitRows(structure));
		const maxRows = Math.max(0, ...rowsByGlyph.map((rows) => rows.length));
		const mergedRows: Array<string> = [];

		for (let row = 0; row < maxRows; row++) {
			const rowParts = rowsByGlyph.map((rows) => rows[row] ?? '');
			mergedRows.push(trimRight(rowParts.join(' ')));
		}

		while (mergedRows.length && mergedRows[mergedRows.length - 1] === '') {
			mergedRows.pop();
		}

		return mergedRows.join('\n');
	}

	function getAutoStructure(glyphName: string): string {
		if (!glyphName) return '';

		const components = getLigatureComponentNames(glyphName);
		if (components.length > 1) {
			const componentGlyphs = components.map((name) => findGlyphByName(name));
			if (componentGlyphs.some((glyph) => !glyph)) return '';

			const componentStructures = (componentGlyphs as Array<GlyphInput>).map((glyph) => glyph.structure);
			if (componentStructures.some((structure) => !structure.trim())) return '';

			return mergeStructuresHorizontally(componentStructures);
		}

		const baseName = getAlternateBaseName(glyphName);
		if (!baseName) return '';

		const baseGlyph = findGlyphByName(baseName);
		if (!baseGlyph) return '';

		return baseGlyph.structure ?? '';
	}

	function addGlyph() {
		if (!canAdd) return;

		const newGlyph: GlyphInput = {
			id: nanoid(5),
			name: normalizedGlyphName,
			structure: autoStructure
		};

		$glyphs = [...$glyphs, newGlyph];
		$selectedGlyph = newGlyph.id;
		glyphNameInput = '';
		open = false;
	}

	async function focusGlyphNameInput() {
		glyphNameInput = '';
		await tick();
		glyphNameInputEl?.focus();
		glyphNameInputEl?.select();
	}
</script>

<Modal
	outsideclose
	class="!rounded-none-none !font-mono"
	bind:open
	title="Aggiungi glifo"
	on:open={focusGlyphNameInput}
>
	<div class="space-y-3 font-mono">
		<div class="space-y-1">
			<p class="text-xs text-slate-600">Nome glifo o carattere (es: `a`, `a.ss01`, `f_f`, `f_l`)</p>
			<input
				class="w-full border hover:border-blue-600 px-4 py-2"
				bind:this={glyphNameInputEl}
				bind:value={glyphNameInput}
				placeholder="a.ss01"
				autocomplete="off"
			/>
		</div>

		<div class="grow bg-gray-100 flex items-center px-4 py-3 min-h-[3rem]">
			{#if !glyphNameInput.trim()}
				<p>Inserisci un nome glifo</p>
			{:else if !isValidGlyphName(normalizedGlyphName)}
				<p>Nome non valido: evita spazi e newline</p>
			{:else if glyphAlreadyExists}
				<p>Questo glifo è già presente</p>
			{:else if ligatureComponents.length > 1}
				{#if missingLigatureComponents.length}
					<p>
						Ligatura rilevata ({ligatureComponents.join(' + ')}). Mancano i componenti:
						{missingLigatureComponents.join(', ')}
					</p>
				{:else if autoStructure}
					<p>
						Ligatura rilevata ({ligatureComponents.join(' + ')}). Struttura precompilata dai glifi
						componenti.
					</p>
				{:else}
					<p>Ligatura rilevata ({ligatureComponents.join(' + ')}), ma i componenti sono vuoti.</p>
				{/if}
			{:else if alternateBase}
				{#if autoStructure}
					<p>Alternativa rilevata su base `{alternateBase}`. Struttura copiata dal glifo base.</p>
				{:else}
					<p>Alternativa rilevata su base `{alternateBase}` (nessuna struttura da copiare).</p>
				{/if}
			{:else if unicodeMatch}
				<p>Carattere Unicode riconosciuto: `{unicodeMatch[0]}`.</p>
			{:else}
				<p>Glifo custom pronto: `{normalizedGlyphName}`.</p>
			{/if}
		</div>

		<div class="flex gap-2">
			<Button on:click={() => (open = false)}>Annulla</Button>
			<Button disabled={!canAdd} on:click={addGlyph}>+ Aggiungi glifo</Button>
		</div>
	</div>
</Modal>

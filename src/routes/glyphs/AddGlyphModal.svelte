<script lang="ts">
	import {
		getAlternateBaseName,
		getLigatureComponentNames,
		isValidGlyphName,
		normalizeGlyphNameInput
	} from '$lib/GTL/glyphName';
	import { inferGlyphSetIDByName } from '$lib/GTL/glyphSets';
	import {
		isComponentGlyphName,
		parseGlyphStructure,
		resolveGlyphStructures,
		serializeGlyphStructure
	} from '$lib/GTL/structure';
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
	let cloneSourceGlyphName = '';

	$: normalizedGlyphName = normalizeGlyphNameInput(glyphNameInput);
	$: cloneSourceGlyph = cloneSourceGlyphName ? findGlyphByName(cloneSourceGlyphName) : undefined;
	$: unicodeMatch =
		glyphNameInput.trim().length === 1 ? findCharInUnicodeList(glyphNameInput.trim()) : undefined;
	$: glyphAlreadyExists = doesGlyphAlreadyExist(normalizedGlyphName);
	$: ligatureComponents = getLigatureComponentNames(normalizedGlyphName);
	$: alternateBase = getAlternateBaseName(normalizedGlyphName);
	$: resolvedGlyphStructures = resolveGlyphStructures($glyphs);
	$: autoStructure = getAutoStructure(normalizedGlyphName);
	$: newGlyphStructure = cloneSourceGlyph?.structure ?? autoStructure;
	$: sortedGlyphNames = [...$glyphs.map((glyph) => glyph.name)].sort((a, b) => a.localeCompare(b));
	$: canAdd = Boolean(normalizedGlyphName && isValidGlyphName(normalizedGlyphName) && !glyphAlreadyExists);
	$: linkedComponentAlternates = getLinkedComponentAlternatesForNewComponent(normalizedGlyphName);
	$: missingLigatureComponents =
		ligatureComponents.length > 1
			? ligatureComponents.filter((name) => !findGlyphByName(name))
			: [];
	$: if (cloneSourceGlyphName && !sortedGlyphNames.includes(cloneSourceGlyphName)) {
		cloneSourceGlyphName = '';
	}

	//

	function doesGlyphAlreadyExist(glyphName: string): boolean {
		return Boolean($glyphs.find((g) => g.name === glyphName));
	}

	function findGlyphByName(name: string): GlyphInput | undefined {
		return $glyphs.find((glyph) => glyph.name === name);
	}

	function getResolvedStructureByGlyphName(name: string): string {
		if (resolvedGlyphStructures.has(name)) {
			return resolvedGlyphStructures.get(name) ?? '';
		}

		const glyph = findGlyphByName(name);
		if (!glyph) return '';
		return parseGlyphStructure(glyph.structure).body;
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

		const componentStylisticSet = parseComponentStylisticSetName(glyphName);
		if (componentStylisticSet) {
			const baseComponentGlyph = findGlyphByName(componentStylisticSet.baseComponentName);
			if (!baseComponentGlyph) return '';
			return getResolvedStructureByGlyphName(baseComponentGlyph.name);
		}

		const components = getLigatureComponentNames(glyphName);
		if (components.length > 1) {
			const componentGlyphs = components.map((name) => findGlyphByName(name));
			if (componentGlyphs.some((glyph) => !glyph)) return '';

			const componentStructures = (componentGlyphs as Array<GlyphInput>).map((glyph) =>
				getResolvedStructureByGlyphName(glyph.name)
			);
			if (componentStructures.some((structure) => !structure.trim())) return '';

			return mergeStructuresHorizontally(componentStructures);
		}

		const baseName = getAlternateBaseName(glyphName);
		if (!baseName) return '';

		const baseGlyph = findGlyphByName(baseName);
		if (!baseGlyph) return '';

		return getResolvedStructureByGlyphName(baseGlyph.name);
	}

	type ComponentStylisticSetName = {
		baseComponentName: string;
		stylisticSetTag: string;
	};

	type LinkedComponentAlternate = {
		baseGlyphName: string;
		targetGlyphName: string;
	};

	function parseComponentStylisticSetName(name: string): ComponentStylisticSetName | undefined {
		const trimmed = name.trim();
		const match = trimmed.match(/^(.*)\.(ss\d\d)\.component$/i);
		if (!match) return undefined;
		const baseStem = match[1]?.trim();
		const stylisticSetTag = match[2]?.toLowerCase();
		if (!baseStem || !stylisticSetTag) return undefined;
		return {
			baseComponentName: `${baseStem}.component`,
			stylisticSetTag
		};
	}

	function getFirstAvailableStylisticSetTag(
		baseGlyphName: string,
		preferredTag: string,
		takenGlyphNames: Set<string>
	): string | undefined {
		const preferredName = `${baseGlyphName}.${preferredTag}`;
		if (!takenGlyphNames.has(preferredName)) return preferredTag;

		for (let index = 1; index <= 99; index++) {
			const candidateTag = `ss${String(index).padStart(2, '0')}`;
			const candidateName = `${baseGlyphName}.${candidateTag}`;
			if (!takenGlyphNames.has(candidateName)) return candidateTag;
		}

		return undefined;
	}

	function getLinkedComponentAlternatesForNewComponent(newComponentName: string): Array<LinkedComponentAlternate> {
		const componentStylisticSet = parseComponentStylisticSetName(newComponentName);
		if (!componentStylisticSet) return [];

		const takenGlyphNames = new Set($glyphs.map((glyph) => glyph.name));
		const matches: Array<LinkedComponentAlternate> = [];
		const candidateGlyphs = [...$glyphs].sort((a, b) => a.name.localeCompare(b.name));

		for (const glyph of candidateGlyphs) {
			if (isComponentGlyphName(glyph.name)) continue;
			if (getAlternateBaseName(glyph.name)) continue;

			const parsed = parseGlyphStructure(glyph.structure);
			const usesBaseComponent = parsed.components.some(
				(component) => component.name === componentStylisticSet.baseComponentName
			);
			if (!usesBaseComponent) continue;

			const tag = getFirstAvailableStylisticSetTag(
				glyph.name,
				componentStylisticSet.stylisticSetTag,
				takenGlyphNames
			);
			if (!tag) continue;

			const targetGlyphName = `${glyph.name}.${tag}`;
			takenGlyphNames.add(targetGlyphName);
			matches.push({
				baseGlyphName: glyph.name,
				targetGlyphName
			});
		}

		return matches;
	}

	function addGlyph() {
		if (!canAdd) return;

		const newGlyph: GlyphInput = {
			id: nanoid(5),
			name: normalizedGlyphName,
			structure: newGlyphStructure,
			set: inferGlyphSetIDByName(normalizedGlyphName)
		};

		const nextGlyphs: Array<GlyphInput> = [...$glyphs, newGlyph];
		const componentStylisticSet = parseComponentStylisticSetName(normalizedGlyphName);
		if (componentStylisticSet) {
			const targetNameByBaseGlyph = new Map(
				linkedComponentAlternates.map((item) => [item.baseGlyphName, item.targetGlyphName])
			);
			for (const sourceGlyph of $glyphs) {
				const targetGlyphName = targetNameByBaseGlyph.get(sourceGlyph.name);
				if (!targetGlyphName) continue;

				const parsedSource = parseGlyphStructure(sourceGlyph.structure);
				const replacedComponents = parsedSource.components.map((component) =>
					component.name === componentStylisticSet.baseComponentName
						? { ...component, name: normalizedGlyphName }
						: component
				);
				const targetStructure = serializeGlyphStructure({
					...parsedSource,
					components: replacedComponents
				});

				nextGlyphs.push({
					id: nanoid(5),
					name: targetGlyphName,
					structure: targetStructure,
					set: inferGlyphSetIDByName(targetGlyphName)
				});
			}
		}

		$glyphs = nextGlyphs;
		$selectedGlyph = newGlyph.id;
		glyphNameInput = '';
		open = false;
	}

	async function focusGlyphNameInput() {
		glyphNameInput = '';
		cloneSourceGlyphName = '';
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
		<div class="space-y-1">
			<p class="text-xs text-slate-600">
				Clona da glifo esistente (opzionale, copia anche componenti/frontmatter)
			</p>
			<select class="w-full border hover:border-blue-600 px-4 py-2" bind:value={cloneSourceGlyphName}>
				<option value="">Nessun clone</option>
				{#each sortedGlyphNames as glyphName (glyphName)}
					<option value={glyphName}>{glyphName}</option>
				{/each}
			</select>
		</div>

		<div class="grow bg-gray-100 flex items-center px-4 py-3 min-h-[3rem]">
			{#if !glyphNameInput.trim()}
				<p>Inserisci un nome glifo</p>
			{:else if !isValidGlyphName(normalizedGlyphName)}
				<p>Nome non valido: evita spazi e newline</p>
			{:else if glyphAlreadyExists}
				<p>Questo glifo è già presente</p>
			{:else if cloneSourceGlyph}
				<p>
					Clone attivo da `{cloneSourceGlyph.name}`. Il nuovo glifo parte con la stessa struttura
					(componenti + override).
				</p>
			{:else if linkedComponentAlternates.length}
				<p>
					Componente stilistico rilevato. Verranno creati: {linkedComponentAlternates
						.map((item) => item.targetGlyphName)
						.join(', ')}
				</p>
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

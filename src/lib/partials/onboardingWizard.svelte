<script lang="ts">
	import { createEventDispatcher } from 'svelte';
	import { nanoid } from 'nanoid';
	import { goto } from '$app/navigation';
	import { base } from '$app/paths';
	import { Modal } from 'flowbite-svelte';
	import Button from '$lib/ui/button.svelte';
	import RuleShapePreview from '$lib/components/glyph/ruleShapePreview.svelte';
	import { ShapeKind, orientations } from '$lib/types';
	import type { Syntax, Rule } from '$lib/types';
	import { getGeneratableGlyphSetDefinitions, getGlyphNamesForSet } from '$lib/GTL/glyphSets';
	import type { GlyphSetID } from '$lib/GTL/glyphSets';
	import { normalizeFontMetrics } from '$lib/GTL/metrics';
	import { normalizeFontMetadata } from '$lib/GTL/metadata';
	import {
		syntaxes,
		metricsPresets,
		metadataPresets,
		fontDefinitions,
		metrics,
		fontMetadata,
		glyphs,
		activeFontId,
		projectInfo,
		saveGlyphsForFont,
		resetProjectState
	} from '$lib/stores';

	export let open = false;

	let step = 0;
	const totalSteps = 5;
	const stepLabels = ['Metadata', 'Griglia', 'Sintassi', 'Metriche', 'Glifi'];

	let familyName = '';
	let designer = '';
	let version = 'Version 1.0';
	let gridRows = 5;
	let gridColumns = 5;

	const autoSymbols = 'abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789';

	let shapeChoices: Array<{
		kind: ShapeKind;
		label: string;
		symbol: string;
		enabled: boolean;
		rule: Rule;
	}> = [];

	function makeRule(kind: ShapeKind, symbol: string, orientation?: string): Rule {
		const props: Record<string, unknown> = {};
		if (orientation) {
			props.orientation = { kind: 'orientation', value: { kind: 'fixed', data: orientation } };
		}
		return { symbol, shape: { kind, props: props as Rule['shape']['props'] } };
	}

	function initShapes() {
		shapeChoices = [
			{
				kind: ShapeKind.Void,
				label: 'Vuoto (spazio)',
				symbol: ' ',
				enabled: true,
				rule: { symbol: ' ', shape: { kind: ShapeKind.Void, props: {} } }
			},
			{
				kind: ShapeKind.Rectangle,
				label: 'Pieno',
				symbol: '#',
				enabled: true,
				rule: makeRule(ShapeKind.Rectangle, '#')
			}
		];

		const triSymbols: Record<string, string> = { NE: '^', NW: '\\', SE: '/', SW: 'v' };
		const triLabels: Record<string, string> = {
			NE: 'Triangolo ↗',
			NW: 'Triangolo ↖',
			SE: 'Triangolo ↘',
			SW: 'Triangolo ↙'
		};
		for (const o of orientations) {
			shapeChoices.push({
				kind: ShapeKind.Triangle,
				label: triLabels[o] ?? `Triangolo ${o}`,
				symbol: triSymbols[o] ?? '?',
				enabled: false,
				rule: makeRule(ShapeKind.Triangle, triSymbols[o] ?? '?', o)
			});
		}

		const qtrSymbols: Record<string, string> = { NE: '┐', NW: '┌', SE: '┘', SW: '└' };
		const qtrLabels: Record<string, string> = {
			NE: 'Quarto ↗',
			NW: 'Quarto ↖',
			SE: 'Quarto ↘',
			SW: 'Quarto ↙'
		};
		for (const o of orientations) {
			shapeChoices.push({
				kind: ShapeKind.Quarter,
				label: qtrLabels[o] ?? `Quarto ${o}`,
				symbol: qtrSymbols[o] ?? '?',
				enabled: false,
				rule: makeRule(ShapeKind.Quarter, qtrSymbols[o] ?? '?', o)
			});
		}

		shapeChoices.push(
			{
				kind: ShapeKind.Circle,
				label: 'Cerchio',
				symbol: 'o',
				enabled: false,
				rule: makeRule(ShapeKind.Circle, 'o')
			},
			{
				kind: ShapeKind.Ellipse,
				label: 'Ellisse',
				symbol: '0',
				enabled: false,
				rule: makeRule(ShapeKind.Ellipse, '0')
			},
			{
				kind: ShapeKind.SVG,
				label: 'Curva',
				symbol: 's',
				enabled: false,
				rule: makeRule(ShapeKind.SVG, 's')
			}
		);
	}

	function onSymbolChange(sc: { kind: number; symbol: string; rule: { symbol: string } }) {
		sc.rule.symbol = sc.symbol;
	}

	let upm = 5000;
	let height = 5;
	let descender = 1;
	let capHeight = 4;
	let xHeight = 3;

	let glyphSetDefs = getGeneratableGlyphSetDefinitions();
	let selectedGlyphSetIds = new Set<string>();
	let selectedGlyphSetsVersion = 0;

	function toggleGlyphSet(id: string) {
		if (selectedGlyphSetIds.has(id)) {
			selectedGlyphSetIds.delete(id);
		} else {
			selectedGlyphSetIds.add(id);
		}
		selectedGlyphSetIds = selectedGlyphSetIds; // trigger reactivity
		selectedGlyphSetsVersion++;
	}

	$: selectedGlyphNames = (() => {
		void selectedGlyphSetsVersion; // reactivity anchor
		return glyphSetDefs
			.filter((d) => selectedGlyphSetIds.has(d.id))
			.flatMap((d) => getGlyphNamesForSet(d.id));
	})();

	$: suggestedGlyphs = selectedGlyphNames.join('\n');

	let wizardInitialized = false;
	$: if (open && !wizardInitialized) {
		familyName = $projectInfo.name || 'GTL';
		designer = $fontMetadata.designer || '';
		initShapes();
		wizardInitialized = true;
	}
	$: if (!open) wizardInitialized = false;

	$: stepClasses = stepLabels.map((_, i) =>
		i === step
			? 'rounded px-3 py-1 text-xs bg-blue-600 text-white'
			: i < step
				? 'rounded px-3 py-1 text-xs bg-green-200 text-green-800'
				: 'rounded px-3 py-1 text-xs bg-slate-100 text-slate-400'
	);

	$: canNext =
		step === 0
			? familyName.trim().length > 0
			: step === 1
				? gridRows >= 1 && gridColumns >= 1
				: step === 2
					? shapeChoices.some((s) => s.enabled)
					: step === 3
						? upm > 0 && height > 0
						: true;

	function next() {
		if (step < totalSteps - 1) step++;
	}
	function prev() {
		if (step > 0) step--;
	}

	function finish() {
		resetProjectState();
		const rules: Rule[] = [];
		for (const sc of shapeChoices) {
			if (sc.enabled) rules.push({ ...sc.rule });
		}
		const syntax: Syntax = {
			id: nanoid(),
			name: 'Default',
			rules,
			grid: { rows: gridRows, columns: gridColumns }
		};
		const mp = {
			id: 'default',
			name: 'Default',
			...normalizeFontMetrics({ UPM: upm, height, descender, capHeight, xHeight })
		};
		const meta = {
			id: 'default',
			name: 'Default',
			...normalizeFontMetadata({ familyName, designer, version })
		};
		const font = {
			id: nanoid(),
			name: 'Regular',
			syntaxId: syntax.id,
			metricsId: 'default',
			metadataId: 'default',
			outputName: `${familyName || 'GTL'}-Regular.otf`,
			enabled: true
		};

		syntaxes.set([...$syntaxes, syntax]);
		metricsPresets.set([mp]);
		metrics.set(mp);
		metadataPresets.set([meta]);
		fontMetadata.set(meta);
		fontDefinitions.set([font]);
		activeFontId.set(font.id);

		const seen = new Set<string>();
		const newGlyphs: Array<{ id: string; name: string; structure: string; set?: string }> = [];
		for (const line of suggestedGlyphs.split('\n')) {
			const name = line.trim();
			if (!name || seen.has(name)) continue;
			seen.add(name);
			const rule = syntax.rules.find((r) => r.symbol === name);
			newGlyphs.push({
				id: nanoid(5),
				name,
				structure: rule && rule.shape.kind !== ShapeKind.Void ? name : ' '
			});
		}
		glyphs.set(newGlyphs);
		saveGlyphsForFont(font.id, newGlyphs);
		open = false;
		// Wait for collab full-snapshot push (debouncedFullPush at 300ms + PUT roundtrip)
		// before navigating — otherwise the SSE stream may trigger a version-gap
		// reload that fetches partial server state and wipes the just-created data.
		setTimeout(() => goto(`${base}/glyphs`), 600);
	}
</script>

<Modal
	outsideclose
	class="!rounded-none !font-mono"
	bind:open
	title="Configurazione nuovo font"
	size="xl"
>
	<div class="flex flex-col gap-4" style="min-height: 60vh;">
		<div class="flex items-center gap-1">
			{#each stepLabels as name, i}
				<button class={stepClasses[i]} on:click={() => (step = i)} disabled={i > step}>
					{i + 1}. {name}
				</button>
				{#if i < stepLabels.length - 1}<span class="text-slate-300">→</span>{/if}
			{/each}
		</div>

		<div class="flex grow gap-6">
			<div class="w-2/3 space-y-4">
				{#if step === 0}
					<p class="text-sm font-semibold">Come si chiama il tuo font?</p>
					<div>
						<label class="text-xs text-slate-600">Nome famiglia</label>
						<input
							class="w-full border border-slate-400 px-3 py-2 text-sm"
							bind:value={familyName}
							placeholder="GTL"
						/>
					</div>
					<div>
						<label class="text-xs text-slate-600">Progettista</label>
						<input
							class="w-full border border-slate-400 px-3 py-2 text-sm"
							bind:value={designer}
							placeholder="Il tuo nome"
						/>
					</div>
					<div>
						<label class="text-xs text-slate-600">Versione</label>
						<input class="w-full border border-slate-400 px-3 py-2 text-sm" bind:value={version} />
					</div>
				{:else if step === 1}
					<p class="text-sm font-semibold">Quanto è grande la griglia di disegno?</p>
					<p class="text-xs text-slate-500">
						Ogni lettera viene disegnata su una griglia di celle. Più celle = più dettaglio.
					</p>
					<div class="flex gap-4">
						<div>
							<label class="text-xs text-slate-600">Righe (altezza)</label>
							<input
								type="number"
								class="w-24 border border-slate-400 px-3 py-2 text-sm"
								bind:value={gridRows}
								min="1"
								max="20"
							/>
						</div>
						<div>
							<label class="text-xs text-slate-600">Colonne</label>
							<input
								type="number"
								class="w-24 border border-slate-400 px-3 py-2 text-sm"
								bind:value={gridColumns}
								min="1"
								max="20"
							/>
						</div>
					</div>
					<div class="rounded border bg-slate-50 p-3">
						<p class="text-xs text-slate-500">Anteprima {gridRows}×{gridColumns}:</p>
						<div
							class="mt-1 grid gap-px bg-slate-300"
							style="grid-template-columns: repeat({gridColumns}, minmax(16px, 1fr)); width: {gridColumns *
								20}px;"
						>
							{#each Array(gridRows * gridColumns) as _}<div
									class="aspect-square bg-white"
								></div>{/each}
						</div>
					</div>
				{:else if step === 2}
					<p class="text-sm font-semibold">Scegli le forme base (pennelli)</p>
					<p class="text-xs text-slate-500">
						Ogni forma viene assegnata a un simbolo. Puoi cambiarli dopo.
					</p>
					<div class="grid grid-cols-2 gap-2">
						{#each shapeChoices as sc (sc.label)}
							{@const lblClass = sc.enabled
								? 'flex items-center gap-2 rounded border p-2 cursor-pointer border-blue-400 bg-blue-50'
								: 'flex items-center gap-2 rounded border p-2 cursor-pointer border-slate-200'}
							<label class={lblClass}>
								<input type="checkbox" bind:checked={sc.enabled} />
								<div class="flex h-10 w-10 items-center justify-center">
									<RuleShapePreview rule={sc.rule} className="w-8 h-8" />
								</div>
								<div class="flex-1 text-xs">
									<div class="font-semibold">{sc.label}</div>
									<div class="flex items-center gap-1 text-slate-400">
										Simbolo:
										<input
											class="w-8 border border-slate-300 px-1 py-0.5 text-center font-mono text-xs"
											maxlength="1"
											bind:value={sc.symbol}
											on:input={() => onSymbolChange(sc)}
										/>
									</div>
								</div>
							</label>
						{/each}
					</div>
				{:else if step === 3}
					<p class="text-sm font-semibold">Metriche del font</p>
					<p class="text-xs text-slate-500">Proporzioni verticali del font.</p>
					<div class="grid grid-cols-2 gap-3">
						<div>
							<label class="text-xs text-slate-600">UPM</label>
							<input
								type="number"
								class="w-full border border-slate-400 px-3 py-2 text-sm"
								bind:value={upm}
								min="100"
								step="100"
							/>
						</div>
						<div>
							<label class="text-xs text-slate-600">Altezza (celle)</label>
							<input
								type="number"
								class="w-full border border-slate-400 px-3 py-2 text-sm"
								bind:value={height}
								min="1"
								max="20"
							/>
						</div>
						<div>
							<label class="text-xs text-slate-600">Discendente</label>
							<input
								type="number"
								class="w-full border border-slate-400 px-3 py-2 text-sm"
								bind:value={descender}
								min="0"
								max={height - 1}
							/>
						</div>
						<div>
							<label class="text-xs text-slate-600">Altezza maiuscole</label>
							<input
								type="number"
								class="w-full border border-slate-400 px-3 py-2 text-sm"
								bind:value={capHeight}
								min="1"
							/>
						</div>
						<div>
							<label class="text-xs text-slate-600">Altezza minuscole</label>
							<input
								type="number"
								class="w-full border border-slate-400 px-3 py-2 text-sm"
								bind:value={xHeight}
								min="1"
							/>
						</div>
					</div>
				{:else if step === 4}
					<p class="text-sm font-semibold">Glifi iniziali</p>
					<p class="text-xs text-slate-500">Scegli i set di glifi da includere:</p>
					<div class="grid grid-cols-2 gap-2">
						{#each glyphSetDefs as def (def.id)}
							{@const checked = selectedGlyphSetIds.has(def.id)}
							{@const names = checked ? getGlyphNamesForSet(def.id) : []}
							<label
								class="cursor-pointer rounded border p-2 {checked
									? 'border-blue-400 bg-blue-50'
									: 'border-slate-200 hover:bg-slate-50'}"
							>
								<div class="flex items-center gap-2">
									<input
										type="checkbox"
										checked={selectedGlyphSetIds.has(def.id)}
										on:change={() => toggleGlyphSet(def.id)}
									/>
									<span class="text-sm font-semibold">{def.label}</span>
									<span class="text-xs text-slate-400">({names.length} glifi)</span>
								</div>
								{#if checked && names.length > 0}
									<div class="mt-1 max-h-20 overflow-y-auto font-mono text-xs text-slate-500">
										{names.slice(0, 30).join(' ')}{names.length > 30 ? ' ...' : ''}
									</div>
								{/if}
							</label>
						{/each}
					</div>
					{#if selectedGlyphNames.length > 0}
						<p class="text-xs text-slate-400">
							Saranno creati {selectedGlyphNames.length} glifi totali.
						</p>
					{/if}
				{/if}
			</div>

			<div class="w-1/3 space-y-3 rounded bg-slate-50 p-4 text-xs text-slate-600">
				<p class="font-semibold text-slate-800">💡 Guida</p>
				{#if step === 0}
					<p>
						Il <b>nome famiglia</b> è il nome del font (es. "GTL"). Appare nei menu dei programmi.
					</p>
					<p>Il <b>progettista</b> sei tu! Viene incluso nei metadati del file OTF.</p>
				{:else if step === 1}
					<p>
						La <b>griglia</b> definisce in quante celle viene divisa ogni lettera. Una griglia 5×5 è
						un buon punto di partenza.
					</p>
					<p>Più celle = più dettaglio, ma più lavoro per disegnare ogni glifo.</p>
				{:else if step === 2}
					<p>
						I <b>pennelli</b> sono le forme base per disegnare i glifi. Ogni forma ha un simbolo sulla
						tastiera.
					</p>
					<p>
						<b>Pieno</b> riempie la cella. <b>Quarto</b> e <b>Triangolo</b> per diagonali.
						<b>Curva</b> per forme SVG.
					</p>
					<p>Puoi modificare tutto dopo nella pagina Sintassi.</p>
				{:else if step === 3}
					<p><b>UPM</b> = unità per em, la risoluzione interna. 5000 va bene.</p>
					<p>
						<b>Altezza</b> = celle totali. Con 5 celle e discendente 1: 4 celle sopra, 1 sotto la baseline.
					</p>
				{:else if step === 4}
					<p>
						I <b>glifi</b> sono i caratteri del font. Inizia con lettere, numeri e punteggiatura base.
					</p>
					<p>Ogni glifo parte con una cella vuota. Li disegnerai nella pagina <b>Glifi</b>.</p>
				{/if}
			</div>
		</div>

		<div class="flex items-center justify-between border-t pt-3">
			<Button on:click={prev} disabled={step === 0}>← Indietro</Button>
			<span class="text-xs text-slate-400">{step + 1} / {totalSteps}</span>
			{#if step < totalSteps - 1}
				<Button on:click={next} disabled={!canNext}>Avanti →</Button>
			{:else}
				<button
					class="flex bg-green-700 p-3 font-mono text-sm text-white hover:bg-green-800"
					on:click={finish}
					disabled={!canNext}
				>
					Crea font e inizia a disegnare!
				</button>
			{/if}
		</div>
	</div>
</Modal>

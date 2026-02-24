<script lang="ts">
	import type { GlyphInput, Rule, Syntax } from '$lib/types';
	import { glyphs, metrics, selectedGlyph, syntaxes } from '$lib/stores';

	import Sidebar from '$lib/ui/sidebar.svelte';
	import SidebarTile from '$lib/ui/sidebarTile.svelte';
	import Button from '$lib/ui/button.svelte';
	import GlyphPainter from '$lib/components/glyph/glyphPainter.svelte';
	import GlyphPreview from '$lib/partials/glyphPreview.svelte';

	import { ShapeKind, createEmptyRule } from '$lib/types';
	import DeleteButton from '$lib/ui/deleteButton.svelte';
	import AddGlyphModal from './AddGlyphModal.svelte';
	import AddGlyphSetModal from './AddGlyphSetModal.svelte';
	import { UNICODE } from '$lib/GTL/unicode';
	import { resolveUnicodeNumber } from '$lib/GTL/glyphName';
	import {
		getGlyphSetByID,
		getGlyphSetDefinitions,
		inferGlyphSetID
	} from '$lib/GTL/glyphSets';
	import {
		isComponentGlyphName,
		parseGlyphStructure,
		replaceGlyphStructureBody,
		replaceGlyphStructureComponents,
		resolveGlyphStructures,
		resolveGlyphStructuresWithComponentMask,
		serializeGlyphStructure,
		type GlyphComponentRef
	} from '$lib/GTL/structure';

	//

	function getUniqueSymbols(
		glyphs: Array<GlyphInput>,
		resolvedTextByName: Map<string, string>,
		resolvedVisualByName: Map<string, string>
	): Array<string> {
		const symbols = new Set<string>();
		for (const glyph of glyphs) {
			const parsed = parseGlyphStructure(glyph.structure);
			const textStructure = resolvedTextByName.get(glyph.name) ?? parsed.body;
			const visualStructure = resolvedVisualByName.get(glyph.name) ?? parsed.body;
			for (const char of Array.from(textStructure.replace(/\n/g, ''))) {
				symbols.add(char);
			}
			for (const char of Array.from(visualStructure.replace(/\n/g, ''))) {
				symbols.add(char);
			}
		}
		return Array.from(symbols);
	}

	function updateSyntaxSymbols(syntax: Syntax, symbols: Array<string>): boolean {
		let changed = false;
		const usedSymbols = new Set(symbols);

		// Getting all symbols in syntax
		const syntaxSymbols = syntax.rules.map((r) => r.symbol);
		// Checking for additions
		for (let symbol of symbols) {
			if (!syntaxSymbols.includes(symbol)) {
				const newRule = createEmptyRule(symbol);
				newRule.unused = false;
				syntax.rules.push(newRule);
				changed = true;
			}
		}

		// Keep rules, but mark whether they are currently unused in glyphs.
		for (const rule of syntax.rules) {
			const nextUnused = !usedSymbols.has(rule.symbol);
			if ((rule.unused ?? false) !== nextUnused) {
				rule.unused = nextUnused;
				changed = true;
			}
		}

		return changed;
	}

	$: resolvedGlyphStructuresText = resolveGlyphStructures($glyphs, {
		transparentSymbols: [' ', '.']
	});
	$: resolvedGlyphStructuresVisualData = resolveGlyphStructuresWithComponentMask($glyphs, {
		transparentSymbols: [' ', '.'],
		applySymbolOverride: false
	});
	$: resolvedGlyphStructuresVisual = new Map(
		Array.from(resolvedGlyphStructuresVisualData.entries()).map(([name, resolved]) => [
			name,
			resolved.body
		])
	);
	$: {
		const uniqueSymbols = getUniqueSymbols(
			$glyphs,
			resolvedGlyphStructuresText,
			resolvedGlyphStructuresVisual
		);
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
		const parsed = parseGlyphStructure(structure);
		const sourceRows = parsed.body ? parsed.body.split(/\r?\n/) : [];
		const width = Math.max(1, ...sourceRows.map((row) => row.length));
		const height = Math.max(1, targetHeight, sourceRows.length);
		const rows = Array.from({ length: height }, (_, index) => sourceRows[index] ?? '');

		if (width <= 0) return structure;

		const nextBody = rows
			.map((row) =>
				row
					.padEnd(width, ' ')
					.split('')
					.map((char) => (char === ' ' ? fillSymbol : char))
					.join('')
			)
			.join('\n');

		return replaceGlyphStructureBody(structure, nextBody);
	}

	function getGlyphComponents(glyph: GlyphInput): Array<GlyphComponentRef> {
		return parseGlyphStructure(glyph.structure).components;
	}

	function getGlyphBody(glyph: GlyphInput): string {
		return parseGlyphStructure(glyph.structure).body;
	}

	function getResolvedGlyphBody(glyph: GlyphInput): string {
		return resolvedGlyphStructuresVisualData.get(glyph.name)?.body ?? getGlyphBody(glyph);
	}

	function getResolvedGlyphComponentSources(glyph: GlyphInput): Array<Array<Array<string>>> {
		return resolvedGlyphStructuresVisualData.get(glyph.name)?.componentSources ?? [];
	}

	function structureHasDesignMarks(structure: string): boolean {
		for (const char of Array.from(structure.replace(/\n/g, ''))) {
			if (char !== ' ' && char !== '.') return true;
		}
		return false;
	}

	function isGlyphDesigned(glyph: GlyphInput): boolean {
		const resolved = resolvedGlyphStructuresVisualData.get(glyph.name)?.body ?? getGlyphBody(glyph);
		return structureHasDesignMarks(resolved);
	}

	function getGlyphSetID(glyph: GlyphInput) {
		return inferGlyphSetID(glyph);
	}

	function getGlyphStructureTextareaValue(glyph: GlyphInput): string {
		const parsed = parseGlyphStructure(glyph.structure);
		if (!parsed.components.length) return glyph.structure;
		return serializeGlyphStructure({
			components: parsed.components,
			body: resolvedGlyphStructuresText.get(glyph.name) ?? parsed.body
		});
	}

	function handleGlyphStructureInput(glyph: GlyphInput, nextValue: string) {
		const currentParsed = parseGlyphStructure(glyph.structure);
		const nextParsed = parseGlyphStructure(nextValue);

		if (!currentParsed.components.length) {
			glyph.structure = nextValue;
			scheduleTouchGlyphs();
			return;
		}

		const nextBody = nextParsed.components.length ? currentParsed.body : nextParsed.body;
		glyph.structure = serializeGlyphStructure({
			components: nextParsed.components,
			body: nextBody
		});
		scheduleTouchGlyphs();
	}

	let touchGlyphsTimer: ReturnType<typeof setTimeout> | undefined;

	function touchGlyphs() {
		$glyphs = [...$glyphs];
	}

	function scheduleTouchGlyphs() {
		if (touchGlyphsTimer) return;
		touchGlyphsTimer = setTimeout(() => {
			touchGlyphsTimer = undefined;
			touchGlyphs();
		}, 16);
	}

	function inputValue(event: Event): string {
		const input = (event.target || event.currentTarget) as
			| HTMLInputElement
			| HTMLTextAreaElement
			| null;
		return input?.value ?? '';
	}

	function inputNumericValue(event: Event): number {
		return Number(inputValue(event));
	}

	function getAvailableComponentGlyphs(targetGlyphName: string): Array<GlyphInput> {
		return sortGlyphs(
			$glyphs.filter(
				(glyph) => isComponentGlyphName(glyph.name) && glyph.name !== targetGlyphName
			)
		);
	}

	function normalizeComponentSymbolInput(value: string): string {
		return Array.from((value ?? '').trim())[0] ?? '';
	}

	function normalizeComponentRotationInput(value: number): number {
		if (!Number.isFinite(value)) return 0;
		const stepped = Math.round(value / 15) * 15;
		const normalized = ((stepped % 360) + 360) % 360;
		return normalized === 360 ? 0 : normalized;
	}

	function addComponentReference(targetGlyph: GlyphInput) {
		if (!newComponentName || newComponentName === targetGlyph.name) return;

		const component: GlyphComponentRef = {
			name: newComponentName,
			symbol: normalizeComponentSymbolInput(newComponentSymbol),
			x: Math.max(1, Math.trunc(newComponentX || 1)),
			y: Math.max(1, Math.trunc(newComponentY || 1)),
			rotation: normalizeComponentRotationInput(newComponentRotation)
		};

		targetGlyph.structure = replaceGlyphStructureComponents(targetGlyph.structure, [
			...getGlyphComponents(targetGlyph),
			component
		]);
		touchGlyphs();
	}

	function removeComponentReference(targetGlyph: GlyphInput, componentIndex: number) {
		const currentComponents = getGlyphComponents(targetGlyph);
		targetGlyph.structure = replaceGlyphStructureComponents(
			targetGlyph.structure,
			currentComponents.filter((_, index) => index !== componentIndex)
		);
		touchGlyphs();
	}

	function updateComponentReference(
		targetGlyph: GlyphInput,
		componentIndex: number,
		patch: Partial<GlyphComponentRef>
	) {
		const currentComponents = getGlyphComponents(targetGlyph);
		const updated = currentComponents.map((component, index) => {
			if (index !== componentIndex) return component;

			return {
				...component,
				...patch,
				symbol:
					patch.symbol !== undefined
						? normalizeComponentSymbolInput(patch.symbol)
						: component.symbol,
				x: patch.x !== undefined ? Math.max(1, Math.trunc(patch.x || 1)) : component.x,
				y: patch.y !== undefined ? Math.max(1, Math.trunc(patch.y || 1)) : component.y,
				rotation:
					patch.rotation !== undefined
						? normalizeComponentRotationInput(patch.rotation)
						: component.rotation
			};
		});

		targetGlyph.structure = replaceGlyphStructureComponents(targetGlyph.structure, updated);
		touchGlyphs();
	}

	//

	let isAddGlyphModalOpen = false;
	let isAddGlyphSetModalOpen = false;
	let designTopRatio = 0.45;
	let isResizingDesignPanels = false;
	let designPanelsEl: HTMLDivElement | undefined;
	let newComponentName = '';
	let newComponentSymbol = '';
	let newComponentX = 1;
	let newComponentY = 1;
	let newComponentRotation = 0;
	let selectedGlyphSetFilter = 'all';
	let showOnlyUndesignedGlyphs = false;

	$: brushSymbols = getSyntaxSymbols($syntaxes);
	$: rulesBySymbol = getRulesBySymbol($syntaxes);
	$: voidFillSymbol = getVoidFillSymbol(rulesBySymbol);
	$: fillTargetHeight = Math.max(1, Math.round($metrics.height || 1));
	$: glyphSetDefinitions = getGlyphSetDefinitions();
	$: filteredGlyphs = sortGlyphs($glyphs).filter((glyph) => {
		if (selectedGlyphSetFilter !== 'all' && getGlyphSetID(glyph) !== selectedGlyphSetFilter) {
			return false;
		}
		if (showOnlyUndesignedGlyphs && isGlyphDesigned(glyph)) {
			return false;
		}
		return true;
	});
	$: if (filteredGlyphs.length && !filteredGlyphs.some((glyph) => glyph.id === $selectedGlyph)) {
		$selectedGlyph = filteredGlyphs[0].id;
	}
	$: selectedGlyphData = $glyphs.find((glyph) => glyph.id === $selectedGlyph);
	$: selectableComponentGlyphNames = selectedGlyphData
		? getAvailableComponentGlyphs(selectedGlyphData.name).map((glyph) => glyph.name)
		: [];
	$: if (!selectableComponentGlyphNames.includes(newComponentName)) {
		newComponentName = selectableComponentGlyphNames[0] ?? '';
	}

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
				<div class="space-y-2">
					<div class="flex gap-2">
						<Button
							on:click={() => {
								isAddGlyphModalOpen = true;
							}}>+ Aggiungi glifo</Button
						>
						<Button
							on:click={() => {
								isAddGlyphSetModalOpen = true;
							}}>+ Aggiungi set</Button
						>
					</div>

					<div class="space-y-1 font-mono text-xs">
						<p class="text-slate-600">Filtro set</p>
						<select class="w-full h-9 bg-slate-200 px-2" bind:value={selectedGlyphSetFilter}>
							<option value="all">Tutti i set</option>
							{#each glyphSetDefinitions as definition (definition.id)}
								<option value={definition.id}>{definition.label}</option>
							{/each}
						</select>
					</div>

					<label class="flex items-center gap-2 font-mono text-xs text-slate-700">
						<input type="checkbox" bind:checked={showOnlyUndesignedGlyphs} />
						<span>Mostra solo glifi vuoti</span>
					</label>
				</div>
			</svelte:fragment>
			<svelte:fragment slot="listTitle">Lista glifi</svelte:fragment>
			<svelte:fragment slot="items">
				{#if filteredGlyphs.length === 0}
					<p class="font-mono text-xs text-slate-500 px-2 py-1">
						Nessun glifo nel filtro corrente.
					</p>
				{/if}
				{#each filteredGlyphs as g (g.id)}
					{@const glyphString = getGlyphString(g.name)}
					{@const glyphSet = getGlyphSetByID(getGlyphSetID(g))}
					{@const designed = isGlyphDesigned(g)}
					<SidebarTile selection={selectedGlyph} id={g.id}>
						<div class="flex items-center justify-between gap-2">
							<div class="min-w-0 truncate">
								{#if glyphString}
									{glyphString}
								{/if}
								<span class="opacity-25"> – {g.name}</span>
								<span class="opacity-50"> [{glyphSet.label}]</span>
							</div>
							<span class={designed ? 'text-emerald-500' : 'text-rose-500'}>
								{designed ? '●' : '○'}
							</span>
						</div>
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
				{@const componentGlyphs = getAvailableComponentGlyphs(g.name)}
				{@const glyphComponents = getGlyphComponents(g)}
				{@const resolvedGlyphBody = getResolvedGlyphBody(g)}
				{@const resolvedGlyphComponentSources = getResolvedGlyphComponentSources(g)}
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
											touchGlyphs();
										}}
									>
										Fill void ({voidFillSymbol} / h={fillTargetHeight})
									</Button>
								</div>
								<div class="mb-2 p-2 border border-slate-300 bg-slate-50 space-y-2 font-mono text-xs">
									<p class="text-slate-900">
										Componenti ({glyphComponents.length})
									</p>

									{#if componentGlyphs.length}
										<div class="flex flex-wrap items-end gap-2">
											<div class="flex flex-col gap-1">
												<label class="text-slate-500" for="component-name-select">Nome</label>
												<select
													id="component-name-select"
													class="h-10 bg-slate-200 px-2"
													bind:value={newComponentName}
												>
													{#each componentGlyphs as componentGlyph (componentGlyph.id)}
														<option value={componentGlyph.name}>{componentGlyph.name}</option>
													{/each}
												</select>
											</div>
											<div class="flex flex-col gap-1">
												<label class="text-slate-500" for="component-symbol-input">Simbolo</label>
												<input
													id="component-symbol-input"
													class="h-10 w-16 bg-slate-200 px-2"
													type="text"
													maxlength="2"
													bind:value={newComponentSymbol}
												/>
											</div>
											<div class="flex flex-col gap-1">
												<label class="text-slate-500" for="component-x-input">x</label>
												<input
													id="component-x-input"
													class="h-10 w-16 bg-slate-200 px-2"
													type="number"
													min="1"
													step="1"
													bind:value={newComponentX}
												/>
											</div>
											<div class="flex flex-col gap-1">
												<label class="text-slate-500" for="component-y-input">y</label>
												<input
													id="component-y-input"
													class="h-10 w-16 bg-slate-200 px-2"
													type="number"
													min="1"
													step="1"
													bind:value={newComponentY}
												/>
											</div>
											<div class="flex flex-col gap-1">
												<label class="text-slate-500" for="component-rotation-input">rot°</label>
												<input
													id="component-rotation-input"
													class="h-10 w-20 bg-slate-200 px-2"
													type="number"
													step="15"
													bind:value={newComponentRotation}
												/>
											</div>
											<Button on:click={() => addComponentReference(g)}>+ Aggiungi componente</Button>
										</div>
									{:else}
										<p class="text-slate-500">
											Nessun glifo `.component` disponibile. Crea un glifo come `etom.component`.
										</p>
									{/if}

									{#if glyphComponents.length}
										<div class="space-y-1">
											{#each glyphComponents as component, index (`${component.name}:${index}`)}
												<div
													class="flex items-center justify-between gap-2 border border-slate-200 bg-white px-2 py-1"
												>
													<div class="flex items-center gap-2 text-[11px]">
														<p class="truncate w-32">{component.name}</p>
														<label class="flex items-center gap-1">
															<span>s</span>
															<input
																class="w-10 h-7 border border-slate-300 px-1"
																type="text"
																maxlength="2"
																value={component.symbol}
																on:input={(event) => {
																	updateComponentReference(g, index, {
																		symbol: inputValue(event)
																	});
																}}
																on:change={(event) => {
																	updateComponentReference(g, index, {
																		symbol: inputValue(event)
																	});
																}}
																on:blur={(event) => {
																	updateComponentReference(g, index, {
																		symbol: inputValue(event)
																	});
																}}
															/>
														</label>
														<label class="flex items-center gap-1">
															<span>x</span>
															<input
																class="w-12 h-7 border border-slate-300 px-1"
																type="number"
																min="1"
																step="1"
																value={component.x}
																on:input={(event) => {
																	updateComponentReference(g, index, {
																		x: inputNumericValue(event)
																	});
																}}
																on:change={(event) => {
																	updateComponentReference(g, index, {
																		x: inputNumericValue(event)
																	});
																}}
																on:blur={(event) => {
																	updateComponentReference(g, index, {
																		x: inputNumericValue(event)
																	});
																}}
															/>
														</label>
														<label class="flex items-center gap-1">
															<span>y</span>
															<input
																class="w-12 h-7 border border-slate-300 px-1"
																type="number"
																min="1"
																step="1"
																value={component.y}
																on:input={(event) => {
																	updateComponentReference(g, index, {
																		y: inputNumericValue(event)
																	});
																}}
																on:change={(event) => {
																	updateComponentReference(g, index, {
																		y: inputNumericValue(event)
																	});
																}}
																on:blur={(event) => {
																	updateComponentReference(g, index, {
																		y: inputNumericValue(event)
																	});
																}}
															/>
														</label>
														<label class="flex items-center gap-1">
															<span>rot</span>
															<input
																class="w-16 h-7 border border-slate-300 px-1"
																type="number"
																step="15"
																value={component.rotation}
																on:input={(event) => {
																	updateComponentReference(g, index, {
																		rotation: inputNumericValue(event)
																	});
																}}
																on:change={(event) => {
																	updateComponentReference(g, index, {
																		rotation: inputNumericValue(event)
																	});
																}}
																on:blur={(event) => {
																	updateComponentReference(g, index, {
																		rotation: inputNumericValue(event)
																	});
																}}
															/>
														</label>
													</div>
													<button
														type="button"
														class="text-red-600 hover:text-red-800"
														on:click={() => removeComponentReference(g, index)}
													>
														rimuovi
													</button>
												</div>
											{/each}
										</div>
									{/if}
								</div>
								<textarea
									class="h-0 grow min-h-0 p-2 bg-slate-200 tracking-[0.75em] hover:bg-slate-300 font-mono focus:ring-4"
									value={getGlyphStructureTextareaValue(g)}
									on:input={(event) => {
										handleGlyphStructureInput(g, inputValue(event));
									}}
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
									<GlyphPainter
										bind:structure={g.structure}
										resolvedBody={resolvedGlyphBody}
										resolvedComponentSources={resolvedGlyphComponentSources}
										brushes={brushSymbols}
										{rulesBySymbol}
										on:change={scheduleTouchGlyphs}
									/>
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
<AddGlyphSetModal bind:open={isAddGlyphSetModalOpen} />

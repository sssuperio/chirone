<script lang="ts">
	import { onMount } from 'svelte';
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
	import { reflectGlyphStructureBody } from '$lib/GTL/structureTransforms';

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

	type GlyphEditorTab = 'visualDesign' | 'glyphStructure';

	let isAddGlyphModalOpen = false;
	let isAddGlyphSetModalOpen = false;
	let activeGlyphEditorTab: GlyphEditorTab = 'visualDesign';
	let isZenMode = false;
	let isDesignFullscreen = false;
	let designWorkspaceElement: HTMLDivElement | undefined;
	let floatingToolbarElement: HTMLDivElement | undefined;
	let floatingToolbarX = 24;
	let floatingToolbarY = 88;
	let isFloatingToolbarDragging = false;
	let floatingToolbarDragOffsetX = 0;
	let floatingToolbarDragOffsetY = 0;
	const previewCanvasHeight = 500;
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
	function isTypingTarget(target: EventTarget | null): boolean {
		if (!(target instanceof HTMLElement)) return false;
		const tagName = target.tagName.toLowerCase();
		if (tagName === 'input' || tagName === 'textarea' || tagName === 'select') return true;
		return target.isContentEditable;
	}

	function setZenMode(nextValue: boolean) {
		isZenMode = nextValue;
		if (isZenMode) {
			activeGlyphEditorTab = 'visualDesign';
		}
	}

	function toggleZenMode() {
		setZenMode(!isZenMode);
	}

	function clearSelectedGlyphDesign() {
		if (!selectedGlyphData) return;
		selectedGlyphData.structure = replaceGlyphStructureBody(selectedGlyphData.structure, '');
		touchGlyphs();
	}

	function fillVoidSelectedGlyph() {
		if (!selectedGlyphData) return;
		selectedGlyphData.structure = fillVoidInStructure(
			selectedGlyphData.structure,
			voidFillSymbol,
			fillTargetHeight
		);
		touchGlyphs();
	}

	function mirrorSelectedGlyphLeftRight() {
		if (!selectedGlyphData) return;
		const parsed = parseGlyphStructure(selectedGlyphData.structure);
		selectedGlyphData.structure = replaceGlyphStructureBody(
			selectedGlyphData.structure,
			reflectGlyphStructureBody(parsed.body, 'vertical', rulesBySymbol)
		);
		touchGlyphs();
	}

	function flipSelectedGlyphTopBottom() {
		if (!selectedGlyphData) return;
		const parsed = parseGlyphStructure(selectedGlyphData.structure);
		selectedGlyphData.structure = replaceGlyphStructureBody(
			selectedGlyphData.structure,
			reflectGlyphStructureBody(parsed.body, 'horizontal', rulesBySymbol)
		);
		touchGlyphs();
	}

	function getFloatingToolbarSize(): { width: number; height: number } {
		if (!floatingToolbarElement) {
			return { width: 160, height: 220 };
		}
		const rect = floatingToolbarElement.getBoundingClientRect();
		return {
			width: Math.max(80, Math.round(rect.width)),
			height: Math.max(80, Math.round(rect.height))
		};
	}

	function clampFloatingToolbarPosition(x: number, y: number): { x: number; y: number } {
		if (typeof window === 'undefined') {
			return { x, y };
		}
		const margin = 8;
		const { width, height } = getFloatingToolbarSize();
		const maxX = Math.max(margin, window.innerWidth - width - margin);
		const maxY = Math.max(margin, window.innerHeight - height - margin);
		return {
			x: Math.min(Math.max(margin, Math.round(x)), maxX),
			y: Math.min(Math.max(margin, Math.round(y)), maxY)
		};
	}

	function setFloatingToolbarPosition(x: number, y: number) {
		const next = clampFloatingToolbarPosition(x, y);
		floatingToolbarX = next.x;
		floatingToolbarY = next.y;
	}

	function positionFloatingToolbarDefault() {
		if (typeof window === 'undefined') return;
		const { width } = getFloatingToolbarSize();
		setFloatingToolbarPosition(window.innerWidth - width - 16, 96);
	}

	function startFloatingToolbarDrag(event: PointerEvent) {
		if (!(event.currentTarget instanceof HTMLElement)) return;
		event.preventDefault();
		isFloatingToolbarDragging = true;
		floatingToolbarDragOffsetX = event.clientX - floatingToolbarX;
		floatingToolbarDragOffsetY = event.clientY - floatingToolbarY;
		event.currentTarget.setPointerCapture(event.pointerId);
	}

	function onFloatingToolbarPointerMove(event: PointerEvent) {
		if (!isFloatingToolbarDragging) return;
		setFloatingToolbarPosition(
			event.clientX - floatingToolbarDragOffsetX,
			event.clientY - floatingToolbarDragOffsetY
		);
	}

	function stopFloatingToolbarDrag() {
		isFloatingToolbarDragging = false;
	}

	function onFloatingToolbarResize() {
		setFloatingToolbarPosition(floatingToolbarX, floatingToolbarY);
	}

	function syncDesignFullscreenState() {
		if (typeof document === 'undefined') return;
		isDesignFullscreen = document.fullscreenElement === designWorkspaceElement;
	}

	async function toggleDesignFullscreen() {
		if (!designWorkspaceElement || typeof document === 'undefined') return;
		try {
			if (document.fullscreenElement === designWorkspaceElement) {
				await document.exitFullscreen();
				return;
			}
			await designWorkspaceElement.requestFullscreen();
		} catch (error) {
			console.error('Unable to toggle design fullscreen', error);
		}
	}

	onMount(() => {
		if (typeof document === 'undefined') return;

		const onFullscreenChange = () => {
			syncDesignFullscreenState();
		};

		const onKeyDown = (event: KeyboardEvent) => {
			if (isTypingTarget(event.target)) return;
			if (event.repeat) return;

			const key = event.key.toLowerCase();
			const plain = !event.ctrlKey && !event.metaKey && !event.altKey && !event.shiftKey;
			if (plain && key === 'z') {
				event.preventDefault();
				toggleZenMode();
				return;
			}

			if (plain && key === 'f') {
				event.preventDefault();
				void toggleDesignFullscreen();
				return;
			}

			if (plain && key === 'v') {
				event.preventDefault();
				fillVoidSelectedGlyph();
				return;
			}

			if (plain && key === 'm') {
				event.preventDefault();
				mirrorSelectedGlyphLeftRight();
				return;
			}

			if (plain && key === 'x') {
				event.preventDefault();
				flipSelectedGlyphTopBottom();
				return;
			}

			if (plain && key === 'c') {
				event.preventDefault();
				clearSelectedGlyphDesign();
				return;
			}

			if (key === 'escape' && isZenMode) {
				event.preventDefault();
				setZenMode(false);
			}
		};

		document.addEventListener('fullscreenchange', onFullscreenChange);
		window.addEventListener('keydown', onKeyDown);
		syncDesignFullscreenState();
		requestAnimationFrame(() => {
			positionFloatingToolbarDefault();
		});

		return () => {
			document.removeEventListener('fullscreenchange', onFullscreenChange);
			window.removeEventListener('keydown', onKeyDown);
		};
	});
</script>

<!--  -->

<svelte:window
	on:pointermove={onFloatingToolbarPointerMove}
	on:pointerup={stopFloatingToolbarDrag}
	on:pointercancel={stopFloatingToolbarDrag}
	on:resize={onFloatingToolbarResize}
/>

<div class="flex flex-row flex-nowrap items-stretch overflow-hidden grow">
	{#if !isZenMode}
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
	{/if}

	<!-- Glyph area -->
	<div class={`grow flex flex-col items-stretch ${isZenMode ? 'p-2 space-y-2' : 'p-8 space-y-8'}`}>
		{#each $glyphs as g}
			{#if g.id == $selectedGlyph}
				{@const glyphString = getGlyphString(g.name)}
				{@const glyphName = UNICODE[g.name]}
				{@const componentGlyphs = getAvailableComponentGlyphs(g.name)}
				{@const glyphComponents = getGlyphComponents(g)}
				{@const resolvedGlyphBody = getResolvedGlyphBody(g)}
				{@const resolvedGlyphComponentSources = getResolvedGlyphComponentSources(g)}
					{#if !isZenMode}
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
					{/if}
						<div
							bind:this={designWorkspaceElement}
							class={`h-0 grow min-h-0 flex gap-4 ${isZenMode ? 'flex-row' : 'flex-col lg:flex-row'}`}
						>
								<div class="min-h-0 min-w-0 flex-1 flex flex-col gap-2">
								<div class="h-0 grow min-h-0 flex flex-col">
									<div class="mb-2 flex items-center gap-2 border-b border-slate-300 pb-2">
										<button
											type="button"
											class={`px-3 py-2 text-sm font-mono ${
											activeGlyphEditorTab === 'visualDesign'
												? 'bg-slate-800 text-white'
												: 'bg-slate-200 text-slate-800 hover:bg-slate-300'
										}`}
										aria-pressed={activeGlyphEditorTab === 'visualDesign'}
										on:click={() => {
											activeGlyphEditorTab = 'visualDesign';
										}}
									>
										Visual design
									</button>
									<button
										type="button"
										class={`px-3 py-2 text-sm font-mono ${
											activeGlyphEditorTab === 'glyphStructure'
												? 'bg-slate-800 text-white'
												: 'bg-slate-200 text-slate-800 hover:bg-slate-300'
										}`}
										aria-pressed={activeGlyphEditorTab === 'glyphStructure'}
										on:click={() => {
											activeGlyphEditorTab = 'glyphStructure';
										}}
										>
											Glyph structure
										</button>
									</div>

								{#if activeGlyphEditorTab === 'glyphStructure'}
									<div class="h-0 grow min-h-0 flex flex-col">
										<div
											class="mb-2 p-2 border border-slate-300 bg-slate-50 space-y-2 font-mono text-xs"
										>
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
														<label class="text-slate-500" for="component-symbol-input"
															>Simbolo</label
														>
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
														<label class="text-slate-500" for="component-rotation-input"
															>rot°</label
														>
														<input
															id="component-rotation-input"
															class="h-10 w-20 bg-slate-200 px-2"
															type="number"
															step="15"
															bind:value={newComponentRotation}
														/>
													</div>
													<Button on:click={() => addComponentReference(g)}
														>+ Aggiungi componente</Button
													>
												</div>
											{:else}
												<p class="text-slate-500">
													Nessun glifo `.component` disponibile. Crea un glifo come
													`etom.component`.
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
								{:else}
									<div class="h-0 grow min-h-0 flex flex-col">
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
								{/if}
								</div>
							</div>

								<div
									class={`min-h-0 min-w-0 flex flex-col bg-slate-50 ${
										isZenMode
											? 'flex-[0_0_42%] border-l border-slate-300 pl-2'
											: 'flex-1 lg:border-l lg:border-slate-300 lg:pl-4'
									}`}
								>
								<p class="text-small font-mono text-slate-900 mb-2 text-sm">
									Anteprima e metriche
								</p>
								<div class="h-0 grow min-h-0 overflow-y-auto overflow-x-hidden">
									<GlyphPreview
										canvasHeight={previewCanvasHeight}
										showTitle={false}
										showLegend={false}
									/>
							</div>
						</div>
					</div>
				{/if}
				{/each}
	</div>
</div>

<div
	bind:this={floatingToolbarElement}
	class={`fixed z-50 w-44 border border-slate-300 bg-white shadow-lg ${isFloatingToolbarDragging ? 'cursor-grabbing' : ''}`}
	style={`left: ${floatingToolbarX}px; top: ${floatingToolbarY}px;`}
>
	<button
		type="button"
		class="h-8 px-2 border-b border-slate-300 bg-slate-900 text-white font-mono text-xs flex items-center justify-between cursor-grab select-none"
		title="Trascina toolbar"
		on:pointerdown={startFloatingToolbarDrag}
	>
		<span>Tools</span>
		<span>::</span>
	</button>
	<div class="p-2 flex flex-col gap-1 bg-white">
		<button
			type="button"
			class="w-full px-2 py-1 border border-slate-300 hover:bg-slate-100 font-mono text-xs flex items-center gap-2"
			title="Zen mode (Z)"
			on:click={toggleZenMode}
		>
			<span class="inline-flex h-4 w-4 items-center justify-center border border-slate-400 text-[10px]"
				>Z</span
			>
			<span>Zen mode</span>
			<span class="ml-auto text-[10px] text-slate-500">Z</span>
		</button>
		<button
			type="button"
			class="w-full px-2 py-1 border border-slate-300 hover:bg-slate-100 font-mono text-xs flex items-center gap-2"
			title="Full screen (F)"
			on:click={toggleDesignFullscreen}
		>
			<span class="inline-flex h-4 w-4 items-center justify-center border border-slate-400 text-[10px]"
				>F</span
			>
			<span>{isDesignFullscreen ? 'Exit full screen' : 'Full screen'}</span>
			<span class="ml-auto text-[10px] text-slate-500">F</span>
		</button>
		<button
			type="button"
			class="w-full px-2 py-1 border border-slate-300 hover:bg-slate-100 font-mono text-xs flex items-center gap-2 disabled:opacity-40 disabled:hover:bg-white"
			title="Mirror L/R (M)"
			on:click={mirrorSelectedGlyphLeftRight}
			disabled={!selectedGlyphData}
		>
			<span class="inline-flex h-4 w-4 items-center justify-center border border-slate-400 text-[10px]"
				>M</span
			>
			<span>Mirror L/R</span>
			<span class="ml-auto text-[10px] text-slate-500">M</span>
		</button>
		<button
			type="button"
			class="w-full px-2 py-1 border border-slate-300 hover:bg-slate-100 font-mono text-xs flex items-center gap-2 disabled:opacity-40 disabled:hover:bg-white"
			title="Flip U/D (X)"
			on:click={flipSelectedGlyphTopBottom}
			disabled={!selectedGlyphData}
		>
			<span class="inline-flex h-4 w-4 items-center justify-center border border-slate-400 text-[10px]"
				>X</span
			>
			<span>Flip U/D</span>
			<span class="ml-auto text-[10px] text-slate-500">X</span>
		</button>
		<button
			type="button"
			class="w-full px-2 py-1 border border-slate-300 hover:bg-slate-100 font-mono text-xs flex items-center gap-2 disabled:opacity-40 disabled:hover:bg-white"
			title="Fill void (V)"
			on:click={fillVoidSelectedGlyph}
			disabled={!selectedGlyphData}
		>
			<span class="inline-flex h-4 w-4 items-center justify-center border border-slate-400 text-[10px]"
				>V</span
			>
			<span>Fill void</span>
			<span class="ml-auto text-[10px] text-slate-500">V</span>
		</button>
		<button
			type="button"
			class="w-full px-2 py-1 border border-slate-300 hover:bg-slate-100 font-mono text-xs flex items-center gap-2 disabled:opacity-40 disabled:hover:bg-white"
			title="Pulisci (C)"
			on:click={clearSelectedGlyphDesign}
			disabled={!selectedGlyphData}
		>
			<span class="inline-flex h-4 w-4 items-center justify-center border border-slate-400 text-[10px]"
				>C</span
			>
			<span>Pulisci</span>
			<span class="ml-auto text-[10px] text-slate-500">C</span>
		</button>
	</div>
</div>

<!--  -->

<AddGlyphModal bind:open={isAddGlyphModalOpen} />
<AddGlyphSetModal bind:open={isAddGlyphSetModalOpen} />

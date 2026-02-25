<script lang="ts">
	import { parseGlyphStructure, replaceGlyphStructureBody } from '$lib/GTL/structure';
	import { ShapeKind, type Rule } from '$lib/types';
	import { createEventDispatcher } from 'svelte';
	import RuleShapePreview from './ruleShapePreview.svelte';

	export let structure = '';
	export let resolvedBody = '';
	export let resolvedComponentSources: Array<Array<Array<string>>> = [];
	export let brushes: Array<string> = [];
	export let rulesBySymbol: Record<string, Rule> = {};
	export let minRows = 12;
	export let minColumns = 12;
	export let showGrid = true;

	const dispatch = createEventDispatcher<{ change: { structure: string } }>();

	export let selectedBrush = '';
	let drawingMode: 'paint' | 'erase' | null = null;
	let activePointerId: number | null = null;
	let lastPaintedCellKey = '';

	$: parsed = parseGlyphStructure(structure);
	$: sourceBody = resolvedBody || parsed.body;
	$: lines = sourceBody ? sourceBody.split(/\r?\n/) : [];
	$: editableLines = parsed.body ? parsed.body.split(/\r?\n/) : [];
	$: contentRows = lines.length;
	$: contentColumns = Math.max(0, ...lines.map((line) => line.length));

	$: availableBrushes = Array.from(
		new Set(brushes.filter((symbol) => typeof symbol === 'string' && symbol.length === 1))
	);

	$: gridRows = Math.max(minRows, contentRows || 1);
	$: gridColumns = Math.max(minColumns, contentColumns || 1);
	$: gridMatrix = Array.from({ length: gridRows }, (_, row) =>
		Array.from({ length: gridColumns }, (_, col) => lines[row]?.[col] ?? ' ')
	);
	$: overlayMatrix = Array.from({ length: gridRows }, (_, row) =>
		Array.from({ length: gridColumns }, (_, col) => editableLines[row]?.[col] ?? ' ')
	);
	$: componentSourceMatrix = Array.from({ length: gridRows }, (_, row) =>
		Array.from({ length: gridColumns }, (_, col) => resolvedComponentSources[row]?.[col] ?? [])
	);
	$: componentNames = Array.from(
		new Set(componentSourceMatrix.flatMap((row) => row.flatMap((names) => names.filter(Boolean))))
	).sort((a, b) => a.localeCompare(b));
	$: componentColorByName = new Map<string, string>(
		componentNames.map((name, index) => [name, getPaletteColor(index, name)])
	);
	$: {
		componentColorByName;
		componentMixCache.clear();
	}

	$: if (!availableBrushes.includes(selectedBrush)) {
		selectedBrush = availableBrushes[0] ?? '';
	}

	function createEditableMatrix(minRowCount: number, minColumnCount: number): Array<Array<string>> {
		const rowCount = Math.max(minRowCount, editableLines.length || 1);
		const columnCount = Math.max(
			minColumnCount,
			Math.max(0, ...editableLines.map((line) => line.length)) || 1
		);

		return Array.from({ length: rowCount }, (_, row) =>
			Array.from({ length: columnCount }, (_, col) => editableLines[row]?.[col] ?? ' ')
		);
	}

	function updateCell(row: number, col: number, value: string) {
		const matrix = createEditableMatrix(gridRows, gridColumns);

		matrix[row][col] = value;
		structure = replaceGlyphStructureBody(structure, serialize(matrix));
		dispatch('change', { structure });
	}

	function serialize(matrix: Array<Array<string>>): string {
		const rows = matrix.map((chars) => chars.join('').replace(/\s+$/g, ''));

		while (rows.length && rows[rows.length - 1] === '') {
			rows.pop();
		}

		return rows.join('\n');
	}

	function paintCell(row: number, col: number, erase = false) {
		if (!erase && !selectedBrush) return;
		updateCell(row, col, erase ? ' ' : selectedBrush);
	}

	function paintCellWithMode(row: number, col: number) {
		const cellKey = `${row}:${col}:${drawingMode}`;
		if (cellKey === lastPaintedCellKey) return;
		lastPaintedCellKey = cellKey;
		paintCell(row, col, drawingMode === 'erase');
	}

	function onPointerDown(row: number, col: number, event: PointerEvent) {
		drawingMode = event.button === 2 || event.altKey ? 'erase' : 'paint';
		activePointerId = event.pointerId;
		lastPaintedCellKey = '';
		paintCellWithMode(row, col);
	}

	function stopDrawing(event?: PointerEvent) {
		if (!drawingMode) return;
		if (event && activePointerId !== null && event.pointerId !== activePointerId) return;
		drawingMode = null;
		activePointerId = null;
		lastPaintedCellKey = '';
	}

	function onPointerMove(event: PointerEvent) {
		if (!drawingMode) return;
		if (activePointerId !== null && event.pointerId !== activePointerId) return;

		const element = document.elementFromPoint(event.clientX, event.clientY) as HTMLElement | null;
		const cell = element?.closest('[data-grid-cell="true"]') as HTMLElement | null;
		if (!cell) return;

		const row = Number(cell.dataset.row);
		const col = Number(cell.dataset.col);
		if (Number.isNaN(row) || Number.isNaN(col)) return;
		paintCellWithMode(row, col);
	}

	function onTouchStart(row: number, col: number) {
		drawingMode = 'paint';
		activePointerId = null;
		lastPaintedCellKey = '';
		paintCellWithMode(row, col);
	}

	function onTouchMove(event: TouchEvent) {
		if (!drawingMode) return;
		const touch = event.touches[0];
		if (!touch) return;

		const element = document.elementFromPoint(touch.clientX, touch.clientY) as HTMLElement | null;
		const cell = element?.closest('[data-grid-cell="true"]') as HTMLElement | null;
		if (!cell) return;

		const row = Number(cell.dataset.row);
		const col = Number(cell.dataset.col);
		if (Number.isNaN(row) || Number.isNaN(col)) return;
		paintCellWithMode(row, col);
	}

	const componentPalette = [
		'#fecaca', // red-200
		'#bfdbfe', // blue-200
		'#bbf7d0', // green-200
		'#fde68a', // amber-200
		'#ddd6fe', // violet-200
		'#a5f3fc', // cyan-200
		'#fbcfe8', // pink-200
		'#c7d2fe', // indigo-200
		'#99f6e4', // teal-200
		'#e9d5ff', // purple-200
		'#fed7aa', // orange-200
		'#f5d0fe' // fuchsia-200
	];

	function hashString(value: string): number {
		let hash = 0;
		for (const char of value) {
			hash = (hash << 5) - hash + char.charCodeAt(0);
			hash |= 0;
		}
		return Math.abs(hash);
	}

	function getPaletteColor(index: number, componentName: string): string {
		let accent = '';
		if (index < componentPalette.length) {
			accent = componentPalette[index];
		} else {
			const hash = hashString(componentName);
			const hue = hash % 360;
			accent = `hsl(${hue} 78% 82%)`;
		}
		return accent;
	}

	const componentMixCache = new Map<string, string>();

	function getCombinedComponentColor(componentSources: Array<string>): string {
		if (!componentSources.length) return '';
		const key = componentSources.join('|');
		const cached = componentMixCache.get(key);
		if (cached) return cached;

		const colors = componentSources
			.map((componentName) => componentColorByName.get(componentName) ?? '')
			.filter(Boolean);
		if (!colors.length) return '';
		if (colors.length === 1) return colors[0];

		let mixedColor = colors[0];
		for (let i = 1; i < colors.length; i++) {
			const previousWeight = (i / (i + 1)) * 100;
			const currentWeight = (1 / (i + 1)) * 100;
			mixedColor = `color-mix(in srgb, ${mixedColor} ${previousWeight}%, ${colors[i]} ${currentWeight}%)`;
		}

		componentMixCache.set(key, mixedColor);
		return mixedColor;
	}
</script>

<svelte:window
	on:pointermove={onPointerMove}
	on:pointerup={stopDrawing}
	on:pointercancel={stopDrawing}
	on:touchmove={onTouchMove}
	on:touchend={() => stopDrawing()}
	on:touchcancel={() => stopDrawing()}
/>

<div class="h-full min-h-0 flex flex-col gap-3">
	<div class="shrink-0 flex flex-wrap gap-2 bg-slate-100 p-2">
		{#if availableBrushes.length}
			{#each availableBrushes as symbol (symbol)}
				{@const brushRule = rulesBySymbol[symbol]}
				<button
					type="button"
					title={symbol}
							class={`relative w-14 h-14 border font-mono text-sm ${
								selectedBrush === symbol
									? 'bg-amber-100 border-amber-500 ring-2 ring-amber-500 ring-offset-1 ring-offset-white hover:bg-amber-200'
									: 'bg-white hover:bg-slate-200 border-slate-300'
							}`}
					on:click={() => (selectedBrush = symbol)}
				>
					{#if brushRule && brushRule.shape.kind !== ShapeKind.Void}
						<div class="relative h-full w-full text-pink-300">
							<div class="absolute inset-[1px] rounded-sm bg-pink-100"></div>
							<RuleShapePreview
								rule={brushRule}
								className="relative z-10 h-full w-full p-[1px]"
							/>
							<span
								class="absolute inset-0 z-20 flex select-none items-center justify-center text-3xl font-black leading-none text-black"
								>{symbol}</span
							>
						</div>
					{:else}
						<span class="z-20 select-none text-3xl font-black leading-none text-black"
							>{symbol}</span
						>
					{/if}
					{#if selectedBrush === symbol}
						<span
							class="pointer-events-none absolute right-1 top-1 z-30 h-2.5 w-2.5 rounded-full bg-amber-500 shadow"
						></span>
					{/if}
				</button>
			{/each}
			<p class="text-xs text-slate-500 font-mono">Alt o tasto destro: gomma</p>
			{:else}
				<p class="text-xs text-slate-500 font-mono">
					Nessun simbolo disponibile dalla sintassi. Aggiungi simboli in Struttura glifo.
				</p>
			{/if}
	</div>

	{#if showGrid}
		<div class="h-0 grow min-h-0 overflow-auto bg-slate-100 p-2" style="touch-action: none;">
			<div
				class="grid w-max border border-slate-300 bg-white"
				style={`grid-template-columns: repeat(${gridColumns}, 2rem);`}
			>
				{#each Array.from({ length: gridRows }) as _, row}
					{#each Array.from({ length: gridColumns }) as __, col}
						{@const cellValue = gridMatrix[row][col]}
						{@const overlayValue = overlayMatrix[row][col]}
						{@const componentSources = componentSourceMatrix[row][col]}
						{@const isComponentCell = componentSources.length > 0}
						{@const isOverlayCell = overlayValue !== ' '}
						{@const isOverriddenComponentCell = isComponentCell && isOverlayCell}
						{@const componentColor = isComponentCell ? getCombinedComponentColor(componentSources) : ''}
						<button
							type="button"
							data-grid-cell="true"
								data-row={row}
								data-col={col}
							style={`touch-action: none;${
								isComponentCell && !isOverriddenComponentCell && componentColor
									? ` color: ${componentColor};`
									: ''
							}${
								isOverriddenComponentCell && componentColor
									? ` background-color: color-mix(in srgb, ${componentColor} 26%, white);`
									: ''
							}`}
							class={`relative w-8 h-8 border border-slate-200 font-mono text-sm text-slate-900 hover:bg-blue-100 ${
								isOverriddenComponentCell
									? 'shadow-[inset_0_0_0_1px_rgba(217,119,6,0.6)]'
									: 'bg-white'
							}`}
							on:pointerdown={(event) => onPointerDown(row, col, event)}
							on:touchstart|preventDefault={() => onTouchStart(row, col)}
							on:contextmenu|preventDefault={(event) => paintCell(row, col, true)}
						>
							{#if cellValue === ' '}
								<span class="text-slate-300">·</span>
							{:else if rulesBySymbol[cellValue]}
								{#if rulesBySymbol[cellValue].shape.kind === ShapeKind.Void}
									<span class={isComponentCell ? 'font-mono' : 'text-slate-500 font-mono'}
										>{cellValue}</span
									>
								{:else}
									<div class="relative h-full w-full">
										<div class="absolute inset-[2px] rounded-sm bg-slate-100"></div>
										<RuleShapePreview
											rule={rulesBySymbol[cellValue]}
											className="relative z-10 h-full w-full p-1"
										/>
									</div>
								{/if}
							{:else}
								<span class={isComponentCell ? 'font-mono text-slate-800' : 'font-mono text-red-500'}
									>{cellValue}</span
								>
							{/if}
							{#if isOverriddenComponentCell}
								<span
									class="pointer-events-none absolute right-0.5 top-0.5 h-1.5 w-1.5 rounded-full bg-amber-500"
								></span>
							{/if}
						</button>
					{/each}
				{/each}
			</div>
		</div>
	{/if}
</div>

<script lang="ts">
	import Button from '$lib/ui/button.svelte';
	import type { Rule } from '$lib/types';
	import RuleShapePreview from './ruleShapePreview.svelte';

	export let structure = '';
	export let brushes: Array<string> = [];
	export let rulesBySymbol: Record<string, Rule> = {};
	export let minRows = 12;
	export let minColumns = 12;

	let selectedBrush = '';
	let drawingMode: 'paint' | 'erase' | null = null;
	let activePointerId: number | null = null;
	let lastPaintedCellKey = '';

	$: lines = structure ? structure.split(/\r?\n/) : [];
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

	$: if (!availableBrushes.includes(selectedBrush)) {
		selectedBrush = availableBrushes[0] ?? '';
	}

	function updateCell(row: number, col: number, value: string) {
		const matrix: Array<Array<string>> = gridMatrix.map((matrixRow) => [...matrixRow]);

		matrix[row][col] = value;
		structure = serialize(matrix);
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

	function clearAll() {
		structure = '';
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
	<div class="shrink-0 flex flex-wrap items-center gap-2">
		<p class="text-small font-mono text-slate-900 text-sm">Visual designer</p>
		<Button on:click={clearAll}>Pulisci</Button>
	</div>

	<div class="shrink-0 flex flex-wrap gap-2 bg-slate-100 p-2">
		{#if availableBrushes.length}
			{#each availableBrushes as symbol (symbol)}
				<button
					type="button"
					title={symbol}
					class={`w-8 h-8 border font-mono text-sm ${
						selectedBrush === symbol
							? 'bg-slate-800 text-white border-slate-800'
							: 'bg-white hover:bg-slate-200 border-slate-300'
					}`}
					on:click={() => (selectedBrush = symbol)}
				>
					<span>{symbol}</span>
				</button>
			{/each}
			<p class="text-xs text-slate-500 font-mono">Alt o tasto destro: gomma</p>
		{:else}
			<p class="text-xs text-slate-500 font-mono">
				Nessun simbolo disponibile dalla sintassi. Aggiungi simboli in Struttura glifo.
			</p>
		{/if}
	</div>

		<div class="h-0 grow min-h-0 bg-slate-100 p-2 overflow-auto" style="touch-action: none;">
			<div
				class="grid w-max border border-slate-300 bg-white"
				style={`grid-template-columns: repeat(${gridColumns}, 2rem);`}
			>
				{#each Array.from({ length: gridRows }) as _, row}
					{#each Array.from({ length: gridColumns }) as __, col}
						{@const cellValue = gridMatrix[row][col]}
						<button
							type="button"
							data-grid-cell="true"
							data-row={row}
							data-col={col}
							style="touch-action: none;"
							class="w-8 h-8 border border-slate-200 font-mono text-sm text-slate-900 hover:bg-blue-100"
							on:pointerdown={(event) => onPointerDown(row, col, event)}
							on:touchstart|preventDefault={() => onTouchStart(row, col)}
							on:contextmenu|preventDefault={(event) => paintCell(row, col, true)}
						>
							{#if cellValue === ' '}
								<span class="text-slate-300">·</span>
							{:else if rulesBySymbol[cellValue]}
								<RuleShapePreview rule={rulesBySymbol[cellValue]} className="w-full h-full p-1" />
							{:else}
								<span class="text-red-400">•</span>
							{/if}
						</button>
					{/each}
				{/each}
			</div>
		</div>
	</div>

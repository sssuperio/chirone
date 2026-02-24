<script lang="ts">
	import { onMount } from 'svelte';
	import type opentype from 'opentype.js';
	import { metrics } from '$lib/stores';
	import { cellsToUnits, normalizeFontMetrics } from '$lib/GTL/metrics';

	export let font: opentype.Font;
	export let text: string;
	export let canvasWidth = 520;
	export let canvasHeight = 260;
	export let showLegend = true;
	export let responsive = true;
	export let debug = false;

	const colors = {
		baseline: '#22c55e',
		xHeight: '#3b82f6',
		capHeight: '#8b5cf6',
		ascender: '#f59e0b',
		descender: '#ef4444',
		origin: '#06b6d4',
		leftBound: '#a855f7',
		rightBound: '#a855f7',
		advance: '#ec4899'
	};

	let canvas: HTMLCanvasElement;
	let container: HTMLDivElement;
	let observedWidth = canvasWidth;
	let renderError = '';

	type PreviewMetrics = {
		upm: number;
		ascender: number;
		descender: number;
		capHeight: number;
		xHeight: number;
		advanceWidth: number;
		leftSideBearing: number;
		rightSideBearing: number;
	};

	let previewMetrics: PreviewMetrics | undefined;
	let debugSnapshotUrl = '';

	function finite(value: unknown, fallback: number): number {
		if (typeof value === 'number' && Number.isFinite(value)) return value;
		return fallback;
	}

	function drawLine(
		ctx: CanvasRenderingContext2D,
		y: number,
		color: string,
		label: string,
		fromX: number,
		toX: number
	) {
		ctx.save();
		ctx.strokeStyle = color;
		ctx.lineWidth = 1.5;
		ctx.beginPath();
		ctx.moveTo(fromX, y);
		ctx.lineTo(toX, y);
		ctx.stroke();
		ctx.fillStyle = color;
		ctx.font = '11px monospace';
		ctx.fillText(label, fromX + 4, y - 4);
		ctx.restore();
	}

	function drawVerticalLine(
		ctx: CanvasRenderingContext2D,
		x: number,
		color: string,
		label: string,
		fromY: number,
		toY: number
	) {
		ctx.save();
		ctx.strokeStyle = color;
		ctx.lineWidth = 1.5;
		ctx.beginPath();
		ctx.moveTo(x, fromY);
		ctx.lineTo(x, toY);
		ctx.stroke();
		ctx.fillStyle = color;
		ctx.font = '11px monospace';
		ctx.fillText(label, x + 4, toY + 12);
		ctx.restore();
	}

	function drawGlyphPathStroke(ctx: CanvasRenderingContext2D, commands: Array<any>) {
		ctx.beginPath();
		for (const command of commands) {
			if (command.type === 'M') {
				ctx.moveTo(command.x, command.y);
			} else if (command.type === 'L') {
				ctx.lineTo(command.x, command.y);
			} else if (command.type === 'C') {
				ctx.bezierCurveTo(command.x1, command.y1, command.x2, command.y2, command.x, command.y);
			} else if (command.type === 'Q') {
				ctx.quadraticCurveTo(command.x1, command.y1, command.x, command.y);
			} else if (command.type === 'Z') {
				ctx.closePath();
			}
		}
	}

	function buildPreviewMetrics(glyph: opentype.Glyph): PreviewMetrics {
		const normalizedMetrics = normalizeFontMetrics($metrics);
		const safeUpm = Math.max(1, finite(font?.unitsPerEm, finite(normalizedMetrics.UPM, 1000)));
		const ascender = cellsToUnits(normalizedMetrics, normalizedMetrics.ascender);
		const descender = -cellsToUnits(normalizedMetrics, normalizedMetrics.descender);
		const capHeight = cellsToUnits(normalizedMetrics, normalizedMetrics.capHeight);
		const xHeight = cellsToUnits(normalizedMetrics, normalizedMetrics.xHeight);

		const bbox = glyph.getBoundingBox();
		const advanceWidth = finite(glyph.advanceWidth, safeUpm * 0.6);
		const leftSideBearing = Number.isFinite(bbox.x1) ? bbox.x1 : 0;
		const rightSideBearing = Number.isFinite(bbox.x2) ? advanceWidth - bbox.x2 : 0;

		return {
			upm: safeUpm,
			ascender,
			descender,
			capHeight,
			xHeight,
			advanceWidth,
			leftSideBearing,
			rightSideBearing
		};
	}

	function render() {
		if (!canvas || !font) return;

		try {
			const width = Math.max(320, observedWidth);
			const height = Math.max(160, canvasHeight);
			if (canvas.width !== width) {
				canvas.width = width;
			}
			if (canvas.height !== height) {
				canvas.height = height;
			}
			const ctx = canvas.getContext('2d', { willReadFrequently: true }) as CanvasRenderingContext2D;
			if (!ctx) return;
			const char = text?.[0] || ' ';
			const glyph = font.charToGlyph(char);
			const m = buildPreviewMetrics(glyph);
			previewMetrics = m;

			ctx.clearRect(0, 0, width, height);
			ctx.fillStyle = '#f8fafc';
			ctx.fillRect(0, 0, width, height);

			const padLeft = 68;
			const padRight = 18;
			const padTop = 18;
			const padBottom = 40;
			const previewHeight = height - padTop - padBottom;

			const normalizedMetrics = normalizeFontMetrics($metrics);
			const safeHeightMetric = Math.max(1, finite(normalizedMetrics.height, 5));
			const safeDescenderMetric = Math.min(
				safeHeightMetric - 1,
				Math.max(0, finite(normalizedMetrics.descender, 1))
			);
			const ascenderCells = normalizedMetrics.ascender;
			const fontSize = previewHeight;
			const unit = fontSize / safeHeightMetric;
			const baselineY = padTop + previewHeight - unit * safeDescenderMetric;
			const ascenderY = baselineY - unit * ascenderCells;
			const descenderY = baselineY + unit * safeDescenderMetric;
			const capHeightY = baselineY - unit * normalizedMetrics.capHeight;
			const xHeightY = baselineY - unit * normalizedMetrics.xHeight;

			const scale = fontSize / m.upm;
			const bbox = glyph.getBoundingBox();
			const glyphWidthPx = Math.max(0, (Number.isFinite(bbox.x2) ? bbox.x2 : 0) - (Number.isFinite(bbox.x1) ? bbox.x1 : 0)) * scale;
			const previewWidth = width - padLeft - padRight;
			const centeredLeft = padLeft + Math.max(0, (previewWidth - glyphWidthPx) / 2);
			const originX = centeredLeft - (Number.isFinite(bbox.x1) ? bbox.x1 : 0) * scale;
			const glyphPath = glyph.getPath(originX, baselineY, fontSize, { kerning: false });

			const leftBoundX = originX + (Number.isFinite(bbox.x1) ? bbox.x1 : 0) * scale;
			const rightBoundX = originX + (Number.isFinite(bbox.x2) ? bbox.x2 : 0) * scale;
			const advanceX = originX + m.advanceWidth * scale;
			const verticalFromY = padTop;
			const verticalToY = height - padBottom;

			drawLine(ctx, ascenderY, colors.ascender, 'Ascender', padLeft, width - padRight);
			drawLine(
				ctx,
				capHeightY,
				colors.capHeight,
				`Cap height`,
				padLeft,
				width - padRight
			);
			drawLine(
				ctx,
				xHeightY,
				colors.xHeight,
				`x-height`,
				padLeft,
				width - padRight
			);
			drawLine(ctx, baselineY, colors.baseline, 'Baseline', padLeft, width - padRight);
			drawLine(ctx, descenderY, colors.descender, 'Descender', padLeft, width - padRight);
			drawVerticalLine(ctx, originX, colors.origin, 'Origin', verticalFromY, verticalToY);
			drawVerticalLine(ctx, leftBoundX, colors.leftBound, 'Left bound', verticalFromY, verticalToY);
			drawVerticalLine(ctx, rightBoundX, colors.rightBound, 'Right bound', verticalFromY, verticalToY);
			drawVerticalLine(ctx, advanceX, colors.advance, 'Advance width', verticalFromY, verticalToY);

			// Draw glyph on top of guides to keep it always visible.
			ctx.save();
			ctx.fillStyle = '#0f172a';
			glyphPath.draw(ctx);
			// Stroke fallback makes open contours visible as in a technical preview.
			ctx.strokeStyle = '#020617';
			ctx.lineWidth = 1.6;
			drawGlyphPathStroke(ctx, glyphPath.commands);
			ctx.stroke();
			ctx.restore();

			if (debug) {
				debugSnapshotUrl = canvas.toDataURL('image/png');
			} else {
				debugSnapshotUrl = '';
			}

			renderError = '';
		} catch (error) {
			const width = Math.max(320, observedWidth);
			const height = Math.max(160, canvasHeight);
			const ctx = canvas.getContext('2d', { willReadFrequently: true }) as CanvasRenderingContext2D | null;
				if (ctx) {
					ctx.clearRect(0, 0, width, height);
					ctx.fillStyle = '#f8fafc';
					ctx.fillRect(0, 0, width, height);
				}
				debugSnapshotUrl = '';
				renderError = error instanceof Error ? error.message : String(error);
			console.error('fontDisplayMetrics render error', error);
		}
	}

	onMount(() => {
		if (!responsive || !container) return;
		if (typeof ResizeObserver === 'undefined') return;
		const observer = new ResizeObserver((entries) => {
			const entry = entries[0];
			if (!entry) return;
			observedWidth = Math.max(320, Math.floor(entry.contentRect.width));
		});
		observer.observe(container);
		return () => {
			observer.disconnect();
		};
	});

	$: observedWidth = responsive ? Math.max(320, observedWidth) : canvasWidth;
	$: {
		const canRender = Boolean(canvas && font && text !== undefined && observedWidth > 0 && canvasHeight > 0);
		if (canRender) {
			render();
		}
	}
</script>

<div bind:this={container} class="space-y-3">
	<div class="relative">
		<canvas
			bind:this={canvas}
			class="block bg-slate-50 border border-slate-300 w-full"
			class:opacity-0={debug && Boolean(debugSnapshotUrl)}
			style={`height: ${canvasHeight}px; max-height: ${canvasHeight}px;`}
			width={observedWidth}
			height={canvasHeight}
		/>
		{#if debug && debugSnapshotUrl}
			<img
				class="absolute inset-0 w-full border border-slate-300 bg-white"
				style={`height: ${canvasHeight}px; max-height: ${canvasHeight}px;`}
				src={debugSnapshotUrl}
				alt="Debug preview snapshot"
			/>
		{/if}
	</div>

	{#if previewMetrics}
		<div class="grid grid-cols-2 lg:grid-cols-4 gap-x-4 gap-y-1 text-xs font-mono text-slate-700">
			<p>UPM: {previewMetrics.upm}</p>
			<p>Ascender: {previewMetrics.ascender}</p>
			<p>Descender: {previewMetrics.descender}</p>
			<p>Baseline: 0</p>
			<p>Cap height: {previewMetrics.capHeight}</p>
			<p>x-height: {previewMetrics.xHeight}</p>
			<p>Advance: {previewMetrics.advanceWidth}</p>
			<p>LSB/RSB: {previewMetrics.leftSideBearing} / {previewMetrics.rightSideBearing}</p>
		</div>
	{/if}

	{#if renderError}
		<p class="text-xs font-mono text-red-600">Errore preview: {renderError}</p>
	{/if}

	{#if showLegend}
		<div class="grid grid-cols-1 md:grid-cols-2 gap-1 text-xs font-mono text-slate-600">
			<p><span style={`color: ${colors.baseline}`}>●</span> verde: baseline (linea di scrittura)</p>
			<p><span style={`color: ${colors.xHeight}`}>●</span> blu: x-height (altezza minuscole)</p>
			<p><span style={`color: ${colors.capHeight}`}>●</span> viola: cap height (altezza maiuscole)</p>
			<p><span style={`color: ${colors.ascender}`}>●</span> ambra: ascender (massima salita)</p>
			<p><span style={`color: ${colors.descender}`}>●</span> rosso: descender (massima discesa)</p>
			<p><span style={`color: ${colors.origin}`}>●</span> ciano: origine glyph (x=0)</p>
			<p><span style={`color: ${colors.leftBound}`}>●</span> fucsia: bordo reale sx del disegno</p>
			<p><span style={`color: ${colors.rightBound}`}>●</span> fucsia: bordo reale dx del disegno</p>
			<p><span style={`color: ${colors.advance}`}>●</span> rosa: advance width (fine box metrica)</p>
		</div>
	{/if}
</div>

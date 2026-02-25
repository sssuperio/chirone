<script lang="ts">
	import type opentype from 'opentype.js';
	import { metrics } from '$lib/stores';
	import { normalizeFontMetrics } from '$lib/GTL/metrics';

	export let font: opentype.Font;
	export let text: string;
	export let glyphSequence: Array<string> = [];
	export let useGlyphSequence = false;
	export let canvasWidth = 1200;
	export let canvasHeight = 640;
	export let fontSize = 120;
	export let padding = 32;
	export let lineGapRatio = 0.3;
	export let className = '';
	let canvas: HTMLCanvasElement | undefined;
	let host: HTMLDivElement | undefined;
	let observedWidth = canvasWidth;
	let observedHeight = canvasHeight;

	function findGlyphByName(font: opentype.Font, name: string): opentype.Glyph | undefined {
		const total = Number((font as any).glyphs?.length ?? 0);
		for (let i = 0; i < total; i++) {
			const glyph = font.glyphs.get(i);
			if (glyph?.name === name) return glyph;
		}
		return undefined;
	}

	function drawGlyphSequence(
		ctx: CanvasRenderingContext2D,
		font: opentype.Font,
		sequence: Array<string>,
		fontSize: number,
		baselineY: number,
		startX: number
	) {
		const fontScale = fontSize / font.unitsPerEm;
		let cursorX = startX;

		for (const glyphName of sequence) {
			const glyph = findGlyphByName(font, glyphName);
			if (!glyph) continue;

			glyph.getPath(cursorX, baselineY, fontSize, { kerning: false }, font).draw(ctx);
			if (glyph.advanceWidth) {
				cursorX += glyph.advanceWidth * fontScale;
			}
		}
	}

	function measureGlyphSequenceWidth(
		font: opentype.Font,
		sequence: Array<string>,
		fontSize: number
	): number {
		const scale = fontSize / font.unitsPerEm;
		let width = 0;

		for (const glyphName of sequence) {
			const glyph = findGlyphByName(font, glyphName);
			if (!glyph || !glyph.advanceWidth) continue;
			width += glyph.advanceWidth * scale;
		}

		return width;
	}

	function splitWordToFit(font: opentype.Font, word: string, maxWidth: number, size: number): Array<string> {
		const chunks: Array<string> = [];
		let current = '';

		for (const char of Array.from(word)) {
			const candidate = `${current}${char}`;
			if (
				current &&
				font.getAdvanceWidth(candidate, size, { kerning: false }) > maxWidth
			) {
				chunks.push(current);
				current = char;
			} else {
				current = candidate;
			}
		}

		if (current) chunks.push(current);
		return chunks.length ? chunks : [word];
	}

	function wrapText(font: opentype.Font, content: string, maxWidth: number, size: number): Array<string> {
		const output: Array<string> = [];
		for (const paragraph of content.split(/\r?\n/)) {
			const words = paragraph.split(/\s+/).filter(Boolean);
			if (!words.length) {
				output.push('');
				continue;
			}

			const expandedWords: Array<string> = [];
			for (const word of words) {
				const wordWidth = font.getAdvanceWidth(word, size, { kerning: false });
				if (wordWidth <= maxWidth) {
					expandedWords.push(word);
				} else {
					expandedWords.push(...splitWordToFit(font, word, maxWidth, size));
				}
			}

			let line = expandedWords[0];
			for (let i = 1; i < expandedWords.length; i++) {
				const candidate = `${line} ${expandedWords[i]}`;
				const candidateWidth = font.getAdvanceWidth(candidate, size, { kerning: false });
				if (candidateWidth <= maxWidth) {
					line = candidate;
				} else {
					output.push(line);
					line = expandedWords[i];
				}
			}
			output.push(line);
		}

		return output;
	}

	function fitText(
		font: opentype.Font,
		content: string,
		targetSize: number,
		maxWidth: number,
		maxHeight: number
	): { size: number; lines: Array<string> } {
		let size = Math.max(1, targetSize <= 0 ? 800 : targetSize);
		let lines = wrapText(font, content, maxWidth, size);
		for (let i = 0; i < 120; i++) {
			const lineHeight = size * (1 + Math.max(0, lineGapRatio));
			const neededHeight = Math.max(lineHeight, lines.length * lineHeight);
			if (neededHeight <= maxHeight) break;

			if (size <= 1) break;
			size = Math.max(1, size * 0.94);
			lines = wrapText(font, content, maxWidth, size);
		}

		const lineHeight = Math.max(1, size * (1 + Math.max(0, lineGapRatio)));
		const maxLines = Math.max(1, Math.floor(maxHeight / lineHeight));
		return { size, lines: lines.slice(0, maxLines) };
	}

	function fitGlyphSequenceSize(
		font: opentype.Font,
		sequence: Array<string>,
		targetSize: number,
		maxWidth: number,
		maxHeight: number
	): number {
		let size = Math.max(1, targetSize <= 0 ? 800 : targetSize);
		size = Math.min(size, maxHeight / (1 + Math.max(0, lineGapRatio)));

		const width = measureGlyphSequenceWidth(font, sequence, size);
		if (width > 0 && width > maxWidth) {
			size = Math.max(1, (size * maxWidth) / width);
		}

		return size;
	}

	function observeSize(node: HTMLDivElement) {
		if (typeof ResizeObserver === 'undefined') {
			observedWidth = Math.max(1, Math.floor(node.clientWidth || canvasWidth));
			observedHeight = Math.max(1, Math.floor(node.clientHeight || canvasHeight));
			return {
				destroy() {}
			};
		}

		const resizeObserver = new ResizeObserver((entries) => {
			const entry = entries[0];
			if (!entry) return;

			const nextWidth = Math.max(1, Math.floor(entry.contentRect.width));
			const nextHeight = Math.max(1, Math.floor(entry.contentRect.height));
			observedWidth = nextWidth;
			observedHeight = nextHeight;
		});

		resizeObserver.observe(node);
		return {
			destroy() {
				resizeObserver.disconnect();
			}
		};
	}

	function renderFont(el: HTMLCanvasElement) {
		const ctx = el.getContext('2d', {
			willReadFrequently: true
		}) as CanvasRenderingContext2D;
		if (!ctx || !font) return;
		const normalizedMetrics = normalizeFontMetrics($metrics);
		const safePadding = Math.max(0, padding);
		const activeWidth = Math.max(1, observedWidth || canvasWidth);
		const activeHeight = Math.max(1, observedHeight || canvasHeight);
		const maxWidth = Math.max(1, activeWidth - safePadding * 2);
		const maxHeight = Math.max(1, activeHeight - safePadding * 2);
		ctx.clearRect(0, 0, activeWidth, activeHeight);
		ctx.save();
		ctx.beginPath();
		ctx.rect(safePadding, safePadding, maxWidth, maxHeight);
		ctx.clip();
		const x = safePadding;

		if (useGlyphSequence && glyphSequence.length) {
			const fittedSize = fitGlyphSequenceSize(font, glyphSequence, fontSize, maxWidth, maxHeight);
			const unit = fittedSize / normalizedMetrics.height;
			const baselineY = safePadding + unit * normalizedMetrics.ascender;
			drawGlyphSequence(ctx, font, glyphSequence, fittedSize, baselineY, x);
			ctx.restore();
			return;
		}

		const fitted = fitText(font, text, fontSize, maxWidth, maxHeight);
		const unit = fitted.size / normalizedMetrics.height;
		const firstBaselineY = safePadding + unit * normalizedMetrics.ascender;
		const lineHeight = fitted.size * (1 + Math.max(0, lineGapRatio));

		for (let i = 0; i < fitted.lines.length; i++) {
			font.draw(ctx, fitted.lines[i], x, firstBaselineY + i * lineHeight, fitted.size, {
				kerning: false
			});
		}
		ctx.restore();
	}

	$: {
		font;
		text;
		glyphSequence;
		useGlyphSequence;
		canvasWidth;
		canvasHeight;
		fontSize;
		padding;
		lineGapRatio;
		observedWidth;
		observedHeight;
		$metrics;
		if (canvas) {
			renderFont(canvas);
		}
	}
</script>

<div
	bind:this={host}
	use:observeSize
	class={`w-full h-full min-h-[24rem] rounded border border-slate-200 bg-white overflow-hidden ${className}`}
>
	<canvas
		class="block h-full w-full"
		bind:this={canvas}
		width={Math.max(1, observedWidth || canvasWidth)}
		height={Math.max(1, observedHeight || canvasHeight)}
	/>
</div>

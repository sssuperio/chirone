import { persisted } from 'svelte-local-storage-store';
import { writable } from 'svelte/store';
import type { Syntax, GlyphInput } from '$lib/types';
import type { FontMetrics } from '$lib/GTL/metrics';
import { areMetricsEqual, defaultFontMetrics, normalizeFontMetrics } from '$lib/GTL/metrics';

export const syntaxes = persisted<Array<Syntax>>('syntaxes', []);
export const glyphs = persisted<Array<GlyphInput>>('glyphs', []);

export const defaultMetrics: FontMetrics = defaultFontMetrics;

export const metrics = persisted<FontMetrics>('metrics', defaultMetrics);

metrics.subscribe((current) => {
	const normalized = normalizeFontMetrics(current as any);
	if (!areMetricsEqual(current as any, normalized)) {
		metrics.set(normalized);
	}
});

export const selectedGlyph = writable('');
export const previewText = writable('Hello World!');
export const syntaxPreviewText = writable('hey');

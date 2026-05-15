import { persisted } from 'svelte-local-storage-store';
import { get, writable } from 'svelte/store';
import type {
	Syntax,
	GlyphInput,
	MetricsPreset,
	MetadataPreset,
	FontDefinition,
	ProjectInfo
} from '$lib/types';
import type { FontMetrics } from '$lib/GTL/metrics';
import { areMetricsEqual, defaultFontMetrics, normalizeFontMetrics } from '$lib/GTL/metrics';
import type { FontMetadata } from '$lib/GTL/metadata';
import {
	areFontMetadataEqual,
	defaultFontMetadata,
	normalizeFontMetadata
} from '$lib/GTL/metadata';
import { nanoid } from 'nanoid';

export const syntaxes = persisted<Array<Syntax>>('syntaxes', []);
export const glyphs = persisted<Array<GlyphInput>>('glyphs', []);

export const defaultMetrics: FontMetrics = defaultFontMetrics;
export const defaultMetadata: FontMetadata = defaultFontMetadata;

export const metrics = persisted<FontMetrics>('metrics', defaultMetrics);
export const fontMetadata = persisted<FontMetadata>('fontMetadata', defaultMetadata);

metrics.subscribe((current) => {
	const normalized = normalizeFontMetrics(current as any);
	if (!areMetricsEqual(current as any, normalized)) {
		metrics.set(normalized);
	}
});

fontMetadata.subscribe((current) => {
	const normalized = normalizeFontMetadata(current as any);
	if (!areFontMetadataEqual(current as any, normalized)) {
		fontMetadata.set(normalized);
	}
});

export const selectedGlyph = writable('');
export const activeFontId = persisted<string>('activeFontId', '');
export const previewText = writable('Hello World!');
export const syntaxPreviewText = writable('hey');

function defaultProjectInfo(): ProjectInfo {
	return {
		id: nanoid(),
		name: 'My Project',
		createdAt: new Date().toISOString(),
		updatedAt: new Date().toISOString()
	};
}

export const projectInfo = persisted<ProjectInfo>('projectInfo', defaultProjectInfo());
export const metricsPresets = persisted<Array<MetricsPreset>>('metricsPresets', []);
export const metadataPresets = persisted<Array<MetadataPreset>>('metadataPresets', []);
export const fontDefinitions = persisted<Array<FontDefinition>>('fontDefinitions', []);

export function resolvePresetName(
	presets: Array<{ id: string; name: string }>,
	id: string
): string {
	const preset = presets.find((p) => p.id === id);
	return preset?.name ?? id;
}

export function resolveSyntaxName(
	syntaxes: Array<{ id: string; name: string }>,
	id: string
): string {
	const s = syntaxes.find((s) => s.id === id);
	return s?.name ?? id;
}

const FONT_GLYPHS_PREFIX = 'chirone-glyphs-';

function fontGlyphsKey(fontId: string): string {
	return FONT_GLYPHS_PREFIX + fontId;
}

export function saveGlyphsForFont(fontId: string, glyphsList: Array<GlyphInput>): void {
	if (typeof window === 'undefined' || !fontId) return;
	const list = glyphsList ?? [];
	try {
		window.localStorage.setItem(fontGlyphsKey(fontId), JSON.stringify(list));
	} catch {
		/* ignore */
	}
}

export function loadGlyphsForFont(fontId: string): Array<GlyphInput> {
	if (typeof window === 'undefined' || !fontId) return [];
	try {
		const raw = window.localStorage.getItem(fontGlyphsKey(fontId));
		if (!raw) return [];
		const parsed = JSON.parse(raw);
		if (Array.isArray(parsed)) return parsed as Array<GlyphInput>;
	} catch {
		/* ignore */
	}
	return [];
}

export function switchToFont(newFontId: string, oldFontId: string): void {
	if (oldFontId && oldFontId !== newFontId) {
		saveGlyphsForFont(oldFontId, get(glyphs));
	}
	const loaded = loadGlyphsForFont(newFontId);
	glyphs.set(loaded);
	activeFontId.set(newFontId);
}

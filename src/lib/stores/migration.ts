import { nanoid } from 'nanoid';
import { metricsPresets, metadataPresets, fontDefinitions, glyphs, saveGlyphsForFont } from '$lib/stores';
import { get } from 'svelte/store';
import type { FontMetrics } from '$lib/GTL/metrics';
import { defaultFontMetrics } from '$lib/GTL/metrics';
import type { FontMetadata } from '$lib/GTL/metadata';
import { defaultFontMetadata } from '$lib/GTL/metadata';
import type { MetricsPreset, MetadataPreset, FontDefinition } from '$lib/types';

const MIGRATION_FLAG = 'migration-v1-done';

function metricsToPreset(metrics: FontMetrics, id: string, name: string): MetricsPreset {
	return {
		id,
		name,
		UPM: metrics.UPM,
		height: metrics.height,
		baseline: metrics.baseline,
		descender: metrics.descender,
		ascender: metrics.ascender,
		capHeight: metrics.capHeight,
		xHeight: metrics.xHeight
	};
}

function metadataToPreset(metadata: FontMetadata, id: string, name: string): MetadataPreset {
	return {
		id,
		name,
		familyName: metadata.familyName,
		version: metadata.version,
		createdDate: metadata.createdDate,
		designer: metadata.designer,
		manufacturer: metadata.manufacturer,
		designerURL: metadata.designerURL,
		manufacturerURL: metadata.manufacturerURL,
		license: metadata.license,
		vendorID: metadata.vendorID,
		glyphOrder: metadata.glyphOrder
	};
}

export function migrateFromLegacyStores(): boolean {
	if (typeof window === 'undefined') return false;

	try {
		if (window.localStorage.getItem(MIGRATION_FLAG)) return false;

		const storedMetrics = window.localStorage.getItem('metrics');
		const storedMetadata = window.localStorage.getItem('fontMetadata');
		const storedSyntaxes = window.localStorage.getItem('syntaxes');

		if (!storedMetrics && !storedMetadata) {
			window.localStorage.setItem(MIGRATION_FLAG, '1');
			return false;
		}

		const parsedMetrics: FontMetrics | null = storedMetrics
			? (() => {
					try {
						return JSON.parse(storedMetrics);
					} catch {
						return null;
					}
				})()
			: null;

		const parsedMetadata: FontMetadata | null = storedMetadata
			? (() => {
					try {
						return JSON.parse(storedMetadata);
					} catch {
						return null;
					}
				})()
			: null;

		const parsedSyntaxes: Array<{ id: string }> = storedSyntaxes
			? (() => {
					try {
						return JSON.parse(storedSyntaxes);
					} catch {
						return [];
					}
				})()
			: [];

		const firstSyntaxId = parsedSyntaxes.length > 0 ? parsedSyntaxes[0].id : '';

		const preset = metricsToPreset(parsedMetrics ?? defaultFontMetrics, 'default', 'Default');
		const metadata = metadataToPreset(parsedMetadata ?? defaultFontMetadata, 'default', 'Default');
		const fontFamilyName = parsedMetadata?.familyName?.trim() || 'GTL';
		const fontStyleName = 'Regular';

		const fontDef: FontDefinition = {
			id: nanoid(),
			name: fontStyleName,
			syntaxId: firstSyntaxId,
			metricsId: preset.id,
			metadataId: metadata.id,
			outputName: `${fontFamilyName}-${fontStyleName}.otf`,
			enabled: true
		};

		const currentMetricsPresets = get(metricsPresets);
		const currentMetadataPresets = get(metadataPresets);
		const currentFontDefs = get(fontDefinitions);

		if (currentMetricsPresets.length === 0) {
			metricsPresets.set([preset]);
		}
		if (currentMetadataPresets.length === 0) {
			metadataPresets.set([metadata]);
		}
		if (currentFontDefs.length === 0) {
			fontDefinitions.set([fontDef]);
		}

		// Save existing glyphs to the default font's per-font storage
		saveGlyphsForFont(fontDef.id, get(glyphs));

		window.localStorage.setItem(MIGRATION_FLAG, '1');
		return true;
	} catch {
		return false;
	}
}

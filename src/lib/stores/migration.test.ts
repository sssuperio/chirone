// @vitest-environment jsdom
import { describe, it, expect, beforeEach } from 'vitest';
import { nanoid } from 'nanoid';
import type { FontMetrics } from '$lib/GTL/metrics';
import { defaultFontMetrics } from '$lib/GTL/metrics';
import type { FontMetadata } from '$lib/GTL/metadata';
import { defaultFontMetadata } from '$lib/GTL/metadata';
import type { MetricsPreset, MetadataPreset, FontDefinition } from '$lib/types';

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
	return { ...metadata, id, name };
}

function createDefaultFont(syntaxId: string, familyName: string): FontDefinition {
	return {
		id: nanoid(),
		name: 'Regular',
		syntaxId,
		metricsId: 'default',
		metadataId: 'default',
		outputName: `${familyName || 'GTL'}-Regular.otf`,
		enabled: true
	};
}

describe('migration data transformation', () => {
	it('converts FontMetrics to MetricsPreset', () => {
		const metrics: FontMetrics = {
			UPM: 500,
			height: 4,
			baseline: 1,
			descender: 1,
			ascender: 3,
			capHeight: 2,
			xHeight: 1
		};
		const preset = metricsToPreset(metrics, 'my-id', 'My Preset');
		expect(preset.id).toBe('my-id');
		expect(preset.name).toBe('My Preset');
		expect(preset.UPM).toBe(500);
		expect(preset.height).toBe(4);
	});

	it('converts FontMetadata to MetadataPreset', () => {
		const metadata = defaultFontMetadata;
		const preset = metadataToPreset(metadata, 'meta-1', 'Meta 1');
		expect(preset.id).toBe('meta-1');
		expect(preset.name).toBe('Meta 1');
		expect(preset.familyName).toBe(defaultFontMetadata.familyName);
	});

	it('creates valid FontDefinition defaults', () => {
		const font = createDefaultFont('syntax-abc', 'TestFamily');
		expect(font.name).toBe('Regular');
		expect(font.syntaxId).toBe('syntax-abc');
		expect(font.metricsId).toBe('default');
		expect(font.metadataId).toBe('default');
		expect(font.outputName).toBe('TestFamily-Regular.otf');
		expect(font.enabled).toBe(true);
	});

	it('uses fallback family name when empty', () => {
		const font = createDefaultFont('s1', '');
		expect(font.outputName).toBe('GTL-Regular.otf');
	});

	it('handles empty syntaxId', () => {
		const font = createDefaultFont('', 'Test');
		expect(font.syntaxId).toBe('');
	});
});

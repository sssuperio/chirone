// @vitest-environment jsdom
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';

// SvelteKit virtual module — mocked for unit tests
vi.mock('$env/dynamic/public', () => ({
	env: {}
}));

import { get } from 'svelte/store';
import {
	glyphs,
	syntaxes,
	metrics,
	fontMetadata,
	fontDefinitions,
	metricsPresets,
	metadataPresets,
	activeFontId,
	resetProjectState,
	clearStalePerFontGlyphs,
	syncMetadataPreset,
	defaultMetrics,
	defaultMetadata
} from '$lib/stores';
import {
	syncProjectNow,
	collabStatus,
	collabConfig,
	setCollabProject
} from '$lib/collab/client';

function resetAllStores() {
	resetProjectState();
}

describe('resetProjectState', () => {
	beforeEach(() => {
		resetAllStores();
	});

	it('clears all project stores to defaults', () => {
		// Put some data in stores
		glyphs.set([{ id: 'g1', name: 'A', structure: 'A' }]);
		syntaxes.set([{ id: 's1', name: 'Test', rules: [], grid: { rows: 5, columns: 5 } }]);
		fontDefinitions.set([{ id: 'f1', name: 'Bold', syntaxId: 's1', metricsId: 'm1', metadataId: 'd1', outputName: 'Test.otf', enabled: true }]);
		activeFontId.set('f1');

		resetProjectState();

		expect(get(glyphs)).toEqual([]);
		expect(get(syntaxes)).toEqual([]);
		expect(get(fontDefinitions)).toEqual([]);
		expect(get(activeFontId)).toBe('');
		expect(get(metricsPresets)).toEqual([]);
		expect(get(metadataPresets)).toEqual([]);
	});

	it('resets metadata to defaults', () => {
		fontMetadata.set({ ...defaultMetadata, familyName: 'Custom', designer: 'Someone' });
		resetProjectState();
		expect(get(fontMetadata).familyName).toBe(defaultMetadata.familyName);
		expect(get(fontMetadata).designer).toBe(defaultMetadata.designer);
	});

	it('resets metrics to defaults', () => {
		metrics.set({ ...defaultMetrics, height: 999 });
		resetProjectState();
		expect(get(metrics).height).toBe(defaultMetrics.height);
	});
});

describe('clearStalePerFontGlyphs', () => {
	const ls = typeof localStorage !== 'undefined' ? localStorage : undefined;

	beforeEach(() => {
		ls?.clear();
	});

	it('removes chirone-glyphs-* keys', () => {
		if (!ls) return; // skip if no localStorage
		ls.setItem('chirone-glyphs-font1', '[]');
		ls.setItem('chirone-glyphs-font2', '[]');
		ls.setItem('chirone-sync-project', 'myproject');
		ls.setItem('other-key', 'value');

		clearStalePerFontGlyphs();

		expect(ls.getItem('chirone-glyphs-font1')).toBeNull();
		expect(ls.getItem('chirone-glyphs-font2')).toBeNull();
		expect(ls.getItem('chirone-sync-project')).toBe('myproject');
		expect(ls.getItem('other-key')).toBe('value');
	});

	it('does nothing when no chirone-glyphs keys exist', () => {
		if (!ls) return;
		ls.setItem('other', 'x');
		clearStalePerFontGlyphs();
		expect(ls.getItem('other')).toBe('x');
	});
});

describe('syncMetadataPreset', () => {
	it('creates a default preset when presets list is empty', () => {
		metadataPresets.set([]);
		const meta = { ...defaultMetadata, familyName: 'New Font', designer: 'Puria' };
		fontMetadata.set(meta);

		syncMetadataPreset(get(fontMetadata), get(metadataPresets));

		const presets = get(metadataPresets);
		expect(presets).toHaveLength(1);
		expect(presets[0].familyName).toBe('New Font');
		expect(presets[0].id).toBe('default');
		expect(presets[0].name).toBe('Default');
	});

	it('updates existing preset when metadata changes', () => {
		metadataPresets.set([{ ...defaultMetadata, id: 'default', name: 'Default' }]);
		const meta = { ...defaultMetadata, familyName: 'Changed', version: 'v2' };
		fontMetadata.set(meta);

		syncMetadataPreset(get(fontMetadata), get(metadataPresets));

		const presets = get(metadataPresets);
		expect(presets).toHaveLength(1);
		expect(presets[0].familyName).toBe('Changed');
		expect(presets[0].version).toBe('v2');
	});

	it('does not write when nothing changed', () => {
		const preset = { ...defaultMetadata, id: 'p1', name: 'My Preset' };
		metadataPresets.set([preset]);
		fontMetadata.set({ ...preset });

		const before = get(metadataPresets);
		syncMetadataPreset(get(fontMetadata), get(metadataPresets));
		const after = get(metadataPresets);

		// Same reference means no write occurred
		expect(after).toBe(before);
	});
});

describe('syncProjectNow', () => {
	it('resolves immediately when no collab runtime is active', async () => {
		// Without calling initCollabSync, there's no active runtime
		await expect(syncProjectNow()).resolves.toBeUndefined();
	});

	it('is callable multiple times in succession', async () => {
		await syncProjectNow();
		await syncProjectNow();
		await syncProjectNow();
		// Should not throw
	});
});

describe('collabStatus defaults', () => {
	it('starts in correct initial state', () => {
		const status = get(collabStatus);
		expect(status).toHaveProperty('state');
		expect(status).toHaveProperty('project');
		expect(status).toHaveProperty('version');
	});

	it('collabConfig has expected shape', () => {
		const config = get(collabConfig);
		expect(config).toHaveProperty('enabled');
		expect(config).toHaveProperty('server');
		expect(config).toHaveProperty('base');
	});
});

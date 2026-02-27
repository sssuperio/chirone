import { describe, expect, it } from 'vitest';
import { getGlyphNamesForSet, inferGlyphSetIDByName, normalizeGlyphSetID } from './glyphSets';

describe('inferGlyphSetIDByName', () => {
	it('classifies arrows in the simple symbols set', () => {
		expect(inferGlyphSetIDByName('arrowleft')).toBe('symbols-simple');
		expect(inferGlyphSetIDByName('arrowright')).toBe('symbols-simple');
	});

	it('classifies advanced symbols in the full symbols set', () => {
		expect(inferGlyphSetIDByName('integral')).toBe('symbols');
		expect(inferGlyphSetIDByName('summation')).toBe('symbols');
	});
});

describe('getGlyphNamesForSet', () => {
	it('returns a compact symbols subset for symbols-simple', () => {
		const names = getGlyphNamesForSet('symbols-simple');

		expect(names).toContain('arrowleft');
		expect(names).toContain('blacksquare');
		expect(names).not.toContain('integral');
	});

	it('returns the full symbols range for symbols', () => {
		const names = getGlyphNamesForSet('symbols');

		expect(names).toContain('arrowleft');
		expect(names).toContain('integral');
		expect(names).toContain('therefore');
	});
});

describe('normalizeGlyphSetID', () => {
	it('recognizes the new symbols set IDs', () => {
		expect(normalizeGlyphSetID('symbols-simple')).toBe('symbols-simple');
		expect(normalizeGlyphSetID('symbols')).toBe('symbols');
	});
});

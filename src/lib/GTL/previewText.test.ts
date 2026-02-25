import { describe, expect, it } from 'vitest';
import type { GlyphInput } from '$lib/types';
import { parsePreviewText } from './previewText';

function glyph(name: string, structure = ''): GlyphInput {
	return {
		id: name,
		name,
		structure
	};
}

describe('parsePreviewText', () => {
	it('parses slash tokens like /a.ss01 inside normal text', () => {
		const available = [
			glyph('C'),
			glyph('i'),
			glyph('a'),
			glyph('o'),
			glyph('space'),
			glyph('a.ss01')
		];
		const parsed = parsePreviewText('Ciao Cia/a.ss01o', available);

		expect(parsed.hasNamedTokens).toBe(true);
		expect(parsed.glyphSequence).toEqual([
			'C',
			'i',
			'a',
			'o',
			'space',
			'C',
			'i',
			'a',
			'a.ss01',
			'o'
		]);
		expect(parsed.validText).toBe('Ciao Ciaao');
		expect(parsed.foundGlyphs.map((item) => item.name)).toEqual([
			'C',
			'i',
			'a',
			'o',
			'space',
			'a.ss01'
		]);
	});

	it('parses adjacent slash tokens like /etom.component/a', () => {
		const available = [glyph('a'), glyph('etom.component')];
		const parsed = parsePreviewText('/etom.component/a', available);

		expect(parsed.glyphSequence).toEqual(['etom.component', 'a']);
		expect(parsed.validText).toBe('a');
		expect(parsed.foundGlyphs.map((item) => item.name)).toEqual(['etom.component', 'a']);
	});

	it('includes recursive component dependencies in found glyphs', () => {
		const componentStructure = `
x
		`.trim();
		const aStructure = `
---
components:
  - name: etom.component
    symbol: x
    x: 1
    y: 1
    rotation: 0
---
x
		`.trim();
		const available = [glyph('a', aStructure), glyph('etom.component', componentStructure)];
		const parsed = parsePreviewText('a', available);

		expect(parsed.hasNamedTokens).toBe(false);
		expect(parsed.glyphSequence).toEqual(['a']);
		expect(parsed.foundGlyphs.map((item) => item.name)).toEqual(['a', 'etom.component']);
	});
});

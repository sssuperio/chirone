import { describe, expect, it } from 'vitest';
import {
	Orientation,
	ShapeKind,
	booleanPropFixed,
	numberPropFixed,
	orientationPropFixed,
	type Rule
} from '$lib/types';
import {
	getCrossedQuarterTurns,
	mapDirectionalSymbolsForRotation,
	reflectGlyphStructureBody,
	rotateGlyphStructureBody,
	replaceMappedSymbols
} from './structureTransforms';

function quarterRule(symbol: string, orientation: Orientation, squaring = 0.56): Rule {
	return {
		symbol,
		shape: {
			kind: ShapeKind.Quarter,
			props: {
				scale_x: numberPropFixed(1),
				scale_y: numberPropFixed(1),
				rotation: numberPropFixed(0),
				squaring: numberPropFixed(squaring),
				negative: booleanPropFixed(false),
				orientation: orientationPropFixed(orientation)
			}
		}
	};
}

describe('getCrossedQuarterTurns', () => {
	it('counts quarter-turn boundaries crossed by small steps', () => {
		expect(getCrossedQuarterTurns(0, 15)).toBe(0);
		expect(getCrossedQuarterTurns(75, 15)).toBe(1);
		expect(getCrossedQuarterTurns(165, 15)).toBe(1);
		expect(getCrossedQuarterTurns(345, 15)).toBe(1);
	});
});

describe('reflectGlyphStructureBody', () => {
	it('mirrors left/right and swaps quarter NE <-> NW symbols', () => {
		const rulesBySymbol: Record<string, Rule> = {
			a: quarterRule('a', Orientation.NE),
			b: quarterRule('b', Orientation.NW)
		};

		expect(reflectGlyphStructureBody('a', 'vertical', rulesBySymbol)).toBe('b');
	});

	it('flips top/bottom and swaps quarter NE <-> SE symbols', () => {
		const rulesBySymbol: Record<string, Rule> = {
			n: quarterRule('n', Orientation.NE),
			s: quarterRule('s', Orientation.SE)
		};

		expect(reflectGlyphStructureBody('n\n.', 'horizontal', rulesBySymbol)).toBe('.\ns');
	});

	it('keeps symbols when the reflected orientation partner has different shape props', () => {
		const rulesBySymbol: Record<string, Rule> = {
			a: quarterRule('a', Orientation.NE, 0.56),
			b: quarterRule('b', Orientation.NW, 0.9)
		};

		expect(reflectGlyphStructureBody('a', 'vertical', rulesBySymbol)).toBe('a');
	});
});

describe('mapDirectionalSymbolsForRotation', () => {
	it('maps quarter symbols clockwise for 90° rotation', () => {
		const rulesBySymbol: Record<string, Rule> = {
			a: quarterRule('a', Orientation.NE),
			b: quarterRule('b', Orientation.SE),
			c: quarterRule('c', Orientation.SW),
			d: quarterRule('d', Orientation.NW)
		};

		const mapping = mapDirectionalSymbolsForRotation(rulesBySymbol, 90);
		expect(replaceMappedSymbols('abcd', mapping)).toBe('bcda');
	});

	it('does not map when rotation is not cardinal', () => {
		const rulesBySymbol: Record<string, Rule> = {
			a: quarterRule('a', Orientation.NE),
			b: quarterRule('b', Orientation.SE),
			c: quarterRule('c', Orientation.SW),
			d: quarterRule('d', Orientation.NW)
		};

		const mapping = mapDirectionalSymbolsForRotation(rulesBySymbol, 15);
		expect(replaceMappedSymbols('abcd', mapping)).toBe('abcd');
	});
});

describe('rotateGlyphStructureBody', () => {
	it('rotates body clockwise and remaps directional symbols', () => {
		const rulesBySymbol: Record<string, Rule> = {
			a: quarterRule('a', Orientation.NE),
			b: quarterRule('b', Orientation.SE),
			c: quarterRule('c', Orientation.SW),
			d: quarterRule('d', Orientation.NW)
		};

		expect(rotateGlyphStructureBody('a ', 90, rulesBySymbol)).toBe('b');
	});
});

import { describe, expect, it } from 'vitest';
import {
	Orientation,
	ShapeKind,
	booleanPropFixed,
	numberPropFixed,
	orientationPropFixed,
	type GlyphInput,
	type Rule
} from '$lib/types';
import {
	resolveGlyphStructures,
	resolveGlyphStructuresWithComponentMask,
	serializeGlyphStructure
} from './structure';

function quarterRule(symbol: string, orientation: Orientation): Rule {
	return {
		symbol,
		shape: {
			kind: ShapeKind.Quarter,
			props: {
				scale_x: numberPropFixed(1),
				scale_y: numberPropFixed(1),
				rotation: numberPropFixed(0),
				squaring: numberPropFixed(0.56),
				negative: booleanPropFixed(false),
				orientation: orientationPropFixed(orientation)
			}
		}
	};
}

describe('resolveGlyphStructures component rotation', () => {
	it('remaps directional symbols when a component is rotated', () => {
		const rulesBySymbol: Record<string, Rule> = {
			a: quarterRule('a', Orientation.NE),
			b: quarterRule('b', Orientation.SE),
			c: quarterRule('c', Orientation.SW),
			d: quarterRule('d', Orientation.NW)
		};

		const part: GlyphInput = {
			id: 'part',
			name: 'part.component',
			structure: 'a'
		};
		const target: GlyphInput = {
			id: 'target',
			name: 'target',
			structure: serializeGlyphStructure({
				components: [{ name: 'part.component', symbol: '', x: 1, y: 1, rotation: 90 }],
				body: ''
			})
		};

		const resolved = resolveGlyphStructures([part, target], {
			transparentSymbols: [' '],
			rulesBySymbol
		});

		expect(resolved.get('target')).toBe('b');
	});

	it('keeps symbols unchanged when no directional rule map is provided', () => {
		const part: GlyphInput = {
			id: 'part',
			name: 'part.component',
			structure: 'a'
		};
		const target: GlyphInput = {
			id: 'target',
			name: 'target',
			structure: serializeGlyphStructure({
				components: [{ name: 'part.component', symbol: '', x: 1, y: 1, rotation: 90 }],
				body: ''
			})
		};

		const resolved = resolveGlyphStructures([part, target], {
			transparentSymbols: [' ']
		});

		expect(resolved.get('target')).toBe('a');
	});
});

describe('resolveGlyphStructures layering', () => {
	it('keeps component cells as background when body has spaces', () => {
		const component: GlyphInput = {
			id: 'part',
			name: 'part.component',
			structure: 'ab'
		};
		const target: GlyphInput = {
			id: 'target',
			name: 'target',
			structure: serializeGlyphStructure({
				components: [{ name: 'part.component', symbol: '', x: 1, y: 1, rotation: 0 }],
				body: ' z'
			})
		};

		const resolved = resolveGlyphStructures([component, target], {
			transparentSymbols: [' ', '.']
		});

		expect(resolved.get('target')).toBe('az');
	});

	it('allows void symbols in body to override component cells', () => {
		const component: GlyphInput = {
			id: 'part',
			name: 'part.component',
			structure: 'ab'
		};
		const target: GlyphInput = {
			id: 'target',
			name: 'target',
			structure: serializeGlyphStructure({
				components: [{ name: 'part.component', symbol: '', x: 1, y: 1, rotation: 0 }],
				body: '. '
			})
		};

		const resolved = resolveGlyphStructures([component, target], {
			transparentSymbols: [' ', '.']
		});

		expect(resolved.get('target')).toBe('.b');
	});

	it('preserves component source metadata under overridden cells', () => {
		const component: GlyphInput = {
			id: 'part',
			name: 'part.component',
			structure: 'ab'
		};
		const target: GlyphInput = {
			id: 'target',
			name: 'target',
			structure: serializeGlyphStructure({
				components: [{ name: 'part.component', symbol: '', x: 1, y: 1, rotation: 0 }],
				body: ' z'
			})
		};

		const resolved = resolveGlyphStructuresWithComponentMask([component, target], {
			transparentSymbols: [' ', '.']
		});
		const targetVisual = resolved.get('target');

		expect(targetVisual?.body).toBe('az');
		expect(targetVisual?.componentSources[0]?.[0]).toContain('part.component');
		expect(targetVisual?.componentSources[0]?.[1]).toContain('part.component');
	});
});

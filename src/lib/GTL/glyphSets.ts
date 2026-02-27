import type { GlyphInput } from '$lib/types';
import { resolveUnicodeNumber } from './glyphName';
import { isComponentGlyphName } from './structure';
import { UNICODE } from './unicode';

export type GlyphSetID =
	| 'latin-base'
	| 'numbers'
	| 'punctuation'
	| 'symbols-simple'
	| 'symbols'
	| 'components'
	| 'custom';

export interface GlyphSetDefinition {
	id: GlyphSetID;
	label: string;
	description: string;
	canGenerate: boolean;
}

const GLYPH_SET_MAP: Record<GlyphSetID, GlyphSetDefinition> = {
	'latin-base': {
		id: 'latin-base',
		label: 'Latin Base',
		description: 'A-Z e a-z',
		canGenerate: true
	},
	numbers: {
		id: 'numbers',
		label: 'Numbers',
		description: '0-9',
		canGenerate: true
	},
	punctuation: {
		id: 'punctuation',
		label: 'Punctuation',
		description: 'Punteggiatura ASCII',
		canGenerate: true
	},
	'symbols-simple': {
		id: 'symbols-simple',
		label: 'Symbols (Simple)',
		description: 'Frecce + simboli comuni',
		canGenerate: true
	},
	symbols: {
		id: 'symbols',
		label: 'Symbols (Full)',
		description: 'Intera sezione simboli Unicode',
		canGenerate: true
	},
	components: {
		id: 'components',
		label: 'Components',
		description: 'Glifi .component',
		canGenerate: false
	},
	custom: {
		id: 'custom',
		label: 'Custom',
		description: 'Alternates, ligature, custom',
		canGenerate: false
	}
};

const GLYPH_SET_ORDER: Array<GlyphSetID> = [
	'latin-base',
	'numbers',
	'punctuation',
	'symbols-simple',
	'symbols',
	'components',
	'custom'
];

const generatedNamesBySet = new Map<GlyphSetID, Array<string>>();

const SIMPLE_SYMBOL_RANGES: Array<readonly [number, number]> = [
	[0x2190, 0x21ff], // Arrows
	[0x25a0, 0x25ff], // Geometric shapes
	[0x2600, 0x266f] // Common symbols (weather, stars, suits, music)
];

const FULL_SYMBOL_RANGES: Array<readonly [number, number]> = [
	[0x20a0, 0x20cf], // Currency symbols
	[0x2100, 0x214f], // Letterlike symbols
	[0x2190, 0x21ff], // Arrows
	[0x2200, 0x22ff], // Mathematical operators
	[0x2300, 0x23ff], // Misc technical
	[0x2460, 0x24ff], // Enclosed alphanumerics
	[0x2500, 0x257f], // Box drawing
	[0x2580, 0x259f], // Block elements
	[0x25a0, 0x25ff], // Geometric shapes
	[0x2600, 0x26ff], // Misc symbols
	[0x2700, 0x27bf], // Dingbats
	[0x27c0, 0x27ef], // Misc mathematical symbols-A
	[0x27f0, 0x27ff], // Supplemental arrows-A
	[0x2900, 0x297f], // Supplemental arrows-B
	[0x2980, 0x29ff], // Misc mathematical symbols-B
	[0x2a00, 0x2aff], // Supplemental mathematical operators
	[0x2b00, 0x2bff] // Misc symbols and arrows
];

function isCodepointInRanges(codepoint: number, ranges: Array<readonly [number, number]>): boolean {
	return ranges.some(([start, end]) => codepoint >= start && codepoint <= end);
}

function isSimpleSymbolCodepoint(codepoint: number): boolean {
	return isCodepointInRanges(codepoint, SIMPLE_SYMBOL_RANGES);
}

function isFullSymbolCodepoint(codepoint: number): boolean {
	return isCodepointInRanges(codepoint, FULL_SYMBOL_RANGES);
}

function classifyCodepoint(codepoint: number): GlyphSetID | undefined {
	const isLatinBaseUpper = codepoint >= 0x41 && codepoint <= 0x5a;
	const isLatinBaseLower = codepoint >= 0x61 && codepoint <= 0x7a;
	if (isLatinBaseUpper || isLatinBaseLower) {
		return 'latin-base';
	}

	if (codepoint >= 0x30 && codepoint <= 0x39) {
		return 'numbers';
	}

	const isAsciiPunctuation =
		(codepoint >= 0x21 && codepoint <= 0x2f) ||
		(codepoint >= 0x3a && codepoint <= 0x40) ||
		(codepoint >= 0x5b && codepoint <= 0x60) ||
		(codepoint >= 0x7b && codepoint <= 0x7e);
	if (isAsciiPunctuation) {
		return 'punctuation';
	}

	if (isSimpleSymbolCodepoint(codepoint)) {
		return 'symbols-simple';
	}

	if (isFullSymbolCodepoint(codepoint)) {
		return 'symbols';
	}

	return undefined;
}

function isCodepointInSet(setID: GlyphSetID, codepoint: number): boolean {
	const isLatinBaseUpper = codepoint >= 0x41 && codepoint <= 0x5a;
	const isLatinBaseLower = codepoint >= 0x61 && codepoint <= 0x7a;
	const isAsciiPunctuation =
		(codepoint >= 0x21 && codepoint <= 0x2f) ||
		(codepoint >= 0x3a && codepoint <= 0x40) ||
		(codepoint >= 0x5b && codepoint <= 0x60) ||
		(codepoint >= 0x7b && codepoint <= 0x7e);

	switch (setID) {
		case 'latin-base':
			return isLatinBaseUpper || isLatinBaseLower;
		case 'numbers':
			return codepoint >= 0x30 && codepoint <= 0x39;
		case 'punctuation':
			return isAsciiPunctuation;
		case 'symbols-simple':
			return isSimpleSymbolCodepoint(codepoint);
		case 'symbols':
			return isFullSymbolCodepoint(codepoint);
		default:
			return false;
	}
}

export function getGlyphSetDefinitions(): Array<GlyphSetDefinition> {
	return GLYPH_SET_ORDER.map((id) => GLYPH_SET_MAP[id]);
}

export function getGeneratableGlyphSetDefinitions(): Array<GlyphSetDefinition> {
	return getGlyphSetDefinitions().filter((definition) => definition.canGenerate);
}

export function normalizeGlyphSetID(input: string | undefined): GlyphSetID | undefined {
	if (!input) return undefined;
	const normalized = input.trim().toLowerCase();
	if (!normalized) return undefined;
	if (normalized in GLYPH_SET_MAP) {
		return normalized as GlyphSetID;
	}
	return undefined;
}

export function getGlyphSetByID(id: GlyphSetID): GlyphSetDefinition {
	return GLYPH_SET_MAP[id];
}

export function inferGlyphSetIDByName(glyphName: string): GlyphSetID {
	const trimmed = glyphName.trim();
	if (!trimmed) return 'custom';
	if (isComponentGlyphName(trimmed)) return 'components';

	const codepoint = resolveUnicodeNumber(trimmed);
	if (codepoint === undefined) return 'custom';

	return classifyCodepoint(codepoint) ?? 'custom';
}

export function inferGlyphSetID(glyph: Pick<GlyphInput, 'name' | 'set'>): GlyphSetID {
	const normalizedSet = normalizeGlyphSetID(glyph.set);
	if (normalizedSet) return normalizedSet;
	return inferGlyphSetIDByName(glyph.name);
}

export function getGlyphNamesForSet(setID: GlyphSetID): Array<string> {
	if (!GLYPH_SET_MAP[setID].canGenerate) return [];
	if (generatedNamesBySet.has(setID)) {
		return generatedNamesBySet.get(setID) ?? [];
	}

	const names = Object.keys(UNICODE)
		.filter((glyphName) => {
			const codepoint = resolveUnicodeNumber(glyphName);
			if (codepoint === undefined) return false;
			return isCodepointInSet(setID, codepoint);
		})
		.sort((a, b) => {
			const aCodepoint = resolveUnicodeNumber(a) ?? 0;
			const bCodepoint = resolveUnicodeNumber(b) ?? 0;
			if (aCodepoint !== bCodepoint) return aCodepoint - bCodepoint;
			return a.localeCompare(b);
		});

	generatedNamesBySet.set(setID, names);
	return names;
}

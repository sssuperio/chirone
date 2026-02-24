import type { GlyphInput } from '$lib/types';
import { resolveUnicodeNumber } from './glyphName';
import { isComponentGlyphName } from './structure';
import { UNICODE } from './unicode';

export type GlyphSetID = 'latin-base' | 'numbers' | 'punctuation' | 'components' | 'custom';

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
	'components',
	'custom'
];

const generatedNamesBySet = new Map<GlyphSetID, Array<string>>();

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

	return undefined;
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
			return classifyCodepoint(codepoint) === setID;
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

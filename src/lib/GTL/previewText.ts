import type { GlyphInput } from '$lib/types';
import { getAlternateBaseName, resolveUnicodeNumber } from './glyphName';
import { parseGlyphStructure } from './structure';

export type ParsedPreviewText = {
	glyphSequence: Array<string>;
	foundGlyphs: Array<GlyphInput>;
	validText: string;
	missing: Array<string>;
	hasNamedTokens: boolean;
};

function buildGlyphLookup(glyphs: Array<GlyphInput>): Map<string, GlyphInput> {
	const byName = new Map<string, GlyphInput>();
	for (const glyph of glyphs) {
		if (!glyph.name || byName.has(glyph.name)) continue;
		byName.set(glyph.name, glyph);
	}
	return byName;
}

function buildUnicodeLookup(glyphs: Array<GlyphInput>): Map<number, GlyphInput> {
	const byUnicode = new Map<number, GlyphInput>();
	for (const glyph of glyphs) {
		const unicode = resolveUnicodeNumber(glyph.name);
		if (unicode === undefined || byUnicode.has(unicode)) continue;
		byUnicode.set(unicode, glyph);
	}
	return byUnicode;
}

function findTokenGlyphName(
	input: string,
	fromIndex: number,
	sortedGlyphNames: Array<string>
): string | undefined {
	for (const glyphName of sortedGlyphNames) {
		if (input.startsWith(glyphName, fromIndex)) {
			return glyphName;
		}
	}
	return undefined;
}

function fallbackTextForGlyphName(
	glyphName: string,
	glyphByName: Map<string, GlyphInput>
): string | undefined {
	const codepoint = resolveUnicodeNumber(glyphName);
	if (codepoint !== undefined) return String.fromCodePoint(codepoint);

	const baseName = getAlternateBaseName(glyphName);
	if (!baseName) return undefined;
	const baseGlyph = glyphByName.get(baseName);
	if (!baseGlyph) return undefined;

	const baseCodepoint = resolveUnicodeNumber(baseGlyph.name);
	if (baseCodepoint !== undefined) return String.fromCodePoint(baseCodepoint);
	return undefined;
}

function collectGlyphDependencies(
	glyphSequence: Array<string>,
	glyphByName: Map<string, GlyphInput>
): Array<GlyphInput> {
	const visited = new Set<string>();
	const ordered: Array<GlyphInput> = [];

	const walk = (glyphName: string) => {
		if (!glyphName || visited.has(glyphName)) return;
		visited.add(glyphName);

		const glyph = glyphByName.get(glyphName);
		if (!glyph) return;

		ordered.push(glyph);

		const parsed = parseGlyphStructure(glyph.structure);
		for (const component of parsed.components) {
			walk(component.name);
		}
	};

	for (const glyphName of glyphSequence) {
		walk(glyphName);
	}

	return ordered;
}

export function parsePreviewText(
	text: string,
	availableGlyphs: Array<GlyphInput>
): ParsedPreviewText {
	const glyphByName = buildGlyphLookup(availableGlyphs);
	const glyphByUnicode = buildUnicodeLookup(availableGlyphs);
	const sortedGlyphNames = Array.from(glyphByName.keys()).sort((a, b) => b.length - a.length);

	const glyphSequence: Array<string> = [];
	const validChars: Array<string> = [];
	const missing: Array<string> = [];
	let hasNamedTokens = false;

	for (let index = 0; index < text.length; ) {
		const isTokenStart = text[index] === '/';
		if (isTokenStart) {
			const matchedGlyphName = findTokenGlyphName(text, index + 1, sortedGlyphNames);
			if (matchedGlyphName) {
				glyphSequence.push(matchedGlyphName);
				hasNamedTokens = true;
				const fallbackChar = fallbackTextForGlyphName(matchedGlyphName, glyphByName);
				if (fallbackChar) validChars.push(fallbackChar);
				index += matchedGlyphName.length + 1;
				continue;
			}
		}

		const codepoint = text.codePointAt(index);
		if (codepoint === undefined) break;
		const char = String.fromCodePoint(codepoint);
		const glyph = glyphByUnicode.get(codepoint);
		if (glyph) {
			glyphSequence.push(glyph.name);
			validChars.push(char);
		} else {
			missing.push(char);
		}
		index += char.length;
	}

	return {
		glyphSequence,
		foundGlyphs: collectGlyphDependencies(glyphSequence, glyphByName),
		validText: validChars.join(''),
		missing,
		hasNamedTokens
	};
}

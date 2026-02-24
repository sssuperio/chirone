import { UNICODE, findCharInUnicodeList } from './unicode';

export function isValidGlyphName(name: string): boolean {
	if (!name) return false;
	return !/\s/.test(name);
}

export function normalizeGlyphNameInput(raw: string): string {
	const trimmed = raw.trim();
	if (!trimmed) return '';

	// If user types a literal character, map it to known glyph name when possible.
	if (trimmed.length === 1) {
		const match = findCharInUnicodeList(trimmed);
		if (match) return match[0];
	}

	return trimmed;
}

export function resolveUnicodeNumber(name: string): number | undefined {
	const hex = UNICODE[name];
	if (hex) return parseInt(hex, 16);
	if (name.length === 1) {
		return name.codePointAt(0);
	}
	return undefined;
}

export function getLigatureComponentNames(name: string): Array<string> {
	if (!name.includes('_')) return [];
	const components = name.split('_').map((part) => part.trim());
	if (components.length < 2) return [];
	if (components.some((part) => !part)) return [];
	return components;
}

export function getAlternateBaseName(name: string): string | undefined {
	const dotIndex = name.indexOf('.');
	if (dotIndex <= 0) return undefined;
	return name.slice(0, dotIndex);
}

export function getStylisticSetFeature(name: string): string | undefined {
	const match = name.match(/\.((?:ss\d\d))$/i);
	if (!match) return undefined;
	return match[1].toLowerCase();
}

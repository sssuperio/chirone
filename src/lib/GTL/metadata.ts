export interface FontMetadata {
	name: string;
	familyName: string;
	version: string;
	createdDate: string;
	designer: string;
	manufacturer: string;
	designerURL: string;
	manufacturerURL: string;
	license: string;
}

type SafeFontMetadataLike = Partial<FontMetadata> | null | undefined;

function todayISODate(): string {
	const now = new Date();
	const year = now.getFullYear();
	const month = `${now.getMonth() + 1}`.padStart(2, '0');
	const day = `${now.getDate()}`.padStart(2, '0');
	return `${year}-${month}-${day}`;
}

function normalizeString(value: unknown, fallback = ''): string {
	if (typeof value !== 'string') return fallback;
	return value.trim();
}

function normalizeDate(value: unknown, fallback: string): string {
	if (typeof value !== 'string') return fallback;
	const trimmed = value.trim();
	if (!/^\d{4}-\d{2}-\d{2}$/.test(trimmed)) return fallback;
	const [year, month, day] = trimmed.split('-').map((part) => Number(part));
	const parsed = new Date(Date.UTC(year, month - 1, day));
	if (
		Number.isNaN(parsed.getTime()) ||
		parsed.getUTCFullYear() !== year ||
		parsed.getUTCMonth() !== month - 1 ||
		parsed.getUTCDate() !== day
	) {
		return fallback;
	}
	return trimmed;
}

export function normalizeFontMetadata(input: SafeFontMetadataLike): FontMetadata {
	const fallbackDate = todayISODate();
	return {
		name: normalizeString(input?.name),
		familyName: normalizeString(input?.familyName, 'GTL'),
		version: normalizeString(input?.version, 'Version 1.0'),
		createdDate: normalizeDate(input?.createdDate, fallbackDate),
		designer: normalizeString(input?.designer),
		manufacturer: normalizeString(input?.manufacturer),
		designerURL: normalizeString(input?.designerURL),
		manufacturerURL: normalizeString(input?.manufacturerURL),
		license: normalizeString(input?.license)
	};
}

export const defaultFontMetadata: FontMetadata = normalizeFontMetadata(undefined);

export function areFontMetadataEqual(
	a: SafeFontMetadataLike,
	b: SafeFontMetadataLike
): boolean {
	const aa = normalizeFontMetadata(a);
	const bb = normalizeFontMetadata(b);
	return (
		aa.name === bb.name &&
		aa.familyName === bb.familyName &&
		aa.version === bb.version &&
		aa.createdDate === bb.createdDate &&
		aa.designer === bb.designer &&
		aa.manufacturer === bb.manufacturer &&
		aa.designerURL === bb.designerURL &&
		aa.manufacturerURL === bb.manufacturerURL &&
		aa.license === bb.license
	);
}


export interface FontMetrics {
	UPM: number;
	height: number;
	baseline: number;
	descender: number;
	ascender: number;
	capHeight: number;
	xHeight: number;
}

type SafeFontMetricsLike = Partial<FontMetrics> | null | undefined;

function toFiniteNumber(value: unknown, fallback: number): number {
	if (typeof value === 'number' && Number.isFinite(value)) return value;
	return fallback;
}

function toDiscrete(value: number, min: number, max: number): number {
	const rounded = Math.round(value);
	return Math.min(max, Math.max(min, rounded));
}

function snapUPMToDiscreteCellGrid(UPM: number, height: number): number {
	const safeHeight = Math.max(1, Math.round(height));
	const safeUPM = Math.max(safeHeight, Math.round(UPM));
	const snapped = Math.round(safeUPM / safeHeight) * safeHeight;
	return Math.max(safeHeight, snapped);
}

export function estimateVerticalMetrics(height: number, descender: number) {
	const safeHeight = Math.max(1, Math.round(height));
	const safeDescender = toDiscrete(descender, 0, Math.max(0, safeHeight - 1));
	const ascender = Math.max(1, safeHeight - safeDescender);
	const capHeight = toDiscrete(Math.round(ascender * 0.9), 1, ascender);
	const xHeight = toDiscrete(Math.round(capHeight * 0.7), 1, capHeight);

	return {
		height: safeHeight,
		descender: safeDescender,
		ascender,
		capHeight,
		xHeight
	};
}

export function normalizeFontMetrics(input: SafeFontMetricsLike): FontMetrics {
	const defaultHeight = 5;
	const defaultDescender = 1;
	const rawAscender = toFiniteNumber(input?.ascender, NaN);
	const rawDescender = toFiniteNumber(input?.descender, toFiniteNumber(input?.baseline, NaN));
	const rawHeight = toFiniteNumber(input?.height, NaN);

	let height = Number.isFinite(rawHeight) ? Math.round(rawHeight) : defaultHeight;
	if (!Number.isFinite(rawHeight) && Number.isFinite(rawAscender) && Number.isFinite(rawDescender)) {
		height = Math.max(1, Math.round(rawAscender + rawDescender));
	}
	height = Math.max(1, height);

	const descender = toDiscrete(
		Number.isFinite(rawDescender) ? rawDescender : defaultDescender,
		0,
		Math.max(0, height - 1)
	);
	const vertical = estimateVerticalMetrics(height, descender);

	const capHeight = toDiscrete(toFiniteNumber(input?.capHeight, vertical.capHeight), 1, vertical.ascender);
	const xHeight = toDiscrete(
		toFiniteNumber(input?.xHeight, Math.round(capHeight * 0.7)),
		1,
		capHeight
	);

	const defaultUpm = 5 * 5 * 5 * 2 * 2 * 5;
	const requestedUPM = Math.max(
		height,
		toDiscrete(toFiniteNumber(input?.UPM, defaultUpm), height, Number.MAX_SAFE_INTEGER)
	);
	const UPM = snapUPMToDiscreteCellGrid(requestedUPM, vertical.height);

	return {
		UPM,
		height: vertical.height,
		baseline: vertical.descender,
		descender: vertical.descender,
		ascender: vertical.ascender,
		capHeight,
		xHeight
	};
}

export const defaultFontMetrics: FontMetrics = normalizeFontMetrics({
	UPM: 5 * 5 * 5 * 2 * 2 * 5,
	height: 5,
	baseline: 1
});

export function unitsPerCell(metrics: FontMetrics): number {
	const safeHeight = Math.max(1, Math.round(metrics.height));
	return Math.round(metrics.UPM / safeHeight);
}

export function cellsToUnits(metrics: FontMetrics, cells: number): number {
	return Math.round(unitsPerCell(metrics) * cells);
}

export function areMetricsEqual(a: SafeFontMetricsLike, b: SafeFontMetricsLike): boolean {
	const aa = normalizeFontMetrics(a);
	const bb = normalizeFontMetrics(b);
	return (
		aa.UPM === bb.UPM &&
		aa.height === bb.height &&
		aa.baseline === bb.baseline &&
		aa.descender === bb.descender &&
		aa.ascender === bb.ascender &&
		aa.capHeight === bb.capHeight &&
		aa.xHeight === bb.xHeight
	);
}

import { Orientation, ShapeKind, ValueKind, type Rule } from '$lib/types';

export type ReflectionAxis = 'vertical' | 'horizontal';

const REFLECTION_ORIENTATION_MAP: Record<ReflectionAxis, Record<Orientation, Orientation>> = {
	vertical: {
		[Orientation.NE]: Orientation.NW,
		[Orientation.NW]: Orientation.NE,
		[Orientation.SE]: Orientation.SW,
		[Orientation.SW]: Orientation.SE
	},
	horizontal: {
		[Orientation.NE]: Orientation.SE,
		[Orientation.SE]: Orientation.NE,
		[Orientation.NW]: Orientation.SW,
		[Orientation.SW]: Orientation.NW
	}
};

const CLOCKWISE_ORIENTATION_MAP: Record<Orientation, Orientation> = {
	[Orientation.NE]: Orientation.SE,
	[Orientation.SE]: Orientation.SW,
	[Orientation.SW]: Orientation.NW,
	[Orientation.NW]: Orientation.NE
};

function getDirectionalOrientation(rule: Rule): Orientation | undefined {
	if (rule.shape.kind !== ShapeKind.Quarter && rule.shape.kind !== ShapeKind.Triangle) {
		return undefined;
	}

	const orientationProp = rule.shape.props.orientation;
	if (orientationProp.value.kind !== ValueKind.Fixed) return undefined;
	return orientationProp.value.data;
}

function getDirectionalShapeSignature(rule: Rule): string | undefined {
	if (rule.shape.kind !== ShapeKind.Quarter && rule.shape.kind !== ShapeKind.Triangle) {
		return undefined;
	}

	const { orientation: _orientation, ...restProps } = rule.shape.props;
	return JSON.stringify({
		kind: rule.shape.kind,
		props: restProps
	});
}

function mapDirectionalSymbols(
	rulesBySymbol: Record<string, Rule>,
	resolveTargetOrientation: (orientation: Orientation) => Orientation
): Map<string, string> {
	const symbolsBySignature = new Map<string, Map<Orientation, string>>();

	for (const [symbol, rule] of Object.entries(rulesBySymbol)) {
		if (symbol.length !== 1) continue;
		const orientation = getDirectionalOrientation(rule);
		const signature = getDirectionalShapeSignature(rule);
		if (!orientation || !signature) continue;

		const byOrientation = symbolsBySignature.get(signature) ?? new Map<Orientation, string>();
		if (!byOrientation.has(orientation)) {
			byOrientation.set(orientation, symbol);
		}
		symbolsBySignature.set(signature, byOrientation);
	}

	const reflectedSymbols = new Map<string, string>();

	for (const [symbol, rule] of Object.entries(rulesBySymbol)) {
		if (symbol.length !== 1) continue;
		const orientation = getDirectionalOrientation(rule);
		const signature = getDirectionalShapeSignature(rule);
		if (!orientation || !signature) continue;

		const targetOrientation = resolveTargetOrientation(orientation);
		const targetSymbol = symbolsBySignature.get(signature)?.get(targetOrientation);
		if (targetSymbol) {
			reflectedSymbols.set(symbol, targetSymbol);
		}
	}

	return reflectedSymbols;
}

function normalizeRotation(rotation: number): number {
	if (!Number.isFinite(rotation)) return 0;
	const rounded = Math.round(rotation);
	const normalized = ((rounded % 360) + 360) % 360;
	return normalized === 360 ? 0 : normalized;
}

function rotateOrientationClockwise(orientation: Orientation, quarterTurns: number): Orientation {
	let next = orientation;
	for (let index = 0; index < quarterTurns; index++) {
		next = CLOCKWISE_ORIENTATION_MAP[next];
	}
	return next;
}

export function mapDirectionalSymbolsForReflection(
	rulesBySymbol: Record<string, Rule>,
	axis: ReflectionAxis
): Map<string, string> {
	return mapDirectionalSymbols(rulesBySymbol, (orientation) => {
		return REFLECTION_ORIENTATION_MAP[axis][orientation];
	});
}

export function mapDirectionalSymbolsForRotation(
	rulesBySymbol: Record<string, Rule>,
	rotation: number
): Map<string, string> {
	const normalized = normalizeRotation(rotation);
	if (normalized === 0) return new Map<string, string>();
	if (normalized % 90 !== 0) return new Map<string, string>();

	const quarterTurns = Math.trunc(normalized / 90);
	return mapDirectionalSymbols(rulesBySymbol, (orientation) => {
		return rotateOrientationClockwise(orientation, quarterTurns);
	});
}

export function replaceMappedSymbols(source: string, mappedSymbols: Map<string, string>): string {
	if (!mappedSymbols.size) return source;
	return Array.from(source)
		.map((char) => mappedSymbols.get(char) ?? char)
		.join('');
}

export function reflectGlyphStructureBody(
	body: string,
	axis: ReflectionAxis,
	rulesBySymbol: Record<string, Rule>
): string {
	const rows = (body ?? '').split('\n');
	const reflectedSymbols = mapDirectionalSymbolsForReflection(rulesBySymbol, axis);

	if (axis === 'horizontal') {
		return [...rows].reverse().map((row) => replaceMappedSymbols(row, reflectedSymbols)).join('\n');
	}

	const width = Math.max(0, ...rows.map((row) => Array.from(row).length));
	return rows
		.map((row) =>
			replaceMappedSymbols(Array.from(row.padEnd(width, ' ')).reverse().join(''), reflectedSymbols)
		)
		.join('\n');
}

export function rotateGlyphStructureBody(
	body: string,
	rotation: number,
	rulesBySymbol: Record<string, Rule>
): string {
	const rows = (body ?? '').split('\n');
	const sourceHeight = rows.length;
	const sourceWidth = Math.max(0, ...rows.map((row) => Array.from(row).length));
	if (!sourceHeight || !sourceWidth) return body ?? '';

	const normalized = ((Math.round(rotation) % 360) + 360) % 360;
	if (!normalized) return body ?? '';
	const sourceMatrix = rows.map((row) => Array.from(row.padEnd(sourceWidth, ' ')));
	const centerX = (sourceWidth - 1) / 2;
	const centerY = (sourceHeight - 1) / 2;
	const radians = (normalized * Math.PI) / 180;
	const cos = Math.cos(radians);
	const sin = Math.sin(radians);

	const rotatedCells: Array<{ x: number; y: number; value: string }> = [];
	for (let y = 0; y < sourceHeight; y++) {
		for (let x = 0; x < sourceWidth; x++) {
			const value = sourceMatrix[y][x] ?? ' ';
			if (value === ' ') continue;

			const relativeX = x - centerX;
			const relativeY = y - centerY;
			const rotatedX = relativeX * cos - relativeY * sin + centerX;
			const rotatedY = relativeX * sin + relativeY * cos + centerY;

			rotatedCells.push({
				x: Math.round(rotatedX),
				y: Math.round(rotatedY),
				value
			});
		}
	}

	if (!rotatedCells.length) return body ?? '';

	const minX = Math.min(...rotatedCells.map((cell) => cell.x));
	const maxX = Math.max(...rotatedCells.map((cell) => cell.x));
	const minY = Math.min(...rotatedCells.map((cell) => cell.y));
	const maxY = Math.max(...rotatedCells.map((cell) => cell.y));
	const targetWidth = maxX - minX + 1;
	const targetHeight = maxY - minY + 1;
	const targetRows = Array.from({ length: targetHeight }, () => Array(targetWidth).fill(' '));

	for (const cell of rotatedCells) {
		const targetX = cell.x - minX;
		const targetY = cell.y - minY;
		targetRows[targetY][targetX] = cell.value;
	}

	const mappedSymbols = mapDirectionalSymbolsForRotation(rulesBySymbol, normalized);
	const serializedRows = targetRows.map((row) =>
		replaceMappedSymbols(row.join(''), mappedSymbols).replace(/\s+$/g, '')
	);
	while (serializedRows.length && serializedRows[serializedRows.length - 1] === '') {
		serializedRows.pop();
	}
	return serializedRows.join('\n');
}

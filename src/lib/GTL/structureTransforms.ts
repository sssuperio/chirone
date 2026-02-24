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

function mapDirectionalSymbolsForAxis(
	rulesBySymbol: Record<string, Rule>,
	axis: ReflectionAxis
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

		const reflectedOrientation = REFLECTION_ORIENTATION_MAP[axis][orientation];
		const reflectedSymbol = symbolsBySignature.get(signature)?.get(reflectedOrientation);
		if (reflectedSymbol) {
			reflectedSymbols.set(symbol, reflectedSymbol);
		}
	}

	return reflectedSymbols;
}

function replaceMappedSymbols(source: string, reflectedSymbols: Map<string, string>): string {
	if (!reflectedSymbols.size) return source;
	return Array.from(source)
		.map((char) => reflectedSymbols.get(char) ?? char)
		.join('');
}

export function reflectGlyphStructureBody(
	body: string,
	axis: ReflectionAxis,
	rulesBySymbol: Record<string, Rule>
): string {
	const rows = (body ?? '').split('\n');
	const reflectedSymbols = mapDirectionalSymbolsForAxis(rulesBySymbol, axis);

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

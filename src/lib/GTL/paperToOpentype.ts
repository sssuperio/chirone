import SVGPathCommander from 'svg-path-commander';

//

export const PathLetters = ['C', 'M', 'Z', 'L'] as const;
export type PathLetter = (typeof PathLetters)[number];
const PATH_DECIMALS = 6;
const AXIS_SNAP_EPSILON = 1e-4;

type ArrayDirective =
	| ['Z']
	| ['M' | 'L', number, number]
	| ['C', number, number, number, number, number, number];

type Point = {
	x: number;
	y: number;
};

function roundCoord(value: number): number {
	const rounded = Number(value.toFixed(PATH_DECIMALS));
	return Object.is(rounded, -0) ? 0 : rounded;
}

function snapToAxis(value: number, axis: number): number {
	if (Math.abs(value - axis) <= AXIS_SNAP_EPSILON) {
		return axis;
	}
	return value;
}

export function normalizeAbsoluteDirectives(
	directives: Array<ArrayDirective>
): Array<ArrayDirective> {
	const normalized: Array<ArrayDirective> = [];
	let current: Point | undefined;
	let subpathStart: Point | undefined;

	for (const directive of directives) {
		if (
			directive[0] !== 'Z' &&
			directive[0] !== 'M' &&
			directive[0] !== 'L' &&
			directive[0] !== 'C'
		) {
			continue;
		}

		if (directive[0] === 'Z') {
			normalized.push(['Z']);
			current = subpathStart;
			continue;
		}

		if (directive[0] === 'M' || directive[0] === 'L') {
			let x = roundCoord(directive[1]);
			let y = roundCoord(directive[2]);

			if (current) {
				x = snapToAxis(x, current.x);
				y = snapToAxis(y, current.y);
			}

			const point = { x: roundCoord(x), y: roundCoord(y) };
			normalized.push([directive[0], point.x, point.y]);

			current = point;
			if (directive[0] === 'M') {
				subpathStart = point;
			}
			continue;
		}

		const cubic = directive as ['C', number, number, number, number, number, number];
		const [, cx1, cy1, cx2, cy2, cx, cy] = cubic;
		let x1 = roundCoord(cx1);
		let y1 = roundCoord(cy1);
		let x2 = roundCoord(cx2);
		let y2 = roundCoord(cy2);
		let x = roundCoord(cx);
		let y = roundCoord(cy);

		if (current) {
			x1 = snapToAxis(x1, current.x);
			y1 = snapToAxis(y1, current.y);
			x = snapToAxis(x, current.x);
			y = snapToAxis(y, current.y);

			if (x === current.x) {
				x2 = snapToAxis(x2, current.x);
			}
			if (y === current.y) {
				y2 = snapToAxis(y2, current.y);
			}
		}

		const end = { x: roundCoord(x), y: roundCoord(y) };
		normalized.push([
			'C',
			roundCoord(x1),
			roundCoord(y1),
			roundCoord(x2),
			roundCoord(y2),
			end.x,
			end.y
		]);
		current = end;
	}

	return normalized;
}

export function getAbsoluteSVGPath(
	path: paper.PathItem
): Array<ArrayDirective> {
	// Getting SVG path
	const svg = path.exportSVG({ asString: false }) as SVGElement;
	// Getting path attribute
	const d = svg.getAttribute('d');

	if (d) {
		// Normalize first so the path is reduced to supported directive kinds (M/L/C/Z),
		// then convert to absolute coordinates for OpenType conversion.
		const absPath = new SVGPathCommander(d)
			.normalize()
			.toAbsolute().segments as Array<ArrayDirective>;

		return normalizeAbsoluteDirectives(absPath);
	} else {
		return [];
	}
}

type OpentypeDirective =
	| { type: 'Z' }
	| { type: 'M' | 'L'; x: number; y: number }
	| {
			type: 'C';
			x1: number;
			y1: number;
			x2: number;
			y2: number;
			x: number;
			y: number;
	  };

//

export function arrayToDirectives(
	directives: Array<ArrayDirective>
): Array<OpentypeDirective> {
	const dirs: Array<OpentypeDirective> = [];

	if (!directives.length) {
		return [];
	}

	const normalized = normalizeAbsoluteDirectives(directives);

	for (const item of normalized) {
		if (item[0] == 'Z') {
			dirs.push({ type: item[0] });
		}
		if (item[0] == 'L' || item[0] == 'M') {
			dirs.push({
				type: item[0],
				x: item[1],
				y: item[2]
			});
		}
		if (item[0] == 'C') {
			dirs.push({
				type: item[0],
				x1: item[1],
				y1: item[2],
				x2: item[3],
				y2: item[4],
				x: item[5],
				y: item[6]
			});
		}
	}

	return dirs;
}

//

export function editPathFromDirectives(
	p: opentype.Path,
	d: Array<OpentypeDirective>,
	ty = 0
): opentype.Path {
	for (const i of d) {
		if (i.type == 'M') {
			p.moveTo(i.x, i.y + ty);
		} else if (i.type == 'L') {
			p.lineTo(i.x, i.y + ty);
		} else if (i.type == 'Z') {
			p.close();
		} else if (i.type == 'C') {
			p.bezierCurveTo(i.x1, i.y1 + ty, i.x2, i.y2 + ty, i.x, i.y + ty);
		}
	}
	return p;
}

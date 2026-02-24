import type { GlyphInput } from '../types';

export interface GlyphComponentRef {
	name: string;
	symbol: string;
	x: number;
	y: number;
}

export interface ParsedGlyphStructure {
	components: Array<GlyphComponentRef>;
	body: string;
}

export interface ResolveStructureOptions {
	transparentSymbols?: Iterable<string>;
	applySymbolOverride?: boolean;
}

export interface ResolvedGlyphVisualData {
	body: string;
	componentMask: string;
	componentSources: Array<Array<Array<string>>>;
}

const FRONTMATTER_SEPARATOR = '---';
const SIMPLE_SCALAR_PATTERN = /^[A-Za-z0-9._-]+$/;
const DEFAULT_COMPONENT_POSITION = 1;
const MAX_COMPONENT_DEPTH = 32;

function normalizeLineEndings(value: string): string {
	return value.replace(/\r\n?/g, '\n');
}

function sanitizeComponentPosition(value: number | undefined): number {
	if (!Number.isFinite(value)) return DEFAULT_COMPONENT_POSITION;
	return Math.max(DEFAULT_COMPONENT_POSITION, Math.trunc(value as number));
}

function sanitizeComponentSymbol(value: string | undefined): string {
	if (typeof value !== 'string') return '';
	const first = Array.from(value.trim())[0];
	return first ?? '';
}

function sanitizeComponentName(value: string | undefined): string {
	return (value ?? '').trim();
}

function sanitizeComponentRef(partial: Partial<GlyphComponentRef>): GlyphComponentRef | undefined {
	const name = sanitizeComponentName(partial.name);
	if (!name) return undefined;

	return {
		name,
		symbol: sanitizeComponentSymbol(partial.symbol),
		x: sanitizeComponentPosition(partial.x),
		y: sanitizeComponentPosition(partial.y)
	};
}

function parseScalar(rawValue: string): string {
	const value = rawValue.trim();
	if (!value) return '';

	if (value.startsWith('"') && value.endsWith('"')) {
		try {
			const parsed = JSON.parse(value);
			return typeof parsed === 'string' ? parsed : value.slice(1, -1);
		} catch {
			return value.slice(1, -1);
		}
	}

	if (value.startsWith("'") && value.endsWith("'")) {
		return value.slice(1, -1);
	}

	return value;
}

function formatScalar(value: string): string {
	if (!value) return '""';
	if (SIMPLE_SCALAR_PATTERN.test(value)) return value;
	return JSON.stringify(value);
}

function parseFrontmatter(frontmatter: string | undefined): Array<GlyphComponentRef> {
	if (!frontmatter) return [];

	const lines = normalizeLineEndings(frontmatter).split('\n');
	const components: Array<GlyphComponentRef> = [];
	let current: Partial<GlyphComponentRef> | null = null;
	let inComponentsSection = false;

	const flushCurrent = () => {
		if (!current) return;
		const sanitized = sanitizeComponentRef(current);
		if (sanitized) {
			components.push(sanitized);
		}
		current = null;
	};

	for (const line of lines) {
		const trimmed = line.trim();
		if (!trimmed || trimmed.startsWith('#')) continue;

		if (/^components\s*:/.test(trimmed)) {
			inComponentsSection = true;
			continue;
		}

		if (!inComponentsSection) continue;

		if (trimmed.startsWith('-')) {
			flushCurrent();
			current = {};
			const inline = trimmed.slice(1).trim();
			if (!inline) continue;

			const inlineMatch = inline.match(/^([A-Za-z0-9_-]+)\s*:\s*(.*)$/);
			if (!inlineMatch) continue;

			const inlineKey = inlineMatch[1];
			const inlineValue = parseScalar(inlineMatch[2]);
			if (inlineKey === 'name') current.name = inlineValue;
			if (inlineKey === 'symbol') current.symbol = inlineValue;
			if (inlineKey === 'x') current.x = Number.parseInt(inlineValue, 10);
			if (inlineKey === 'y') current.y = Number.parseInt(inlineValue, 10);
			continue;
		}

		if (!current) continue;

		const propertyMatch = trimmed.match(/^([A-Za-z0-9_-]+)\s*:\s*(.*)$/);
		if (!propertyMatch) continue;

		const key = propertyMatch[1];
		const parsedValue = parseScalar(propertyMatch[2]);

		if (key === 'name') current.name = parsedValue;
		if (key === 'symbol') current.symbol = parsedValue;
		if (key === 'x') current.x = Number.parseInt(parsedValue, 10);
		if (key === 'y') current.y = Number.parseInt(parsedValue, 10);
	}

	flushCurrent();

	return components;
}

function splitStructure(raw: string): { frontmatter: string | undefined; body: string } {
	const normalized = normalizeLineEndings(raw ?? '');
	const lines = normalized.split('\n');
	if (!lines.length || lines[0].trim() !== FRONTMATTER_SEPARATOR) {
		return { frontmatter: undefined, body: normalized };
	}

	let closingIndex = -1;
	for (let i = 1; i < lines.length; i++) {
		if (lines[i].trim() === FRONTMATTER_SEPARATOR) {
			closingIndex = i;
			break;
		}
	}

	if (closingIndex === -1) {
		return { frontmatter: undefined, body: normalized };
	}

	return {
		frontmatter: lines.slice(1, closingIndex).join('\n'),
		body: lines.slice(closingIndex + 1).join('\n')
	};
}

export function parseGlyphStructure(raw: string): ParsedGlyphStructure {
	const { frontmatter, body } = splitStructure(raw ?? '');
	return {
		components: parseFrontmatter(frontmatter),
		body
	};
}

export function serializeGlyphStructure(parsed: ParsedGlyphStructure): string {
	const body = normalizeLineEndings(parsed.body ?? '');
	const components = parsed.components
		.map((component) => sanitizeComponentRef(component))
		.filter((component): component is GlyphComponentRef => Boolean(component));

	if (!components.length) return body;

	const lines: Array<string> = [FRONTMATTER_SEPARATOR, 'components:'];
	for (const component of components) {
		lines.push(`  - name: ${formatScalar(component.name)}`);
		lines.push(`    symbol: ${formatScalar(component.symbol)}`);
		lines.push(`    x: ${sanitizeComponentPosition(component.x)}`);
		lines.push(`    y: ${sanitizeComponentPosition(component.y)}`);
	}
	lines.push(FRONTMATTER_SEPARATOR);

	return body ? `${lines.join('\n')}\n${body}` : `${lines.join('\n')}\n`;
}

export function replaceGlyphStructureBody(raw: string, body: string): string {
	const parsed = parseGlyphStructure(raw ?? '');
	return serializeGlyphStructure({ ...parsed, body });
}

export function replaceGlyphStructureComponents(
	raw: string,
	components: Array<GlyphComponentRef>
): string {
	const parsed = parseGlyphStructure(raw ?? '');
	return serializeGlyphStructure({ ...parsed, components });
}

function splitRows(body: string): Array<Array<string>> {
	if (!body) return [];
	return normalizeLineEndings(body)
		.split('\n')
		.map((row) => Array.from(row));
}

function rowsToBody(rows: Array<Array<string>>): string {
	const serializedRows = rows.map((row) => row.join('').replace(/\s+$/g, ''));

	while (serializedRows.length && serializedRows[serializedRows.length - 1] === '') {
		serializedRows.pop();
	}

	return serializedRows.join('\n');
}

function ensureCell(matrix: Array<Array<string>>, row: number, col: number) {
	while (matrix.length <= row) {
		matrix.push([]);
	}
	while (matrix[row].length <= col) {
		matrix[row].push(' ');
	}
}

function ensureComponentSourceCell(matrix: Array<Array<Array<string>>>, row: number, col: number) {
	while (matrix.length <= row) {
		matrix.push([]);
	}
	while (matrix[row].length <= col) {
		matrix[row].push([]);
	}
}

function createComponentSourceRowsFromBody(rows: Array<Array<string>>): Array<Array<Array<string>>> {
	return rows.map((row) => Array.from({ length: row.length }, () => [] as Array<string>));
}

function normalizeTransparentSymbols(options?: ResolveStructureOptions): Set<string> {
	const symbols = new Set<string>([' ']);
	if (!options?.transparentSymbols) return symbols;

	for (const symbol of options.transparentSymbols) {
		if (!symbol) continue;
		symbols.add(symbol);
	}
	return symbols;
}

function shouldApplySymbolOverride(options?: ResolveStructureOptions): boolean {
	return options?.applySymbolOverride ?? true;
}

function applyComponent(
	baseRows: Array<Array<string>>,
	componentBody: string,
	component: GlyphComponentRef,
	transparentSymbols: Set<string>,
	applySymbolOverride: boolean
): Array<Array<string>> {
	const matrix = baseRows.map((row) => [...row]);
	const componentRows = splitRows(componentBody);
	if (!componentRows.length) return matrix;

	const offsetX = Math.max(0, sanitizeComponentPosition(component.x) - 1);
	const offsetY = Math.max(0, sanitizeComponentPosition(component.y) - 1);
	const overrideSymbol = sanitizeComponentSymbol(component.symbol);

	for (let y = 0; y < componentRows.length; y++) {
		for (let x = 0; x < componentRows[y].length; x++) {
			const value = componentRows[y][x];
			if (transparentSymbols.has(value)) continue;

			const nextValue = applySymbolOverride && overrideSymbol ? overrideSymbol : value;
			const targetRow = offsetY + y;
			const targetCol = offsetX + x;
			ensureCell(matrix, targetRow, targetCol);
			matrix[targetRow][targetCol] = nextValue;
		}
	}

	return matrix;
}

function applyComponentWithMask(
	baseRows: Array<Array<string>>,
	baseComponentSources: Array<Array<Array<string>>>,
	componentBody: string,
	componentSources: Array<Array<Array<string>>>,
	component: GlyphComponentRef,
	transparentSymbols: Set<string>,
	applySymbolOverride: boolean
): { rows: Array<Array<string>>; componentSources: Array<Array<Array<string>>> } {
	const matrix = baseRows.map((row) => [...row]);
	const sourceMatrix = baseComponentSources.map((row) => row.map((cell) => [...cell]));
	const componentRows = splitRows(componentBody);
	if (!componentRows.length) {
		return { rows: matrix, componentSources: sourceMatrix };
	}

	const offsetX = Math.max(0, sanitizeComponentPosition(component.x) - 1);
	const offsetY = Math.max(0, sanitizeComponentPosition(component.y) - 1);
	const overrideSymbol = sanitizeComponentSymbol(component.symbol);
	const componentName = sanitizeComponentName(component.name);

	for (let y = 0; y < componentRows.length; y++) {
		for (let x = 0; x < componentRows[y].length; x++) {
			const value = componentRows[y][x];
			if (transparentSymbols.has(value)) continue;

			const nextValue = applySymbolOverride && overrideSymbol ? overrideSymbol : value;
			const targetRow = offsetY + y;
			const targetCol = offsetX + x;
			ensureCell(matrix, targetRow, targetCol);
			ensureComponentSourceCell(sourceMatrix, targetRow, targetCol);
			matrix[targetRow][targetCol] = nextValue;

			const nestedSources = componentSources[y]?.[x] ?? [];
			const incomingSources = nestedSources.length ? nestedSources : [componentName];
			const currentSources = sourceMatrix[targetRow][targetCol] ?? [];
			const mergedSources = [...currentSources];
			for (const sourceName of incomingSources) {
				if (!sourceName) continue;
				if (!mergedSources.includes(sourceName)) {
					mergedSources.push(sourceName);
				}
			}
			sourceMatrix[targetRow][targetCol] = mergedSources;
		}
	}

	return {
		rows: matrix,
		componentSources: sourceMatrix
	};
}

function rowsToMaskBody(rows: Array<Array<Array<string>>>): string {
	const serializedRows = rows.map((row) =>
		row
			.map((componentSourceNames) => (componentSourceNames.length ? '1' : ' '))
			.join('')
			.replace(/\s+$/g, '')
	);

	while (serializedRows.length && serializedRows[serializedRows.length - 1] === '') {
		serializedRows.pop();
	}

	return serializedRows.join('\n');
}

function resolveStructureBody(
	glyphName: string,
	glyphMap: Map<string, GlyphInput>,
	cache: Map<string, string>,
	visiting: Set<string>,
	depth = 0,
	transparentSymbols: Set<string> = new Set<string>([' ']),
	applySymbolOverride = true
): string {
	if (cache.has(glyphName)) {
		return cache.get(glyphName) ?? '';
	}

	const glyph = glyphMap.get(glyphName);
	if (!glyph) return '';

	const parsed = parseGlyphStructure(glyph.structure ?? '');
	if (depth >= MAX_COMPONENT_DEPTH || visiting.has(glyphName)) {
		return parsed.body;
	}

	visiting.add(glyphName);
	let rows = splitRows(parsed.body);

	for (const component of parsed.components) {
		const componentBody = resolveStructureBody(
			component.name,
			glyphMap,
			cache,
			visiting,
			depth + 1,
			transparentSymbols,
			applySymbolOverride
		);
		rows = applyComponent(rows, componentBody, component, transparentSymbols, applySymbolOverride);
	}

	const resolved = rowsToBody(rows);
	visiting.delete(glyphName);
	cache.set(glyphName, resolved);
	return resolved;
}

function resolveStructureWithMask(
	glyphName: string,
	glyphMap: Map<string, GlyphInput>,
	cache: Map<string, ResolvedGlyphVisualData>,
	visiting: Set<string>,
	depth = 0,
	transparentSymbols: Set<string> = new Set<string>([' ']),
	applySymbolOverride = true
): ResolvedGlyphVisualData {
	if (cache.has(glyphName)) {
		return cache.get(glyphName) ?? { body: '', componentMask: '', componentSources: [] };
	}

	const glyph = glyphMap.get(glyphName);
	if (!glyph) return { body: '', componentMask: '', componentSources: [] };

	const parsed = parseGlyphStructure(glyph.structure ?? '');
	if (depth >= MAX_COMPONENT_DEPTH || visiting.has(glyphName)) {
		const unresolvedRows = splitRows(parsed.body);
		const componentSources = createComponentSourceRowsFromBody(unresolvedRows);
		return {
			body: parsed.body,
			componentMask: rowsToMaskBody(componentSources),
			componentSources
		};
	}

	visiting.add(glyphName);
	let rows = splitRows(parsed.body);
	let componentSources = createComponentSourceRowsFromBody(rows);

	for (const component of parsed.components) {
		const componentResolved = resolveStructureWithMask(
			component.name,
			glyphMap,
			cache,
			visiting,
			depth + 1,
			transparentSymbols,
			applySymbolOverride
		);
		const next = applyComponentWithMask(
			rows,
			componentSources,
			componentResolved.body,
			componentResolved.componentSources,
			component,
			transparentSymbols,
			applySymbolOverride
		);
		rows = next.rows;
		componentSources = next.componentSources;
	}

	const resolvedData: ResolvedGlyphVisualData = {
		body: rowsToBody(rows),
		componentMask: rowsToMaskBody(componentSources),
		componentSources
	};
	visiting.delete(glyphName);
	cache.set(glyphName, resolvedData);
	return resolvedData;
}

export function resolveGlyphStructures(
	glyphs: Array<GlyphInput>,
	options?: ResolveStructureOptions
): Map<string, string> {
	const glyphMap = new Map<string, GlyphInput>();
	for (const glyph of glyphs) {
		if (!glyphMap.has(glyph.name)) {
			glyphMap.set(glyph.name, glyph);
		}
	}

	const cache = new Map<string, string>();
	const transparentSymbols = normalizeTransparentSymbols(options);
	const applySymbolOverride = shouldApplySymbolOverride(options);
	for (const glyph of glyphs) {
		resolveStructureBody(
			glyph.name,
			glyphMap,
			cache,
			new Set<string>(),
			0,
			transparentSymbols,
			applySymbolOverride
		);
	}

	return cache;
}

export function resolveGlyphStructuresWithComponentMask(
	glyphs: Array<GlyphInput>,
	options?: ResolveStructureOptions
): Map<string, ResolvedGlyphVisualData> {
	const glyphMap = new Map<string, GlyphInput>();
	for (const glyph of glyphs) {
		if (!glyphMap.has(glyph.name)) {
			glyphMap.set(glyph.name, glyph);
		}
	}

	const cache = new Map<string, ResolvedGlyphVisualData>();
	const transparentSymbols = normalizeTransparentSymbols(options);
	const applySymbolOverride = shouldApplySymbolOverride(options);
	for (const glyph of glyphs) {
		resolveStructureWithMask(
			glyph.name,
			glyphMap,
			cache,
			new Set<string>(),
			0,
			transparentSymbols,
			applySymbolOverride
		);
	}

	return cache;
}

export function resolveGlyphStructure(
	glyph: GlyphInput,
	glyphs: Array<GlyphInput>,
	resolvedByName?: Map<string, string>,
	options?: ResolveStructureOptions
): string {
	if (resolvedByName?.has(glyph.name)) {
		return resolvedByName.get(glyph.name) ?? '';
	}

	const resolved = resolveGlyphStructures(glyphs, options);
	return resolved.get(glyph.name) ?? parseGlyphStructure(glyph.structure).body;
}

export function resolveGlyphStructureWithComponentMask(
	glyph: GlyphInput,
	glyphs: Array<GlyphInput>,
	resolvedByName?: Map<string, ResolvedGlyphVisualData>,
	options?: ResolveStructureOptions
): ResolvedGlyphVisualData {
	if (resolvedByName?.has(glyph.name)) {
		return resolvedByName.get(glyph.name) ?? { body: '', componentMask: '', componentSources: [] };
	}

	const resolved = resolveGlyphStructuresWithComponentMask(glyphs, options);
	const parsed = parseGlyphStructure(glyph.structure);
	return (
		resolved.get(glyph.name) ?? {
			body: parsed.body,
			componentMask: '',
			componentSources: createComponentSourceRowsFromBody(splitRows(parsed.body))
		}
	);
}

export function getUniqueSymbolsFromGlyphs(
	glyphs: Array<GlyphInput>,
	options?: ResolveStructureOptions
): Array<string> {
	const resolvedByName = resolveGlyphStructures(glyphs, options);
	const symbols = new Set<string>();

	for (const glyph of glyphs) {
		const structure = resolvedByName.get(glyph.name) ?? parseGlyphStructure(glyph.structure).body;
		const chars = Array.from(structure.replace(/\n/g, ''));
		for (const char of chars) {
			symbols.add(char);
		}
	}

	return Array.from(symbols);
}

export function isComponentGlyphName(glyphName: string): boolean {
	return /\.component$/i.test(glyphName.trim());
}

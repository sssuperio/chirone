import type { Syntax, GlyphInput, Rule } from '../types';
import { ShapeKind } from '../types';
import {
	structureToArray,
	type Cell,
	createBox,
	getRule,
	drawPath,
	getGlyphWidth
} from './drawGlyph';
import { calcTransform, applyTransform } from './shapes';
import { getAbsoluteSVGPath, arrayToDirectives, editPathFromDirectives } from './paperToOpentype';
import paper from 'paper';
import opentype from 'opentype.js';
import type { FontMetrics } from './metrics';
import { cellsToUnits, normalizeFontMetrics, unitsPerCell } from './metrics';
import type { FontMetadata } from './metadata';
import { normalizeFontMetadata } from './metadata';
import {
	getAlternateBaseName,
	getLigatureComponentNames,
	getStylisticSetFeature,
	resolveUnicodeNumber
} from './glyphName';
import { parseGlyphStructure, resolveGlyphStructures } from './structure';

//

export async function generateGlyph(
	glyph: GlyphInput,
	syntax: Syntax,
	baseSize = 100,
	widthRatio = 1,
	baseline = 1
): Promise<opentype.Glyph> {
	/**
	 * Paperjs part
	 */

	// Initializing paperjs
	const size = new paper.Size(baseSize, baseSize);
	paper.setup(size);

	// Listing all the paths
	const paths: Array<paper.PathItem> = [];
	const parsedStructure = parseGlyphStructure(glyph.structure);
	const renderStructure = parsedStructure.body;

	// Converting structure to array
	const cells: Array<Cell> = structureToArray(renderStructure);

	// Iterating over cells (saving all the paths)
	for (const c of cells) {
		// Getting rule
		const rule = getRule(syntax, c.symbol);

		// If rule is not void
		if (rule.shape.kind != ShapeKind.Void) {
			const box = createBox(c, baseSize, widthRatio);

			// Calculating sub-boxes
			const w = box.width / syntax.grid.columns;
			const h = box.height / syntax.grid.rows;
			const x = (col: number) => box.x + col * w;
			const y = (row: number) => box.y + row * h;

			for (let row = 0; row < syntax.grid.rows; row++) {
				for (let col = 0; col < syntax.grid.columns; col++) {
					const subBox = new paper.Rectangle({
						x: x(col),
						y: y(row),
						width: w,
						height: h
					});
					const subPaths = await drawPath(subBox, rule);
					const transform = calcTransform(rule.shape.props);
					for (const p of subPaths) {
						applyTransform(p, transform, box.center);
					}
					// // Reorienting paths
					// for (const p of boxPaths) {
					// 	p.reorient(true, true);
					// }
					paths.push(...subPaths);
				}
			}
		}
	}

	/**
	 * Opentype.js part
	 */

	// Initializing path
	const oPath = new opentype.Path();

	// Converting paths to opentype
	for (const p of paths) {
		const svgpath = getAbsoluteSVGPath(p);
		const directives = arrayToDirectives(svgpath);
		editPathFromDirectives(oPath, directives, -baseSize * baseline);
	}

	// Adding glyph metadata
	const name = glyph.name;
	const advanceWidth = getGlyphWidth(renderStructure, baseSize * widthRatio);
	const unicode = resolveUnicodeNumber(name);

	// Clearing paperjs
	paper.project.clear();

	const glyphOptions: opentype.GlyphOptions = {
		name,
		advanceWidth,
		path: oPath
	};

	if (unicode !== undefined) {
		glyphOptions.unicode = unicode;
	}

	return new opentype.Glyph(glyphOptions);
}

//

export type FontMetricsKeys = keyof FontMetrics;
export type { FontMetrics };
export type { FontMetadata };

function toPostScriptName(value: string): string {
	const normalized = value
		.normalize('NFKD')
		.replace(/[^\x20-\x7E]/g, '')
		.replace(/\s+/g, '')
		.replace(/[^a-zA-Z0-9-]/g, '');
	return normalized || 'GTL-Regular';
}

function isoDateToUnixTimestamp(dateValue: string): number | undefined {
	const [yearString, monthString, dayString] = dateValue.split('-');
	const year = Number(yearString);
	const month = Number(monthString);
	const day = Number(dayString);
	if (!Number.isFinite(year) || !Number.isFinite(month) || !Number.isFinite(day)) return undefined;
	if (month < 1 || month > 12 || day < 1 || day > 31) return undefined;
	const utcMillis = Date.UTC(year, month - 1, day);
	if (!Number.isFinite(utcMillis)) return undefined;
	return Math.floor(utcMillis / 1000);
}

function orderGlyphInputs(glyphs: Array<GlyphInput>, glyphOrder: string): Array<GlyphInput> {
	const orderTokens = glyphOrder
		.split(/[\s,]+/)
		.map((token) => token.trim())
		.filter((token) => token.length > 0);

	if (orderTokens.length === 0) return glyphs;

	const glyphByName = new Map<string, GlyphInput>();
	for (const glyph of glyphs) {
		if (!glyphByName.has(glyph.name)) {
			glyphByName.set(glyph.name, glyph);
		}
	}

	const consumedNames = new Set<string>();
	const ordered: Array<GlyphInput> = [];

	for (const name of orderTokens) {
		const glyph = glyphByName.get(name);
		if (!glyph || consumedNames.has(name)) continue;
		ordered.push(glyph);
		consumedNames.add(name);
	}

	for (const glyph of glyphs) {
		if (!consumedNames.has(glyph.name)) {
			ordered.push(glyph);
		}
	}

	return ordered;
}

function toOs2VendorID(value: string): string {
	const normalized = value
		.toUpperCase()
		.replace(/[^A-Z0-9]/g, '')
		.slice(0, 4);
	return normalized.padEnd(4, 'X');
}

//

export async function generateFont(
	syntax: Syntax,
	glyphs: Array<GlyphInput>,
	metrics: FontMetrics,
	metadata?: FontMetadata
): Promise<opentype.Font> {
	const normalizedMetrics = normalizeFontMetrics(metrics as any);
	const normalizedMetadata = normalizeFontMetadata(metadata as any);
	const fallbackGlyphOrder = glyphs.map((glyph) => glyph.name).join(' ');
	const effectiveGlyphOrder = normalizedMetadata.glyphOrder || fallbackGlyphOrder;
	const orderedGlyphs = orderGlyphInputs(glyphs, effectiveGlyphOrder);
	const baseSquare = unitsPerCell(normalizedMetrics);
	const ascenderUnits = cellsToUnits(normalizedMetrics, normalizedMetrics.ascender);
	const descenderUnits = -cellsToUnits(normalizedMetrics, normalizedMetrics.descender);
	const capHeightUnits = cellsToUnits(normalizedMetrics, normalizedMetrics.capHeight);
	const xHeightUnits = cellsToUnits(normalizedMetrics, normalizedMetrics.xHeight);

	// Listing all the glyphs
	const opentypeGlyphs = [];

	// Adding Notdef - is required
	const notdefGlyph = new opentype.Glyph({
		name: '.notdef',
		unicode: 0,
		advanceWidth: baseSquare * 4,
		path: new opentype.Path()
	});
	opentypeGlyphs.push(notdefGlyph);

	const transparentSymbols = new Set<string>([' ']);
	for (const rule of syntax.rules) {
		if (rule.shape.kind === ShapeKind.Void && rule.symbol) {
			transparentSymbols.add(rule.symbol);
		}
	}

	const rulesBySymbol: Record<string, Rule> = {};
	for (const rule of syntax.rules) {
		if (rule.symbol?.length === 1 && !rulesBySymbol[rule.symbol]) {
			rulesBySymbol[rule.symbol] = rule;
		}
	}

	const resolvedGlyphStructures = resolveGlyphStructures(orderedGlyphs, {
		transparentSymbols,
		rulesBySymbol
	});
	const resolvedGlyphVisualStructures = resolveGlyphStructures(orderedGlyphs, {
		transparentSymbols,
		applySymbolOverride: false,
		rulesBySymbol
	});

	for (const g of orderedGlyphs) {
		const resolvedStructure =
			resolvedGlyphVisualStructures.get(g.name) ??
			resolvedGlyphStructures.get(g.name) ??
			parseGlyphStructure(g.structure).body;
		opentypeGlyphs.push(
			await generateGlyph(
				{
					...g,
					structure: resolvedStructure
				},
				syntax,
				baseSquare,
				1,
				normalizedMetrics.descender
			)
		);
	}

	// Ensure stable glyph indexes and keep a direct name->index map.
	// We cannot rely on font.nameToGlyphIndex() at this stage because glyphNames
	// may not be initialized yet in freshly-constructed fonts.
	const glyphIndexByName = new Map<string, number>();
	for (let i = 0; i < opentypeGlyphs.length; i++) {
		const glyph = opentypeGlyphs[i] as opentype.Glyph & { index?: number };
		glyph.index = i;
		const glyphName = glyph.name ?? '';
		if (glyphName && !glyphIndexByName.has(glyphName)) {
			glyphIndexByName.set(glyphName, i);
		}
	}

	// Creating font
	const familyName = normalizedMetadata.familyName || 'GTL';
	const styleName = syntax.name?.trim() || 'Regular';
	const fullName = normalizedMetadata.name || `${familyName} ${styleName}`.trim();
	const createdTimestamp = isoDateToUnixTimestamp(normalizedMetadata.createdDate);
	const fontOptions: any = {
		familyName,
		styleName,
		fullName,
		postScriptName: toPostScriptName(`${familyName}-${styleName}`),
		unitsPerEm: normalizedMetrics.UPM,
		ascender: ascenderUnits,
		descender: descenderUnits,
		glyphs: opentypeGlyphs
	};
	if (normalizedMetadata.designer) fontOptions.designer = normalizedMetadata.designer;
	if (normalizedMetadata.designerURL) fontOptions.designerURL = normalizedMetadata.designerURL;
	if (normalizedMetadata.manufacturer) fontOptions.manufacturer = normalizedMetadata.manufacturer;
	if (normalizedMetadata.manufacturerURL) {
		fontOptions.manufacturerURL = normalizedMetadata.manufacturerURL;
	}
	if (normalizedMetadata.license) fontOptions.license = normalizedMetadata.license;
	if (normalizedMetadata.version) fontOptions.version = normalizedMetadata.version;
	if (createdTimestamp !== undefined) fontOptions.createdTimestamp = createdTimestamp;

	const font = new opentype.Font(fontOptions);

	const os2 = ((font.tables as any).os2 = (font.tables as any).os2 || {});
	os2.sTypoAscender = ascenderUnits;
	os2.sTypoDescender = descenderUnits;
	os2.sTypoLineGap = 0;
	os2.usWinAscent = ascenderUnits;
	os2.usWinDescent = Math.abs(descenderUnits);
	os2.sCapHeight = capHeightUnits;
	os2.sxHeight = xHeightUnits;
	os2.achVendID = toOs2VendorID(normalizedMetadata.vendorID);

	const hhea = ((font.tables as any).hhea = (font.tables as any).hhea || {});
	hhea.ascender = ascenderUnits;
	hhea.descender = descenderUnits;
	hhea.lineGap = 0;

	delete (font.names as any).trademark;
	delete (font.names as any).description;

	// opentype.js 1.3.x can crash on generated fonts when a character is missing
	// (charToGlyphIndex returns null, then GlyphSet.get calls _push()).
	// Normalize missing lookups to .notdef (index 0).
	const baseCharToGlyphIndex = font.charToGlyphIndex.bind(font);
	(font as any).charToGlyphIndex = (value: string): number => {
		const index = baseCharToGlyphIndex(value);
		if (typeof index !== 'number' || Number.isNaN(index) || index < 0) {
			return 0;
		}
		return index;
	};

	applyOpenTypeFeatures(font, orderedGlyphs, glyphIndexByName);

	return font;
}

function applyOpenTypeFeatures(
	font: opentype.Font,
	glyphs: Array<GlyphInput>,
	glyphIndexByName: Map<string, number>
) {
	const substitution = (font as any).substitution as
		| {
				add: (
					feature: string,
					substitution: { sub: number | Array<number>; by: number | Array<number> },
					script?: string,
					language?: string
				) => void;
		  }
		| undefined;
	if (!substitution || typeof substitution.add !== 'function') return;

	const nameToIndex = (name: string): number | undefined => {
		const index = glyphIndexByName.get(name);
		if (index === undefined || index <= 0) return undefined;
		return index;
	};

	const substitutionsByFeature = new Map<
		string,
		Array<{ sub: number | Array<number>; by: number | Array<number> }>
	>();
	const pushSubstitution = (
		feature: string,
		entry: { sub: number | Array<number>; by: number | Array<number> }
	) => {
		if (!substitutionsByFeature.has(feature)) {
			substitutionsByFeature.set(feature, []);
		}
		substitutionsByFeature.get(feature)?.push(entry);
	};

	// Ligatures: glyph names like "f_f", "f_f_i", "f_l".
	const ligatures: Array<{ sub: Array<number>; by: number }> = [];
	for (const glyph of glyphs) {
		const components = getLigatureComponentNames(glyph.name);
		if (components.length < 2) continue;

		const ligatureGlyphIndex = nameToIndex(glyph.name);
		if (!ligatureGlyphIndex) continue;

		const componentIndexes = components.map((component) => nameToIndex(component));
		if (componentIndexes.some((value) => value === undefined)) continue;

		ligatures.push({
			sub: componentIndexes as Array<number>,
			by: ligatureGlyphIndex
		});
	}

	// Prefer longest ligatures first.
	ligatures.sort((a, b) => b.sub.length - a.sub.length);
	for (const ligature of ligatures) {
		pushSubstitution('liga', ligature);
	}

	// Alternates:
	// - stylistic sets from "base.ss01", "base.ss02", ...
	// - general alternates in "salt"/"aalt" using dotted suffix names.
	const alternatesByBase = new Map<number, Set<number>>();
	for (const glyph of glyphs) {
		const baseName = getAlternateBaseName(glyph.name);
		if (!baseName) continue;

		const alternateGlyphIndex = nameToIndex(glyph.name);
		const baseGlyphIndex = nameToIndex(baseName);
		if (!alternateGlyphIndex || !baseGlyphIndex) continue;
		if (alternateGlyphIndex === baseGlyphIndex) continue;

		const stylisticSet = getStylisticSetFeature(glyph.name);
		if (stylisticSet) {
			pushSubstitution(stylisticSet, { sub: baseGlyphIndex, by: alternateGlyphIndex });
		}

		if (!alternatesByBase.has(baseGlyphIndex)) {
			alternatesByBase.set(baseGlyphIndex, new Set<number>());
		}
		alternatesByBase.get(baseGlyphIndex)?.add(alternateGlyphIndex);
	}

	for (const [baseGlyphIndex, alternateSet] of alternatesByBase.entries()) {
		const alternates = Array.from(alternateSet);
		if (!alternates.length) continue;
		pushSubstitution('aalt', { sub: baseGlyphIndex, by: alternates });
		pushSubstitution('salt', { sub: baseGlyphIndex, by: alternates });
	}

	// OpenType layout helper in opentype.js requires feature tags to be added alphabetically.
	const sortedFeatures = Array.from(substitutionsByFeature.keys()).sort();
	for (const feature of sortedFeatures) {
		const entries = substitutionsByFeature.get(feature) ?? [];
		for (const entry of entries) {
			substitution.add(feature, entry, 'DFLT', 'dflt');
		}
	}
}

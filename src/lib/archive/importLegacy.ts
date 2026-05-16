import { nanoid } from 'nanoid';
import { normalizeFontMetrics } from '$lib/GTL/metrics';
import type { FontMetrics } from '$lib/GTL/metrics';
import { normalizeFontMetadata } from '$lib/GTL/metadata';
import type { FontMetadata } from '$lib/GTL/metadata';
import type {
	GlyphInput,
	Syntax,
	MetricsPreset,
	MetadataPreset,
	FontDefinition,
	ProjectInfo
} from '$lib/types';

interface LegacyGTL {
	glyphs?: unknown;
	syntaxes?: unknown;
	metrics?: unknown;
	metadata?: unknown;
}

function metricsToPreset(metrics: FontMetrics, id: string, name: string): MetricsPreset {
	return {
		id,
		name,
		UPM: metrics.UPM,
		height: metrics.height,
		baseline: metrics.baseline,
		descender: metrics.descender,
		ascender: metrics.ascender,
		capHeight: metrics.capHeight,
		xHeight: metrics.xHeight
	};
}

function metadataToPreset(metadata: FontMetadata, id: string, name: string): MetadataPreset {
	return { ...metadata, id, name };
}

interface ParsedLegacy {
	projectInfo: ProjectInfo;
	perFontGlyphs: Map<string, GlyphInput[]>;
	syntaxes: Syntax[];
	metricsPresets: MetricsPreset[];
	metadataPresets: MetadataPreset[];
	fontDefinitions: FontDefinition[];
}

export function parseLegacyProjectFile(jsonText: string): ParsedLegacy {
	let raw: LegacyGTL;
	try {
		raw = JSON.parse(jsonText);
	} catch {
		throw new Error('Invalid JSON file');
	}

	if (!raw || typeof raw !== 'object' || Array.isArray(raw)) {
		throw new Error('Invalid project format: expected object');
	}

	const glyphs: GlyphInput[] = Array.isArray(raw.glyphs) ? (raw.glyphs as GlyphInput[]) : [];
	const syntaxes: Syntax[] = Array.isArray(raw.syntaxes) ? (raw.syntaxes as Syntax[]) : [];
	const parsedMetrics: FontMetrics = raw.metrics
		? normalizeFontMetrics(raw.metrics as Partial<FontMetrics> | undefined)
		: normalizeFontMetrics({});
	const parsedMetadata: FontMetadata = raw.metadata
		? normalizeFontMetadata(raw.metadata as Partial<FontMetadata> | undefined)
		: normalizeFontMetadata({});

	const now = new Date().toISOString();
	const firstSyntaxId = syntaxes.length > 0 ? syntaxes[0].id : '';

	const metricsPreset = metricsToPreset(parsedMetrics, 'default', 'Default');
	const metadataPreset = metadataToPreset(parsedMetadata, 'default', 'Default');
	const familyName = parsedMetadata.familyName || 'GTL';

	const fontDef: FontDefinition = {
		id: nanoid(),
		name: 'Regular',
		syntaxId: firstSyntaxId,
		metricsId: metricsPreset.id,
		metadataId: metadataPreset.id,
		outputName: `${familyName}-Regular.otf`,
		enabled: true
	};

	const fontGlyphMap = new Map<string, GlyphInput[]>();
	fontGlyphMap.set(fontDef.id, glyphs);

	return {
		projectInfo: {
			id: nanoid(),
			name: 'Imported Project',
			createdAt: now,
			updatedAt: now
		},
		perFontGlyphs: fontGlyphMap,
		syntaxes,
		metricsPresets: [metricsPreset],
		metadataPresets: [metadataPreset],
		fontDefinitions: [fontDef]
	};
}

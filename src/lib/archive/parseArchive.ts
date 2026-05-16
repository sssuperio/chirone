import { parseTar } from './tar';
import type {
	GlyphInput,
	Syntax,
	MetricsPreset,
	MetadataPreset,
	FontDefinition,
	ProjectInfo
} from '$lib/types';
import { normalizeFontMetrics } from '$lib/GTL/metrics';
import { normalizeFontMetadata } from '$lib/GTL/metadata';

interface ParsedArchive {
	projectInfo: ProjectInfo;
	perFontGlyphs: Map<string, GlyphInput[]>;
	syntaxes: Syntax[];
	metricsPresets: MetricsPreset[];
	metadataPresets: MetadataPreset[];
	fontDefinitions: FontDefinition[];
}

interface ManifestFile {
	format: string;
	formatVersion: number;
	project?: { name?: string };
	paths?: {
		glyphs?: string[];
		syntaxes?: string[];
		metrics?: string[];
		metadata?: string[];
		fonts?: string[];
	};
}

const decoder = new TextDecoder();

function parseJSONEntry(content: Uint8Array): unknown {
	try {
		return JSON.parse(decoder.decode(content));
	} catch {
		return null;
	}
}

function validateManifest(manifest: unknown): ManifestFile | null {
	if (!manifest || typeof manifest !== 'object') return null;
	const m = manifest as Record<string, unknown>;
	if (m.format !== 'chirone.project') return null;
	if (typeof m.formatVersion !== 'number' || m.formatVersion < 1) return null;
	return manifest as ManifestFile;
}

function coerceGlyph(raw: unknown): GlyphInput | null {
	if (!raw || typeof raw !== 'object') return null;
	const g = raw as Record<string, unknown>;
	if (typeof g.id !== 'string' || !g.id.trim()) return null;
	if (typeof g.name !== 'string') return null;
	if (typeof g.structure !== 'string') return null;
	return raw as GlyphInput;
}

function coerceSyntax(raw: unknown): Syntax | null {
	if (!raw || typeof raw !== 'object') return null;
	const s = raw as Record<string, unknown>;
	if (typeof s.id !== 'string' || !s.id.trim()) return null;
	return raw as Syntax;
}

function coerceMetricsPreset(raw: unknown): MetricsPreset | null {
	if (!raw || typeof raw !== 'object') return null;
	const m = raw as Record<string, unknown>;
	if (typeof m.id !== 'string' || !m.id.trim()) return null;
	const normalized = normalizeFontMetrics({
		UPM: m.UPM as number | undefined,
		height: m.height as number | undefined,
		baseline: m.baseline as number | undefined,
		descender: m.descender as number | undefined,
		ascender: m.ascender as number | undefined,
		capHeight: m.capHeight as number | undefined,
		xHeight: m.xHeight
	});
	return {
		...normalized,
		id: m.id as string,
		name: typeof m.name === 'string' ? m.name : (m.id as string)
	};
}

function coerceMetadataPreset(raw: unknown): MetadataPreset | null {
	if (!raw || typeof raw !== 'object') return null;
	const m = raw as Record<string, unknown>;
	if (typeof m.id !== 'string' || !m.id.trim()) return null;
	const normalized = normalizeFontMetadata(raw as Partial<FontMetadata> | undefined);
	return {
		...normalized,
		id: m.id as string,
		name: typeof m.name === 'string' ? m.name : (m.id as string)
	};
}

function coerceFontDefinition(raw: unknown): FontDefinition | null {
	if (!raw || typeof raw !== 'object') return null;
	const f = raw as Record<string, unknown>;
	if (typeof f.id !== 'string' || !f.id.trim()) return null;
	return {
		id: f.id as string,
		name: typeof f.name === 'string' ? f.name : 'Regular',
		syntaxId: typeof f.syntaxId === 'string' ? f.syntaxId : '',
		metricsId: typeof f.metricsId === 'string' ? f.metricsId : '',
		metadataId: typeof f.metadataId === 'string' ? f.metadataId : '',
		outputName: typeof f.outputName === 'string' ? f.outputName : '',
		enabled: f.enabled !== false
	};
}

export async function parseProjectArchive(file: File): Promise<ParsedArchive> {
	const compressed = await file.arrayBuffer();
	const decompressedStream = new Blob([compressed])
		.stream()
		.pipeThrough(new DecompressionStream('gzip'));
	const decompressed = await new Response(decompressedStream).arrayBuffer();
	const entries = parseTar(new Uint8Array(decompressed));

	const filesByPath = new Map<string, Uint8Array>();
	for (const entry of entries) {
		filesByPath.set(entry.name, entry.content);
	}

	// Parse manifest first
	const manifestRaw = filesByPath.get('chirone.json');
	if (!manifestRaw) throw new Error('Missing chirone.json manifest');

	const manifestParsed = parseJSONEntry(manifestRaw);
	const manifest = validateManifest(manifestParsed);
	if (!manifest) throw new Error('Invalid chirone.json manifest');

	// Parse glyphs grouped by fontId
	const perFontGlyphs = new Map<string, GlyphInput[]>();
	const glyphPaths = manifest.paths?.glyphs ?? [];
	for (const path of glyphPaths) {
		const content = filesByPath.get(path);
		if (!content) continue;
		const parsed = parseJSONEntry(content);
		const glyph = coerceGlyph(parsed);
		if (!glyph) continue;
		// Path format: glyphs/<fontId>/<glyphId>.json
		const parts = path.split('/');
		const fontId = parts.length >= 3 ? parts[1] : 'legacy';
		if (!perFontGlyphs.has(fontId)) {
			perFontGlyphs.set(fontId, []);
		}
		perFontGlyphs.get(fontId)!.push(glyph);
	}

	// Parse syntaxes
	const syntaxes: Syntax[] = [];
	const syntaxPaths = manifest.paths?.syntaxes ?? [];
	for (const path of syntaxPaths) {
		const content = filesByPath.get(path);
		if (!content) continue;
		const parsed = parseJSONEntry(content);
		const syntax = coerceSyntax(parsed);
		if (syntax) syntaxes.push(syntax);
	}

	// Parse metrics presets
	const metricsPresetsList: MetricsPreset[] = [];
	const metricsPaths = manifest.paths?.metrics ?? [];
	for (const path of metricsPaths) {
		const content = filesByPath.get(path);
		if (!content) continue;
		const parsed = parseJSONEntry(content);
		const preset = coerceMetricsPreset(parsed);
		if (preset) metricsPresetsList.push(preset);
	}

	// Parse metadata presets
	const metadataPresetsList: MetadataPreset[] = [];
	const metadataPaths = manifest.paths?.metadata ?? [];
	for (const path of metadataPaths) {
		const content = filesByPath.get(path);
		if (!content) continue;
		const parsed = parseJSONEntry(content);
		const preset = coerceMetadataPreset(parsed);
		if (preset) metadataPresetsList.push(preset);
	}

	// Parse font definitions
	const fontDefs: FontDefinition[] = [];
	const fontPaths = manifest.paths?.fonts ?? [];
	for (const path of fontPaths) {
		const content = filesByPath.get(path);
		if (!content) continue;
		const parsed = parseJSONEntry(content);
		const font = coerceFontDefinition(parsed);
		if (font) fontDefs.push(font);
	}

	const now = new Date().toISOString();
	const projectName = manifest.project?.name || 'My Project';

	return {
		projectInfo: {
			id: projectName.toLowerCase().replace(/\s+/g, '-'),
			name: projectName,
			createdAt: now,
			updatedAt: now
		},
		perFontGlyphs,
		syntaxes,
		metricsPresets: metricsPresetsList,
		metadataPresets: metadataPresetsList,
		fontDefinitions: fontDefs
	};
}

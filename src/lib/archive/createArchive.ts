import { buildTar } from './tar';
import type {
	GlyphInput,
	Syntax,
	MetricsPreset,
	MetadataPreset,
	FontDefinition,
	ProjectInfo
} from '$lib/types';

const encoder = new TextEncoder();

function toJSONBytes(value: unknown): Uint8Array {
	return encoder.encode(JSON.stringify(value, null, 2));
}

interface ArchivePaths {
	glyphs: string[];
	syntaxes: string[];
	metrics: string[];
	metadata: string[];
	fonts: string[];
}

export async function createProjectArchive(
	perFontGlyphs: Map<string, GlyphInput[]>,
	syntaxes: Syntax[],
	metricsPresetsList: MetricsPreset[],
	metadataPresetsList: MetadataPreset[],
	fontDefs: FontDefinition[],
	info: ProjectInfo
): Promise<Blob> {
	const entries: Array<{ name: string; content: Uint8Array }> = [];
	const paths: ArchivePaths = {
		glyphs: [],
		syntaxes: [],
		metrics: [],
		metadata: [],
		fonts: []
	};

	// Per-font glyphs stored under glyphs/<fontId>/<glyphId>.json
	for (const [fontId, glyphs] of perFontGlyphs) {
		for (const glyph of glyphs) {
			const fileName = `${fontId}/${glyph.id}.json`;
			entries.push({ name: `glyphs/${fileName}`, content: toJSONBytes(glyph) });
			paths.glyphs.push(`glyphs/${fileName}`);
		}
	}

	for (const syntax of syntaxes) {
		const fileName = `${syntax.id}.json`;
		entries.push({ name: `syntaxes/${fileName}`, content: toJSONBytes(syntax) });
		paths.syntaxes.push(`syntaxes/${fileName}`);
	}

	for (const preset of metricsPresetsList) {
		const fileName = `${preset.id}.json`;
		entries.push({ name: `metrics/${fileName}`, content: toJSONBytes(preset) });
		paths.metrics.push(`metrics/${fileName}`);
	}

	for (const preset of metadataPresetsList) {
		const fileName = `${preset.id}.json`;
		entries.push({ name: `metadata/${fileName}`, content: toJSONBytes(preset) });
		paths.metadata.push(`metadata/${fileName}`);
	}

	for (const font of fontDefs) {
		const fileName = `${font.id}.json`;
		entries.push({ name: `fonts/${fileName}`, content: toJSONBytes(font) });
		paths.fonts.push(`fonts/${fileName}`);
	}

	const manifest = {
		format: 'chirone.project',
		formatVersion: 1,
		application: 'chirone',
		project: { name: info.name },
		fonts: Object.fromEntries(fontDefs.map((f) => [f.id, f.name])),
		paths
	};
	entries.push({ name: 'chirone.json', content: toJSONBytes(manifest) });

	entries.sort((a, b) => a.name.localeCompare(b.name));

	const tarBuffer = buildTar(entries);

	const tarStream = new ReadableStream({
		start(controller) {
			controller.enqueue(tarBuffer);
			controller.close();
		}
	});
	const gzipStream = tarStream.pipeThrough(new CompressionStream('gzip'));
	return new Response(gzipStream).blob();
}

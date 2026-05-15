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

interface ArchiveFiles {
	glyphs: Array<{ name: string; id: string }>;
	syntaxes: Array<{ name: string; id: string }>;
	metrics: Array<{ name: string; id: string }>;
	metadata: Array<{ name: string; id: string }>;
	fonts: Array<{ name: string; id: string }>;
}

export async function createProjectArchive(
	glyphs: GlyphInput[],
	syntaxes: Syntax[],
	metricsPresetsList: MetricsPreset[],
	metadataPresetsList: MetadataPreset[],
	fontDefs: FontDefinition[],
	info: ProjectInfo
): Promise<Blob> {
	const fileList: ArchiveFiles = {
		glyphs: [],
		syntaxes: [],
		metrics: [],
		metadata: [],
		fonts: []
	};

	const entries: Array<{ name: string; content: Uint8Array }> = [];

	for (const glyph of glyphs) {
		const fileName = `${glyph.id}.json`;
		entries.push({ name: `glyphs/${fileName}`, content: toJSONBytes(glyph) });
		fileList.glyphs.push({ name: fileName, id: glyph.id });
	}

	for (const syntax of syntaxes) {
		const fileName = `${syntax.id}.json`;
		entries.push({ name: `syntaxes/${fileName}`, content: toJSONBytes(syntax) });
		fileList.syntaxes.push({ name: fileName, id: syntax.id });
	}

	for (const preset of metricsPresetsList) {
		const fileName = `${preset.id}.json`;
		entries.push({ name: `metrics/${fileName}`, content: toJSONBytes(preset) });
		fileList.metrics.push({ name: fileName, id: preset.id });
	}

	for (const preset of metadataPresetsList) {
		const fileName = `${preset.id}.json`;
		entries.push({ name: `metadata/${fileName}`, content: toJSONBytes(preset) });
		fileList.metadata.push({ name: fileName, id: preset.id });
	}

	for (const font of fontDefs) {
		const fileName = `${font.id}.json`;
		entries.push({ name: `fonts/${fileName}`, content: toJSONBytes(font) });
		fileList.fonts.push({ name: fileName, id: font.id });
	}

	const manifest = {
		format: 'chirone.project',
		formatVersion: 1,
		application: 'chirone',
		project: {
			name: info.name
		},
		fonts: Object.fromEntries(fontDefs.map((f) => [f.id, f.name])),
		paths: {
			glyphs: fileList.glyphs.map((f) => `glyphs/${f.name}`),
			syntaxes: fileList.syntaxes.map((f) => `syntaxes/${f.name}`),
			metrics: fileList.metrics.map((f) => `metrics/${f.name}`),
			metadata: fileList.metadata.map((f) => `metadata/${f.name}`),
			fonts: fileList.fonts.map((f) => `fonts/${f.name}`)
		}
	};
	entries.push({ name: 'chirone.json', content: toJSONBytes(manifest) });

	// Sort entries for deterministic output
	entries.sort((a, b) => a.name.localeCompare(b.name));

	const tarBuffer = buildTar(entries);

	// Gzip using CompressionStream
	const tarStream = new ReadableStream({
		start(controller) {
			controller.enqueue(tarBuffer);
			controller.close();
		}
	});
	const gzipStream = tarStream.pipeThrough(new CompressionStream('gzip'));
	return new Response(gzipStream).blob();
}

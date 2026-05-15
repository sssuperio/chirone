import { describe, it, expect } from 'vitest';
import { parseLegacyProjectFile } from './importLegacy';
import { detectArchiveFormat } from './detectFormat';
import { buildTar, parseTar } from './tar';

const encoder = new TextEncoder();
const decoder = new TextDecoder();

describe('tar roundtrip', () => {
	it('roundtrips simple data', () => {
		const entries = [
			{
				name: 'chirone.json',
				content: encoder.encode(JSON.stringify({ format: 'chirone.project', formatVersion: 1 }))
			},
			{
				name: 'glyphs/a.json',
				content: encoder.encode(JSON.stringify({ id: 'a', name: 'A', structure: 'x' }))
			}
		];
		const tar = buildTar(entries);
		const parsed = parseTar(tar);
		expect(parsed).toHaveLength(2);
		expect(parsed[0].name).toBe('chirone.json');
		const decoded = JSON.parse(decoder.decode(parsed[0].content));
		expect(decoded.format).toBe('chirone.project');
	});

	it('handles empty entries', () => {
		const tar = buildTar([]);
		const parsed = parseTar(tar);
		expect(parsed).toHaveLength(0);
	});

	it('handles large content', () => {
		const bigString = 'x'.repeat(5000);
		const entries = [{ name: 'big.json', content: encoder.encode(bigString) }];
		const tar = buildTar(entries);
		const parsed = parseTar(tar);
		expect(parsed).toHaveLength(1);
		expect(decoder.decode(parsed[0].content)).toBe(bigString);
	});
});

describe('archive data integrity', () => {
	it('manifest JSON is valid and parsable from tar', () => {
		const manifest = {
			format: 'chirone.project',
			formatVersion: 1,
			application: 'chirone',
			project: { name: 'Test' },
			fonts: { f1: 'Regular' },
			paths: { glyphs: ['glyphs/a.json'] }
		};

		const entries = [{ name: 'chirone.json', content: encoder.encode(JSON.stringify(manifest)) }];
		const tar = buildTar(entries);
		const parsed = parseTar(tar);

		const decoded = JSON.parse(decoder.decode(parsed[0].content));
		expect(decoded.format).toBe('chirone.project');
		expect(decoded.formatVersion).toBe(1);
		expect(decoded.project.name).toBe('Test');
	});
});

describe('legacy import', () => {
	it('parses legacy GTL.json and creates defaults', () => {
		const json = JSON.stringify({
			glyphs: [{ id: 'g1', name: 'A', structure: 'x' }],
			syntaxes: [{ id: 's1', name: 'Style1', rules: [], grid: { rows: 3, columns: 3 } }],
			metrics: { UPM: 500, height: 4, baseline: 1, descender: 1 },
			metadata: { familyName: 'Legacy', version: '1.0' }
		});

		const result = parseLegacyProjectFile(json);

		expect(result.glyphs).toHaveLength(1);
		expect(result.syntaxes).toHaveLength(1);
		expect(result.metricsPresets).toHaveLength(1);
		expect(result.metricsPresets[0].UPM).toBe(500);
		expect(result.metadataPresets).toHaveLength(1);
		expect(result.metadataPresets[0].familyName).toBe('Legacy');
		expect(result.fontDefinitions).toHaveLength(1);
		expect(result.fontDefinitions[0].name).toBe('Regular');
		expect(result.fontDefinitions[0].syntaxId).toBe('s1');
	});

	it('handles missing data gracefully', () => {
		const json = JSON.stringify({});
		const result = parseLegacyProjectFile(json);
		expect(result.glyphs).toHaveLength(0);
		expect(result.syntaxes).toHaveLength(0);
		expect(result.metricsPresets).toHaveLength(1);
		expect(result.fontDefinitions).toHaveLength(1);
		expect(result.fontDefinitions[0].syntaxId).toBe('');
	});

	it('rejects invalid JSON', () => {
		expect(() => parseLegacyProjectFile('not json')).toThrow();
		expect(() => parseLegacyProjectFile('null')).toThrow();
	});
});

describe('detectArchiveFormat', () => {
	it('detects chirone archive', () => {
		const file = new File([], 'project.chirone.tar.gz');
		expect(detectArchiveFormat(file)).toBe('chirone');
	});

	it('detects legacy JSON', () => {
		const file = new File([], 'GTL.json');
		expect(detectArchiveFormat(file)).toBe('legacy');
	});

	it('returns unknown for other files', () => {
		const file = new File([], 'something.pdf');
		expect(detectArchiveFormat(file)).toBe('unknown');
	});
});

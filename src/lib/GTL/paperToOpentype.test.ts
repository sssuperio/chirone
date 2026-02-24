import { describe, expect, it } from 'vitest';
import { normalizeAbsoluteDirectives } from './paperToOpentype';

describe('normalizeAbsoluteDirectives', () => {
	it('snaps near-horizontal and near-vertical line segments to exact axes', () => {
		const normalized = normalizeAbsoluteDirectives([
			['M', 0, 0],
			['L', 100, 0.00005],
			['L', 100.00005, 100],
			['Z']
		]);

		expect(normalized).toEqual([
			['M', 0, 0],
			['L', 100, 0],
			['L', 100, 100],
			['Z']
		]);
	});

	it('keeps intentionally diagonal line segments unchanged', () => {
		const normalized = normalizeAbsoluteDirectives([
			['M', 0, 0],
			['L', 100, 10]
		]);

		expect(normalized).toEqual([
			['M', 0, 0],
			['L', 100, 10]
		]);
	});

	it('snaps nearly vertical cubic segments to a single x coordinate', () => {
		const normalized = normalizeAbsoluteDirectives([
			['M', 10, 0],
			['C', 10.00005, 2, 9.99996, 8, 9.99997, 10]
		]);

		expect(normalized).toEqual([
			['M', 10, 0],
			['C', 10, 2, 10, 8, 10, 10]
		]);
	});
});

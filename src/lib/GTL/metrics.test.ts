import { describe, expect, it } from 'vitest';
import { normalizeFontMetrics, unitsPerCell } from './metrics';

describe('normalizeFontMetrics', () => {
	it('snaps UPM to a multiple of height so units-per-cell is integer', () => {
		const metrics = normalizeFontMetrics({
			UPM: 2500,
			height: 12,
			descender: 2
		});

		expect(metrics.UPM).toBe(2496);
		expect(unitsPerCell(metrics)).toBe(208);
		expect(Number.isInteger(unitsPerCell(metrics))).toBe(true);
	});

	it('keeps UPM unchanged when it is already divisible by height', () => {
		const metrics = normalizeFontMetrics({
			UPM: 2400,
			height: 12,
			descender: 2
		});

		expect(metrics.UPM).toBe(2400);
		expect(unitsPerCell(metrics)).toBe(200);
	});
});

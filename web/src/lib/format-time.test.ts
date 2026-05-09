import { describe, it, expect } from 'vitest';
import { formatTime } from './format-time';

describe('formatTime', () => {
	it('formats sub-minute as 0:SS', () => {
		expect(formatTime(45_000)).toBe('0:45');
	});

	it('zero-pads seconds', () => {
		expect(formatTime(125_000)).toBe('2:05');
	});

	it('handles zero', () => {
		expect(formatTime(0)).toBe('0:00');
	});

	it('formats exactly one minute', () => {
		expect(formatTime(60_000)).toBe('1:00');
	});

	it('handles large values', () => {
		expect(formatTime(3_661_000)).toBe('61:01');
	});
});

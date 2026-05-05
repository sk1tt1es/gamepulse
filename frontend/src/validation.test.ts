import { describe, it, expect } from 'vitest';
import { formatPhoneDisplay, isValidPhone, normalizePhone } from './validation';

describe('phone validation', () => {
  it('accepts valid E.164 numbers', () => {
    expect(isValidPhone('+14155550123')).toBe(true);
    expect(isValidPhone('+442083661177')).toBe(true);
  });

  it('strips human-friendly formatting', () => {
    expect(normalizePhone('+1 (415) 555-0123')).toBe('+14155550123');
    expect(isValidPhone('+1 415.555.0123')).toBe(true);
  });

  it('rejects malformed numbers', () => {
    expect(isValidPhone('4155550123')).toBe(false); // missing +
    expect(isValidPhone('+0445550123')).toBe(false); // leading 0
    expect(isValidPhone('')).toBe(false);
    expect(isValidPhone('+')).toBe(false);
  });
});

describe('formatPhoneDisplay', () => {
  it('formats partial NANP numbers progressively', () => {
    expect(formatPhoneDisplay('+1')).toBe('+1');
    expect(formatPhoneDisplay('+14')).toBe('+1 4');
    expect(formatPhoneDisplay('+1415')).toBe('+1 415');
    expect(formatPhoneDisplay('+14155')).toBe('+1 415 5');
    expect(formatPhoneDisplay('+1415555')).toBe('+1 415 555');
    expect(formatPhoneDisplay('+141555501')).toBe('+1 415 555 01');
    expect(formatPhoneDisplay('+14155550123')).toBe('+1 415 555 0123');
  });

  it('groups generic international numbers in threes', () => {
    expect(formatPhoneDisplay('+442083661177')).toBe('+442 083 661 177');
  });

  it('returns the input untouched when no leading +', () => {
    expect(formatPhoneDisplay('4155550123')).toBe('4155550123');
  });

  it('does not introduce spaces in the canonical value', () => {
    const formatted = formatPhoneDisplay('+14155550123');
    expect(normalizePhone(formatted)).toBe('+14155550123');
  });
});

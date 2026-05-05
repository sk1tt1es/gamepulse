// Client-side input validation + display helpers. Mirrors the server's
// rules so we can surface helpful errors before issuing a network request.
// The server remains the source of truth — the API still rejects malformed
// input.

export const E164 = /^\+[1-9]\d{1,14}$/;

// normalizePhone strips human-friendly formatting (spaces, parens, dashes,
// dots) so the result is the canonical E.164 string we send to the API.
export function normalizePhone(input: string): string {
  return input.replace(/[\s().\-]/g, '');
}

export function isValidPhone(input: string): boolean {
  return E164.test(normalizePhone(input));
}

// formatPhoneDisplay produces a visually-grouped representation of an
// E.164 phone number. For NANP numbers (country code 1) we use the
// familiar "+1 NNN NNN NNNN" shape; for other countries we space digits
// in groups of three after the country code. Pure function — never mutates
// the underlying canonical value.
export function formatPhoneDisplay(raw: string): string {
  const cleaned = normalizePhone(raw);
  if (!cleaned.startsWith('+')) return cleaned;

  const digits = cleaned.slice(1).replace(/\D/g, '');
  if (digits.length === 0) return '+';

  // NANP (+1): "+1 AAA BBB CCCC".
  if (digits.startsWith('1')) {
    const rest = digits.slice(1);
    let out = '+1';
    if (rest.length > 0) out += ' ' + rest.slice(0, 3);
    if (rest.length > 3) out += ' ' + rest.slice(3, 6);
    if (rest.length > 6) out += ' ' + rest.slice(6, 10);
    if (rest.length > 10) out += ' ' + rest.slice(10);
    return out;
  }

  // Generic international: "+CC GGG GGG …" — we don't try to know which
  // leading digits form the country code, so we just space groups of 3.
  const groups: string[] = [];
  for (let i = 0; i < digits.length; i += 3) groups.push(digits.slice(i, i + 3));
  return '+' + groups.join(' ');
}

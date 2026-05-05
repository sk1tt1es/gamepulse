import { useEffect, useRef } from 'react';
import { formatPhoneDisplay, isValidPhone, normalizePhone } from '../validation';

interface Props {
  /** Canonical E.164 value (no spaces). Parents store this. */
  value: string;
  /** Called with the canonical E.164 value (no spaces). */
  onChange: (value: string) => void;
  disabled?: boolean;
  showError?: boolean;
}

// PhoneInput is a controlled input that **displays** a formatted phone
// number ("+1 415 555 0123") while the parent sees the canonical E.164
// string ("+14155550123"). The formatting is purely visual — no spaces
// ever leave the component via onChange.
//
// We track the input element with a ref so we can keep the cursor in a
// reasonable place across re-renders triggered by formatting.
export function PhoneInput({ value, onChange, disabled, showError }: Props) {
  const ref = useRef<HTMLInputElement>(null);
  // Track the cursor position the user expects after their last keystroke,
  // expressed as a digit-count from the left of the canonical string. We
  // re-map that to a character index in the formatted display after each
  // render so the caret doesn't snap to the end on every re-format.
  const desiredDigitCursor = useRef<number | null>(null);

  const display = formatPhoneDisplay(value);
  const invalid = showError && value !== '' && !isValidPhone(value);

  function handleChange(e: React.ChangeEvent<HTMLInputElement>) {
    const el = e.currentTarget;
    const rawInput = el.value;
    const selectionStart = el.selectionStart ?? rawInput.length;
    // Count digits before the cursor in the raw input — that's the user's
    // intended position in the canonical (digits-only) representation.
    const digitsBefore = rawInput
      .slice(0, selectionStart)
      .split('')
      .filter((c) => /\d/.test(c)).length;

    let canonical = normalizePhone(rawInput);
    // Drop any embedded "+" beyond the leading one to keep E.164 clean.
    const hasPlus = canonical.startsWith('+');
    canonical = (hasPlus ? '+' : '') + canonical.replace(/\+/g, '');
    // If the user is typing digits with no leading "+", auto-insert it.
    if (!hasPlus && canonical.length > 0) canonical = '+' + canonical;

    desiredDigitCursor.current = digitsBefore;
    onChange(canonical);
  }

  // After every render, re-position the cursor at "digitsBefore"-th digit
  // in the formatted display. This keeps in-place edits feeling natural.
  useEffect(() => {
    const el = ref.current;
    if (!el || desiredDigitCursor.current === null) return;
    const target = desiredDigitCursor.current;
    desiredDigitCursor.current = null;

    let seenDigits = 0;
    let pos = 0;
    for (; pos < display.length; pos++) {
      if (/\d/.test(display[pos])) {
        if (seenDigits === target) break;
        seenDigits++;
      }
    }
    // Place caret AFTER the target-th digit (or at end if we ran out).
    if (seenDigits < target) pos = display.length;
    else {
      // Skip past any trailing space following the digit so typing
      // continues at the start of the next group rather than just after
      // the digit we typed.
      while (pos < display.length && display[pos] === ' ' && seenDigits === target - 1) pos++;
    }
    el.setSelectionRange(pos, pos);
  }, [display]);

  return (
    <label className="field">
      <span className="field__label">Phone number</span>
      <input
        ref={ref}
        type="tel"
        inputMode="tel"
        aria-label="Phone number"
        placeholder="+1 415 555 0123"
        value={display}
        onChange={handleChange}
        disabled={disabled}
        autoComplete="tel"
        required
        aria-invalid={invalid || undefined}
      />
      <span className="field__hint">
        Include your country code, e.g. <code>+1 415 555 0123</code>.
      </span>
      {invalid && (
        <span role="alert" className="field__error">
          Enter a valid phone number in E.164 format.
        </span>
      )}
    </label>
  );
}

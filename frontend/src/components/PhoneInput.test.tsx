import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { useState } from 'react';
import { describe, expect, it } from 'vitest';
import { PhoneInput } from './PhoneInput';

// Harness so `userEvent.type` sees real state updates instead of a
// frozen prop value. The output element exposes the canonical value for
// assertions, mimicking what a real parent would store.
function Harness({ initial = '' }: { initial?: string }) {
  const [val, setVal] = useState(initial);
  return (
    <>
      <PhoneInput value={val} onChange={setVal} showError />
      <output data-testid="canonical">{val}</output>
    </>
  );
}

describe('PhoneInput', () => {
  it('shows formatted display while keeping canonical value clean', async () => {
    render(<Harness />);
    const input = screen.getByLabelText('Phone number') as HTMLInputElement;

    await userEvent.type(input, '14155550123');

    expect(input.value).toBe('+1 415 555 0123');
    expect(screen.getByTestId('canonical')).toHaveTextContent('+14155550123');
  });

  it('auto-prepends + when the user types digits', async () => {
    render(<Harness />);
    const input = screen.getByLabelText('Phone number') as HTMLInputElement;

    await userEvent.type(input, '1415');

    expect(input.value).toBe('+1 415');
    expect(screen.getByTestId('canonical')).toHaveTextContent('+1415');
  });

  it('shows an error when invalid and touched', () => {
    // Leading-zero country codes fail E.164 — used here to trigger the
    // error path without depending on length-specific rules.
    render(<PhoneInput value="+0123" onChange={() => {}} showError />);
    expect(screen.getByRole('alert')).toHaveTextContent(/E\.164/i);
  });

  it('hides the error when the canonical value is valid', () => {
    render(<PhoneInput value="+14155550123" onChange={() => {}} showError />);
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
  });
});

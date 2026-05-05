import '@testing-library/jest-dom/vitest';
import { afterEach, vi } from 'vitest';
import { cleanup } from '@testing-library/react';

// jsdom doesn't implement window.scrollTo. Stub it before any component
// effect runs to avoid noisy warnings in test output.
if (typeof window !== 'undefined' && !('scrollTo' in window)) {
  // @ts-expect-error — assigning to a missing API on purpose
  window.scrollTo = () => {};
} else if (typeof window !== 'undefined') {
  window.scrollTo = vi.fn() as unknown as typeof window.scrollTo;
}

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

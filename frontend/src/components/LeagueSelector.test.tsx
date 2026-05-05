import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';
import { LeagueSelector } from './LeagueSelector';

describe('LeagueSelector', () => {
  it('renders all four leagues with sport taglines', () => {
    render(<LeagueSelector value="" onChange={() => {}} />);
    for (const id of ['NBA', 'NFL', 'MLB', 'NHL'] as const) {
      expect(screen.getByTestId(`league-${id}`)).toBeInTheDocument();
    }
    expect(screen.getByText('Basketball')).toBeInTheDocument();
    expect(screen.getByText('Football')).toBeInTheDocument();
    expect(screen.getByText('Baseball')).toBeInTheDocument();
    expect(screen.getByText('Hockey')).toBeInTheDocument();
  });

  it('calls onChange when a league is clicked', async () => {
    const onChange = vi.fn();
    render(<LeagueSelector value="" onChange={onChange} />);
    await userEvent.click(screen.getByTestId('league-NHL'));
    expect(onChange).toHaveBeenCalledWith('NHL');
  });

  it('marks the selected league with the selected modifier', () => {
    render(<LeagueSelector value="NBA" onChange={() => {}} />);
    expect(screen.getByTestId('league-NBA').className).toContain('league-card--selected');
    expect(screen.getByTestId('league-NFL').className).not.toContain('league-card--selected');
  });
});

import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';
import { TeamSelector } from './TeamSelector';
import type { Team } from '../api/client';

const teams: Team[] = [
  {
    id: 'lal',
    name: 'Los Angeles Lakers',
    league: 'NBA',
    external_id: 'LAL',
    logo_url: 'https://a.espncdn.com/i/teamlogos/nba/500/lal.png',
  },
  {
    id: 'bos',
    name: 'Boston Celtics',
    league: 'NBA',
    external_id: 'BOS',
    logo_url: 'https://a.espncdn.com/i/teamlogos/nba/500/bos.png',
  },
];

describe('TeamSelector', () => {
  it('renders one card per team with logos', () => {
    render(<TeamSelector teams={teams} value="" onChange={() => {}} />);
    expect(screen.getByText('Los Angeles Lakers')).toBeInTheDocument();
    expect(screen.getByText('Boston Celtics')).toBeInTheDocument();
    expect(screen.getByAltText('Los Angeles Lakers logo')).toHaveAttribute('src', teams[0].logo_url);
  });

  it('emits onChange when a team is clicked', async () => {
    const onChange = vi.fn();
    render(<TeamSelector teams={teams} value="" onChange={onChange} />);
    await userEvent.click(screen.getByTestId('team-lal'));
    expect(onChange).toHaveBeenCalledWith('lal');
  });

  it('shows an empty-state message when no teams', () => {
    render(<TeamSelector teams={[]} value="" onChange={() => {}} />);
    expect(screen.getByRole('status')).toHaveTextContent(/no teams/i);
  });

  it('falls back to initials chip when logo image errors', () => {
    const broken: Team[] = [
      { id: 'x', name: 'Some Team', league: 'NBA', external_id: 'X', logo_url: '' },
    ];
    render(<TeamSelector teams={broken} value="" onChange={() => {}} />);
    // Empty logo_url → fallback chip with initials.
    expect(screen.getByText('ST')).toBeInTheDocument();
  });
});

import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { describe, expect, it, vi, beforeEach } from 'vitest';
import App from './App';
import type { LeagueGroup, SubscriptionResponse } from './api/client';

// We mock the API module so the tests focus on UI behaviour and never
// touch the network. Each test installs a fresh mock setup.
vi.mock('./api/client', async () => {
  const actual = await vi.importActual<typeof import('./api/client')>('./api/client');
  return {
    ...actual,
    fetchTeams: vi.fn(),
    createSubscription: vi.fn(),
  };
});

import { fetchTeams, createSubscription, ApiError } from './api/client';

const mockedFetchTeams = vi.mocked(fetchTeams);
const mockedCreateSub = vi.mocked(createSubscription);

const teams: LeagueGroup[] = [
  {
    league: 'NBA',
    teams: [
      { id: 'lal', name: 'Lakers', league: 'NBA', external_id: 'LAL', logo_url: '' },
      { id: 'bos', name: 'Celtics', league: 'NBA', external_id: 'BOS', logo_url: '' },
    ],
  },
  {
    league: 'NFL',
    teams: [{ id: 'kc', name: 'Chiefs', league: 'NFL', external_id: 'KC', logo_url: '' }],
  },
  { league: 'MLB', teams: [] },
  { league: 'NHL', teams: [] },
];

function renderApp() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <App />
    </QueryClientProvider>,
  );
}

describe('App subscription flow', () => {
  beforeEach(() => {
    mockedFetchTeams.mockReset();
    mockedCreateSub.mockReset();
  });

  it('renders headline and league picker after teams load', async () => {
    mockedFetchTeams.mockResolvedValue(teams);
    renderApp();
    expect(screen.getByText(/Live sports updates, by text/i)).toBeInTheDocument();
    await waitFor(() => expect(screen.getByTestId('league-NBA')).toBeInTheDocument());
    // Team picker hidden until a league is chosen.
    expect(screen.queryByText('Pick a team')).not.toBeInTheDocument();
  });

  it('reveals the team picker after a league is selected', async () => {
    mockedFetchTeams.mockResolvedValue(teams);
    renderApp();
    await waitFor(() => expect(screen.getByTestId('league-NBA')).toBeInTheDocument());

    await userEvent.click(screen.getByTestId('league-NBA'));
    expect(screen.getByText('Pick a team')).toBeInTheDocument();
    expect(screen.getByTestId('team-lal')).toBeInTheDocument();
    expect(screen.getByTestId('team-bos')).toBeInTheDocument();
    // NFL teams are filtered out.
    expect(screen.queryByTestId('team-kc')).not.toBeInTheDocument();
  });

  it('clears the team selection when the league changes', async () => {
    mockedFetchTeams.mockResolvedValue(teams);
    renderApp();
    await waitFor(() => expect(screen.getByTestId('league-NBA')).toBeInTheDocument());

    await userEvent.click(screen.getByTestId('league-NBA'));
    await userEvent.click(screen.getByTestId('team-lal'));
    expect(screen.getByTestId('team-lal').className).toContain('team-card--selected');

    await userEvent.click(screen.getByTestId('league-NFL'));
    expect(screen.getByTestId('team-kc').className).not.toContain('team-card--selected');
  });

  it('disables submit until league + team + valid phone + consent are set', async () => {
    mockedFetchTeams.mockResolvedValue(teams);
    renderApp();
    await waitFor(() => expect(screen.getByTestId('league-NBA')).toBeInTheDocument());

    const submit = screen.getByTestId('submit') as HTMLButtonElement;
    expect(submit).toBeDisabled();

    await userEvent.click(screen.getByTestId('league-NBA'));
    expect(submit).toBeDisabled();

    await userEvent.click(screen.getByTestId('team-lal'));
    expect(submit).toBeDisabled(); // still no phone

    await userEvent.type(screen.getByLabelText('Phone number'), '14155550123');
    expect(submit).toBeDisabled(); // still no consent

    await userEvent.click(screen.getByTestId('consent-checkbox'));
    expect(submit).toBeEnabled();
  });

  it('hides the frequency selector for live-only subscriptions', async () => {
    mockedFetchTeams.mockResolvedValue(teams);
    renderApp();
    await waitFor(() => expect(screen.getByTestId('league-NBA')).toBeInTheDocument());

    // Default state shows frequency (because update_type defaults to 'both').
    expect(screen.getByTestId('frequency-daily')).toBeInTheDocument();

    await userEvent.click(screen.getByTestId('update-type-live'));
    expect(screen.queryByTestId('frequency-daily')).not.toBeInTheDocument();

    await userEvent.click(screen.getByTestId('update-type-news'));
    expect(screen.getByTestId('frequency-daily')).toBeInTheDocument();
  });

  it('uses daily/weekly/monthly options', async () => {
    mockedFetchTeams.mockResolvedValue(teams);
    renderApp();
    await waitFor(() => expect(screen.getByTestId('league-NBA')).toBeInTheDocument());

    expect(screen.getByTestId('frequency-daily')).toBeInTheDocument();
    expect(screen.getByTestId('frequency-weekly')).toBeInTheDocument();
    expect(screen.getByTestId('frequency-monthly')).toBeInTheDocument();
    // Old values must not appear.
    expect(screen.queryByTestId('frequency-realtime')).not.toBeInTheDocument();
    expect(screen.queryByTestId('frequency-hourly')).not.toBeInTheDocument();
  });

  it('submits canonical payload and shows confirmation', async () => {
    mockedFetchTeams.mockResolvedValue(teams);
    const response: SubscriptionResponse = {
      subscription: {
        id: 's1',
        user_id: 'u1',
        team_id: 'lal',
        update_type: 'both',
        frequency: 'weekly',
        created_at: new Date().toISOString(),
      },
      team: { id: 'lal', name: 'Lakers', league: 'NBA', external_id: 'LAL', logo_url: '' },
      message: 'ok',
    };
    mockedCreateSub.mockResolvedValue(response);

    renderApp();
    await waitFor(() => expect(screen.getByTestId('league-NBA')).toBeInTheDocument());

    await userEvent.click(screen.getByTestId('league-NBA'));
    await userEvent.click(screen.getByTestId('team-lal'));
    await userEvent.type(screen.getByLabelText('Phone number'), '14155550123');
    await userEvent.click(screen.getByTestId('frequency-weekly'));
    await userEvent.click(screen.getByTestId('consent-checkbox'));
    await userEvent.click(screen.getByTestId('submit'));

    await waitFor(() => expect(screen.getByText(/You're subscribed!/i)).toBeInTheDocument());
    expect(mockedCreateSub.mock.calls[0]?.[0]).toEqual({
      team_id: 'lal',
      phone_number: '+14155550123',
      update_type: 'both',
      frequency: 'weekly',
    });
  });

  it('renders an error when the API rejects the request', async () => {
    mockedFetchTeams.mockResolvedValue(teams);
    mockedCreateSub.mockRejectedValue(
      new ApiError('subscription already exists for this team', 409),
    );

    renderApp();
    await waitFor(() => expect(screen.getByTestId('league-NBA')).toBeInTheDocument());

    await userEvent.click(screen.getByTestId('league-NBA'));
    await userEvent.click(screen.getByTestId('team-lal'));
    await userEvent.type(screen.getByLabelText('Phone number'), '14155550123');
    await userEvent.click(screen.getByTestId('consent-checkbox'));
    await userEvent.click(screen.getByTestId('submit'));

    await waitFor(() => expect(screen.getByRole('alert')).toHaveTextContent(/already exists/i));
  });
});

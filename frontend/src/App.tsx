import { useEffect, useMemo, useState } from 'react';
import { useMutation, useQuery } from '@tanstack/react-query';
import {
  ApiError,
  createSubscription,
  fetchTeams,
  type Frequency,
  type League,
  type SubscriptionResponse,
  type UpdateType,
} from './api/client';
import { LeagueSelector } from './components/LeagueSelector';
import { TeamSelector } from './components/TeamSelector';
import { PhoneInput } from './components/PhoneInput';
import { UpdateTypeSelector } from './components/UpdateTypeSelector';
import { FrequencySelector } from './components/FrequencySelector';
import { ConfirmationMessage } from './components/ConfirmationMessage';
import { ConsentCheckbox } from './components/ConsentCheckbox';
import { PrivacyPage } from './pages/PrivacyPage';
import { TermsPage } from './pages/TermsPage';
import { isValidPhone } from './validation';
import { useHashRoute } from './useHashRoute';

// App is the single-page subscription flow plus two static legal pages
// (privacy / terms). Routing is handled by useHashRoute() — see that
// module for why we deliberately avoid react-router here.
export default function App() {
  const route = useHashRoute();

  // Scroll to top whenever the hash route changes so the user lands at
  // the start of long legal pages.
  useEffect(() => {
    window.scrollTo({ top: 0 });
  }, [route]);

  if (route === 'privacy') {
    return (
      <div className="page">
        <PrivacyPage />
      </div>
    );
  }
  if (route === 'terms') {
    return (
      <div className="page">
        <TermsPage />
      </div>
    );
  }
  return <SubscribeFlow />;
}

function SubscribeFlow() {
  const teamsQuery = useQuery({ queryKey: ['teams'], queryFn: fetchTeams });

  const [league, setLeague] = useState<League | ''>('');
  const [teamId, setTeamId] = useState('');
  const [phone, setPhone] = useState('');
  const [updateType, setUpdateType] = useState<UpdateType>('both');
  const [frequency, setFrequency] = useState<Frequency>('daily');
  const [consent, setConsent] = useState(false);
  const [touched, setTouched] = useState(false);
  const [confirmation, setConfirmation] = useState<SubscriptionResponse | null>(null);

  const mutation = useMutation({
    mutationFn: createSubscription,
    onSuccess: (res) => setConfirmation(res),
  });

  // Drill into the selected league's team list once teams load.
  const teamsForLeague = useMemo(() => {
    if (!league || !teamsQuery.data) return [];
    return teamsQuery.data.find((g) => g.league === league)?.teams ?? [];
  }, [league, teamsQuery.data]);

  // Frequency only matters when the subscription includes news. For
  // live-only subs we still send a default to satisfy the API contract.
  const showFrequency = updateType === 'news' || updateType === 'both';

  const canSubmit =
    teamId !== '' &&
    league !== '' &&
    isValidPhone(phone) &&
    consent &&
    !mutation.isPending;

  function handleLeague(next: League) {
    setLeague(next);
    setTeamId(''); // reset team when league changes
  }

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setTouched(true);
    if (!canSubmit) return;
    mutation.mutate({
      team_id: teamId,
      phone_number: phone,
      update_type: updateType,
      frequency: showFrequency ? frequency : 'daily',
    });
  }

  function reset() {
    setConfirmation(null);
    setLeague('');
    setTeamId('');
    setPhone('');
    setConsent(false);
    setTouched(false);
    mutation.reset();
  }

  return (
    <div className="page">
      <header className="hero">
        <div className="hero__brand">
          <span className="hero__logo" aria-hidden>
            ⚡
          </span>
          <span className="hero__name">GamePulse</span>
        </div>
        <h1 className="hero__title">Live sports updates, by text.</h1>
        <p className="hero__sub">
          Pick your league and team. Drop in your number. Get scores in
          realtime and AI-summarised news on your schedule.
        </p>
      </header>

      <main className="card-shell">
        {confirmation ? (
          <ConfirmationMessage result={confirmation} onSubscribeAnother={reset} />
        ) : (
          <form className="form" onSubmit={handleSubmit} noValidate>
            {teamsQuery.isPending && <p className="muted">Loading teams…</p>}
            {teamsQuery.isError && (
              <p role="alert" className="error">
                Could not load teams. Please refresh the page.
              </p>
            )}

            <LeagueSelector
              value={league}
              onChange={handleLeague}
              disabled={mutation.isPending}
            />

            {league && (
              <TeamSelector
                teams={teamsForLeague}
                value={teamId}
                onChange={setTeamId}
                disabled={mutation.isPending}
              />
            )}

            <PhoneInput
              value={phone}
              onChange={setPhone}
              disabled={mutation.isPending}
              showError={touched}
            />

            <UpdateTypeSelector
              value={updateType}
              onChange={setUpdateType}
              disabled={mutation.isPending}
            />

            {showFrequency && (
              <FrequencySelector
                value={frequency}
                onChange={setFrequency}
                disabled={mutation.isPending}
              />
            )}

            <ConsentCheckbox
              checked={consent}
              onChange={setConsent}
              disabled={mutation.isPending}
            />

            {mutation.isError && (
              <p role="alert" className="error">
                {mutation.error instanceof ApiError
                  ? mutation.error.message
                  : 'Something went wrong. Please try again.'}
              </p>
            )}

            <button
              type="submit"
              className="btn btn--primary"
              disabled={!canSubmit}
              data-testid="submit"
            >
              {mutation.isPending ? 'Subscribing…' : 'Send me updates'}
            </button>
          </form>
        )}
      </main>

      <footer className="footer">
        <span>
          We text you. We never share your number. Reply STOP at any time to
          unsubscribe.
        </span>
        <span className="footer__links">
          <a href="#privacy">Privacy</a>
          <span aria-hidden>·</span>
          <a href="#terms">SMS Terms</a>
        </span>
      </footer>
    </div>
  );
}

import type { SubscriptionResponse } from '../api/client';

interface Props {
  result: SubscriptionResponse;
  onSubscribeAnother: () => void;
}

// ConfirmationMessage is the success state shown after a subscription is
// created. It echoes the chosen team and cadence so the user knows
// exactly what to expect on their phone. Live updates are always
// realtime, so we only mention frequency when news is part of the bundle.
export function ConfirmationMessage({ result, onSubscribeAnother }: Props) {
  const { update_type, frequency } = result.subscription;
  return (
    <div className="confirmation" role="status" aria-live="polite">
      <div className="confirmation__icon" aria-hidden>
        ✓
      </div>
      <h2 className="confirmation__title">You're subscribed!</h2>
      <p className="confirmation__body">
        We've sent a confirmation text. You'll start receiving{' '}
        <strong>{describe(update_type, frequency)}</strong> for the{' '}
        <strong>{result.team.name}</strong>.
      </p>
      <button type="button" className="btn btn--ghost" onClick={onSubscribeAnother}>
        Subscribe another number
      </button>
    </div>
  );
}

function describe(updateType: string, frequency: string): string {
  if (updateType === 'live') return 'realtime score updates';
  if (updateType === 'news') return `${frequency} news summaries`;
  return `realtime score updates and ${frequency} news summaries`;
}

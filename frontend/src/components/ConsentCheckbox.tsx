interface Props {
  checked: boolean;
  onChange: (next: boolean) => void;
  disabled?: boolean;
}

// ConsentCheckbox is the explicit-opt-in checkbox required by US carriers
// for A2P 10DLC compliance. Both the checkbox label and surrounding hint
// must mention message frequency, carrier rates, and the STOP keyword.
//
// We deliberately surface the full disclosure inline rather than hiding it
// behind a "Terms" link — Twilio campaign reviewers want to see it without
// extra clicks, and it makes the consent unambiguous to the subscriber.
export function ConsentCheckbox({ checked, onChange, disabled }: Props) {
  return (
    <label className={`consent${checked ? ' consent--checked' : ''}`}>
      <input
        type="checkbox"
        checked={checked}
        onChange={(e) => onChange(e.target.checked)}
        disabled={disabled}
        data-testid="consent-checkbox"
        required
      />
      <span className="consent__body">
        I agree to receive automated SMS messages from <strong>GamePulse</strong>{' '}
        about my selected team. Message frequency varies by team and game
        schedule. Message and data rates may apply. Reply <code>STOP</code>{' '}
        to cancel, <code>HELP</code> for help. See our{' '}
        <a href="#privacy">privacy policy</a> and{' '}
        <a href="#terms">SMS terms</a>.
      </span>
    </label>
  );
}

import type { Frequency } from '../api/client';

interface Props {
  value: Frequency;
  onChange: (value: Frequency) => void;
  disabled?: boolean;
}

const OPTIONS: { id: Frequency; title: string; description: string }[] = [
  { id: 'daily', title: 'Daily', description: 'One news summary every day.' },
  { id: 'weekly', title: 'Weekly', description: 'One roll-up every week.' },
  { id: 'monthly', title: 'Monthly', description: 'One recap every month.' },
];

// FrequencySelector controls news-summary cadence only. Live score updates
// are inherently realtime and bypass this setting entirely; the parent
// component hides this control when the user picks live-only.
export function FrequencySelector({ value, onChange, disabled }: Props) {
  return (
    <fieldset className="field" disabled={disabled}>
      <legend className="field__label">News summaries — how often?</legend>
      <p className="field__hint field__hint--block">
        This only controls how often we send news summaries. Live scores are
        always sent in realtime.
      </p>
      <div className="cards" role="radiogroup" aria-label="News frequency">
        {OPTIONS.map((opt) => {
          const selected = opt.id === value;
          return (
            <label
              key={opt.id}
              className={`card${selected ? ' card--selected' : ''}`}
              data-testid={`frequency-${opt.id}`}
            >
              <input
                type="radio"
                name="frequency"
                value={opt.id}
                checked={selected}
                onChange={() => onChange(opt.id)}
              />
              <div className="card__body">
                <span className="card__title">{opt.title}</span>
                <span className="card__desc">{opt.description}</span>
              </div>
            </label>
          );
        })}
      </div>
    </fieldset>
  );
}

import type { UpdateType } from '../api/client';

interface Props {
  value: UpdateType;
  onChange: (value: UpdateType) => void;
  disabled?: boolean;
}

const OPTIONS: { id: UpdateType; title: string; description: string }[] = [
  { id: 'live', title: 'Live scores', description: 'Realtime score changes during games.' },
  { id: 'news', title: 'News', description: 'AI-summarised headlines about your team.' },
  { id: 'both', title: 'Both', description: 'Score updates and news in one feed.' },
];

// UpdateTypeSelector is a radio group rendered as cards. Cards make the
// trade-offs between options easy to scan on mobile.
export function UpdateTypeSelector({ value, onChange, disabled }: Props) {
  return (
    <fieldset className="field" disabled={disabled}>
      <legend className="field__label">What should we send?</legend>
      <div className="cards" role="radiogroup" aria-label="Update type">
        {OPTIONS.map((opt) => {
          const selected = opt.id === value;
          return (
            <label
              key={opt.id}
              className={`card${selected ? ' card--selected' : ''}`}
              data-testid={`update-type-${opt.id}`}
            >
              <input
                type="radio"
                name="update_type"
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

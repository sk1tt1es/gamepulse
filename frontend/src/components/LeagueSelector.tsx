import type { League } from '../api/client';

interface Props {
  value: League | '';
  onChange: (league: League) => void;
  disabled?: boolean;
}

// LEAGUES is the canonical display order for the four sports we support.
// Each item carries an inline SVG glyph so the picker doesn't depend on
// any external icon hosting.
const LEAGUES: { id: League; label: string; tagline: string; Icon: () => JSX.Element }[] = [
  { id: 'NBA', label: 'NBA', tagline: 'Basketball', Icon: BasketballIcon },
  { id: 'NFL', label: 'NFL', tagline: 'Football', Icon: FootballIcon },
  { id: 'MLB', label: 'MLB', tagline: 'Baseball', Icon: BaseballIcon },
  { id: 'NHL', label: 'NHL', tagline: 'Hockey', Icon: HockeyIcon },
];

// LeagueSelector is the first step of the team picker. It's a radio group
// rendered as four large cards, each with a sport-specific glyph.
export function LeagueSelector({ value, onChange, disabled }: Props) {
  return (
    <fieldset className="field" disabled={disabled}>
      <legend className="field__label">Pick a league</legend>
      <div className="cards cards--leagues" role="radiogroup" aria-label="League">
        {LEAGUES.map(({ id, label, tagline, Icon }) => {
          const selected = value === id;
          return (
            <label
              key={id}
              className={`league-card${selected ? ' league-card--selected' : ''}`}
              data-testid={`league-${id}`}
            >
              <input
                type="radio"
                name="league"
                value={id}
                checked={selected}
                onChange={() => onChange(id)}
              />
              <div className="league-card__icon" aria-hidden>
                <Icon />
              </div>
              <div className="league-card__body">
                <span className="league-card__title">{label}</span>
                <span className="league-card__tagline">{tagline}</span>
              </div>
            </label>
          );
        })}
      </div>
    </fieldset>
  );
}

// --- Inline SVG sport glyphs ---------------------------------------------
// We hand-roll these tiny icons so the bundle stays small and the visuals
// don't depend on any external CDN. Each glyph fits a 32×32 viewBox.

function BasketballIcon() {
  return (
    <svg viewBox="0 0 32 32" width="32" height="32" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <circle cx="16" cy="16" r="12" />
      <path d="M4 16h24M16 4v24M7 7c4 3 4 15 0 18M25 7c-4 3-4 15 0 18" />
    </svg>
  );
}

function FootballIcon() {
  return (
    <svg viewBox="0 0 32 32" width="32" height="32" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <ellipse cx="16" cy="16" rx="13" ry="8" transform="rotate(-25 16 16)" />
      <path d="M11 13l10 6M13 11l8 10M9 15l4 2M19 15l4 2" />
    </svg>
  );
}

function BaseballIcon() {
  return (
    <svg viewBox="0 0 32 32" width="32" height="32" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <circle cx="16" cy="16" r="12" />
      <path d="M7 9c2 2 3 5 3 7s-1 5-3 7M25 9c-2 2-3 5-3 7s1 5 3 7" />
    </svg>
  );
}

function HockeyIcon() {
  return (
    <svg viewBox="0 0 32 32" width="32" height="32" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <ellipse cx="16" cy="22" rx="11" ry="4" />
      <path d="M5 22V20M27 22V20" />
      <path d="M10 14l8-9M18 5h4M22 5l4 11" />
    </svg>
  );
}

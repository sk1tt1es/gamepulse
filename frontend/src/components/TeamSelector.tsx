import type { Team } from '../api/client';
import { TeamLogo } from './TeamLogo';

interface Props {
  teams: Team[];
  value: string;
  onChange: (teamId: string) => void;
  disabled?: boolean;
}

// TeamSelector is the second step of the picker. It shows every team in
// the chosen league as a logo + name card. Cards are radio inputs so
// keyboard users can tab through them and arrow-key between selections.
export function TeamSelector({ teams, value, onChange, disabled }: Props) {
  if (teams.length === 0) {
    return (
      <p className="muted" role="status">
        No teams available for this league yet.
      </p>
    );
  }

  return (
    <fieldset className="field" disabled={disabled}>
      <legend className="field__label">Pick a team</legend>
      <div className="cards cards--teams" role="radiogroup" aria-label="Team">
        {teams.map((t) => {
          const selected = value === t.id;
          return (
            <label
              key={t.id}
              className={`team-card${selected ? ' team-card--selected' : ''}`}
              data-testid={`team-${t.id}`}
            >
              <input
                type="radio"
                name="team"
                value={t.id}
                checked={selected}
                onChange={() => onChange(t.id)}
              />
              <TeamLogo src={t.logo_url} name={t.name} />
              <span className="team-card__name">{t.name}</span>
            </label>
          );
        })}
      </div>
    </fieldset>
  );
}

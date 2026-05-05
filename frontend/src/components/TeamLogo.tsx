import { useState } from 'react';

interface Props {
  src: string;
  /** Display name used to compute fallback initials and alt text. */
  name: string;
}

// TeamLogo renders the remote logo and falls back to a colored circle with
// the team's initials if the image fails to load. The fallback is keyed
// off the team name so the same team always gets the same color.
export function TeamLogo({ src, name }: Props) {
  const [failed, setFailed] = useState(false);

  if (failed || !src) {
    return (
      <span
        className="team-logo team-logo--fallback"
        style={{ background: colorFromName(name) }}
        aria-hidden
      >
        {initials(name)}
      </span>
    );
  }

  return (
    <img
      className="team-logo"
      src={src}
      alt={`${name} logo`}
      loading="lazy"
      onError={() => setFailed(true)}
    />
  );
}

function initials(name: string): string {
  const tokens = name.split(/\s+/).filter(Boolean);
  if (tokens.length === 0) return '?';
  if (tokens.length === 1) return tokens[0].slice(0, 2).toUpperCase();
  return (tokens[0][0] + tokens[tokens.length - 1][0]).toUpperCase();
}

// colorFromName produces a deterministic pleasant background color from a
// team name so fallback chips don't all look the same.
function colorFromName(name: string): string {
  let hash = 0;
  for (let i = 0; i < name.length; i++) hash = (hash * 31 + name.charCodeAt(i)) | 0;
  const hue = Math.abs(hash) % 360;
  return `hsl(${hue}, 55%, 35%)`;
}

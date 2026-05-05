// Thin wrapper around fetch for the GamePulse API.
//
// We keep all server interactions here so React components don't have to
// know about request shapes or error parsing. Each function returns either
// the parsed JSON body or throws an `ApiError` whose message is suitable
// for surfacing directly to the user.

export type League = 'NBA' | 'NFL' | 'MLB' | 'NHL';
export type UpdateType = 'live' | 'news' | 'both';
// Frequency only governs news summaries. Live updates are always realtime.
export type Frequency = 'daily' | 'weekly' | 'monthly';

export interface Team {
  id: string;
  name: string;
  league: League;
  external_id: string;
  logo_url: string;
}

export interface LeagueGroup {
  league: League;
  teams: Team[];
}

export interface SubscriptionResponse {
  subscription: {
    id: string;
    user_id: string;
    team_id: string;
    update_type: UpdateType;
    frequency: Frequency;
    created_at: string;
  };
  team: Team;
  message: string;
}

export interface CreateSubscriptionInput {
  phone_number: string;
  team_id: string;
  update_type: UpdateType;
  frequency: Frequency;
}

export class ApiError extends Error {
  status: number;
  constructor(message: string, status: number) {
    super(message);
    this.status = status;
  }
}

const API_BASE = '/api/v1';

async function handle<T>(res: Response): Promise<T> {
  if (!res.ok) {
    let message = `Request failed (${res.status})`;
    try {
      const body = await res.json();
      if (body && typeof body.error === 'string') message = body.error;
    } catch {
      // ignore parse errors; fall back to status code message
    }
    throw new ApiError(message, res.status);
  }
  return res.json() as Promise<T>;
}

export async function fetchTeams(): Promise<LeagueGroup[]> {
  const res = await fetch(`${API_BASE}/teams`);
  const body = await handle<{ leagues: LeagueGroup[] }>(res);
  return body.leagues;
}

export async function createSubscription(
  input: CreateSubscriptionInput,
): Promise<SubscriptionResponse> {
  const res = await fetch(`${API_BASE}/subscriptions`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(input),
  });
  return handle<SubscriptionResponse>(res);
}

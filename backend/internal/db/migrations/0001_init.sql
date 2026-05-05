-- GamePulse initial schema.
-- All tables use uuid primary keys generated client-side so we don't depend on
-- a uuid extension in the database. Timestamps default to UTC.

CREATE TABLE IF NOT EXISTS users (
    id           uuid PRIMARY KEY,
    phone_number text NOT NULL UNIQUE,
    created_at   timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS teams (
    id          uuid PRIMARY KEY,
    name        text NOT NULL,
    league      text NOT NULL,
    external_id text NOT NULL,
    UNIQUE (league, external_id)
);

CREATE INDEX IF NOT EXISTS teams_league_idx ON teams(league);

CREATE TABLE IF NOT EXISTS subscriptions (
    id          uuid PRIMARY KEY,
    user_id     uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    team_id     uuid NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    update_type text NOT NULL CHECK (update_type IN ('live','news','both')),
    frequency   text NOT NULL CHECK (frequency IN ('realtime','hourly','daily')),
    created_at  timestamptz NOT NULL DEFAULT now(),
    UNIQUE (user_id, team_id)
);

CREATE INDEX IF NOT EXISTS subscriptions_team_idx ON subscriptions(team_id);

CREATE TABLE IF NOT EXISTS games (
    id          uuid PRIMARY KEY,
    team_id     uuid NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    opponent    text NOT NULL,
    status      text NOT NULL CHECK (status IN ('scheduled','live','finished')),
    start_time  timestamptz NOT NULL,
    external_id text NOT NULL,
    home_score  int NOT NULL DEFAULT 0,
    away_score  int NOT NULL DEFAULT 0,
    period      text NOT NULL DEFAULT '',
    UNIQUE (team_id, external_id)
);

CREATE INDEX IF NOT EXISTS games_status_idx ON games(status);

CREATE TABLE IF NOT EXISTS game_events (
    id         uuid PRIMARY KEY,
    game_id    uuid NOT NULL REFERENCES games(id) ON DELETE CASCADE,
    event_type text NOT NULL,
    payload    jsonb NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS game_events_game_idx ON game_events(game_id, created_at);

CREATE TABLE IF NOT EXISTS news_articles (
    id           uuid PRIMARY KEY,
    team_id      uuid NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    title        text NOT NULL,
    content      text NOT NULL,
    source       text NOT NULL,
    url          text NOT NULL DEFAULT '',
    published_at timestamptz NOT NULL,
    summary      text NOT NULL DEFAULT '',
    UNIQUE (team_id, url)
);

CREATE INDEX IF NOT EXISTS news_team_pub_idx ON news_articles(team_id, published_at DESC);

CREATE TABLE IF NOT EXISTS notifications_log (
    id              uuid PRIMARY KEY,
    subscription_id uuid NOT NULL REFERENCES subscriptions(id) ON DELETE CASCADE,
    message_type    text NOT NULL,
    content         text NOT NULL,
    sent_at         timestamptz NOT NULL DEFAULT now(),
    dedupe_key      text NOT NULL,
    UNIQUE (subscription_id, dedupe_key)
);

CREATE INDEX IF NOT EXISTS notifications_sub_idx ON notifications_log(subscription_id, sent_at DESC);

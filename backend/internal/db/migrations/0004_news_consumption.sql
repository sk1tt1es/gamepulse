-- v3 schema upgrade: summarize-on-send pipeline + per-subscription article
-- consumption tracking + initial-news-after-subscribe flags + housekeeping.
--
-- Background:
--   v1/v2 had the news aggregator summarize and fan out immediately,
--   queuing rows in notifications_log with message_type='pending'. The
--   digest builder rolled those up.
--
--   v3 flips the pipeline: the aggregator only INSERTs raw articles, and
--   summarization happens at SEND time inside the digest builder. To keep
--   shared-team subs working (e.g. one daily + one weekly subscriber on
--   the same team) we track per-subscription consumption in a side table
--   so we only delete an article from news_articles once every eligible
--   subscriber has actually included it in a sent digest.

-- ---- Per-subscription consumption ---------------------------------------
-- A row here means: subscription S included article A in a digest SMS.
-- The composite primary key + cascading FKs keep the table self-cleaning
-- when either side is removed.

CREATE TABLE IF NOT EXISTS subscription_article_sent (
    subscription_id uuid NOT NULL REFERENCES subscriptions(id)  ON DELETE CASCADE,
    article_id      uuid NOT NULL REFERENCES news_articles(id)  ON DELETE CASCADE,
    sent_at         timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (subscription_id, article_id)
);

CREATE INDEX IF NOT EXISTS subscription_article_sent_article_idx
    ON subscription_article_sent(article_id);

-- ---- Initial-news-after-subscribe flags ---------------------------------
-- News and "both" subscribers receive their first digest a short cooldown
-- after signup (default 5 minutes — see config.InitialNewsDelay) so they
-- get an immediate-feeling welcome digest without waiting for the next
-- daily/weekly cycle. We track the boundary explicitly rather than
-- recomputing from created_at + interval everywhere.

ALTER TABLE subscriptions
    ADD COLUMN IF NOT EXISTS initial_news_sent BOOLEAN NOT NULL DEFAULT false;

ALTER TABLE subscriptions
    ADD COLUMN IF NOT EXISTS initial_news_not_before TIMESTAMPTZ;

-- Backfill any pre-existing rows so they don't trigger a phantom initial
-- send the next time the digest builder runs.
UPDATE subscriptions
   SET initial_news_sent = true
 WHERE initial_news_not_before IS NULL;

-- Index keeps the per-tick "who's due for an initial send?" query cheap.
CREATE INDEX IF NOT EXISTS subscriptions_initial_due_idx
    ON subscriptions(initial_news_not_before)
 WHERE initial_news_sent = false;

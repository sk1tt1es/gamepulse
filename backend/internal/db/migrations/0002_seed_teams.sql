-- Seed a representative set of teams across the four supported leagues.
-- The external_id is a sportsdata-style slug; mock providers use it directly.
-- ON CONFLICT keeps this migration idempotent if it runs more than once.

INSERT INTO teams (id, name, league, external_id) VALUES
    -- NBA
    ('11111111-0000-0000-0000-000000000001','Los Angeles Lakers','NBA','LAL'),
    ('11111111-0000-0000-0000-000000000002','Boston Celtics','NBA','BOS'),
    ('11111111-0000-0000-0000-000000000003','Golden State Warriors','NBA','GSW'),
    ('11111111-0000-0000-0000-000000000004','Miami Heat','NBA','MIA'),
    ('11111111-0000-0000-0000-000000000005','Denver Nuggets','NBA','DEN'),
    ('11111111-0000-0000-0000-000000000006','Milwaukee Bucks','NBA','MIL'),
    -- NFL
    ('22222222-0000-0000-0000-000000000001','Kansas City Chiefs','NFL','KC'),
    ('22222222-0000-0000-0000-000000000002','San Francisco 49ers','NFL','SF'),
    ('22222222-0000-0000-0000-000000000003','Dallas Cowboys','NFL','DAL'),
    ('22222222-0000-0000-0000-000000000004','Buffalo Bills','NFL','BUF'),
    ('22222222-0000-0000-0000-000000000005','Philadelphia Eagles','NFL','PHI'),
    ('22222222-0000-0000-0000-000000000006','Green Bay Packers','NFL','GB'),
    -- MLB
    ('33333333-0000-0000-0000-000000000001','New York Yankees','MLB','NYY'),
    ('33333333-0000-0000-0000-000000000002','Los Angeles Dodgers','MLB','LAD'),
    ('33333333-0000-0000-0000-000000000003','Boston Red Sox','MLB','BOS'),
    ('33333333-0000-0000-0000-000000000004','Chicago Cubs','MLB','CHC'),
    ('33333333-0000-0000-0000-000000000005','Houston Astros','MLB','HOU'),
    ('33333333-0000-0000-0000-000000000006','Atlanta Braves','MLB','ATL'),
    -- NHL
    ('44444444-0000-0000-0000-000000000001','Toronto Maple Leafs','NHL','TOR'),
    ('44444444-0000-0000-0000-000000000002','Montreal Canadiens','NHL','MTL'),
    ('44444444-0000-0000-0000-000000000003','Edmonton Oilers','NHL','EDM'),
    ('44444444-0000-0000-0000-000000000004','New York Rangers','NHL','NYR'),
    ('44444444-0000-0000-0000-000000000005','Vegas Golden Knights','NHL','VGK'),
    ('44444444-0000-0000-0000-000000000006','Colorado Avalanche','NHL','COL')
ON CONFLICT (league, external_id) DO NOTHING;

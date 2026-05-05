-- v2 schema upgrade: news-only frequencies + complete team rosters.
--
-- 1. Frequency was originally (realtime, hourly, daily). Live updates are
--    inherently realtime, so frequency is now news-only and supports
--    daily, weekly, monthly. Existing rows are migrated to "daily".
-- 2. Team seed expanded from 24 representative teams to the full rosters
--    of all four leagues (NBA 30, NFL 32, MLB 30, NHL 32). Idempotent.

-- ---- Frequency constraint -----------------------------------------------

UPDATE subscriptions
   SET frequency = 'daily'
 WHERE frequency NOT IN ('daily', 'weekly', 'monthly');

ALTER TABLE subscriptions DROP CONSTRAINT IF EXISTS subscriptions_frequency_check;
ALTER TABLE subscriptions
    ADD CONSTRAINT subscriptions_frequency_check
    CHECK (frequency IN ('daily', 'weekly', 'monthly'));

-- ---- Full team rosters --------------------------------------------------
-- external_id mirrors the abbreviation used by ESPN's public team-logo CDN
-- (lower-cased at read time) so the UI can render real team logos with no
-- separate logo column. ON CONFLICT keeps this idempotent and additive,
-- which means existing dev databases pick up the new teams without
-- disturbing any subscriptions tied to the original 24.

INSERT INTO teams (id, name, league, external_id) VALUES
    -- ===== NBA (30) =====
    ('a0000000-0000-0000-0000-0000000000a1','Atlanta Hawks','NBA','ATL'),
    ('a0000000-0000-0000-0000-0000000000a2','Boston Celtics','NBA','BOS'),
    ('a0000000-0000-0000-0000-0000000000a3','Brooklyn Nets','NBA','BKN'),
    ('a0000000-0000-0000-0000-0000000000a4','Charlotte Hornets','NBA','CHA'),
    ('a0000000-0000-0000-0000-0000000000a5','Chicago Bulls','NBA','CHI'),
    ('a0000000-0000-0000-0000-0000000000a6','Cleveland Cavaliers','NBA','CLE'),
    ('a0000000-0000-0000-0000-0000000000a7','Dallas Mavericks','NBA','DAL'),
    ('a0000000-0000-0000-0000-0000000000a8','Denver Nuggets','NBA','DEN'),
    ('a0000000-0000-0000-0000-0000000000a9','Detroit Pistons','NBA','DET'),
    ('a0000000-0000-0000-0000-0000000000aa','Golden State Warriors','NBA','GS'),
    ('a0000000-0000-0000-0000-0000000000ab','Houston Rockets','NBA','HOU'),
    ('a0000000-0000-0000-0000-0000000000ac','Indiana Pacers','NBA','IND'),
    ('a0000000-0000-0000-0000-0000000000ad','LA Clippers','NBA','LAC'),
    ('a0000000-0000-0000-0000-0000000000ae','Los Angeles Lakers','NBA','LAL'),
    ('a0000000-0000-0000-0000-0000000000af','Memphis Grizzlies','NBA','MEM'),
    ('a0000000-0000-0000-0000-0000000000b0','Miami Heat','NBA','MIA'),
    ('a0000000-0000-0000-0000-0000000000b1','Milwaukee Bucks','NBA','MIL'),
    ('a0000000-0000-0000-0000-0000000000b2','Minnesota Timberwolves','NBA','MIN'),
    ('a0000000-0000-0000-0000-0000000000b3','New Orleans Pelicans','NBA','NO'),
    ('a0000000-0000-0000-0000-0000000000b4','New York Knicks','NBA','NY'),
    ('a0000000-0000-0000-0000-0000000000b5','Oklahoma City Thunder','NBA','OKC'),
    ('a0000000-0000-0000-0000-0000000000b6','Orlando Magic','NBA','ORL'),
    ('a0000000-0000-0000-0000-0000000000b7','Philadelphia 76ers','NBA','PHI'),
    ('a0000000-0000-0000-0000-0000000000b8','Phoenix Suns','NBA','PHX'),
    ('a0000000-0000-0000-0000-0000000000b9','Portland Trail Blazers','NBA','POR'),
    ('a0000000-0000-0000-0000-0000000000ba','Sacramento Kings','NBA','SAC'),
    ('a0000000-0000-0000-0000-0000000000bb','San Antonio Spurs','NBA','SA'),
    ('a0000000-0000-0000-0000-0000000000bc','Toronto Raptors','NBA','TOR'),
    ('a0000000-0000-0000-0000-0000000000bd','Utah Jazz','NBA','UTAH'),
    ('a0000000-0000-0000-0000-0000000000be','Washington Wizards','NBA','WSH'),

    -- ===== NFL (32) =====
    ('b0000000-0000-0000-0000-0000000000a1','Arizona Cardinals','NFL','ARI'),
    ('b0000000-0000-0000-0000-0000000000a2','Atlanta Falcons','NFL','ATL'),
    ('b0000000-0000-0000-0000-0000000000a3','Baltimore Ravens','NFL','BAL'),
    ('b0000000-0000-0000-0000-0000000000a4','Buffalo Bills','NFL','BUF'),
    ('b0000000-0000-0000-0000-0000000000a5','Carolina Panthers','NFL','CAR'),
    ('b0000000-0000-0000-0000-0000000000a6','Chicago Bears','NFL','CHI'),
    ('b0000000-0000-0000-0000-0000000000a7','Cincinnati Bengals','NFL','CIN'),
    ('b0000000-0000-0000-0000-0000000000a8','Cleveland Browns','NFL','CLE'),
    ('b0000000-0000-0000-0000-0000000000a9','Dallas Cowboys','NFL','DAL'),
    ('b0000000-0000-0000-0000-0000000000aa','Denver Broncos','NFL','DEN'),
    ('b0000000-0000-0000-0000-0000000000ab','Detroit Lions','NFL','DET'),
    ('b0000000-0000-0000-0000-0000000000ac','Green Bay Packers','NFL','GB'),
    ('b0000000-0000-0000-0000-0000000000ad','Houston Texans','NFL','HOU'),
    ('b0000000-0000-0000-0000-0000000000ae','Indianapolis Colts','NFL','IND'),
    ('b0000000-0000-0000-0000-0000000000af','Jacksonville Jaguars','NFL','JAX'),
    ('b0000000-0000-0000-0000-0000000000b0','Kansas City Chiefs','NFL','KC'),
    ('b0000000-0000-0000-0000-0000000000b1','Las Vegas Raiders','NFL','LV'),
    ('b0000000-0000-0000-0000-0000000000b2','Los Angeles Chargers','NFL','LAC'),
    ('b0000000-0000-0000-0000-0000000000b3','Los Angeles Rams','NFL','LAR'),
    ('b0000000-0000-0000-0000-0000000000b4','Miami Dolphins','NFL','MIA'),
    ('b0000000-0000-0000-0000-0000000000b5','Minnesota Vikings','NFL','MIN'),
    ('b0000000-0000-0000-0000-0000000000b6','New England Patriots','NFL','NE'),
    ('b0000000-0000-0000-0000-0000000000b7','New Orleans Saints','NFL','NO'),
    ('b0000000-0000-0000-0000-0000000000b8','New York Giants','NFL','NYG'),
    ('b0000000-0000-0000-0000-0000000000b9','New York Jets','NFL','NYJ'),
    ('b0000000-0000-0000-0000-0000000000ba','Philadelphia Eagles','NFL','PHI'),
    ('b0000000-0000-0000-0000-0000000000bb','Pittsburgh Steelers','NFL','PIT'),
    ('b0000000-0000-0000-0000-0000000000bc','San Francisco 49ers','NFL','SF'),
    ('b0000000-0000-0000-0000-0000000000bd','Seattle Seahawks','NFL','SEA'),
    ('b0000000-0000-0000-0000-0000000000be','Tampa Bay Buccaneers','NFL','TB'),
    ('b0000000-0000-0000-0000-0000000000bf','Tennessee Titans','NFL','TEN'),
    ('b0000000-0000-0000-0000-0000000000c0','Washington Commanders','NFL','WSH'),

    -- ===== MLB (30) =====
    ('c0000000-0000-0000-0000-0000000000a1','Arizona Diamondbacks','MLB','ARI'),
    ('c0000000-0000-0000-0000-0000000000a2','Atlanta Braves','MLB','ATL'),
    ('c0000000-0000-0000-0000-0000000000a3','Baltimore Orioles','MLB','BAL'),
    ('c0000000-0000-0000-0000-0000000000a4','Boston Red Sox','MLB','BOS'),
    ('c0000000-0000-0000-0000-0000000000a5','Chicago Cubs','MLB','CHC'),
    ('c0000000-0000-0000-0000-0000000000a6','Chicago White Sox','MLB','CHW'),
    ('c0000000-0000-0000-0000-0000000000a7','Cincinnati Reds','MLB','CIN'),
    ('c0000000-0000-0000-0000-0000000000a8','Cleveland Guardians','MLB','CLE'),
    ('c0000000-0000-0000-0000-0000000000a9','Colorado Rockies','MLB','COL'),
    ('c0000000-0000-0000-0000-0000000000aa','Detroit Tigers','MLB','DET'),
    ('c0000000-0000-0000-0000-0000000000ab','Houston Astros','MLB','HOU'),
    ('c0000000-0000-0000-0000-0000000000ac','Kansas City Royals','MLB','KC'),
    ('c0000000-0000-0000-0000-0000000000ad','Los Angeles Angels','MLB','LAA'),
    ('c0000000-0000-0000-0000-0000000000ae','Los Angeles Dodgers','MLB','LAD'),
    ('c0000000-0000-0000-0000-0000000000af','Miami Marlins','MLB','MIA'),
    ('c0000000-0000-0000-0000-0000000000b0','Milwaukee Brewers','MLB','MIL'),
    ('c0000000-0000-0000-0000-0000000000b1','Minnesota Twins','MLB','MIN'),
    ('c0000000-0000-0000-0000-0000000000b2','New York Mets','MLB','NYM'),
    ('c0000000-0000-0000-0000-0000000000b3','New York Yankees','MLB','NYY'),
    ('c0000000-0000-0000-0000-0000000000b4','Oakland Athletics','MLB','OAK'),
    ('c0000000-0000-0000-0000-0000000000b5','Philadelphia Phillies','MLB','PHI'),
    ('c0000000-0000-0000-0000-0000000000b6','Pittsburgh Pirates','MLB','PIT'),
    ('c0000000-0000-0000-0000-0000000000b7','San Diego Padres','MLB','SD'),
    ('c0000000-0000-0000-0000-0000000000b8','San Francisco Giants','MLB','SF'),
    ('c0000000-0000-0000-0000-0000000000b9','Seattle Mariners','MLB','SEA'),
    ('c0000000-0000-0000-0000-0000000000ba','St. Louis Cardinals','MLB','STL'),
    ('c0000000-0000-0000-0000-0000000000bb','Tampa Bay Rays','MLB','TB'),
    ('c0000000-0000-0000-0000-0000000000bc','Texas Rangers','MLB','TEX'),
    ('c0000000-0000-0000-0000-0000000000bd','Toronto Blue Jays','MLB','TOR'),
    ('c0000000-0000-0000-0000-0000000000be','Washington Nationals','MLB','WSH'),

    -- ===== NHL (32) =====
    ('d0000000-0000-0000-0000-0000000000a1','Anaheim Ducks','NHL','ANA'),
    ('d0000000-0000-0000-0000-0000000000a2','Boston Bruins','NHL','BOS'),
    ('d0000000-0000-0000-0000-0000000000a3','Buffalo Sabres','NHL','BUF'),
    ('d0000000-0000-0000-0000-0000000000a4','Calgary Flames','NHL','CGY'),
    ('d0000000-0000-0000-0000-0000000000a5','Carolina Hurricanes','NHL','CAR'),
    ('d0000000-0000-0000-0000-0000000000a6','Chicago Blackhawks','NHL','CHI'),
    ('d0000000-0000-0000-0000-0000000000a7','Colorado Avalanche','NHL','COL'),
    ('d0000000-0000-0000-0000-0000000000a8','Columbus Blue Jackets','NHL','CBJ'),
    ('d0000000-0000-0000-0000-0000000000a9','Dallas Stars','NHL','DAL'),
    ('d0000000-0000-0000-0000-0000000000aa','Detroit Red Wings','NHL','DET'),
    ('d0000000-0000-0000-0000-0000000000ab','Edmonton Oilers','NHL','EDM'),
    ('d0000000-0000-0000-0000-0000000000ac','Florida Panthers','NHL','FLA'),
    ('d0000000-0000-0000-0000-0000000000ad','Los Angeles Kings','NHL','LA'),
    ('d0000000-0000-0000-0000-0000000000ae','Minnesota Wild','NHL','MIN'),
    ('d0000000-0000-0000-0000-0000000000af','Montreal Canadiens','NHL','MTL'),
    ('d0000000-0000-0000-0000-0000000000b0','Nashville Predators','NHL','NSH'),
    ('d0000000-0000-0000-0000-0000000000b1','New Jersey Devils','NHL','NJ'),
    ('d0000000-0000-0000-0000-0000000000b2','New York Islanders','NHL','NYI'),
    ('d0000000-0000-0000-0000-0000000000b3','New York Rangers','NHL','NYR'),
    ('d0000000-0000-0000-0000-0000000000b4','Ottawa Senators','NHL','OTT'),
    ('d0000000-0000-0000-0000-0000000000b5','Philadelphia Flyers','NHL','PHI'),
    ('d0000000-0000-0000-0000-0000000000b6','Pittsburgh Penguins','NHL','PIT'),
    ('d0000000-0000-0000-0000-0000000000b7','San Jose Sharks','NHL','SJ'),
    ('d0000000-0000-0000-0000-0000000000b8','Seattle Kraken','NHL','SEA'),
    ('d0000000-0000-0000-0000-0000000000b9','St. Louis Blues','NHL','STL'),
    ('d0000000-0000-0000-0000-0000000000ba','Tampa Bay Lightning','NHL','TB'),
    ('d0000000-0000-0000-0000-0000000000bb','Toronto Maple Leafs','NHL','TOR'),
    ('d0000000-0000-0000-0000-0000000000bc','Utah Hockey Club','NHL','UTAH'),
    ('d0000000-0000-0000-0000-0000000000bd','Vancouver Canucks','NHL','VAN'),
    ('d0000000-0000-0000-0000-0000000000be','Vegas Golden Knights','NHL','VGK'),
    ('d0000000-0000-0000-0000-0000000000bf','Washington Capitals','NHL','WSH'),
    ('d0000000-0000-0000-0000-0000000000c0','Winnipeg Jets','NHL','WPG')
ON CONFLICT (league, external_id) DO NOTHING;

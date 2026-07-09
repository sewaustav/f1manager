CREATE TABLE IF NOT EXISTS users (
    id BIGSERIAL PRIMARY KEY,
    email TEXT NOT NULL UNIQUE,
    username TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS refresh_tokens (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash TEXT NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    revoked BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_refresh_tokens_user_id ON refresh_tokens(user_id);

CREATE TABLE IF NOT EXISTS pilots_initial (
  id BIGSERIAL PRIMARY KEY,
  name TEXT,
  rating INTEGER,
  garage_id INTEGER,
  quali_rating INTEGER,
  style INTEGER,
  expirince INTEGER,
  adaptiveness INTEGER,
  emotions INTEGER,
  stability INTEGER,
  rain INTEGER,
  settings_angle INTEGER,
  starting INTEGER,
  tyre_management INTEGER,
  mistake_possibility INTEGER,
  price INTEGER,
  sponsors INTEGER,
  car_fit INTEGER
);

CREATE TABLE IF NOT EXISTS tracks (
    id BIGSERIAL PRIMARY KEY,
    name TEXT,
    downforce INTEGER,
    type INTEGER,
    difficulity INTEGER,
    quali_impact INTEGER,
    rain INTEGER,
    tyre INTEGER
);

CREATE TABLE IF NOT EXISTS pilots_track_initial (
    id BIGSERIAL PRIMARY KEY,
    pilot_id BIGINT,
    track_id BIGINT,
    level INTEGER,
    FOREIGN KEY(pilot_id) REFERENCES pilots_initial(id),
    FOREIGN KEY(track_id) REFERENCES tracks(id)
);

CREATE TABLE IF NOT EXISTS teams_principals (
    id BIGSERIAL PRIMARY KEY,
    name TEXT,
    price INTEGER,
    level INTEGER
);

CREATE TABLE IF NOT EXISTS engine (
    id BIGSERIAL PRIMARY KEY,
    manufacturer INTEGER UNIQUE,
    price INTEGER,
    power INTEGER
);

CREATE TABLE IF NOT EXISTS base_team (
    id BIGSERIAL PRIMARY KEY,
    name TEXT,
    car_lvl INTEGER,
    ice INTEGER,
    base_lvl INTEGER,
    engineer INTEGER,
    tube INTEGER,
    sim INTEGER,
    update_rtg INTEGER,
    is_manufacturer INTEGER,
    budget INTEGER,
    car_settings INTEGER
);

CREATE TABLE IF NOT EXISTS groups (
    id BIGSERIAL PRIMARY KEY,
    name TEXT,
    password TEXT
);

CREATE TABLE IF NOT EXISTS teams (
    id BIGSERIAL PRIMARY KEY,
    group_id BIGINT,
    name TEXT,
    car_lvl INTEGER,
    ice INTEGER,
    base_lvl INTEGER,
    engineer INTEGER,
    tube INTEGER,
    sim INTEGER,
    update_rtg INTEGER,
    is_manufacturer INTEGER,
    budget INTEGER,
    car_settings INTEGER,
    FOREIGN KEY(ice) REFERENCES engine(manufacturer),
    FOREIGN KEY(group_id) REFERENCES groups(id)
);

CREATE TABLE IF NOT EXISTS players (
    id BIGINT PRIMARY KEY REFERENCES users(id),
    name TEXT,
    group_id BIGINT,
    team_id BIGINT,
    principal_id BIGINT,
    budget INTEGER,
    tokens INTEGER DEFAULT 120,
    FOREIGN KEY(group_id) REFERENCES groups(id),
    FOREIGN KEY(team_id) REFERENCES teams(id),
    FOREIGN KEY(principal_id) REFERENCES teams_principals(id)
);

CREATE TABLE IF NOT EXISTS pilots (
    id BIGSERIAL PRIMARY KEY,
    name TEXT,
    garage_id BIGINT,
    team_id BIGINT,
    rating INTEGER,
    quali_rating INTEGER,
    style INTEGER,
    expirince INTEGER,
    adaptiveness INTEGER,
    emotions INTEGER,
    stability INTEGER,
    rain INTEGER,
    settings_angle INTEGER,
    starting INTEGER,
    tyre_management INTEGER,
    mistake_possibility INTEGER,
    price INTEGER,
    sponsors INTEGER,
    car_fit INTEGER,
    FOREIGN KEY(team_id) REFERENCES players(id),
    FOREIGN KEY(garage_id) REFERENCES teams(id)
);

CREATE TABLE IF NOT EXISTS car (
    id BIGSERIAL PRIMARY KEY,
    team_id BIGINT,
    aerodynamic INTEGER,
    engine INTEGER,
    chassis INTEGER,
    floor INTEGER,
    tyres INTEGER,
    reliability INTEGER,
    settings_angle INTEGER,
    FOREIGN KEY(team_id) REFERENCES teams(id)
)




-- 0002_core: identity, fleet, trips, Wi-Fi quota ledger and payments.
-- Money is integer paise. Quota is integer bytes. Ledgers are append-only.

CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE users (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    phone      text NOT NULL UNIQUE CHECK (phone ~ '^\+[1-9][0-9]{7,14}$'),
    name       text NOT NULL DEFAULT '' CHECK (char_length(name) <= 80),
    role       text NOT NULL DEFAULT 'passenger' CHECK (role IN ('passenger', 'driver', 'admin')),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE TRIGGER users_updated_at BEFORE UPDATE ON users FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Only a SHA-256 of each refresh token is stored. Rotation revokes the previous row.
CREATE TABLE refresh_tokens (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash bytea NOT NULL UNIQUE,
    expires_at timestamptz NOT NULL,
    revoked_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX refresh_tokens_user ON refresh_tokens(user_id);

CREATE TABLE cabs (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    plate        text NOT NULL UNIQUE,
    model        text NOT NULL,
    seats        int  NOT NULL CHECK (seats BETWEEN 1 AND 12),
    electric     boolean NOT NULL DEFAULT false,
    driver_id    uuid UNIQUE REFERENCES users(id) ON DELETE SET NULL,
    driver_name  text NOT NULL DEFAULT '',
    simulated    boolean NOT NULL DEFAULT false,
    status       text NOT NULL DEFAULT 'offline' CHECK (status IN ('offline', 'available', 'on_trip')),
    wifi_ssid    text NOT NULL DEFAULT '',
    last_lat     double precision,
    last_lng     double precision,
    last_heading double precision NOT NULL DEFAULT 0,
    last_seen_at timestamptz,
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now()
);
CREATE TRIGGER cabs_updated_at BEFORE UPDATE ON cabs FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE INDEX cabs_status ON cabs(status);

CREATE TABLE trips (
    id                 uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    passenger_id       uuid NOT NULL REFERENCES users(id),
    cab_id             uuid REFERENCES cabs(id),
    status             text NOT NULL CHECK (status IN ('requested', 'assigned', 'in_progress', 'completed', 'cancelled')),
    pickup_place       text NOT NULL,
    drop_place         text NOT NULL,
    pickup_lat         double precision NOT NULL,
    pickup_lng         double precision NOT NULL,
    drop_lat           double precision NOT NULL,
    drop_lng           double precision NOT NULL,
    distance_m         int    NOT NULL CHECK (distance_m > 0),
    duration_s         int    NOT NULL CHECK (duration_s > 0),
    fare_paise         bigint NOT NULL CHECK (fare_paise > 0),
    idempotency_key    text,
    requested_at       timestamptz NOT NULL DEFAULT now(),
    assigned_at        timestamptz,
    started_at         timestamptz,
    completed_at       timestamptz,
    cancelled_at       timestamptz,
    updated_at         timestamptz NOT NULL DEFAULT now()
);
CREATE TRIGGER trips_updated_at BEFORE UPDATE ON trips FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE INDEX trips_passenger ON trips(passenger_id, requested_at DESC);
CREATE INDEX trips_cab_active ON trips(cab_id) WHERE status IN ('assigned', 'in_progress');
-- A passenger can hold at most one live trip; a cab can serve at most one.
CREATE UNIQUE INDEX trips_one_active_per_passenger ON trips(passenger_id)
    WHERE status IN ('requested', 'assigned', 'in_progress');
CREATE UNIQUE INDEX trips_one_active_per_cab ON trips(cab_id)
    WHERE status IN ('assigned', 'in_progress');
CREATE UNIQUE INDEX trips_idempotency ON trips(passenger_id, idempotency_key)
    WHERE idempotency_key IS NOT NULL;

CREATE TABLE wifi_plans (
    id          text PRIMARY KEY,
    name        text   NOT NULL,
    quota_bytes bigint NOT NULL CHECK (quota_bytes > 0),
    price_paise bigint NOT NULL CHECK (price_paise >= 0),
    active      boolean NOT NULL DEFAULT true
);

CREATE TABLE payments (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      uuid NOT NULL REFERENCES users(id),
    purpose      text NOT NULL CHECK (purpose IN ('wifi_plan', 'trip_fare')),
    reference    text NOT NULL,
    amount_paise bigint NOT NULL CHECK (amount_paise >= 0),
    provider     text NOT NULL,
    provider_ref text NOT NULL DEFAULT '',
    status       text NOT NULL CHECK (status IN ('pending', 'succeeded', 'failed')),
    created_at   timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX payments_user ON payments(user_id, created_at DESC);

-- Append-only. Balance = SUM(delta_bytes). Grants are positive, usage is negative.
CREATE TABLE quota_ledger (
    id          bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id     uuid   NOT NULL REFERENCES users(id),
    delta_bytes bigint NOT NULL CHECK (delta_bytes <> 0),
    reason      text   NOT NULL CHECK (reason IN ('signup_grant', 'plan_purchase', 'usage', 'adjustment')),
    payment_id  uuid REFERENCES payments(id),
    trip_id     uuid REFERENCES trips(id),
    -- Gateway report id: makes usage reports safe to retry.
    report_id   text UNIQUE,
    created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX quota_ledger_user ON quota_ledger(user_id, created_at DESC);

CREATE FUNCTION forbid_mutation() RETURNS trigger AS $$
BEGIN
    RAISE EXCEPTION '% is append-only', TG_TABLE_NAME;
END;
$$ LANGUAGE plpgsql;
CREATE TRIGGER quota_ledger_append_only BEFORE UPDATE OR DELETE ON quota_ledger
    FOR EACH ROW EXECUTE FUNCTION forbid_mutation();

INSERT INTO wifi_plans (id, name, quota_bytes, price_paise) VALUES
    ('ride-1g',  'Ride Pass 1 GiB',   1073741824,  2900),
    ('day-5g',   'Day Pass 5 GiB',    5368709120,  9900),
    ('week-20g', 'Week Pass 20 GiB', 21474836480, 29900);

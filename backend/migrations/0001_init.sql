-- Sarathi: Postgres is the source of truth; Redis holds the hot / live state.

CREATE TABLE IF NOT EXISTS car_specs (
    model        text PRIMARY KEY,
    display_name text NOT NULL,
    data         jsonb NOT NULL,
    updated_at   timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS roads (
    id           text PRIMARY KEY,
    name         text NOT NULL DEFAULT '',
    road_class   text NOT NULL DEFAULT 'unclassified',
    width_m      double precision NOT NULL DEFAULT 0,
    one_way      boolean NOT NULL DEFAULT false,
    lanes        int NOT NULL DEFAULT 0,
    max_height_m double precision NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS route_searches (
    id         bigserial PRIMARY KEY,
    user_id    text,
    car_model  text,
    origin_lat double precision, origin_lng double precision,
    dest_lat   double precision, dest_lng   double precision,
    routes     jsonb,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS petrol_stations (
    id        text PRIMARY KEY,
    name      text NOT NULL,
    brand     text NOT NULL DEFAULT '',
    address   text NOT NULL DEFAULT '',
    lat       double precision NOT NULL,
    lng       double precision NOT NULL,
    fuels     text[] NOT NULL DEFAULT '{petrol,diesel}',
    open_24x7 boolean NOT NULL DEFAULT false,
    active    boolean NOT NULL DEFAULT true
);

CREATE TABLE IF NOT EXISTS station_ratings (
    station_id    text NOT NULL REFERENCES petrol_stations(id),
    day           date NOT NULL,
    weekly_visits bigint NOT NULL,
    rating        double precision NOT NULL,
    PRIMARY KEY (station_id, day)
);

CREATE TABLE IF NOT EXISTS ev_stations (
    id          text PRIMARY KEY,
    name        text NOT NULL,
    operator    text NOT NULL DEFAULT '',
    lat         double precision NOT NULL,
    lng         double precision NOT NULL,
    connectors  text[] NOT NULL DEFAULT '{}',
    power_kw    double precision NOT NULL DEFAULT 0,
    total_ports int NOT NULL DEFAULT 1,
    active      boolean NOT NULL DEFAULT true
);

CREATE TABLE IF NOT EXISTS vehicle_locations (
    vehicle_id  text NOT NULL,
    trip_id     text,
    lat         double precision NOT NULL,
    lng         double precision NOT NULL,
    speed_kmh   double precision,
    recorded_at timestamptz NOT NULL
);
CREATE INDEX IF NOT EXISTS vehicle_locations_vehicle_time ON vehicle_locations (vehicle_id, recorded_at DESC);

CREATE TABLE IF NOT EXISTS fleet_vehicles (
    id        text PRIMARY KEY,
    model     text NOT NULL,
    seats     int NOT NULL,
    classes   text[] NOT NULL,  -- cab, seat5/seat7, shared, premium, vintage, tourist, testdrive
    dealer_id text
);

CREATE TABLE IF NOT EXISTS drivers (
    id         text PRIMARY KEY,
    name       text NOT NULL,
    phone      text NOT NULL,
    vehicle_id text NOT NULL REFERENCES fleet_vehicles(id),
    rating     double precision NOT NULL DEFAULT 4.5,
    active     boolean NOT NULL DEFAULT true
);

CREATE TABLE IF NOT EXISTS driver_earnings (
    driver_id  text NOT NULL,
    booking_id text NOT NULL,
    amount     numeric(12,2) NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS bookings (
    id           text PRIMARY KEY,
    user_id      text NOT NULL,
    type         text NOT NULL,
    state        text NOT NULL,
    class        text,
    driver_id    text,
    fare_total   numeric(12,2) NOT NULL DEFAULT 0,
    scheduled_at timestamptz,
    data         jsonb NOT NULL,
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz
);
CREATE INDEX IF NOT EXISTS bookings_user ON bookings (user_id, created_at DESC);

CREATE TABLE IF NOT EXISTS payments (
    id         text PRIMARY KEY,
    module     text NOT NULL,
    ref_id     text NOT NULL,
    user_id    text NOT NULL,
    order_id   text NOT NULL,
    payment_id text,
    amount     numeric(12,2) NOT NULL,
    refunded   numeric(12,2) NOT NULL DEFAULT 0,
    status     text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    paid_at    timestamptz,
    UNIQUE (module, ref_id)
);

CREATE TABLE IF NOT EXISTS refunds (
    id         text PRIMARY KEY,
    payment_id text NOT NULL REFERENCES payments(id),
    amount     numeric(12,2) NOT NULL,
    reason     text,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS parking_tariffs (
    hub_id       text PRIMARY KEY,
    name         text NOT NULL,
    free_minutes double precision NOT NULL DEFAULT 10,
    base_fee     double precision NOT NULL,
    per_hour     double precision NOT NULL,
    max_fee      double precision NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS premium_cars (
    id            text PRIMARY KEY,
    model         text NOT NULL,
    category      text NOT NULL CHECK (category IN ('premium','vintage')),
    city          text NOT NULL,
    year          int,
    seats         int NOT NULL DEFAULT 4,
    color         text,
    rate_per_hour numeric(10,2) NOT NULL,
    active        boolean NOT NULL DEFAULT true
);

CREATE TABLE IF NOT EXISTS transit_fares (
    mode      text NOT NULL,
    from_code text NOT NULL,
    to_code   text NOT NULL,
    fare      double precision NOT NULL,
    PRIMARY KEY (mode, from_code, to_code)
);

CREATE TABLE IF NOT EXISTS transit_journeys (
    id         text PRIMARY KEY,
    user_id    text NOT NULL,
    route_name text NOT NULL,
    total      numeric(10,2) NOT NULL,
    paid_at    timestamptz
);

CREATE TABLE IF NOT EXISTS transit_settlements (
    journey_id text NOT NULL,
    leg        int NOT NULL,
    operator   text,
    mode       text NOT NULL,
    amount     numeric(10,2) NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (journey_id, leg)
);

CREATE TABLE IF NOT EXISTS test_drives (
    id          text PRIMARY KEY,
    showroom_id text NOT NULL,
    model       text NOT NULL,
    customer_id text NOT NULL,
    status      text NOT NULL
);
-- one test drive per customer per model per showroom
CREATE UNIQUE INDEX IF NOT EXISTS test_drives_once
    ON test_drives (showroom_id, model, customer_id) WHERE status <> 'cancelled';

CREATE TABLE IF NOT EXISTS daily_stats (
    day  date PRIMARY KEY,
    data jsonb NOT NULL
);

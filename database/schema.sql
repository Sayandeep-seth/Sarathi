--
-- PostgreSQL database dump
--

\restrict omeaTB82BVdA0FV7hO60V3K7wgis2ntcsBhVCpNT0c4OfBT6ixMpXGlP6eVqEvq

-- Dumped from database version 18.6
-- Dumped by pg_dump version 18.6

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET transaction_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

--
-- Name: ai; Type: SCHEMA; Schema: -; Owner: -
--

CREATE SCHEMA ai;


--
-- Name: pgcrypto; Type: EXTENSION; Schema: -; Owner: -
--

CREATE EXTENSION IF NOT EXISTS pgcrypto WITH SCHEMA public;


--
-- Name: EXTENSION pgcrypto; Type: COMMENT; Schema: -; Owner: -
--

COMMENT ON EXTENSION pgcrypto IS 'cryptographic functions';


--
-- Name: fn_haversine_m(numeric, numeric, numeric, numeric); Type: FUNCTION; Schema: ai; Owner: -
--

CREATE FUNCTION ai.fn_haversine_m(lat1 numeric, lon1 numeric, lat2 numeric, lon2 numeric) RETURNS numeric
    LANGUAGE sql IMMUTABLE
    AS $$
    SELECT ROUND((6371000 * 2 * asin(sqrt(
        power(sin(radians(lat2-lat1)/2), 2) +
        cos(radians(lat1)) * cos(radians(lat2)) *
        power(sin(radians(lon2-lon1)/2), 2)
    )))::numeric, 1);
$$;


--
-- Name: fn_build_dialog_vars(bigint, text); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.fn_build_dialog_vars(p_seed bigint, p_intent text) RETURNS jsonb
    LANGUAGE plpgsql
    AS $$
DECLARE
    v JSONB; v_fare NUMERIC;
    v_km NUMERIC; v_h INT; v_days INT; v_p INT; v_q INT;
BEGIN
    v := fn_build_vars(p_seed);
    v_km   := (v->>'km')::numeric;
    v_h    := COALESCE((v->>'h')::int, 2);
    v_days := COALESCE((v->>'days')::int, 1);
    v_p    := COALESCE((v->>'p')::int, 1);
    v_q    := COALESCE((v->>'q')::int, 1);

    v_fare := CASE COALESCE(p_intent, 'book_cab')
        WHEN 'book_shared_cab'    THEN ROUND(30 + 10 * v_km, 0)
        WHEN 'book_pickup_return' THEN ROUND((50 + 14 * v_km) * 2 + 60, 0)
        WHEN 'book_premium_hourly'  THEN 500 + 450 * v_h
        WHEN 'book_vintage_hourly'  THEN 700 + 550 * v_h
        WHEN 'book_tourist_daily'   THEN ROUND(500 + 12 * v_km + 2500 * v_days, 0)
        WHEN 'book_self_drive'      THEN (v->>'band_fare')::numeric * v_days
        WHEN 'buy_daily_pass_qr'    THEN ROUND(20 + 9 * v_km, 0)
        WHEN 'book_bus_ticket'      THEN 899 * GREATEST(v_p, 2)
        WHEN 'subscribe_premium_bus' THEN 1200
        WHEN 'order_food_on_trip'   THEN 180 * GREATEST(v_q, 1)
        WHEN 'book_test_drive'      THEN 0
        WHEN 'wallet_payment'       THEN 500
        ELSE ROUND(50 + 15 * v_km, 0)
    END;

    v := jsonb_set(v, '{fare}',   to_jsonb(v_fare));
    v := jsonb_set(v, '{refund}', to_jsonb(v_fare));
    RETURN v;
END $$;


--
-- Name: fn_build_vars(bigint); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.fn_build_vars(p_seed bigint) RETURNS jsonb
    LANGUAGE plpgsql
    AS $$
DECLARE
    o RECORD; d RECORD; m RECORD; st RECORD; rt RECORD; fd RECORD;
    v_km NUMERIC; v_fare NUMERIC;
    v_i INT; v_points INT; v_seq INT;
    v_bands   TEXT[]     := ARRAY['0-10L','10-20L','20-35L','35-50L','50-60L','60-80L'];
    v_bfare   NUMERIC[]  := ARRAY[1500,2200,3000,4000,5000,6500];
    v_deposit NUMERIC[]  := ARRAY[3000,5000,8000,12000,15000,20000];
BEGIN
    SELECT * INTO o  FROM wb_locations  ORDER BY random() LIMIT 1;
    SELECT * INTO d  FROM wb_locations  ORDER BY random() LIMIT 1;
    SELECT * INTO m  FROM wb_car_models ORDER BY random() LIMIT 1;
    SELECT * INTO st FROM wb_stations   ORDER BY random() LIMIT 1;
    SELECT * INTO rt FROM wb_bus_routes ORDER BY random() LIMIT 1;
    SELECT * INTO fd FROM wb_food_items ORDER BY random() LIMIT 1;

    v_km := GREATEST(1, fn_wb_hav_km(o.latitude, o.longitude, d.latitude, d.longitude) * 1.3);
    v_fare := ROUND(50 + 15 * v_km, 0);
    v_i := 1 + floor(random() * 6)::int;
    v_points := 150 + floor(random() * 400)::int;
    v_seq := (1000 + p_seed) % 10000;

    RETURN jsonb_build_object(
        'o',   o.name,
        'd',   d.name,
        't',   (SELECT phrase FROM wb_times  ORDER BY random() LIMIT 1),
        'dt',  (SELECT phrase FROM wb_dates ORDER BY random() LIMIT 1),
        'p',   (1 + floor(random() * 6))::int,
        'm',   m.brand || ' ' || m.model_name,
        'h',   (ARRAY[2,3,4,6,8,12])[1 + floor(random() * 6)::int],
        'days',(ARRAY[1,2,3,4,5,7])[1 + floor(random() * 6)::int],
        'seat',(1 + floor(random() * 4))::int,
        'bat', (ARRAY[12,18,25,35,45,60])[1 + floor(random() * 6)::int],
        'f',   fd.item_name,
        'q',   (1 + floor(random() * 4))::int,
        'code',    'BK' || lpad((100000 + p_seed)::text, 6, '0'),
        'r',       rt.route_name,
        'station', st.name,
        'km',      ROUND(v_km, 1),
        'fare',    v_fare,
        'eta',     (5 + floor(random() * 30))::int,
        'band',      v_bands[v_i],
        'band_fare', v_bfare[v_i],
        'deposit',   v_deposit[v_i],
        'points',    v_points,
        'balance',   v_points + 500,
        'reg', 'WB-' || lpad((10 + floor(random() * 89))::int::text, 2, '0') || '-'
               || upper(substr(md5(p_seed::text), 1, 2)) || '-'
               || lpad(v_seq::text, 4, '0')
    );
END $$;


--
-- Name: fn_complete_pass(uuid); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.fn_complete_pass(p_qr uuid) RETURNS text
    LANGUAGE sql
    AS $$     UPDATE travel_passes SET status='used'
    WHERE qr_code = p_qr AND status='active'
      AND travel_date >= CURRENT_DATE;
    SELECT 'JOURNEY COMPLETED';
 $$;


--
-- Name: fn_export_jsonl(text); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.fn_export_jsonl(p_data_type text) RETURNS SETOF text
    LANGUAGE sql STABLE
    AS $$
    SELECT j.jsonl::text
    FROM v_jsonl_all j
    WHERE p_data_type = 'all' OR j.data_type = p_data_type
$$;


--
-- Name: fn_find_compatible_routes(bigint, bigint, bigint); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.fn_find_compatible_routes(p_origin_junction bigint, p_dest_junction bigint, p_model_id bigint) RETURNS TABLE(o_junction_path bigint[], o_road_path bigint[], o_total_km numeric, o_total_min numeric, o_min_clear_m numeric)
    LANGUAGE plpgsql STABLE
    AS $$ DECLARE v_car_w NUMERIC;
BEGIN
    SELECT width_cm / 100.0 INTO v_car_w FROM car_models WHERE model_id = p_model_id;
    IF v_car_w IS NULL THEN RAISE EXCEPTION 'Unknown car model %', p_model_id; END IF;

    RETURN QUERY
    WITH RECURSIVE walk AS (
        -- ANCHOR TERM: every numeric column explicitly cast to plain
        -- NUMERIC (::numeric) so its type matches the recursive term.
        SELECT e.to_node                                                  AS node,
               ARRAY[e.from_node, e.to_node]                              AS js,
               ARRAY[e.road_id]                                           AS rs,
               e.length_km::numeric                                       AS km,
               (e.length_km / NULLIF(e.speed_limit_kmph,0) * 60)::numeric AS mins,
               (e.width_m - (CASE WHEN e.road_type='two_way' THEN 2
                                  ELSE 1 END) * v_car_w)::numeric         AS clr
        FROM v_road_edges e
        WHERE e.from_node = p_origin_junction
          AND fn_road_fits_car(v_car_w, e.width_m, e.road_type)

        UNION ALL

        SELECT e.to_node,
               w.js || e.to_node,
               w.rs || e.road_id,
               w.km  + e.length_km,
               w.mins + (e.length_km / NULLIF(e.speed_limit_kmph,0) * 60),
               LEAST(w.clr, e.width_m - (CASE WHEN e.road_type='two_way' THEN 2
                                              ELSE 1 END) * v_car_w)
        FROM v_road_edges e
        JOIN walk w ON e.from_node = w.node
        WHERE NOT (e.to_node = ANY(w.js))
          AND cardinality(w.js) < 25
          AND fn_road_fits_car(v_car_w, e.width_m, e.road_type)
    )
    SELECT w.js, w.rs, ROUND(w.km,2), ROUND(w.mins,1), ROUND(w.clr,2)
    FROM walk w
    WHERE w.node = p_dest_junction;
END;
 $$;


--
-- Name: fn_find_stations(numeric, numeric, bigint, numeric, numeric); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.fn_find_stations(p_lat numeric, p_lon numeric, p_model_id bigint, p_current_fuel numeric, p_radius_km numeric DEFAULT 15) RETURNS TABLE(o_station_id bigint, o_name character varying, o_type character varying, o_km_away numeric, o_gps_crowd_rating numeric, o_fuel_price numeric, o_can_reach_with_current_fuel boolean)
    LANGUAGE plpgsql STABLE
    AS $$ DECLARE v_fuel TEXT; v_frac NUMERIC; v_range NUMERIC;
BEGIN
    SELECT fuel_type, range_km, tank_litres
      INTO v_fuel, v_range, v_frac
    FROM car_models WHERE model_id = p_model_id;

    IF v_fuel = 'electric' THEN
        v_frac := p_current_fuel / 100.0;              -- battery percent
    ELSE
        v_frac := p_current_fuel / NULLIF(v_frac,0);   -- tank capacity
    END IF;

    RETURN QUERY
    SELECT s.station_id, s.name, s.station_type,
           ROUND(fn_haversine_km(p_lat,p_lon,s.latitude,s.longitude),2),
           fn_station_crowd_rating(s.station_id),
           CASE WHEN v_fuel='electric' THEN NULL ELSE s.petrol_price END,
           (fn_haversine_km(p_lat,p_lon,s.latitude,s.longitude) <= v_range * v_frac)
    FROM fuel_stations s
    WHERE s.station_type IN (CASE WHEN v_fuel='electric' THEN 'ev' ELSE 'petrol' END, 'both')
      AND fn_haversine_km(p_lat,p_lon,s.latitude,s.longitude) <= p_radius_km
    ORDER BY fn_haversine_km(p_lat,p_lon,s.latitude,s.longitude);
END;
 $$;


--
-- Name: fn_haversine_km(numeric, numeric, numeric, numeric); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.fn_haversine_km(lat1 numeric, lon1 numeric, lat2 numeric, lon2 numeric) RETURNS numeric
    LANGUAGE sql IMMUTABLE
    AS $$     SELECT ROUND((6371 * 2 * asin(sqrt(
        power(sin(radians(lat2-lat1)/2), 2) +
        cos(radians(lat1)) * cos(radians(lat2)) *
        power(sin(radians(lon2-lon1)/2), 2)
    )))::numeric, 3);
 $$;


--
-- Name: fn_log_station_traffic(bigint, integer, numeric); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.fn_log_station_traffic(p_station_id bigint, p_cars integer, p_wait_min numeric DEFAULT NULL::numeric) RETURNS void
    LANGUAGE sql
    AS $$     INSERT INTO station_traffic_log(station_id, cars_count, avg_wait_min)
    VALUES (p_station_id, p_cars, p_wait_min);
 $$;


--
-- Name: fn_log_test_drive_cab_income(bigint, numeric); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.fn_log_test_drive_cab_income(p_test_drive_id bigint, p_cab_fare numeric) RETURNS numeric
    LANGUAGE plpgsql
    AS $$ DECLARE v_rate NUMERIC; v_dealer BIGINT; v_amt NUMERIC;
BEGIN
    SELECT s.commission_rate, s.dealer_user_id INTO v_rate, v_dealer
    FROM test_drive_bookings td JOIN showrooms s USING (showroom_id)
    WHERE td.test_drive_id = p_test_drive_id;

    v_amt := ROUND(p_cab_fare * v_rate / 100.0, 2);
    UPDATE test_drive_bookings SET served_as_cab = TRUE
    WHERE test_drive_id = p_test_drive_id;
    INSERT INTO commissions (bus_owner_id, source, amount)  -- table created in Step 7
    VALUES (v_dealer, 'test_drive_cab', v_amt);
    RETURN v_amt;
END $$;


--
-- Name: fn_pick(text[]); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.fn_pick(p_arr text[]) RETURNS text
    LANGUAGE sql
    AS $$
    SELECT p_arr[1 + floor(random() * array_length(p_arr, 1))::int]
$$;


--
-- Name: fn_road_fits_car(numeric, numeric, text); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.fn_road_fits_car(p_car_width_m numeric, p_road_width_m numeric, p_road_type text) RETURNS boolean
    LANGUAGE sql IMMUTABLE
    AS $$     SELECT CASE
        WHEN p_road_type = 'one_way' THEN p_road_width_m >= p_car_width_m + 0.3
        WHEN p_road_type = 'two_way' THEN p_road_width_m >= 2*p_car_width_m + 0.6
        ELSE FALSE
    END;
 $$;


--
-- Name: fn_road_fits_car(numeric, numeric, character varying); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.fn_road_fits_car(p_car_width_m numeric, p_road_width_m numeric, p_road_type character varying) RETURNS boolean
    LANGUAGE sql IMMUTABLE
    AS $$
    SELECT (p_road_width_m
            - (CASE WHEN p_road_type = 'two_way' THEN 2 ELSE 1 END)
              * p_car_width_m) > 0;
$$;


--
-- Name: fn_scan_pass(uuid, text, bigint); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.fn_scan_pass(p_qr uuid, p_transport text, p_location bigint DEFAULT NULL::bigint) RETURNS text
    LANGUAGE plpgsql
    AS $$ DECLARE v_pass RECORD;
BEGIN
    SELECT * INTO v_pass FROM travel_passes WHERE qr_code = p_qr FOR UPDATE;
    IF NOT FOUND THEN RETURN 'INVALID QR'; END IF;
    IF v_pass.status <> 'active' THEN RETURN 'PASS ' || v_pass.status; END IF;
    IF v_pass.travel_date < CURRENT_DATE THEN
        UPDATE travel_passes SET status='expired' WHERE pass_id = v_pass.pass_id;
        RETURN 'PASS EXPIRED';
    END IF;
    INSERT INTO pass_scans (pass_id, transport_type, scan_location)
    VALUES (v_pass.pass_id, p_transport, p_location);
    RETURN 'SCAN OK — WELCOME';
END $$;


--
-- Name: fn_station_crowd_rating(bigint); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.fn_station_crowd_rating(p_station_id bigint) RETURNS numeric
    LANGUAGE sql STABLE
    AS $$     SELECT ROUND(LEAST(5, GREATEST(1, 5 - (COALESCE(AVG(cars_count),0) / 5.0)))::numeric, 2)
    FROM station_traffic_log
    WHERE station_id = p_station_id
      AND detected_at > now() - INTERVAL '7 days';
 $$;


--
-- Name: fn_subst(text, jsonb); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.fn_subst(p_tmpl text, p_vars jsonb) RETURNS text
    LANGUAGE plpgsql IMMUTABLE
    AS $_$
DECLARE
    k TEXT;
    v TEXT;
    s TEXT := p_tmpl;
BEGIN
    FOR k, v IN SELECT key, value FROM jsonb_each_text(p_vars) LOOP
        -- numeric variable in a quoted JSON slot: emit raw number
        IF v ~ '^-?[0-9]+(\.[0-9]+)?$' THEN
            s := regexp_replace(s, '"\{' || k || '\}"', v, 'g');
        END IF;
        -- normal replacement (plain-text frames + quoted strings)
        s := replace(s, '{' || k || '}', v);
    END LOOP;
    RETURN s;
END $_$;


--
-- Name: fn_wb_hav_km(numeric, numeric, numeric, numeric); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.fn_wb_hav_km(lat1 numeric, lon1 numeric, lat2 numeric, lon2 numeric) RETURNS numeric
    LANGUAGE sql IMMUTABLE
    AS $$
    SELECT ROUND((6371 * 2 * asin(sqrt(
        power(sin(radians(lat2 - lat1) / 2), 2) +
        cos(radians(lat1)) * cos(radians(lat2)) *
        power(sin(radians(lon2 - lon1) / 2), 2)
    )))::numeric, 3)
$$;


--
-- Name: trg_fn_booking_before(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trg_fn_booking_before() RETURNS trigger
    LANGUAGE plpgsql
    AS $$ DECLARE
    c RECORD; band RECORD;
    v_hours NUMERIC := 0; v_days NUMERIC := 1;
    v_seats INT; v_loc_type VARCHAR;
    cap SMALLINT; filled SMALLINT;
BEGIN
    SELECT * INTO c FROM fare_config WHERE service_type = NEW.booking_type;
    IF NOT FOUND THEN
        RAISE EXCEPTION 'No fare configuration for service "%"', NEW.booking_type;
    END IF;

    IF NEW.end_time IS NOT NULL THEN
        v_hours := GREATEST(1, CEIL(EXTRACT(EPOCH FROM (NEW.end_time - NEW.start_time))/3600.0));
        v_days  := GREATEST(1, CEIL(EXTRACT(EPOCH FROM (NEW.end_time - NEW.start_time))/86400.0));
    END IF;

    NEW.parking_fee := 0;   -- parking fee ONLY for pickup_return

    CASE NEW.booking_type

        WHEN 'cab' THEN                    -- 2.i.1 normal cab: no parking fee
            NEW.fare := c.base_fare + c.per_km * COALESCE(NEW.est_distance_km,0);

        WHEN 'shared_cab' THEN             -- 2.i.2 shared cab: cheaper per seat
            IF NEW.shared_ride_id IS NOT NULL THEN
                SELECT seat_capacity, seats_booked INTO cap, filled
                FROM shared_rides WHERE ride_id = NEW.shared_ride_id FOR UPDATE;
                IF cap IS NULL THEN RAISE EXCEPTION 'Shared ride not found'; END IF;
                IF filled >= cap THEN
                    RAISE EXCEPTION 'Shared ride % is full', NEW.shared_ride_id;
                END IF;
                UPDATE shared_rides SET seats_booked = seats_booked + 1
                WHERE ride_id = NEW.shared_ride_id;
                SELECT fare_per_seat INTO NEW.fare
                FROM shared_rides WHERE ride_id = NEW.shared_ride_id;
            ELSE
                NEW.fare := c.base_fare + c.per_km * COALESCE(NEW.est_distance_km,0);
            END IF;

        WHEN 'pickup_return' THEN          -- 2.i.3 airport/station pickup + return
            SELECT seating INTO v_seats FROM car_models WHERE model_id = NEW.model_id;
            IF v_seats NOT IN (5,7) THEN
                RAISE EXCEPTION 'Pickup/Return allows ONLY 5-seater or 7-seater (got %)',
                                COALESCE(v_seats,0);
            END IF;
            SELECT location_type INTO v_loc_type FROM locations
            WHERE location_id = NEW.destination_id;
            IF v_loc_type NOT IN ('airport','railway_station','metro_station','bus_stand') THEN
                RAISE EXCEPTION 'Pickup/Return destination must be airport/railway/metro/bus stand';
            END IF;
            NEW.fare        := c.base_fare + c.per_km * COALESCE(NEW.est_distance_km,0) * 2;
            NEW.parking_fee := c.parking_fee;   -- parking fee charged here

        WHEN 'premium_hourly' THEN         -- 2.ii premium/vintage, model selectable, hourly
            NEW.fare := c.base_fare + c.per_hour * v_hours;

        WHEN 'tourist_daily' THEN          -- 2.iii tourist trip, day-wise
            NEW.fare := c.base_fare + c.per_day * v_days
                      + c.per_km * COALESCE(NEW.est_distance_km,0);

        WHEN 'self_drive' THEN             -- 2.iv self-drive: band decides fare+deposit
            SELECT daily_fare, deposit INTO band
            FROM self_drive_price_bands
            WHERE (SELECT price_lakh FROM car_models WHERE model_id = NEW.model_id)
                    BETWEEN min_price_lakh AND max_price_lakh;
            IF band.daily_fare IS NULL THEN
                RAISE EXCEPTION 'No self-drive price band covers this model';
            END IF;
            NEW.fare           := band.daily_fare * v_days;
            NEW.deposit_amount := band.deposit;

        ELSE
            NEW.fare := COALESCE(NULLIF(NEW.fare,0), c.base_fare);
    END CASE;

    RETURN NEW;
END $$;


--
-- Name: trg_fn_food_item_after(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trg_fn_food_item_after() RETURNS trigger
    LANGUAGE plpgsql
    AS $$ DECLARE v_total NUMERIC; v_owner BIGINT;
BEGIN
    SELECT COALESCE(SUM(unit_price*quantity),0) INTO v_total
    FROM food_order_items WHERE order_id = NEW.order_id;

    UPDATE food_orders SET total_amount = v_total
    WHERE order_id = NEW.order_id;

    SELECT b.owner_user_id INTO v_owner
    FROM food_orders fo
    JOIN bus_tickets tk ON tk.ticket_id = fo.ticket_id
    JOIN luxury_bus_trips t ON t.trip_id = tk.trip_id
    JOIN buses b ON b.bus_id = t.bus_id
    WHERE fo.order_id = NEW.order_id;

    IF NOT EXISTS (SELECT 1 FROM commissions
                   WHERE order_id = NEW.order_id AND source='food_order') THEN
        INSERT INTO commissions (bus_owner_id, order_id, source, amount)
        VALUES (v_owner, NEW.order_id, 'food_order', ROUND(v_total * 0.12, 2));
    ELSE
        UPDATE commissions SET amount = ROUND(v_total * 0.12, 2)
        WHERE order_id = NEW.order_id AND source='food_order';
        UPDATE food_orders SET commission_amount = ROUND(v_total * 0.12, 2)
        WHERE order_id = NEW.order_id;
    END IF;
    RETURN NEW;
END $$;


--
-- Name: trg_fn_pass_fare(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trg_fn_pass_fare() RETURNS trigger
    LANGUAGE plpgsql
    AS $$ DECLARE v_km NUMERIC; c RECORD;
        la1 NUMERIC; lo1 NUMERIC; la2 NUMERIC; lo2 NUMERIC;
BEGIN
    IF NEW.total_fare = 0 THEN
        SELECT l1.latitude, l1.longitude, l2.latitude, l2.longitude
        INTO la1, lo1, la2, lo2
        FROM daily_routes dr
        JOIN locations l1 ON l1.location_id = dr.origin_id
        JOIN locations l2 ON l2.location_id = dr.destination_id
        WHERE dr.route_id = NEW.route_id;

        v_km := fn_haversine_km(la1, lo1, la2, lo2) * 1.3;  -- road factor
        SELECT * INTO c FROM fare_config WHERE service_type = 'daily_pass_qr';
        NEW.total_fare := ROUND(c.base_fare + c.per_km * v_km, 2);
    END IF;
    RETURN NEW;
END $$;


--
-- Name: trg_fn_test_drive_area(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.trg_fn_test_drive_area() RETURNS trigger
    LANGUAGE plpgsql
    AS $$ DECLARE v_s RECORD; v_lat NUMERIC; v_lon NUMERIC; v_km NUMERIC;
BEGIN
    SELECT * INTO v_s FROM showrooms WHERE showroom_id = NEW.showroom_id;
    SELECT l.latitude, l.longitude INTO v_lat, v_lon
    FROM bookings b JOIN locations l ON l.location_id = b.origin_id
    WHERE b.booking_id = NEW.booking_id;

    v_km := fn_haversine_km(v_lat, v_lon, v_s.latitude, v_s.longitude);
    IF v_km > v_s.test_drive_radius_km THEN
        RAISE EXCEPTION 'Start point is % km away — outside showroom test-drive area (max % km)',
                        v_km, v_s.test_drive_radius_km;
    END IF;
    RETURN NEW;
END $$;


SET default_tablespace = '';

SET default_table_access_method = heap;

--
-- Name: bookings; Type: TABLE; Schema: ai; Owner: -
--

CREATE TABLE ai.bookings (
    id bigint NOT NULL,
    rider_id bigint,
    driver_id bigint,
    pickup_lat numeric(10,7) NOT NULL,
    pickup_lon numeric(10,7) NOT NULL,
    dest_lat numeric(10,7) NOT NULL,
    dest_lon numeric(10,7) NOT NULL,
    vehicle_id bigint,
    fare numeric(10,2),
    status character varying(15) DEFAULT 'requested'::character varying NOT NULL,
    created_at timestamp with time zone DEFAULT now(),
    CONSTRAINT bookings_status_check CHECK (((status)::text = ANY ((ARRAY['requested'::character varying, 'assigned'::character varying, 'ongoing'::character varying, 'completed'::character varying, 'cancelled'::character varying])::text[])))
);


--
-- Name: bookings_id_seq; Type: SEQUENCE; Schema: ai; Owner: -
--

ALTER TABLE ai.bookings ALTER COLUMN id ADD GENERATED BY DEFAULT AS IDENTITY (
    SEQUENCE NAME ai.bookings_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: data_sources; Type: TABLE; Schema: ai; Owner: -
--

CREATE TABLE ai.data_sources (
    id bigint NOT NULL,
    provider character varying(80) NOT NULL,
    license character varying(120),
    source_url text,
    retrieved_at timestamp with time zone,
    attribution_text text,
    notes text
);


--
-- Name: data_sources_id_seq; Type: SEQUENCE; Schema: ai; Owner: -
--

ALTER TABLE ai.data_sources ALTER COLUMN id ADD GENERATED BY DEFAULT AS IDENTITY (
    SEQUENCE NAME ai.data_sources_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: ev_charging_stations; Type: TABLE; Schema: ai; Owner: -
--

CREATE TABLE ai.ev_charging_stations (
    id bigint NOT NULL,
    name character varying(120) NOT NULL,
    lat numeric(10,7) NOT NULL,
    lon numeric(10,7) NOT NULL,
    connector_types text[] DEFAULT ARRAY['CCS2'::text] NOT NULL,
    power_kw numeric(6,1),
    availability character varying(20) DEFAULT 'available'::character varying NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: ev_charging_stations_id_seq; Type: SEQUENCE; Schema: ai; Owner: -
--

ALTER TABLE ai.ev_charging_stations ALTER COLUMN id ADD GENERATED BY DEFAULT AS IDENTITY (
    SEQUENCE NAME ai.ev_charging_stations_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: fuel_stations; Type: TABLE; Schema: ai; Owner: -
--

CREATE TABLE ai.fuel_stations (
    id bigint NOT NULL,
    name character varying(120) NOT NULL,
    brand character varying(50),
    lat numeric(10,7) NOT NULL,
    lon numeric(10,7) NOT NULL,
    fuel_types text[] DEFAULT ARRAY['petrol'::text, 'diesel'::text] NOT NULL,
    availability character varying(20) DEFAULT 'open'::character varying NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: fuel_stations_id_seq; Type: SEQUENCE; Schema: ai; Owner: -
--

ALTER TABLE ai.fuel_stations ALTER COLUMN id ADD GENERATED BY DEFAULT AS IDENTITY (
    SEQUENCE NAME ai.fuel_stations_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: road_restrictions; Type: TABLE; Schema: ai; Owner: -
--

CREATE TABLE ai.road_restrictions (
    id bigint NOT NULL,
    road_id bigint NOT NULL,
    restriction_type character varying(30) NOT NULL,
    value text,
    source character varying(80),
    verified_at timestamp with time zone,
    notes text,
    CONSTRAINT road_restrictions_restriction_type_check CHECK (((restriction_type)::text = ANY ((ARRAY['max_height'::character varying, 'max_weight'::character varying, 'max_width'::character varying, 'no_entry'::character varying, 'time_restricted'::character varying, 'vehicle_ban'::character varying])::text[])))
);


--
-- Name: road_restrictions_id_seq; Type: SEQUENCE; Schema: ai; Owner: -
--

ALTER TABLE ai.road_restrictions ALTER COLUMN id ADD GENERATED BY DEFAULT AS IDENTITY (
    SEQUENCE NAME ai.road_restrictions_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: roads; Type: TABLE; Schema: ai; Owner: -
--

CREATE TABLE ai.roads (
    id bigint NOT NULL,
    osm_way_id bigint,
    road_name character varying(200) NOT NULL,
    road_class character varying(30) DEFAULT 'residential'::character varying NOT NULL,
    oneway boolean DEFAULT false NOT NULL,
    lanes smallint,
    width_m numeric(5,2),
    max_height_m numeric(4,2),
    max_weight_t numeric(6,2),
    surface character varying(30),
    start_lat numeric(10,7) NOT NULL,
    start_lon numeric(10,7) NOT NULL,
    end_lat numeric(10,7) NOT NULL,
    end_lon numeric(10,7) NOT NULL,
    length_m numeric(8,1),
    source_id bigint,
    verified_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now(),
    CONSTRAINT roads_road_class_check CHECK (((road_class)::text = ANY ((ARRAY['motorway'::character varying, 'trunk'::character varying, 'primary'::character varying, 'secondary'::character varying, 'tertiary'::character varying, 'unclassified'::character varying, 'residential'::character varying, 'service'::character varying, 'lane'::character varying])::text[]))),
    CONSTRAINT roads_width_m_check CHECK (((width_m IS NULL) OR (width_m > (0)::numeric)))
);


--
-- Name: COLUMN roads.oneway; Type: COMMENT; Schema: ai; Owner: -
--

COMMENT ON COLUMN ai.roads.oneway IS 'Direction tag (OSM oneway). It does NOT prove two vehicles can pass each other; that requires verified width_m.';


--
-- Name: COLUMN roads.width_m; Type: COMMENT; Schema: ai; Owner: -
--

COMMENT ON COLUMN ai.roads.width_m IS 'Usable motorable width in metres, from verified sources only. NULL means unverified: routing and LLM code must treat it as unknown, never fill it with a guess.';


--
-- Name: COLUMN roads.start_lat; Type: COMMENT; Schema: ai; Owner: -
--

COMMENT ON COLUMN ai.roads.start_lat IS 'Centerline endpoints only (no line geometry in this fallback). Demo coordinates are APPROXIMATE - replace with OSM-imported values.';


--
-- Name: roads_id_seq; Type: SEQUENCE; Schema: ai; Owner: -
--

ALTER TABLE ai.roads ALTER COLUMN id ADD GENERATED BY DEFAULT AS IDENTITY (
    SEQUENCE NAME ai.roads_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: traffic_conditions; Type: TABLE; Schema: ai; Owner: -
--

CREATE TABLE ai.traffic_conditions (
    id bigint NOT NULL,
    road_id bigint NOT NULL,
    speed_kmh numeric(5,1),
    congestion_level character varying(12) NOT NULL,
    observed_at timestamp with time zone DEFAULT now() NOT NULL,
    expires_at timestamp with time zone NOT NULL,
    source character varying(80),
    CONSTRAINT traffic_conditions_congestion_level_check CHECK (((congestion_level)::text = ANY ((ARRAY['free'::character varying, 'light'::character varying, 'moderate'::character varying, 'heavy'::character varying, 'gridlock'::character varying, 'closed'::character varying])::text[])))
);


--
-- Name: traffic_conditions_id_seq; Type: SEQUENCE; Schema: ai; Owner: -
--

ALTER TABLE ai.traffic_conditions ALTER COLUMN id ADD GENERATED BY DEFAULT AS IDENTITY (
    SEQUENCE NAME ai.traffic_conditions_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: traffic_signals; Type: TABLE; Schema: ai; Owner: -
--

CREATE TABLE ai.traffic_signals (
    id bigint NOT NULL,
    road_id bigint,
    lat numeric(10,7) NOT NULL,
    lon numeric(10,7) NOT NULL,
    cycle_seconds integer,
    data_source character varying(80),
    observed_at timestamp with time zone
);


--
-- Name: traffic_signals_id_seq; Type: SEQUENCE; Schema: ai; Owner: -
--

ALTER TABLE ai.traffic_signals ALTER COLUMN id ADD GENERATED BY DEFAULT AS IDENTITY (
    SEQUENCE NAME ai.traffic_signals_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: training_examples; Type: TABLE; Schema: ai; Owner: -
--

CREATE TABLE ai.training_examples (
    id bigint NOT NULL,
    category character varying(60) NOT NULL,
    input_json jsonb NOT NULL,
    context_json jsonb,
    expected_output_json jsonb NOT NULL,
    quality_score numeric(3,2),
    source_id bigint,
    split character varying(12) DEFAULT 'train'::character varying NOT NULL,
    created_at timestamp with time zone DEFAULT now(),
    CONSTRAINT training_examples_quality_score_check CHECK (((quality_score >= (0)::numeric) AND (quality_score <= (1)::numeric))),
    CONSTRAINT training_examples_split_check CHECK (((split)::text = ANY ((ARRAY['train'::character varying, 'validation'::character varying, 'test'::character varying])::text[])))
);


--
-- Name: training_examples_id_seq; Type: SEQUENCE; Schema: ai; Owner: -
--

ALTER TABLE ai.training_examples ALTER COLUMN id ADD GENERATED BY DEFAULT AS IDENTITY (
    SEQUENCE NAME ai.training_examples_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: vehicles; Type: TABLE; Schema: ai; Owner: -
--

CREATE TABLE ai.vehicles (
    id bigint NOT NULL,
    make character varying(50) NOT NULL,
    model character varying(80) NOT NULL,
    variant character varying(80),
    model_year integer,
    width_mm integer NOT NULL,
    height_mm integer,
    length_mm integer,
    turning_radius_mm integer,
    curb_weight_kg integer,
    source_id bigint,
    created_at timestamp with time zone DEFAULT now(),
    CONSTRAINT vehicles_width_mm_check CHECK (((width_mm >= 1200) AND (width_mm <= 3000)))
);


--
-- Name: vehicles_id_seq; Type: SEQUENCE; Schema: ai; Owner: -
--

ALTER TABLE ai.vehicles ALTER COLUMN id ADD GENERATED BY DEFAULT AS IDENTITY (
    SEQUENCE NAME ai.vehicles_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: bookings; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.bookings (
    booking_id bigint NOT NULL,
    booking_code uuid DEFAULT gen_random_uuid() NOT NULL,
    user_id bigint NOT NULL,
    booking_type character varying(25) NOT NULL,
    model_id bigint,
    vehicle_id bigint,
    driver_id bigint,
    shared_ride_id bigint,
    origin_id bigint,
    destination_id bigint,
    start_time timestamp with time zone DEFAULT now() NOT NULL,
    end_time timestamp with time zone,
    est_distance_km numeric(8,2),
    fare numeric(10,2) DEFAULT 0 NOT NULL,
    parking_fee numeric(8,2) DEFAULT 0 NOT NULL,
    deposit_amount numeric(10,2) DEFAULT 0 NOT NULL,
    passenger_count smallint DEFAULT 1 NOT NULL,
    status character varying(15) DEFAULT 'pending'::character varying NOT NULL,
    created_at timestamp with time zone DEFAULT now(),
    CONSTRAINT bookings_booking_type_check CHECK (((booking_type)::text = ANY ((ARRAY['cab'::character varying, 'shared_cab'::character varying, 'pickup_return'::character varying, 'premium_hourly'::character varying, 'tourist_daily'::character varying, 'self_drive'::character varying, 'test_drive'::character varying])::text[]))),
    CONSTRAINT bookings_status_check CHECK (((status)::text = ANY ((ARRAY['pending'::character varying, 'confirmed'::character varying, 'ongoing'::character varying, 'completed'::character varying, 'cancelled'::character varying])::text[]))),
    CONSTRAINT chk_model_required CHECK (((((booking_type)::text = ANY ((ARRAY['pickup_return'::character varying, 'premium_hourly'::character varying, 'tourist_daily'::character varying, 'self_drive'::character varying, 'test_drive'::character varying])::text[])) AND (model_id IS NOT NULL)) OR ((booking_type)::text = ANY ((ARRAY['cab'::character varying, 'shared_cab'::character varying])::text[]))))
);


--
-- Name: bookings_booking_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.bookings ALTER COLUMN booking_id ADD GENERATED BY DEFAULT AS IDENTITY (
    SEQUENCE NAME public.bookings_booking_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: bus_routes; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.bus_routes (
    route_id bigint NOT NULL,
    bus_id bigint NOT NULL,
    route_name character varying(100) NOT NULL
);


--
-- Name: bus_routes_route_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.bus_routes ALTER COLUMN route_id ADD GENERATED BY DEFAULT AS IDENTITY (
    SEQUENCE NAME public.bus_routes_route_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: bus_stops; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.bus_stops (
    stop_id bigint NOT NULL,
    route_id bigint NOT NULL,
    stop_seq smallint NOT NULL,
    location_id bigint NOT NULL,
    can_drop_at_workplace boolean DEFAULT false
);


--
-- Name: bus_stops_stop_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.bus_stops ALTER COLUMN stop_id ADD GENERATED BY DEFAULT AS IDENTITY (
    SEQUENCE NAME public.bus_stops_stop_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: bus_tickets; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.bus_tickets (
    ticket_id bigint NOT NULL,
    trip_id bigint NOT NULL,
    user_id bigint NOT NULL,
    seat_no smallint NOT NULL,
    fare numeric(10,2) NOT NULL,
    status character varying(12) DEFAULT 'booked'::character varying,
    booked_at timestamp with time zone DEFAULT now(),
    CONSTRAINT bus_tickets_status_check CHECK (((status)::text = ANY ((ARRAY['booked'::character varying, 'cancelled'::character varying, 'completed'::character varying])::text[])))
);


--
-- Name: bus_tickets_ticket_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.bus_tickets ALTER COLUMN ticket_id ADD GENERATED BY DEFAULT AS IDENTITY (
    SEQUENCE NAME public.bus_tickets_ticket_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: buses; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.buses (
    bus_id bigint NOT NULL,
    registration_no character varying(20) NOT NULL,
    bus_type character varying(15) NOT NULL,
    owner_user_id bigint NOT NULL,
    seat_capacity smallint NOT NULL,
    has_pantry boolean DEFAULT false,
    amenities text,
    CONSTRAINT buses_bus_type_check CHECK (((bus_type)::text = ANY ((ARRAY['premium_daily'::character varying, 'luxury'::character varying])::text[])))
);


--
-- Name: buses_bus_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.buses ALTER COLUMN bus_id ADD GENERATED BY DEFAULT AS IDENTITY (
    SEQUENCE NAME public.buses_bus_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: car_models; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.car_models (
    model_id bigint NOT NULL,
    brand character varying(50) NOT NULL,
    model_name character varying(80) NOT NULL,
    category character varying(30) NOT NULL,
    length_cm integer NOT NULL,
    width_cm integer NOT NULL,
    height_cm integer NOT NULL,
    seating integer NOT NULL,
    fuel_type character varying(15) NOT NULL,
    tank_litres numeric(6,2),
    battery_kwh numeric(6,2),
    range_km numeric(7,2),
    price_lakh numeric(8,2) NOT NULL,
    is_premium boolean DEFAULT false,
    is_vintage boolean DEFAULT false,
    is_self_drive boolean DEFAULT false,
    CONSTRAINT car_models_category_check CHECK (((category)::text = ANY ((ARRAY['hatchback'::character varying, 'sedan'::character varying, 'suv'::character varying, 'muv'::character varying, 'premium'::character varying, 'vintage'::character varying])::text[]))),
    CONSTRAINT car_models_fuel_type_check CHECK (((fuel_type)::text = ANY ((ARRAY['petrol'::character varying, 'diesel'::character varying, 'cng'::character varying, 'electric'::character varying, 'hybrid'::character varying])::text[])))
);


--
-- Name: car_models_model_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.car_models ALTER COLUMN model_id ADD GENERATED BY DEFAULT AS IDENTITY (
    SEQUENCE NAME public.car_models_model_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: commissions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.commissions (
    commission_id bigint NOT NULL,
    bus_owner_id bigint NOT NULL,
    order_id bigint,
    source character varying(20) NOT NULL,
    amount numeric(10,2) NOT NULL,
    created_at timestamp with time zone DEFAULT now(),
    CONSTRAINT commissions_source_check CHECK (((source)::text = ANY ((ARRAY['food_order'::character varying, 'ticket'::character varying, 'test_drive_cab'::character varying])::text[])))
);


--
-- Name: commissions_commission_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.commissions ALTER COLUMN commission_id ADD GENERATED BY DEFAULT AS IDENTITY (
    SEQUENCE NAME public.commissions_commission_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: daily_routes; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.daily_routes (
    route_id bigint NOT NULL,
    user_id bigint NOT NULL,
    route_name character varying(80),
    origin_id bigint NOT NULL,
    destination_id bigint NOT NULL,
    is_active boolean DEFAULT true,
    created_at timestamp with time zone DEFAULT now()
);


--
-- Name: daily_routes_route_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.daily_routes ALTER COLUMN route_id ADD GENERATED BY DEFAULT AS IDENTITY (
    SEQUENCE NAME public.daily_routes_route_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: dialogue_turns; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.dialogue_turns (
    turn_id bigint NOT NULL,
    dialogue_id bigint NOT NULL,
    turn_no smallint NOT NULL,
    speaker character varying(10) NOT NULL,
    utterance text NOT NULL,
    lang character varying(10) DEFAULT 'en'::character varying NOT NULL,
    intent_label character varying(30),
    CONSTRAINT dialogue_turns_lang_check CHECK (((lang)::text = ANY ((ARRAY['en'::character varying, 'hi-en'::character varying])::text[]))),
    CONSTRAINT dialogue_turns_speaker_check CHECK (((speaker)::text = ANY ((ARRAY['user'::character varying, 'assistant'::character varying])::text[])))
);


--
-- Name: dialogue_turns_turn_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.dialogue_turns ALTER COLUMN turn_id ADD GENERATED BY DEFAULT AS IDENTITY (
    SEQUENCE NAME public.dialogue_turns_turn_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: driver_profiles; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.driver_profiles (
    driver_id bigint NOT NULL,
    user_id bigint NOT NULL,
    license_no character varying(30) NOT NULL,
    rating numeric(3,2) DEFAULT 5.00,
    total_trips integer DEFAULT 0,
    is_online boolean DEFAULT false,
    current_lat numeric(10,7),
    current_lon numeric(10,7)
);


--
-- Name: driver_profiles_driver_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.driver_profiles ALTER COLUMN driver_id ADD GENERATED BY DEFAULT AS IDENTITY (
    SEQUENCE NAME public.driver_profiles_driver_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: fare_config; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.fare_config (
    service_type character varying(30) NOT NULL,
    base_fare numeric(8,2) DEFAULT 0 NOT NULL,
    per_km numeric(8,2) DEFAULT 0 NOT NULL,
    per_hour numeric(8,2) DEFAULT 0,
    per_day numeric(8,2) DEFAULT 0,
    parking_fee numeric(8,2) DEFAULT 0,
    min_fare numeric(8,2) DEFAULT 0
);


--
-- Name: fleet_vehicles; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.fleet_vehicles (
    vehicle_id bigint NOT NULL,
    model_id bigint NOT NULL,
    registration_no character varying(20) NOT NULL,
    vehicle_use character varying(20) NOT NULL,
    owner_driver_id bigint,
    showroom_id bigint,
    status character varying(15) DEFAULT 'idle'::character varying NOT NULL,
    current_lat numeric(10,7),
    current_lon numeric(10,7),
    CONSTRAINT fleet_vehicles_status_check CHECK (((status)::text = ANY ((ARRAY['idle'::character varying, 'booked'::character varying, 'on_trip'::character varying, 'maintenance'::character varying])::text[]))),
    CONSTRAINT fleet_vehicles_vehicle_use_check CHECK (((vehicle_use)::text = ANY ((ARRAY['cab'::character varying, 'shared_cab'::character varying, 'premium'::character varying, 'vintage'::character varying, 'self_drive'::character varying, 'test_drive'::character varying, 'daily_bus'::character varying, 'luxury_bus'::character varying])::text[])))
);


--
-- Name: fleet_vehicles_vehicle_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.fleet_vehicles ALTER COLUMN vehicle_id ADD GENERATED BY DEFAULT AS IDENTITY (
    SEQUENCE NAME public.fleet_vehicles_vehicle_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: food_menu_items; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.food_menu_items (
    item_id bigint NOT NULL,
    restaurant_id bigint NOT NULL,
    item_name character varying(100) NOT NULL,
    category character varying(20) DEFAULT 'meal'::character varying,
    is_veg boolean DEFAULT true,
    price numeric(8,2) NOT NULL,
    is_available boolean DEFAULT true,
    CONSTRAINT food_menu_items_category_check CHECK (((category)::text = ANY ((ARRAY['breakfast'::character varying, 'meal'::character varying, 'snack'::character varying, 'beverage'::character varying, 'dessert'::character varying])::text[])))
);


--
-- Name: food_menu_items_item_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.food_menu_items ALTER COLUMN item_id ADD GENERATED BY DEFAULT AS IDENTITY (
    SEQUENCE NAME public.food_menu_items_item_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: food_order_items; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.food_order_items (
    order_item_id bigint NOT NULL,
    order_id bigint NOT NULL,
    item_id bigint NOT NULL,
    quantity smallint NOT NULL,
    unit_price numeric(8,2) NOT NULL,
    CONSTRAINT food_order_items_quantity_check CHECK ((quantity > 0))
);


--
-- Name: food_order_items_order_item_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.food_order_items ALTER COLUMN order_item_id ADD GENERATED BY DEFAULT AS IDENTITY (
    SEQUENCE NAME public.food_order_items_order_item_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: food_orders; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.food_orders (
    order_id bigint NOT NULL,
    ticket_id bigint NOT NULL,
    halt_id bigint NOT NULL,
    total_amount numeric(10,2) DEFAULT 0 NOT NULL,
    commission_amount numeric(10,2) DEFAULT 0 NOT NULL,
    status character varying(12) DEFAULT 'placed'::character varying,
    placed_at timestamp with time zone DEFAULT now(),
    CONSTRAINT food_orders_status_check CHECK (((status)::text = ANY ((ARRAY['placed'::character varying, 'confirmed'::character varying, 'delivered'::character varying, 'cancelled'::character varying])::text[])))
);


--
-- Name: food_orders_order_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.food_orders ALTER COLUMN order_id ADD GENERATED BY DEFAULT AS IDENTITY (
    SEQUENCE NAME public.food_orders_order_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: fuel_stations; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.fuel_stations (
    station_id bigint NOT NULL,
    name character varying(120) NOT NULL,
    station_type character varying(10) NOT NULL,
    brand character varying(50),
    latitude numeric(10,7) NOT NULL,
    longitude numeric(10,7) NOT NULL,
    road_id bigint,
    petrol_price numeric(7,2),
    diesel_price numeric(7,2),
    charging_points integer DEFAULT 0,
    is_open_24x7 boolean DEFAULT true,
    CONSTRAINT fuel_stations_station_type_check CHECK (((station_type)::text = ANY ((ARRAY['petrol'::character varying, 'ev'::character varying, 'both'::character varying])::text[])))
);


--
-- Name: fuel_stations_station_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.fuel_stations ALTER COLUMN station_id ADD GENERATED BY DEFAULT AS IDENTITY (
    SEQUENCE NAME public.fuel_stations_station_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: gen_dialogue_ctx; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.gen_dialogue_ctx (
    dialogue_seq bigint NOT NULL,
    scenario_id integer NOT NULL,
    vars jsonb NOT NULL
);


--
-- Name: gen_dialogue_scenarios; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.gen_dialogue_scenarios (
    turn_id bigint NOT NULL,
    scenario_id integer NOT NULL,
    scenario_name character varying(60) NOT NULL,
    primary_intent character varying(30) NOT NULL,
    turn_no smallint NOT NULL,
    speaker character varying(10) NOT NULL,
    text text NOT NULL,
    lang character varying(10) DEFAULT 'en'::character varying NOT NULL,
    CONSTRAINT gen_dialogue_scenarios_lang_check CHECK (((lang)::text = ANY ((ARRAY['en'::character varying, 'hi-en'::character varying])::text[]))),
    CONSTRAINT gen_dialogue_scenarios_speaker_check CHECK (((speaker)::text = ANY ((ARRAY['user'::character varying, 'assistant'::character varying])::text[])))
);


--
-- Name: gen_dialogue_scenarios_turn_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.gen_dialogue_scenarios ALTER COLUMN turn_id ADD GENERATED BY DEFAULT AS IDENTITY (
    SEQUENCE NAME public.gen_dialogue_scenarios_turn_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: gen_edge_frames; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.gen_edge_frames (
    frame_id bigint NOT NULL,
    edge_type character varying(40) NOT NULL,
    intent character varying(30) NOT NULL,
    frame text NOT NULL,
    lang character varying(10) DEFAULT 'en'::character varying NOT NULL,
    expected_behavior text NOT NULL,
    resolution_spec jsonb NOT NULL,
    CONSTRAINT gen_edge_frames_lang_check CHECK (((lang)::text = ANY ((ARRAY['en'::character varying, 'hi-en'::character varying])::text[])))
);


--
-- Name: gen_edge_frames_frame_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.gen_edge_frames ALTER COLUMN frame_id ADD GENERATED BY DEFAULT AS IDENTITY (
    SEQUENCE NAME public.gen_edge_frames_frame_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: gen_fn_map; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.gen_fn_map (
    intent character varying(30) NOT NULL,
    function_name character varying(40) NOT NULL
);


--
-- Name: gen_intent_frames; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.gen_intent_frames (
    frame_id bigint NOT NULL,
    intent character varying(30) NOT NULL,
    frame text NOT NULL,
    lang character varying(10) DEFAULT 'en'::character varying NOT NULL,
    difficulty character varying(10) DEFAULT 'easy'::character varying NOT NULL,
    slot_spec jsonb,
    CONSTRAINT gen_intent_frames_difficulty_check CHECK (((difficulty)::text = ANY ((ARRAY['easy'::character varying, 'medium'::character varying, 'hard'::character varying])::text[]))),
    CONSTRAINT gen_intent_frames_lang_check CHECK (((lang)::text = ANY ((ARRAY['en'::character varying, 'hi-en'::character varying])::text[])))
);


--
-- Name: gen_intent_frames_frame_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.gen_intent_frames ALTER COLUMN frame_id ADD GENERATED BY DEFAULT AS IDENTITY (
    SEQUENCE NAME public.gen_intent_frames_frame_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: gen_intents; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.gen_intents (
    intent_id integer NOT NULL,
    intent character varying(30) NOT NULL,
    description text NOT NULL
);


--
-- Name: gen_slot_frames; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.gen_slot_frames (
    frame_id bigint NOT NULL,
    intent character varying(30) NOT NULL,
    frame text NOT NULL,
    lang character varying(10) DEFAULT 'en'::character varying NOT NULL,
    difficulty character varying(10) DEFAULT 'medium'::character varying NOT NULL,
    slot_spec jsonb NOT NULL,
    CONSTRAINT gen_slot_frames_difficulty_check CHECK (((difficulty)::text = ANY ((ARRAY['easy'::character varying, 'medium'::character varying, 'hard'::character varying])::text[]))),
    CONSTRAINT gen_slot_frames_lang_check CHECK (((lang)::text = ANY ((ARRAY['en'::character varying, 'hi-en'::character varying])::text[])))
);


--
-- Name: gen_slot_frames_frame_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.gen_slot_frames ALTER COLUMN frame_id ADD GENERATED BY DEFAULT AS IDENTITY (
    SEQUENCE NAME public.gen_slot_frames_frame_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: junctions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.junctions (
    junction_id bigint NOT NULL,
    name character varying(120),
    latitude numeric(10,7) NOT NULL,
    longitude numeric(10,7) NOT NULL
);


--
-- Name: junctions_junction_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.junctions ALTER COLUMN junction_id ADD GENERATED BY DEFAULT AS IDENTITY (
    SEQUENCE NAME public.junctions_junction_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: locations; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.locations (
    location_id bigint NOT NULL,
    name character varying(120) NOT NULL,
    location_type character varying(30) DEFAULT 'general'::character varying NOT NULL,
    address text,
    latitude numeric(10,7) NOT NULL,
    longitude numeric(10,7) NOT NULL,
    nearest_junction bigint,
    CONSTRAINT locations_location_type_check CHECK (((location_type)::text = ANY ((ARRAY['general'::character varying, 'airport'::character varying, 'railway_station'::character varying, 'metro_station'::character varying, 'bus_stand'::character varying, 'tourist_place'::character varying, 'workplace'::character varying, 'residence'::character varying])::text[])))
);


--
-- Name: locations_location_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.locations ALTER COLUMN location_id ADD GENERATED BY DEFAULT AS IDENTITY (
    SEQUENCE NAME public.locations_location_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: luxury_bus_trips; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.luxury_bus_trips (
    trip_id bigint NOT NULL,
    bus_id bigint NOT NULL,
    origin_id bigint NOT NULL,
    destination_id bigint NOT NULL,
    departure_at timestamp with time zone NOT NULL,
    arrival_at timestamp with time zone NOT NULL,
    fare_per_seat numeric(10,2) NOT NULL,
    seats_available smallint NOT NULL
);


--
-- Name: luxury_bus_trips_trip_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.luxury_bus_trips ALTER COLUMN trip_id ADD GENERATED BY DEFAULT AS IDENTITY (
    SEQUENCE NAME public.luxury_bus_trips_trip_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: ml_config; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.ml_config (
    config_key character varying(40) NOT NULL,
    config_value integer NOT NULL,
    description text
);


--
-- Name: pass_scans; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.pass_scans (
    scan_id bigint NOT NULL,
    pass_id bigint NOT NULL,
    transport_type character varying(10) NOT NULL,
    scan_location bigint,
    scanned_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT pass_scans_transport_type_check CHECK (((transport_type)::text = ANY ((ARRAY['train'::character varying, 'metro'::character varying, 'bus'::character varying, 'cab'::character varying])::text[])))
);


--
-- Name: pass_scans_scan_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.pass_scans ALTER COLUMN scan_id ADD GENERATED BY DEFAULT AS IDENTITY (
    SEQUENCE NAME public.pass_scans_scan_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: payments; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.payments (
    payment_id bigint NOT NULL,
    user_id bigint NOT NULL,
    booking_id bigint,
    pass_id bigint,
    subscription_id bigint,
    ticket_id bigint,
    food_order_id bigint,
    amount numeric(12,2) NOT NULL,
    method character varying(15) NOT NULL,
    status character varying(12) DEFAULT 'success'::character varying,
    paid_at timestamp with time zone DEFAULT now(),
    CONSTRAINT payments_method_check CHECK (((method)::text = ANY ((ARRAY['wallet'::character varying, 'upi'::character varying, 'card'::character varying, 'cash'::character varying, 'netbanking'::character varying])::text[])))
);


--
-- Name: payments_payment_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.payments ALTER COLUMN payment_id ADD GENERATED BY DEFAULT AS IDENTITY (
    SEQUENCE NAME public.payments_payment_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: premium_bus_subscriptions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.premium_bus_subscriptions (
    subscription_id bigint NOT NULL,
    user_id bigint NOT NULL,
    route_id bigint NOT NULL,
    pickup_stop bigint NOT NULL,
    drop_stop bigint NOT NULL,
    seat_no smallint NOT NULL,
    monthly_fee numeric(10,2) NOT NULL,
    valid_from date NOT NULL,
    valid_to date NOT NULL,
    status character varying(12) DEFAULT 'active'::character varying
);


--
-- Name: premium_bus_subscriptions_subscription_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.premium_bus_subscriptions ALTER COLUMN subscription_id ADD GENERATED BY DEFAULT AS IDENTITY (
    SEQUENCE NAME public.premium_bus_subscriptions_subscription_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: restaurants; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.restaurants (
    restaurant_id bigint NOT NULL,
    name character varying(120) NOT NULL,
    latitude numeric(10,7) NOT NULL,
    longitude numeric(10,7) NOT NULL,
    rating numeric(3,2) DEFAULT 4.0
);


--
-- Name: restaurants_restaurant_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.restaurants ALTER COLUMN restaurant_id ADD GENERATED BY DEFAULT AS IDENTITY (
    SEQUENCE NAME public.restaurants_restaurant_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: roads; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.roads (
    road_id bigint NOT NULL,
    road_name character varying(150) NOT NULL,
    road_type character varying(10) NOT NULL,
    width_m numeric(5,2) NOT NULL,
    length_km numeric(7,3) NOT NULL,
    speed_limit_kmph integer DEFAULT 40,
    is_toll boolean DEFAULT false,
    from_junction bigint NOT NULL,
    to_junction bigint NOT NULL,
    CONSTRAINT roads_road_type_check CHECK (((road_type)::text = ANY ((ARRAY['one_way'::character varying, 'two_way'::character varying])::text[])))
);


--
-- Name: roads_road_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.roads ALTER COLUMN road_id ADD GENERATED BY DEFAULT AS IDENTITY (
    SEQUENCE NAME public.roads_road_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: self_drive_price_bands; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.self_drive_price_bands (
    band_id bigint NOT NULL,
    band_name character varying(30) NOT NULL,
    min_price_lakh numeric(8,2) NOT NULL,
    max_price_lakh numeric(8,2) NOT NULL,
    daily_fare numeric(10,2) NOT NULL,
    deposit numeric(10,2) NOT NULL,
    CONSTRAINT self_drive_price_bands_check CHECK ((min_price_lakh < max_price_lakh))
);


--
-- Name: self_drive_price_bands_band_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.self_drive_price_bands ALTER COLUMN band_id ADD GENERATED BY DEFAULT AS IDENTITY (
    SEQUENCE NAME public.self_drive_price_bands_band_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: shared_rides; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.shared_rides (
    ride_id bigint NOT NULL,
    vehicle_id bigint NOT NULL,
    driver_id bigint NOT NULL,
    origin_id bigint NOT NULL,
    destination_id bigint NOT NULL,
    depart_at timestamp with time zone NOT NULL,
    seat_capacity smallint DEFAULT 4 NOT NULL,
    seats_booked smallint DEFAULT 0 NOT NULL,
    fare_per_seat numeric(8,2) NOT NULL,
    status character varying(12) DEFAULT 'open'::character varying NOT NULL,
    CONSTRAINT shared_rides_status_check CHECK (((status)::text = ANY ((ARRAY['open'::character varying, 'full'::character varying, 'departed'::character varying, 'done'::character varying, 'cancelled'::character varying])::text[])))
);


--
-- Name: shared_rides_ride_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.shared_rides ALTER COLUMN ride_id ADD GENERATED BY DEFAULT AS IDENTITY (
    SEQUENCE NAME public.shared_rides_ride_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: showrooms; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.showrooms (
    showroom_id bigint NOT NULL,
    dealer_user_id bigint NOT NULL,
    showroom_name character varying(120) NOT NULL,
    latitude numeric(10,7) NOT NULL,
    longitude numeric(10,7) NOT NULL,
    test_drive_radius_km numeric(6,2) DEFAULT 10,
    commission_rate numeric(5,2) DEFAULT 12.00
);


--
-- Name: showrooms_showroom_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.showrooms ALTER COLUMN showroom_id ADD GENERATED BY DEFAULT AS IDENTITY (
    SEQUENCE NAME public.showrooms_showroom_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: station_reviews; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.station_reviews (
    review_id bigint NOT NULL,
    station_id bigint NOT NULL,
    user_id bigint NOT NULL,
    rating integer NOT NULL,
    comment text,
    created_at timestamp with time zone DEFAULT now(),
    CONSTRAINT station_reviews_rating_check CHECK (((rating >= 1) AND (rating <= 5)))
);


--
-- Name: station_reviews_review_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.station_reviews ALTER COLUMN review_id ADD GENERATED BY DEFAULT AS IDENTITY (
    SEQUENCE NAME public.station_reviews_review_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: station_traffic_log; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.station_traffic_log (
    log_id bigint NOT NULL,
    station_id bigint NOT NULL,
    detected_at timestamp with time zone DEFAULT now() NOT NULL,
    cars_count integer NOT NULL,
    avg_wait_min numeric(5,2)
);


--
-- Name: station_traffic_log_log_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.station_traffic_log ALTER COLUMN log_id ADD GENERATED BY DEFAULT AS IDENTITY (
    SEQUENCE NAME public.station_traffic_log_log_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: test_drive_bookings; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.test_drive_bookings (
    test_drive_id bigint NOT NULL,
    booking_id bigint NOT NULL,
    user_id bigint NOT NULL,
    model_id bigint NOT NULL,
    showroom_id bigint NOT NULL,
    slot_start timestamp with time zone NOT NULL,
    slot_end timestamp with time zone NOT NULL,
    served_as_cab boolean DEFAULT false
);


--
-- Name: test_drive_bookings_test_drive_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.test_drive_bookings ALTER COLUMN test_drive_id ADD GENERATED BY DEFAULT AS IDENTITY (
    SEQUENCE NAME public.test_drive_bookings_test_drive_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: training_examples; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.training_examples (
    example_id bigint NOT NULL,
    data_type character varying(25) NOT NULL,
    intent_label character varying(30) NOT NULL,
    utterance text NOT NULL,
    lang character varying(10) DEFAULT 'en'::character varying NOT NULL,
    difficulty character varying(10) DEFAULT 'easy'::character varying NOT NULL,
    slots jsonb,
    function_trace jsonb,
    edge_type character varying(40),
    expected_behavior text,
    edge_resolution jsonb,
    dialogue_id bigint,
    split character varying(10) DEFAULT 'train'::character varying NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT chk_dialogue_link CHECK ((((data_type)::text <> 'multi_turn_dialogue'::text) OR (dialogue_id IS NOT NULL))),
    CONSTRAINT chk_edge_fields CHECK ((((data_type)::text <> 'edge_case'::text) OR ((edge_type IS NOT NULL) AND (expected_behavior IS NOT NULL)))),
    CONSTRAINT training_examples_data_type_check CHECK (((data_type)::text = ANY ((ARRAY['intent_classification'::character varying, 'slot_extraction'::character varying, 'function_calling'::character varying, 'multi_turn_dialogue'::character varying, 'edge_case'::character varying])::text[]))),
    CONSTRAINT training_examples_difficulty_check CHECK (((difficulty)::text = ANY ((ARRAY['easy'::character varying, 'medium'::character varying, 'hard'::character varying])::text[]))),
    CONSTRAINT training_examples_lang_check CHECK (((lang)::text = ANY ((ARRAY['en'::character varying, 'hi-en'::character varying])::text[]))),
    CONSTRAINT training_examples_split_check CHECK (((split)::text = ANY ((ARRAY['train'::character varying, 'val'::character varying, 'test'::character varying])::text[])))
);


--
-- Name: training_examples_example_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.training_examples ALTER COLUMN example_id ADD GENERATED BY DEFAULT AS IDENTITY (
    SEQUENCE NAME public.training_examples_example_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: travel_passes; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.travel_passes (
    pass_id bigint NOT NULL,
    qr_code uuid DEFAULT gen_random_uuid() NOT NULL,
    route_id bigint NOT NULL,
    travel_date date DEFAULT CURRENT_DATE NOT NULL,
    total_fare numeric(10,2) DEFAULT 0 NOT NULL,
    status character varying(12) DEFAULT 'active'::character varying NOT NULL,
    created_at timestamp with time zone DEFAULT now(),
    CONSTRAINT travel_passes_status_check CHECK (((status)::text = ANY ((ARRAY['active'::character varying, 'used'::character varying, 'expired'::character varying, 'cancelled'::character varying])::text[])))
);


--
-- Name: travel_passes_pass_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.travel_passes ALTER COLUMN pass_id ADD GENERATED BY DEFAULT AS IDENTITY (
    SEQUENCE NAME public.travel_passes_pass_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: trip_food_halts; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.trip_food_halts (
    halt_id bigint NOT NULL,
    trip_id bigint NOT NULL,
    restaurant_id bigint NOT NULL,
    halt_seq smallint NOT NULL,
    scheduled_at timestamp with time zone
);


--
-- Name: trip_food_halts_halt_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.trip_food_halts ALTER COLUMN halt_id ADD GENERATED BY DEFAULT AS IDENTITY (
    SEQUENCE NAME public.trip_food_halts_halt_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: user_vehicles; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.user_vehicles (
    user_vehicle_id bigint NOT NULL,
    user_id bigint NOT NULL,
    model_id bigint NOT NULL,
    nickname character varying(50),
    current_fuel_litres numeric(6,2),
    current_battery_pct numeric(5,2)
);


--
-- Name: user_vehicles_user_vehicle_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.user_vehicles ALTER COLUMN user_vehicle_id ADD GENERATED BY DEFAULT AS IDENTITY (
    SEQUENCE NAME public.user_vehicles_user_vehicle_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: users; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.users (
    user_id bigint NOT NULL,
    full_name character varying(100) NOT NULL,
    email character varying(150) NOT NULL,
    phone character varying(15) NOT NULL,
    password_hash character varying(255) NOT NULL,
    user_role character varying(20) NOT NULL,
    wallet_balance numeric(12,2) DEFAULT 0,
    created_at timestamp with time zone DEFAULT now(),
    CONSTRAINT users_user_role_check CHECK (((user_role)::text = ANY ((ARRAY['passenger'::character varying, 'driver'::character varying, 'dealer'::character varying, 'bus_owner'::character varying, 'admin'::character varying])::text[])))
);


--
-- Name: users_user_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.users ALTER COLUMN user_id ADD GENERATED BY DEFAULT AS IDENTITY (
    SEQUENCE NAME public.users_user_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: v_best_petrol_stations; Type: VIEW; Schema: public; Owner: -
--

CREATE VIEW public.v_best_petrol_stations AS
 SELECT s.station_id,
    s.name,
    s.brand,
    s.petrol_price,
    public.fn_station_crowd_rating(s.station_id) AS gps_crowd_rating,
    COALESCE(r.avg_user_rating, (5)::numeric) AS user_rating,
    round(((public.fn_station_crowd_rating(s.station_id) * 0.7) + (COALESCE(r.avg_user_rating, (5)::numeric) * 0.3)), 2) AS final_rating
   FROM (public.fuel_stations s
     LEFT JOIN ( SELECT station_reviews.station_id,
            round(avg(station_reviews.rating), 2) AS avg_user_rating
           FROM public.station_reviews
          GROUP BY station_reviews.station_id) r ON ((r.station_id = s.station_id)))
  WHERE ((s.station_type)::text = ANY ((ARRAY['petrol'::character varying, 'both'::character varying])::text[]));


--
-- Name: v_bus_owner_commissions; Type: VIEW; Schema: public; Owner: -
--

CREATE VIEW public.v_bus_owner_commissions AS
 SELECT u.user_id,
    u.full_name AS owner,
    c.source,
    count(*) AS transactions,
    sum(c.amount) AS total_commission
   FROM (public.commissions c
     JOIN public.users u ON ((u.user_id = c.bus_owner_id)))
  GROUP BY u.user_id, u.full_name, c.source;


--
-- Name: v_driver_earnings; Type: VIEW; Schema: public; Owner: -
--

CREATE VIEW public.v_driver_earnings AS
 SELECT d.driver_id,
    u.full_name,
    count(b.booking_id) AS trips_done,
    round(COALESCE(sum((b.fare * 0.8)), (0)::numeric), 2) AS est_earnings
   FROM ((public.driver_profiles d
     JOIN public.users u ON ((u.user_id = d.user_id)))
     LEFT JOIN public.bookings b ON (((b.driver_id = d.driver_id) AND ((b.status)::text = 'completed'::text))))
  GROUP BY d.driver_id, u.full_name;


--
-- Name: v_intent_distribution; Type: VIEW; Schema: public; Owner: -
--

CREATE VIEW public.v_intent_distribution AS
 SELECT intent_label,
    count(*) FILTER (WHERE ((data_type)::text = 'intent_classification'::text)) AS intent_rows,
    count(*) FILTER (WHERE ((data_type)::text = 'slot_extraction'::text)) AS slot_rows,
    count(*) FILTER (WHERE ((data_type)::text = 'function_calling'::text)) AS fn_rows,
    count(*) FILTER (WHERE ((data_type)::text = 'multi_turn_dialogue'::text)) AS dialogue_rows,
    count(*) FILTER (WHERE ((data_type)::text = 'edge_case'::text)) AS edge_rows
   FROM public.training_examples
  GROUP BY intent_label
  ORDER BY intent_label;


--
-- Name: v_jsonl_classification; Type: VIEW; Schema: public; Owner: -
--

CREATE VIEW public.v_jsonl_classification AS
 SELECT example_id,
    data_type,
    jsonb_build_object('messages', jsonb_build_array(jsonb_build_object('role', 'system', 'content', 'You are the Smart Transit WB assistant. Classify each user message into exactly one intent label.'), jsonb_build_object('role', 'user', 'content', utterance), jsonb_build_object('role', 'assistant', 'content', intent_label))) AS jsonl
   FROM public.training_examples
  WHERE ((data_type)::text = 'intent_classification'::text);


--
-- Name: v_jsonl_dialogue; Type: VIEW; Schema: public; Owner: -
--

CREATE VIEW public.v_jsonl_dialogue AS
 SELECT te.example_id,
    te.data_type,
    jsonb_build_object('messages', (jsonb_build_array(jsonb_build_object('role', 'system', 'content', 'You are the Smart Transit WB booking assistant for West Bengal. Help users book cabs, shared rides, passes, buses, food and tourist packages. Reply in the language the user uses.')) || jsonb_agg(jsonb_build_object('role', t.speaker, 'content', t.utterance) ORDER BY t.turn_no))) AS jsonl
   FROM (public.training_examples te
     JOIN public.dialogue_turns t ON ((t.dialogue_id = te.example_id)))
  WHERE ((te.data_type)::text = 'multi_turn_dialogue'::text)
  GROUP BY te.example_id, te.data_type;


--
-- Name: v_jsonl_edge; Type: VIEW; Schema: public; Owner: -
--

CREATE VIEW public.v_jsonl_edge AS
 SELECT example_id,
    data_type,
    jsonb_build_object('messages', jsonb_build_array(jsonb_build_object('role', 'system', 'content', 'You are the Smart Transit WB assistant. When a business rule blocks the request, explain the rule clearly and offer the next best action.'), jsonb_build_object('role', 'user', 'content', utterance), jsonb_build_object('role', 'assistant', 'content', (jsonb_build_object('edge_type', edge_type, 'response', expected_behavior, 'resolution', edge_resolution))::text))) AS jsonl
   FROM public.training_examples
  WHERE ((data_type)::text = 'edge_case'::text);


--
-- Name: v_jsonl_function; Type: VIEW; Schema: public; Owner: -
--

CREATE VIEW public.v_jsonl_function AS
 SELECT example_id,
    data_type,
    jsonb_build_object('messages', jsonb_build_array(jsonb_build_object('role', 'system', 'content', 'You are the Smart Transit WB assistant with access to map and booking tools. For any actionable request, emit exactly one correct tool call with complete JSON arguments.'), jsonb_build_object('role', 'user', 'content', utterance), jsonb_build_object('role', 'assistant', 'content', (function_trace)::text))) AS jsonl
   FROM public.training_examples
  WHERE ((data_type)::text = 'function_calling'::text);


--
-- Name: v_jsonl_slot; Type: VIEW; Schema: public; Owner: -
--

CREATE VIEW public.v_jsonl_slot AS
 SELECT example_id,
    data_type,
    jsonb_build_object('messages', jsonb_build_array(jsonb_build_object('role', 'system', 'content', 'You are the Smart Transit WB assistant. Extract all booking entities from the message and return them as a JSON object.'), jsonb_build_object('role', 'user', 'content', utterance), jsonb_build_object('role', 'assistant', 'content', COALESCE((slots)::text, '{}'::text)))) AS jsonl
   FROM public.training_examples
  WHERE ((data_type)::text = 'slot_extraction'::text);


--
-- Name: v_jsonl_all; Type: VIEW; Schema: public; Owner: -
--

CREATE VIEW public.v_jsonl_all AS
 SELECT v_jsonl_classification.data_type,
    v_jsonl_classification.example_id,
    v_jsonl_classification.jsonl
   FROM public.v_jsonl_classification
UNION ALL
 SELECT v_jsonl_slot.data_type,
    v_jsonl_slot.example_id,
    v_jsonl_slot.jsonl
   FROM public.v_jsonl_slot
UNION ALL
 SELECT v_jsonl_function.data_type,
    v_jsonl_function.example_id,
    v_jsonl_function.jsonl
   FROM public.v_jsonl_function
UNION ALL
 SELECT v_jsonl_edge.data_type,
    v_jsonl_edge.example_id,
    v_jsonl_edge.jsonl
   FROM public.v_jsonl_edge
UNION ALL
 SELECT v_jsonl_dialogue.data_type,
    v_jsonl_dialogue.example_id,
    v_jsonl_dialogue.jsonl
   FROM public.v_jsonl_dialogue;


--
-- Name: v_premium_vintage_available; Type: VIEW; Schema: public; Owner: -
--

CREATE VIEW public.v_premium_vintage_available AS
 SELECT v.vehicle_id,
    m.brand,
    m.model_name,
    m.category,
    m.seating,
    v.registration_no,
    v.current_lat,
    v.current_lon
   FROM (public.fleet_vehicles v
     JOIN public.car_models m USING (model_id))
  WHERE (((v.vehicle_use)::text = ANY ((ARRAY['premium'::character varying, 'vintage'::character varying])::text[])) AND ((v.status)::text = 'idle'::text));


--
-- Name: v_road_edges; Type: VIEW; Schema: public; Owner: -
--

CREATE VIEW public.v_road_edges AS
 SELECT roads.road_id,
    roads.from_junction AS from_node,
    roads.to_junction AS to_node,
    roads.road_type,
    roads.width_m,
    roads.length_km,
    roads.speed_limit_kmph
   FROM public.roads
UNION ALL
 SELECT roads.road_id,
    roads.to_junction AS from_node,
    roads.from_junction AS to_node,
    roads.road_type,
    roads.width_m,
    roads.length_km,
    roads.speed_limit_kmph
   FROM public.roads
  WHERE ((roads.road_type)::text = 'two_way'::text);


--
-- Name: v_self_drive_fleet; Type: VIEW; Schema: public; Owner: -
--

CREATE VIEW public.v_self_drive_fleet AS
 SELECT v.vehicle_id,
    m.brand,
    m.model_name,
    m.price_lakh,
    pb.band_name,
    pb.daily_fare,
    pb.deposit
   FROM ((public.fleet_vehicles v
     JOIN public.car_models m USING (model_id))
     JOIN public.self_drive_price_bands pb ON (((m.price_lakh >= pb.min_price_lakh) AND (m.price_lakh <= pb.max_price_lakh))))
  WHERE (((v.vehicle_use)::text = 'self_drive'::text) AND ((v.status)::text = 'idle'::text));


--
-- Name: v_training_stats; Type: VIEW; Schema: public; Owner: -
--

CREATE VIEW public.v_training_stats AS
 SELECT data_type,
    count(*) AS total,
    count(*) FILTER (WHERE ((split)::text = 'train'::text)) AS train_rows,
    count(*) FILTER (WHERE ((split)::text = 'val'::text)) AS val_rows,
    count(*) FILTER (WHERE ((split)::text = 'test'::text)) AS test_rows,
    count(DISTINCT intent_label) AS intents_covered,
    round(avg(length(utterance)), 1) AS avg_utterance_len
   FROM public.training_examples
  GROUP BY GROUPING SETS ((data_type), ());


--
-- Name: wb_bus_routes; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.wb_bus_routes (
    route_id bigint NOT NULL,
    route_name character varying(100) NOT NULL,
    fare_per_seat numeric(10,2) NOT NULL
);


--
-- Name: wb_bus_routes_route_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.wb_bus_routes ALTER COLUMN route_id ADD GENERATED BY DEFAULT AS IDENTITY (
    SEQUENCE NAME public.wb_bus_routes_route_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: wb_car_models; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.wb_car_models (
    model_id bigint NOT NULL,
    brand character varying(50) NOT NULL,
    model_name character varying(80) NOT NULL,
    category character varying(30) NOT NULL,
    length_cm integer NOT NULL,
    width_cm integer NOT NULL,
    height_cm integer NOT NULL,
    seating integer NOT NULL,
    fuel_type character varying(15) NOT NULL,
    tank_litres numeric(6,2),
    battery_kwh numeric(6,2),
    range_km numeric(7,2),
    price_lakh numeric(8,2) NOT NULL,
    is_premium boolean DEFAULT false,
    is_vintage boolean DEFAULT false,
    is_self_drive boolean DEFAULT false,
    CONSTRAINT wb_car_models_category_check CHECK (((category)::text = ANY ((ARRAY['hatchback'::character varying, 'sedan'::character varying, 'suv'::character varying, 'muv'::character varying, 'premium'::character varying, 'vintage'::character varying])::text[]))),
    CONSTRAINT wb_car_models_fuel_type_check CHECK (((fuel_type)::text = ANY ((ARRAY['petrol'::character varying, 'diesel'::character varying, 'cng'::character varying, 'electric'::character varying, 'hybrid'::character varying])::text[])))
);


--
-- Name: wb_car_models_model_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.wb_car_models ALTER COLUMN model_id ADD GENERATED BY DEFAULT AS IDENTITY (
    SEQUENCE NAME public.wb_car_models_model_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: wb_dates; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.wb_dates (
    date_id bigint NOT NULL,
    phrase character varying(60) NOT NULL
);


--
-- Name: wb_dates_date_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.wb_dates ALTER COLUMN date_id ADD GENERATED BY DEFAULT AS IDENTITY (
    SEQUENCE NAME public.wb_dates_date_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: wb_food_items; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.wb_food_items (
    item_id bigint NOT NULL,
    item_name character varying(100) NOT NULL,
    is_veg boolean DEFAULT true,
    price numeric(8,2) NOT NULL
);


--
-- Name: wb_food_items_item_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.wb_food_items ALTER COLUMN item_id ADD GENERATED BY DEFAULT AS IDENTITY (
    SEQUENCE NAME public.wb_food_items_item_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: wb_locations; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.wb_locations (
    location_id bigint NOT NULL,
    name character varying(120) NOT NULL,
    district character varying(60) NOT NULL,
    location_type character varying(30) NOT NULL,
    latitude numeric(10,7) NOT NULL,
    longitude numeric(10,7) NOT NULL,
    CONSTRAINT wb_locations_location_type_check CHECK (((location_type)::text = ANY ((ARRAY['general'::character varying, 'airport'::character varying, 'railway_station'::character varying, 'metro_station'::character varying, 'bus_stand'::character varying, 'tourist_place'::character varying, 'workplace'::character varying, 'residence'::character varying])::text[])))
);


--
-- Name: wb_locations_location_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.wb_locations ALTER COLUMN location_id ADD GENERATED BY DEFAULT AS IDENTITY (
    SEQUENCE NAME public.wb_locations_location_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: wb_stations; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.wb_stations (
    station_id bigint NOT NULL,
    name character varying(120) NOT NULL,
    station_type character varying(10) NOT NULL,
    brand character varying(50),
    area character varying(120),
    petrol_price numeric(7,2),
    charging_points integer DEFAULT 0,
    is_open_24x7 boolean DEFAULT true,
    CONSTRAINT wb_stations_station_type_check CHECK (((station_type)::text = ANY ((ARRAY['petrol'::character varying, 'ev'::character varying, 'both'::character varying])::text[])))
);


--
-- Name: wb_stations_station_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.wb_stations ALTER COLUMN station_id ADD GENERATED BY DEFAULT AS IDENTITY (
    SEQUENCE NAME public.wb_stations_station_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: wb_times; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.wb_times (
    phrase_id bigint NOT NULL,
    phrase character varying(60) NOT NULL
);


--
-- Name: wb_times_phrase_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.wb_times ALTER COLUMN phrase_id ADD GENERATED BY DEFAULT AS IDENTITY (
    SEQUENCE NAME public.wb_times_phrase_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: bookings bookings_pkey; Type: CONSTRAINT; Schema: ai; Owner: -
--

ALTER TABLE ONLY ai.bookings
    ADD CONSTRAINT bookings_pkey PRIMARY KEY (id);


--
-- Name: data_sources data_sources_pkey; Type: CONSTRAINT; Schema: ai; Owner: -
--

ALTER TABLE ONLY ai.data_sources
    ADD CONSTRAINT data_sources_pkey PRIMARY KEY (id);


--
-- Name: ev_charging_stations ev_charging_stations_pkey; Type: CONSTRAINT; Schema: ai; Owner: -
--

ALTER TABLE ONLY ai.ev_charging_stations
    ADD CONSTRAINT ev_charging_stations_pkey PRIMARY KEY (id);


--
-- Name: fuel_stations fuel_stations_pkey; Type: CONSTRAINT; Schema: ai; Owner: -
--

ALTER TABLE ONLY ai.fuel_stations
    ADD CONSTRAINT fuel_stations_pkey PRIMARY KEY (id);


--
-- Name: road_restrictions road_restrictions_pkey; Type: CONSTRAINT; Schema: ai; Owner: -
--

ALTER TABLE ONLY ai.road_restrictions
    ADD CONSTRAINT road_restrictions_pkey PRIMARY KEY (id);


--
-- Name: roads roads_osm_way_id_key; Type: CONSTRAINT; Schema: ai; Owner: -
--

ALTER TABLE ONLY ai.roads
    ADD CONSTRAINT roads_osm_way_id_key UNIQUE (osm_way_id);


--
-- Name: roads roads_pkey; Type: CONSTRAINT; Schema: ai; Owner: -
--

ALTER TABLE ONLY ai.roads
    ADD CONSTRAINT roads_pkey PRIMARY KEY (id);


--
-- Name: traffic_conditions traffic_conditions_pkey; Type: CONSTRAINT; Schema: ai; Owner: -
--

ALTER TABLE ONLY ai.traffic_conditions
    ADD CONSTRAINT traffic_conditions_pkey PRIMARY KEY (id);


--
-- Name: traffic_signals traffic_signals_pkey; Type: CONSTRAINT; Schema: ai; Owner: -
--

ALTER TABLE ONLY ai.traffic_signals
    ADD CONSTRAINT traffic_signals_pkey PRIMARY KEY (id);


--
-- Name: training_examples training_examples_pkey; Type: CONSTRAINT; Schema: ai; Owner: -
--

ALTER TABLE ONLY ai.training_examples
    ADD CONSTRAINT training_examples_pkey PRIMARY KEY (id);


--
-- Name: vehicles vehicles_make_model_variant_model_year_key; Type: CONSTRAINT; Schema: ai; Owner: -
--

ALTER TABLE ONLY ai.vehicles
    ADD CONSTRAINT vehicles_make_model_variant_model_year_key UNIQUE (make, model, variant, model_year);


--
-- Name: vehicles vehicles_pkey; Type: CONSTRAINT; Schema: ai; Owner: -
--

ALTER TABLE ONLY ai.vehicles
    ADD CONSTRAINT vehicles_pkey PRIMARY KEY (id);


--
-- Name: bookings bookings_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.bookings
    ADD CONSTRAINT bookings_pkey PRIMARY KEY (booking_id);


--
-- Name: bus_routes bus_routes_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.bus_routes
    ADD CONSTRAINT bus_routes_pkey PRIMARY KEY (route_id);


--
-- Name: bus_stops bus_stops_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.bus_stops
    ADD CONSTRAINT bus_stops_pkey PRIMARY KEY (stop_id);


--
-- Name: bus_stops bus_stops_route_id_stop_seq_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.bus_stops
    ADD CONSTRAINT bus_stops_route_id_stop_seq_key UNIQUE (route_id, stop_seq);


--
-- Name: bus_tickets bus_tickets_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.bus_tickets
    ADD CONSTRAINT bus_tickets_pkey PRIMARY KEY (ticket_id);


--
-- Name: bus_tickets bus_tickets_trip_id_seat_no_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.bus_tickets
    ADD CONSTRAINT bus_tickets_trip_id_seat_no_key UNIQUE (trip_id, seat_no);


--
-- Name: buses buses_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.buses
    ADD CONSTRAINT buses_pkey PRIMARY KEY (bus_id);


--
-- Name: buses buses_registration_no_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.buses
    ADD CONSTRAINT buses_registration_no_key UNIQUE (registration_no);


--
-- Name: car_models car_models_brand_model_name_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.car_models
    ADD CONSTRAINT car_models_brand_model_name_key UNIQUE (brand, model_name);


--
-- Name: car_models car_models_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.car_models
    ADD CONSTRAINT car_models_pkey PRIMARY KEY (model_id);


--
-- Name: commissions commissions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.commissions
    ADD CONSTRAINT commissions_pkey PRIMARY KEY (commission_id);


--
-- Name: daily_routes daily_routes_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.daily_routes
    ADD CONSTRAINT daily_routes_pkey PRIMARY KEY (route_id);


--
-- Name: daily_routes daily_routes_user_id_origin_id_destination_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.daily_routes
    ADD CONSTRAINT daily_routes_user_id_origin_id_destination_id_key UNIQUE (user_id, origin_id, destination_id);


--
-- Name: dialogue_turns dialogue_turns_dialogue_id_turn_no_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dialogue_turns
    ADD CONSTRAINT dialogue_turns_dialogue_id_turn_no_key UNIQUE (dialogue_id, turn_no);


--
-- Name: dialogue_turns dialogue_turns_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dialogue_turns
    ADD CONSTRAINT dialogue_turns_pkey PRIMARY KEY (turn_id);


--
-- Name: driver_profiles driver_profiles_license_no_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.driver_profiles
    ADD CONSTRAINT driver_profiles_license_no_key UNIQUE (license_no);


--
-- Name: driver_profiles driver_profiles_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.driver_profiles
    ADD CONSTRAINT driver_profiles_pkey PRIMARY KEY (driver_id);


--
-- Name: driver_profiles driver_profiles_user_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.driver_profiles
    ADD CONSTRAINT driver_profiles_user_id_key UNIQUE (user_id);


--
-- Name: fare_config fare_config_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.fare_config
    ADD CONSTRAINT fare_config_pkey PRIMARY KEY (service_type);


--
-- Name: fleet_vehicles fleet_vehicles_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.fleet_vehicles
    ADD CONSTRAINT fleet_vehicles_pkey PRIMARY KEY (vehicle_id);


--
-- Name: fleet_vehicles fleet_vehicles_registration_no_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.fleet_vehicles
    ADD CONSTRAINT fleet_vehicles_registration_no_key UNIQUE (registration_no);


--
-- Name: food_menu_items food_menu_items_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.food_menu_items
    ADD CONSTRAINT food_menu_items_pkey PRIMARY KEY (item_id);


--
-- Name: food_order_items food_order_items_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.food_order_items
    ADD CONSTRAINT food_order_items_pkey PRIMARY KEY (order_item_id);


--
-- Name: food_orders food_orders_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.food_orders
    ADD CONSTRAINT food_orders_pkey PRIMARY KEY (order_id);


--
-- Name: fuel_stations fuel_stations_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.fuel_stations
    ADD CONSTRAINT fuel_stations_pkey PRIMARY KEY (station_id);


--
-- Name: gen_dialogue_ctx gen_dialogue_ctx_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.gen_dialogue_ctx
    ADD CONSTRAINT gen_dialogue_ctx_pkey PRIMARY KEY (dialogue_seq);


--
-- Name: gen_dialogue_scenarios gen_dialogue_scenarios_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.gen_dialogue_scenarios
    ADD CONSTRAINT gen_dialogue_scenarios_pkey PRIMARY KEY (turn_id);


--
-- Name: gen_edge_frames gen_edge_frames_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.gen_edge_frames
    ADD CONSTRAINT gen_edge_frames_pkey PRIMARY KEY (frame_id);


--
-- Name: gen_fn_map gen_fn_map_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.gen_fn_map
    ADD CONSTRAINT gen_fn_map_pkey PRIMARY KEY (intent);


--
-- Name: gen_intent_frames gen_intent_frames_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.gen_intent_frames
    ADD CONSTRAINT gen_intent_frames_pkey PRIMARY KEY (frame_id);


--
-- Name: gen_intents gen_intents_intent_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.gen_intents
    ADD CONSTRAINT gen_intents_intent_key UNIQUE (intent);


--
-- Name: gen_intents gen_intents_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.gen_intents
    ADD CONSTRAINT gen_intents_pkey PRIMARY KEY (intent_id);


--
-- Name: gen_slot_frames gen_slot_frames_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.gen_slot_frames
    ADD CONSTRAINT gen_slot_frames_pkey PRIMARY KEY (frame_id);


--
-- Name: junctions junctions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.junctions
    ADD CONSTRAINT junctions_pkey PRIMARY KEY (junction_id);


--
-- Name: locations locations_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.locations
    ADD CONSTRAINT locations_pkey PRIMARY KEY (location_id);


--
-- Name: luxury_bus_trips luxury_bus_trips_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.luxury_bus_trips
    ADD CONSTRAINT luxury_bus_trips_pkey PRIMARY KEY (trip_id);


--
-- Name: ml_config ml_config_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ml_config
    ADD CONSTRAINT ml_config_pkey PRIMARY KEY (config_key);


--
-- Name: pass_scans pass_scans_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.pass_scans
    ADD CONSTRAINT pass_scans_pkey PRIMARY KEY (scan_id);


--
-- Name: payments payments_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.payments
    ADD CONSTRAINT payments_pkey PRIMARY KEY (payment_id);


--
-- Name: premium_bus_subscriptions premium_bus_subscriptions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.premium_bus_subscriptions
    ADD CONSTRAINT premium_bus_subscriptions_pkey PRIMARY KEY (subscription_id);


--
-- Name: premium_bus_subscriptions premium_bus_subscriptions_route_id_seat_no_valid_from_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.premium_bus_subscriptions
    ADD CONSTRAINT premium_bus_subscriptions_route_id_seat_no_valid_from_key UNIQUE (route_id, seat_no, valid_from);


--
-- Name: restaurants restaurants_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.restaurants
    ADD CONSTRAINT restaurants_pkey PRIMARY KEY (restaurant_id);


--
-- Name: roads roads_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.roads
    ADD CONSTRAINT roads_pkey PRIMARY KEY (road_id);


--
-- Name: self_drive_price_bands self_drive_price_bands_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.self_drive_price_bands
    ADD CONSTRAINT self_drive_price_bands_pkey PRIMARY KEY (band_id);


--
-- Name: shared_rides shared_rides_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.shared_rides
    ADD CONSTRAINT shared_rides_pkey PRIMARY KEY (ride_id);


--
-- Name: showrooms showrooms_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.showrooms
    ADD CONSTRAINT showrooms_pkey PRIMARY KEY (showroom_id);


--
-- Name: station_reviews station_reviews_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.station_reviews
    ADD CONSTRAINT station_reviews_pkey PRIMARY KEY (review_id);


--
-- Name: station_traffic_log station_traffic_log_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.station_traffic_log
    ADD CONSTRAINT station_traffic_log_pkey PRIMARY KEY (log_id);


--
-- Name: test_drive_bookings test_drive_bookings_booking_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.test_drive_bookings
    ADD CONSTRAINT test_drive_bookings_booking_id_key UNIQUE (booking_id);


--
-- Name: test_drive_bookings test_drive_bookings_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.test_drive_bookings
    ADD CONSTRAINT test_drive_bookings_pkey PRIMARY KEY (test_drive_id);


--
-- Name: training_examples training_examples_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.training_examples
    ADD CONSTRAINT training_examples_pkey PRIMARY KEY (example_id);


--
-- Name: travel_passes travel_passes_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.travel_passes
    ADD CONSTRAINT travel_passes_pkey PRIMARY KEY (pass_id);


--
-- Name: trip_food_halts trip_food_halts_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.trip_food_halts
    ADD CONSTRAINT trip_food_halts_pkey PRIMARY KEY (halt_id);


--
-- Name: trip_food_halts trip_food_halts_trip_id_restaurant_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.trip_food_halts
    ADD CONSTRAINT trip_food_halts_trip_id_restaurant_id_key UNIQUE (trip_id, restaurant_id);


--
-- Name: test_drive_bookings uq_one_time_per_model; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.test_drive_bookings
    ADD CONSTRAINT uq_one_time_per_model UNIQUE (user_id, model_id, showroom_id);


--
-- Name: user_vehicles user_vehicles_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_vehicles
    ADD CONSTRAINT user_vehicles_pkey PRIMARY KEY (user_vehicle_id);


--
-- Name: users users_email_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_email_key UNIQUE (email);


--
-- Name: users users_phone_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_phone_key UNIQUE (phone);


--
-- Name: users users_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_pkey PRIMARY KEY (user_id);


--
-- Name: wb_bus_routes wb_bus_routes_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.wb_bus_routes
    ADD CONSTRAINT wb_bus_routes_pkey PRIMARY KEY (route_id);


--
-- Name: wb_bus_routes wb_bus_routes_route_name_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.wb_bus_routes
    ADD CONSTRAINT wb_bus_routes_route_name_key UNIQUE (route_name);


--
-- Name: wb_car_models wb_car_models_brand_model_name_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.wb_car_models
    ADD CONSTRAINT wb_car_models_brand_model_name_key UNIQUE (brand, model_name);


--
-- Name: wb_car_models wb_car_models_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.wb_car_models
    ADD CONSTRAINT wb_car_models_pkey PRIMARY KEY (model_id);


--
-- Name: wb_dates wb_dates_phrase_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.wb_dates
    ADD CONSTRAINT wb_dates_phrase_key UNIQUE (phrase);


--
-- Name: wb_dates wb_dates_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.wb_dates
    ADD CONSTRAINT wb_dates_pkey PRIMARY KEY (date_id);


--
-- Name: wb_food_items wb_food_items_item_name_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.wb_food_items
    ADD CONSTRAINT wb_food_items_item_name_key UNIQUE (item_name);


--
-- Name: wb_food_items wb_food_items_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.wb_food_items
    ADD CONSTRAINT wb_food_items_pkey PRIMARY KEY (item_id);


--
-- Name: wb_locations wb_locations_name_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.wb_locations
    ADD CONSTRAINT wb_locations_name_key UNIQUE (name);


--
-- Name: wb_locations wb_locations_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.wb_locations
    ADD CONSTRAINT wb_locations_pkey PRIMARY KEY (location_id);


--
-- Name: wb_stations wb_stations_name_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.wb_stations
    ADD CONSTRAINT wb_stations_name_key UNIQUE (name);


--
-- Name: wb_stations wb_stations_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.wb_stations
    ADD CONSTRAINT wb_stations_pkey PRIMARY KEY (station_id);


--
-- Name: wb_times wb_times_phrase_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.wb_times
    ADD CONSTRAINT wb_times_phrase_key UNIQUE (phrase);


--
-- Name: wb_times wb_times_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.wb_times
    ADD CONSTRAINT wb_times_pkey PRIMARY KEY (phrase_id);


--
-- Name: idx_ai_bookings_status; Type: INDEX; Schema: ai; Owner: -
--

CREATE INDEX idx_ai_bookings_status ON ai.bookings USING btree (status);


--
-- Name: idx_ai_restrictions_road; Type: INDEX; Schema: ai; Owner: -
--

CREATE INDEX idx_ai_restrictions_road ON ai.road_restrictions USING btree (road_id);


--
-- Name: idx_ai_traffic_current; Type: INDEX; Schema: ai; Owner: -
--

CREATE INDEX idx_ai_traffic_current ON ai.traffic_conditions USING btree (road_id, expires_at DESC);


--
-- Name: idx_ai_training_cat; Type: INDEX; Schema: ai; Owner: -
--

CREATE INDEX idx_ai_training_cat ON ai.training_examples USING btree (category, split);


--
-- Name: idx_bookings_usr; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_bookings_usr ON public.bookings USING btree (user_id, status);


--
-- Name: idx_dlg_scenario; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_dlg_scenario ON public.gen_dialogue_scenarios USING btree (scenario_id, turn_no);


--
-- Name: idx_fleet_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_fleet_status ON public.fleet_vehicles USING btree (vehicle_use, status);


--
-- Name: idx_roads_from; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_roads_from ON public.roads USING btree (from_junction);


--
-- Name: idx_roads_to; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_roads_to ON public.roads USING btree (to_junction);


--
-- Name: idx_station_logs; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_station_logs ON public.station_traffic_log USING btree (station_id, detected_at DESC);


--
-- Name: idx_stations_geo; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_stations_geo ON public.fuel_stations USING btree (latitude, longitude);


--
-- Name: idx_train_slots_gin; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_train_slots_gin ON public.training_examples USING gin (slots);


--
-- Name: idx_train_split; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_train_split ON public.training_examples USING btree (split);


--
-- Name: idx_train_type_intent; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_train_type_intent ON public.training_examples USING btree (data_type, intent_label);


--
-- Name: idx_turns_dialogue; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_turns_dialogue ON public.dialogue_turns USING btree (dialogue_id, turn_no);


--
-- Name: bookings trg_bookings_before; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER trg_bookings_before BEFORE INSERT OR UPDATE ON public.bookings FOR EACH ROW EXECUTE FUNCTION public.trg_fn_booking_before();


--
-- Name: food_order_items trg_food_item_after; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER trg_food_item_after AFTER INSERT ON public.food_order_items FOR EACH ROW EXECUTE FUNCTION public.trg_fn_food_item_after();


--
-- Name: travel_passes trg_pass_fare; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER trg_pass_fare BEFORE INSERT ON public.travel_passes FOR EACH ROW EXECUTE FUNCTION public.trg_fn_pass_fare();


--
-- Name: test_drive_bookings trg_test_drive_area; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER trg_test_drive_area BEFORE INSERT ON public.test_drive_bookings FOR EACH ROW EXECUTE FUNCTION public.trg_fn_test_drive_area();


--
-- Name: bookings bookings_vehicle_id_fkey; Type: FK CONSTRAINT; Schema: ai; Owner: -
--

ALTER TABLE ONLY ai.bookings
    ADD CONSTRAINT bookings_vehicle_id_fkey FOREIGN KEY (vehicle_id) REFERENCES ai.vehicles(id);


--
-- Name: road_restrictions road_restrictions_road_id_fkey; Type: FK CONSTRAINT; Schema: ai; Owner: -
--

ALTER TABLE ONLY ai.road_restrictions
    ADD CONSTRAINT road_restrictions_road_id_fkey FOREIGN KEY (road_id) REFERENCES ai.roads(id) ON DELETE CASCADE;


--
-- Name: roads roads_source_id_fkey; Type: FK CONSTRAINT; Schema: ai; Owner: -
--

ALTER TABLE ONLY ai.roads
    ADD CONSTRAINT roads_source_id_fkey FOREIGN KEY (source_id) REFERENCES ai.data_sources(id);


--
-- Name: traffic_conditions traffic_conditions_road_id_fkey; Type: FK CONSTRAINT; Schema: ai; Owner: -
--

ALTER TABLE ONLY ai.traffic_conditions
    ADD CONSTRAINT traffic_conditions_road_id_fkey FOREIGN KEY (road_id) REFERENCES ai.roads(id) ON DELETE CASCADE;


--
-- Name: traffic_signals traffic_signals_road_id_fkey; Type: FK CONSTRAINT; Schema: ai; Owner: -
--

ALTER TABLE ONLY ai.traffic_signals
    ADD CONSTRAINT traffic_signals_road_id_fkey FOREIGN KEY (road_id) REFERENCES ai.roads(id) ON DELETE CASCADE;


--
-- Name: training_examples training_examples_source_id_fkey; Type: FK CONSTRAINT; Schema: ai; Owner: -
--

ALTER TABLE ONLY ai.training_examples
    ADD CONSTRAINT training_examples_source_id_fkey FOREIGN KEY (source_id) REFERENCES ai.data_sources(id);


--
-- Name: vehicles vehicles_source_id_fkey; Type: FK CONSTRAINT; Schema: ai; Owner: -
--

ALTER TABLE ONLY ai.vehicles
    ADD CONSTRAINT vehicles_source_id_fkey FOREIGN KEY (source_id) REFERENCES ai.data_sources(id);


--
-- Name: bookings bookings_destination_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.bookings
    ADD CONSTRAINT bookings_destination_id_fkey FOREIGN KEY (destination_id) REFERENCES public.locations(location_id);


--
-- Name: bookings bookings_driver_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.bookings
    ADD CONSTRAINT bookings_driver_id_fkey FOREIGN KEY (driver_id) REFERENCES public.driver_profiles(driver_id);


--
-- Name: bookings bookings_model_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.bookings
    ADD CONSTRAINT bookings_model_id_fkey FOREIGN KEY (model_id) REFERENCES public.car_models(model_id);


--
-- Name: bookings bookings_origin_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.bookings
    ADD CONSTRAINT bookings_origin_id_fkey FOREIGN KEY (origin_id) REFERENCES public.locations(location_id);


--
-- Name: bookings bookings_shared_ride_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.bookings
    ADD CONSTRAINT bookings_shared_ride_id_fkey FOREIGN KEY (shared_ride_id) REFERENCES public.shared_rides(ride_id);


--
-- Name: bookings bookings_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.bookings
    ADD CONSTRAINT bookings_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(user_id);


--
-- Name: bookings bookings_vehicle_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.bookings
    ADD CONSTRAINT bookings_vehicle_id_fkey FOREIGN KEY (vehicle_id) REFERENCES public.fleet_vehicles(vehicle_id);


--
-- Name: bus_routes bus_routes_bus_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.bus_routes
    ADD CONSTRAINT bus_routes_bus_id_fkey FOREIGN KEY (bus_id) REFERENCES public.buses(bus_id);


--
-- Name: bus_stops bus_stops_location_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.bus_stops
    ADD CONSTRAINT bus_stops_location_id_fkey FOREIGN KEY (location_id) REFERENCES public.locations(location_id);


--
-- Name: bus_stops bus_stops_route_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.bus_stops
    ADD CONSTRAINT bus_stops_route_id_fkey FOREIGN KEY (route_id) REFERENCES public.bus_routes(route_id) ON DELETE CASCADE;


--
-- Name: bus_tickets bus_tickets_trip_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.bus_tickets
    ADD CONSTRAINT bus_tickets_trip_id_fkey FOREIGN KEY (trip_id) REFERENCES public.luxury_bus_trips(trip_id);


--
-- Name: bus_tickets bus_tickets_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.bus_tickets
    ADD CONSTRAINT bus_tickets_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(user_id);


--
-- Name: buses buses_owner_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.buses
    ADD CONSTRAINT buses_owner_user_id_fkey FOREIGN KEY (owner_user_id) REFERENCES public.users(user_id);


--
-- Name: commissions commissions_bus_owner_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.commissions
    ADD CONSTRAINT commissions_bus_owner_id_fkey FOREIGN KEY (bus_owner_id) REFERENCES public.users(user_id);


--
-- Name: daily_routes daily_routes_destination_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.daily_routes
    ADD CONSTRAINT daily_routes_destination_id_fkey FOREIGN KEY (destination_id) REFERENCES public.locations(location_id);


--
-- Name: daily_routes daily_routes_origin_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.daily_routes
    ADD CONSTRAINT daily_routes_origin_id_fkey FOREIGN KEY (origin_id) REFERENCES public.locations(location_id);


--
-- Name: daily_routes daily_routes_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.daily_routes
    ADD CONSTRAINT daily_routes_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(user_id);


--
-- Name: dialogue_turns dialogue_turns_dialogue_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dialogue_turns
    ADD CONSTRAINT dialogue_turns_dialogue_id_fkey FOREIGN KEY (dialogue_id) REFERENCES public.training_examples(example_id) ON DELETE CASCADE;


--
-- Name: dialogue_turns dialogue_turns_intent_label_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.dialogue_turns
    ADD CONSTRAINT dialogue_turns_intent_label_fkey FOREIGN KEY (intent_label) REFERENCES public.gen_intents(intent);


--
-- Name: driver_profiles driver_profiles_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.driver_profiles
    ADD CONSTRAINT driver_profiles_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(user_id);


--
-- Name: fleet_vehicles fleet_vehicles_model_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.fleet_vehicles
    ADD CONSTRAINT fleet_vehicles_model_id_fkey FOREIGN KEY (model_id) REFERENCES public.car_models(model_id);


--
-- Name: fleet_vehicles fleet_vehicles_owner_driver_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.fleet_vehicles
    ADD CONSTRAINT fleet_vehicles_owner_driver_id_fkey FOREIGN KEY (owner_driver_id) REFERENCES public.driver_profiles(driver_id);


--
-- Name: fleet_vehicles fleet_vehicles_showroom_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.fleet_vehicles
    ADD CONSTRAINT fleet_vehicles_showroom_id_fkey FOREIGN KEY (showroom_id) REFERENCES public.showrooms(showroom_id);


--
-- Name: food_menu_items food_menu_items_restaurant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.food_menu_items
    ADD CONSTRAINT food_menu_items_restaurant_id_fkey FOREIGN KEY (restaurant_id) REFERENCES public.restaurants(restaurant_id) ON DELETE CASCADE;


--
-- Name: food_order_items food_order_items_item_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.food_order_items
    ADD CONSTRAINT food_order_items_item_id_fkey FOREIGN KEY (item_id) REFERENCES public.food_menu_items(item_id);


--
-- Name: food_order_items food_order_items_order_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.food_order_items
    ADD CONSTRAINT food_order_items_order_id_fkey FOREIGN KEY (order_id) REFERENCES public.food_orders(order_id) ON DELETE CASCADE;


--
-- Name: food_orders food_orders_halt_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.food_orders
    ADD CONSTRAINT food_orders_halt_id_fkey FOREIGN KEY (halt_id) REFERENCES public.trip_food_halts(halt_id);


--
-- Name: food_orders food_orders_ticket_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.food_orders
    ADD CONSTRAINT food_orders_ticket_id_fkey FOREIGN KEY (ticket_id) REFERENCES public.bus_tickets(ticket_id);


--
-- Name: fuel_stations fuel_stations_road_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.fuel_stations
    ADD CONSTRAINT fuel_stations_road_id_fkey FOREIGN KEY (road_id) REFERENCES public.roads(road_id);


--
-- Name: gen_dialogue_scenarios gen_dialogue_scenarios_primary_intent_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.gen_dialogue_scenarios
    ADD CONSTRAINT gen_dialogue_scenarios_primary_intent_fkey FOREIGN KEY (primary_intent) REFERENCES public.gen_intents(intent);


--
-- Name: gen_edge_frames gen_edge_frames_intent_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.gen_edge_frames
    ADD CONSTRAINT gen_edge_frames_intent_fkey FOREIGN KEY (intent) REFERENCES public.gen_intents(intent);


--
-- Name: gen_fn_map gen_fn_map_intent_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.gen_fn_map
    ADD CONSTRAINT gen_fn_map_intent_fkey FOREIGN KEY (intent) REFERENCES public.gen_intents(intent);


--
-- Name: gen_intent_frames gen_intent_frames_intent_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.gen_intent_frames
    ADD CONSTRAINT gen_intent_frames_intent_fkey FOREIGN KEY (intent) REFERENCES public.gen_intents(intent);


--
-- Name: gen_slot_frames gen_slot_frames_intent_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.gen_slot_frames
    ADD CONSTRAINT gen_slot_frames_intent_fkey FOREIGN KEY (intent) REFERENCES public.gen_intents(intent);


--
-- Name: locations locations_nearest_junction_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.locations
    ADD CONSTRAINT locations_nearest_junction_fkey FOREIGN KEY (nearest_junction) REFERENCES public.junctions(junction_id);


--
-- Name: luxury_bus_trips luxury_bus_trips_bus_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.luxury_bus_trips
    ADD CONSTRAINT luxury_bus_trips_bus_id_fkey FOREIGN KEY (bus_id) REFERENCES public.buses(bus_id);


--
-- Name: luxury_bus_trips luxury_bus_trips_destination_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.luxury_bus_trips
    ADD CONSTRAINT luxury_bus_trips_destination_id_fkey FOREIGN KEY (destination_id) REFERENCES public.locations(location_id);


--
-- Name: luxury_bus_trips luxury_bus_trips_origin_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.luxury_bus_trips
    ADD CONSTRAINT luxury_bus_trips_origin_id_fkey FOREIGN KEY (origin_id) REFERENCES public.locations(location_id);


--
-- Name: pass_scans pass_scans_pass_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.pass_scans
    ADD CONSTRAINT pass_scans_pass_id_fkey FOREIGN KEY (pass_id) REFERENCES public.travel_passes(pass_id);


--
-- Name: pass_scans pass_scans_scan_location_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.pass_scans
    ADD CONSTRAINT pass_scans_scan_location_fkey FOREIGN KEY (scan_location) REFERENCES public.locations(location_id);


--
-- Name: payments payments_booking_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.payments
    ADD CONSTRAINT payments_booking_id_fkey FOREIGN KEY (booking_id) REFERENCES public.bookings(booking_id);


--
-- Name: payments payments_food_order_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.payments
    ADD CONSTRAINT payments_food_order_id_fkey FOREIGN KEY (food_order_id) REFERENCES public.food_orders(order_id);


--
-- Name: payments payments_pass_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.payments
    ADD CONSTRAINT payments_pass_id_fkey FOREIGN KEY (pass_id) REFERENCES public.travel_passes(pass_id);


--
-- Name: payments payments_subscription_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.payments
    ADD CONSTRAINT payments_subscription_id_fkey FOREIGN KEY (subscription_id) REFERENCES public.premium_bus_subscriptions(subscription_id);


--
-- Name: payments payments_ticket_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.payments
    ADD CONSTRAINT payments_ticket_id_fkey FOREIGN KEY (ticket_id) REFERENCES public.bus_tickets(ticket_id);


--
-- Name: payments payments_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.payments
    ADD CONSTRAINT payments_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(user_id);


--
-- Name: premium_bus_subscriptions premium_bus_subscriptions_drop_stop_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.premium_bus_subscriptions
    ADD CONSTRAINT premium_bus_subscriptions_drop_stop_fkey FOREIGN KEY (drop_stop) REFERENCES public.bus_stops(stop_id);


--
-- Name: premium_bus_subscriptions premium_bus_subscriptions_pickup_stop_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.premium_bus_subscriptions
    ADD CONSTRAINT premium_bus_subscriptions_pickup_stop_fkey FOREIGN KEY (pickup_stop) REFERENCES public.bus_stops(stop_id);


--
-- Name: premium_bus_subscriptions premium_bus_subscriptions_route_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.premium_bus_subscriptions
    ADD CONSTRAINT premium_bus_subscriptions_route_id_fkey FOREIGN KEY (route_id) REFERENCES public.bus_routes(route_id);


--
-- Name: premium_bus_subscriptions premium_bus_subscriptions_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.premium_bus_subscriptions
    ADD CONSTRAINT premium_bus_subscriptions_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(user_id);


--
-- Name: roads roads_from_junction_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.roads
    ADD CONSTRAINT roads_from_junction_fkey FOREIGN KEY (from_junction) REFERENCES public.junctions(junction_id);


--
-- Name: roads roads_to_junction_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.roads
    ADD CONSTRAINT roads_to_junction_fkey FOREIGN KEY (to_junction) REFERENCES public.junctions(junction_id);


--
-- Name: shared_rides shared_rides_destination_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.shared_rides
    ADD CONSTRAINT shared_rides_destination_id_fkey FOREIGN KEY (destination_id) REFERENCES public.locations(location_id);


--
-- Name: shared_rides shared_rides_driver_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.shared_rides
    ADD CONSTRAINT shared_rides_driver_id_fkey FOREIGN KEY (driver_id) REFERENCES public.driver_profiles(driver_id);


--
-- Name: shared_rides shared_rides_origin_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.shared_rides
    ADD CONSTRAINT shared_rides_origin_id_fkey FOREIGN KEY (origin_id) REFERENCES public.locations(location_id);


--
-- Name: shared_rides shared_rides_vehicle_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.shared_rides
    ADD CONSTRAINT shared_rides_vehicle_id_fkey FOREIGN KEY (vehicle_id) REFERENCES public.fleet_vehicles(vehicle_id);


--
-- Name: showrooms showrooms_dealer_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.showrooms
    ADD CONSTRAINT showrooms_dealer_user_id_fkey FOREIGN KEY (dealer_user_id) REFERENCES public.users(user_id);


--
-- Name: station_reviews station_reviews_station_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.station_reviews
    ADD CONSTRAINT station_reviews_station_id_fkey FOREIGN KEY (station_id) REFERENCES public.fuel_stations(station_id) ON DELETE CASCADE;


--
-- Name: station_reviews station_reviews_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.station_reviews
    ADD CONSTRAINT station_reviews_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(user_id);


--
-- Name: station_traffic_log station_traffic_log_station_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.station_traffic_log
    ADD CONSTRAINT station_traffic_log_station_id_fkey FOREIGN KEY (station_id) REFERENCES public.fuel_stations(station_id) ON DELETE CASCADE;


--
-- Name: test_drive_bookings test_drive_bookings_booking_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.test_drive_bookings
    ADD CONSTRAINT test_drive_bookings_booking_id_fkey FOREIGN KEY (booking_id) REFERENCES public.bookings(booking_id);


--
-- Name: test_drive_bookings test_drive_bookings_model_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.test_drive_bookings
    ADD CONSTRAINT test_drive_bookings_model_id_fkey FOREIGN KEY (model_id) REFERENCES public.car_models(model_id);


--
-- Name: test_drive_bookings test_drive_bookings_showroom_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.test_drive_bookings
    ADD CONSTRAINT test_drive_bookings_showroom_id_fkey FOREIGN KEY (showroom_id) REFERENCES public.showrooms(showroom_id);


--
-- Name: test_drive_bookings test_drive_bookings_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.test_drive_bookings
    ADD CONSTRAINT test_drive_bookings_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(user_id);


--
-- Name: training_examples training_examples_intent_label_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.training_examples
    ADD CONSTRAINT training_examples_intent_label_fkey FOREIGN KEY (intent_label) REFERENCES public.gen_intents(intent);


--
-- Name: travel_passes travel_passes_route_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.travel_passes
    ADD CONSTRAINT travel_passes_route_id_fkey FOREIGN KEY (route_id) REFERENCES public.daily_routes(route_id);


--
-- Name: trip_food_halts trip_food_halts_restaurant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.trip_food_halts
    ADD CONSTRAINT trip_food_halts_restaurant_id_fkey FOREIGN KEY (restaurant_id) REFERENCES public.restaurants(restaurant_id);


--
-- Name: trip_food_halts trip_food_halts_trip_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.trip_food_halts
    ADD CONSTRAINT trip_food_halts_trip_id_fkey FOREIGN KEY (trip_id) REFERENCES public.luxury_bus_trips(trip_id);


--
-- Name: user_vehicles user_vehicles_model_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_vehicles
    ADD CONSTRAINT user_vehicles_model_id_fkey FOREIGN KEY (model_id) REFERENCES public.car_models(model_id);


--
-- Name: user_vehicles user_vehicles_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_vehicles
    ADD CONSTRAINT user_vehicles_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(user_id);


--
-- PostgreSQL database dump complete
--

\unrestrict omeaTB82BVdA0FV7hO60V3K7wgis2ntcsBhVCpNT0c4OfBT6ixMpXGlP6eVqEvq


import QRCode from "qrcode";
import { useEffect, useRef, useState } from "react";
import { api, ApiError, type Estimate, type Trip } from "../lib/api";
import { bytes, km, minutes, rupees, STATUS_LABEL } from "../lib/format";
import { gridDistanceM, toScene } from "../lib/geo";
import { cabPoses, isActive, useApp } from "../lib/store";

/** Idempotency key; randomUUID needs a secure context, getRandomValues does not. */
function newKey(): string {
  const b = crypto.getRandomValues(new Uint8Array(16));
  return Array.from(b, (x) => x.toString(16).padStart(2, "0")).join("");
}

export function RidePanel() {
  const trip = useApp((s) => s.trip);
  return trip ? <TripCard trip={trip} /> : <Planner />;
}

function Planner() {
  const city = useApp((s) => s.city)!;
  const pickupId = useApp((s) => s.pickupId);
  const dropId = useApp((s) => s.dropId);
  const set = useApp((s) => s.set);
  const [estimate, setEstimate] = useState<Estimate | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  // One key per booking attempt: a retried request cannot create a second trip.
  const idem = useRef<string>(newKey());

  useEffect(() => {
    setEstimate(null);
    setError(null);
    idem.current = newKey();
    if (!pickupId || !dropId) return;
    let stale = false;
    api
      .estimate(pickupId, dropId)
      .then((e) => !stale && setEstimate(e))
      .catch((e: unknown) => !stale && setError(e instanceof ApiError ? e.message : "Could not price this trip."));
    return () => {
      stale = true;
    };
  }, [pickupId, dropId]);

  const book = async () => {
    if (!pickupId || !dropId) return;
    setBusy(true);
    setError(null);
    try {
      const t = await api.book(pickupId, dropId, idem.current);
      useApp.getState().applyTrip(t);
      set({ pickupId: null, dropId: null, followCab: true });
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "Booking failed. Please try again.");
      if (e instanceof ApiError && e.status !== 0) idem.current = newKey();
    } finally {
      setBusy(false);
    }
  };

  const sorted = [...city.places].sort((a, b) => a.name.localeCompare(b.name));

  return (
    <div className="stack">
      <p className="muted">Choose places below, or tap the glowing beacons on the map.</p>
      <div className="field pickup">
        <label htmlFor="pickup">Pickup</label>
        <select id="pickup" value={pickupId ?? ""} onChange={(e) => set({ pickupId: e.target.value || null })}>
          <option value="">Select pickup</option>
          {sorted.map((p) => (
            <option key={p.id} value={p.id} disabled={p.id === dropId}>
              {p.name}
            </option>
          ))}
        </select>
      </div>
      <div className="field drop">
        <label htmlFor="drop">Drop</label>
        <select id="drop" value={dropId ?? ""} onChange={(e) => set({ dropId: e.target.value || null })}>
          <option value="">Select drop</option>
          {sorted.map((p) => (
            <option key={p.id} value={p.id} disabled={p.id === pickupId}>
              {p.name}
            </option>
          ))}
        </select>
      </div>

      {estimate && (
        <div className="card estimate">
          <div>
            <span>Distance</span>
            <b>{km(estimate.distance_m)}</b>
          </div>
          <div>
            <span>Drive time</span>
            <b>{minutes(estimate.duration_s)}</b>
          </div>
          <div>
            <span>Fare</span>
            <b className="accent">{rupees(estimate.fare_paise)}</b>
          </div>
        </div>
      )}
      {error && (
        <p className="error" role="alert">
          {error}
        </p>
      )}
      <button className="primary" disabled={!estimate || busy} onClick={() => void book()}>
        {busy ? "Finding the nearest cab…" : estimate ? `Book cab · ${rupees(estimate.fare_paise)}` : "Book cab"}
      </button>
      <p className="fine">Fare is paid to the driver at the end of the trip. Online payment is not enabled.</p>
    </div>
  );
}

function TripCard({ trip }: { trip: Trip }) {
  const city = useApp((s) => s.city)!;
  const quota = useApp((s) => s.quotaBytes);
  const follow = useApp((s) => s.followCab);
  const set = useApp((s) => s.set);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [away, setAway] = useState<number | null>(null);
  const [startQuota, setStartQuota] = useState<number | null>(null);
  const active = isActive(trip);

  // Distance from the cab to its next stop, from live positions.
  useEffect(() => {
    if (!active || !trip.cab_id) return;
    const tick = () => {
      const pose = cabPoses.get(trip.cab_id!);
      if (!pose) return;
      const goal = trip.status === "assigned" ? [trip.pickup_lat, trip.pickup_lng] : [trip.drop_lat, trip.drop_lng];
      setAway(gridDistanceM(toScene(city, pose.lat, pose.lng), toScene(city, goal[0], goal[1])));
    };
    tick();
    const h = setInterval(tick, 1000);
    return () => clearInterval(h);
  }, [active, trip.cab_id, trip.status, trip.pickup_lat, trip.pickup_lng, trip.drop_lat, trip.drop_lng, city]);

  useEffect(() => {
    if (trip.status === "in_progress" && startQuota === null && quota !== null) setStartQuota(quota);
  }, [trip.status, quota, startQuota]);

  const cancel = async () => {
    setBusy(true);
    setError(null);
    try {
      useApp.getState().applyTrip(await api.cancel(trip.id));
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "Could not cancel the trip.");
    } finally {
      setBusy(false);
    }
  };

  const steps = ["assigned", "in_progress", "completed"];
  const at = steps.indexOf(trip.status);
  const legTotal = trip.distance_m;
  const progress = trip.status === "in_progress" && away !== null ? Math.max(0, Math.min(1, 1 - away / legTotal)) : null;

  return (
    <div className="stack">
      <div className={`status-banner ${trip.status}`}>{STATUS_LABEL[trip.status]}</div>

      {trip.status !== "cancelled" && (
        <ol className="steps">
          {["Cab assigned", "On trip", "Arrived"].map((label, i) => (
            <li key={label} className={i < at ? "done" : i === at ? "now" : ""}>
              {label}
            </li>
          ))}
        </ol>
      )}

      <div className="card route">
        <div className="pt pickup">{trip.pickup_place}</div>
        <div className="pt drop">{trip.drop_place}</div>
        <div className="meta">
          {km(trip.distance_m)} · {rupees(trip.fare_paise)}
        </div>
      </div>

      {trip.cab && (
        <div className="card cab">
          <div>
            <b>{trip.cab.plate}</b>
            <span>
              {trip.cab.model}
              {trip.cab.electric ? " · EV" : ""} · {trip.cab.seats} seats
            </span>
            <span>Driver: {trip.cab.driver_name || "—"}</span>
          </div>
          {active && away !== null && (
            <div className="away">
              <b>{km(away)}</b>
              <span>{trip.status === "assigned" ? "from pickup" : "to drop"}</span>
            </div>
          )}
        </div>
      )}

      {progress !== null && (
        <div className="bar" aria-label="Trip progress">
          <i style={{ width: `${progress * 100}%` }} />
        </div>
      )}

      {trip.status === "assigned" && trip.boarding_code && <Boarding code={trip.boarding_code} tripId={trip.id} />}

      {trip.status === "in_progress" && trip.cab && (
        <div className="card wifi-live">
          <div>
            <span>Connected to</span>
            <b>{trip.cab.wifi_ssid}</b>
          </div>
          <div>
            <span>Used this trip</span>
            <b>{startQuota !== null && quota !== null ? bytes(Math.max(0, startQuota - quota)) : "—"}</b>
          </div>
          <div>
            <span>Quota left</span>
            <b className={quota === 0 ? "bad" : "accent"}>{quota !== null ? bytes(quota) : "—"}</b>
          </div>
        </div>
      )}
      {trip.status === "in_progress" && quota === 0 && (
        <p className="error">Your Wi-Fi quota is used up. Add a plan from the Wi-Fi tab to get back online.</p>
      )}

      {error && (
        <p className="error" role="alert">
          {error}
        </p>
      )}

      {active ? (
        <div className="row">
          <button className="ghost" onClick={() => set({ followCab: !follow })}>
            {follow ? "Free camera" : "Follow my cab"}
          </button>
          {trip.status === "assigned" && (
            <button className="danger" disabled={busy} onClick={() => void cancel()}>
              {busy ? "Cancelling…" : "Cancel trip"}
            </button>
          )}
        </div>
      ) : (
        <button className="primary" onClick={() => set({ trip: null })}>
          Book another ride
        </button>
      )}
    </div>
  );
}

function Boarding({ code, tripId }: { code: string; tripId: string }) {
  const [src, setSrc] = useState<string | null>(null);
  useEffect(() => {
    let stale = false;
    QRCode.toDataURL(JSON.stringify({ v: 1, trip: tripId, code }), { margin: 1, width: 168, color: { dark: "#0b1220", light: "#ffffff" } })
      .then((url) => !stale && setSrc(url))
      .catch(() => undefined);
    return () => {
      stale = true;
    };
  }, [code, tripId]);

  return (
    <div className="card boarding">
      {src ? <img src={src} width={112} height={112} alt="Boarding QR code" /> : <div className="qr-skel" />}
      <div>
        <span>Boarding code</span>
        <b className="digits">{code}</b>
        <p className="fine">Show this to your driver to start the trip.</p>
      </div>
    </div>
  );
}

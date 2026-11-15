import { useEffect, useState } from "react";
import { api, ApiError, type Trip } from "../lib/api";
import { km, rupees, STATUS_LABEL, when } from "../lib/format";
import { useApp } from "../lib/store";

type Timeline = Awaited<ReturnType<typeof api.timeline>>;

const EVENT_LABEL: Record<string, string> = {
  assigned: "Cab assigned",
  in_progress: "Trip started",
  completed: "Arrived",
  cancelled: "Cancelled",
};

export function TripsPanel() {
  const current = useApp((s) => s.trip);
  const [trips, setTrips] = useState<Trip[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [open, setOpen] = useState<string | null>(null);
  const [timeline, setTimeline] = useState<Timeline | null>(null);

  // Refetch when the live trip changes state so the list never lags the map.
  useEffect(() => {
    let stale = false;
    api
      .trips()
      .then((t) => !stale && setTrips(t))
      .catch((e: unknown) => !stale && setError(e instanceof ApiError ? e.message : "Could not load trips."));
    return () => {
      stale = true;
    };
  }, [current?.id, current?.status]);

  useEffect(() => {
    setTimeline(null);
    if (!open) return;
    let stale = false;
    api
      .timeline(open)
      .then((t) => !stale && setTimeline(t))
      .catch(() => !stale && setTimeline({ available: false, events: [], path: [] }));
    return () => {
      stale = true;
    };
  }, [open]);

  if (error) return <p className="error">{error}</p>;
  if (!trips) return <div className="loading">Loading trips…</div>;
  if (trips.length === 0) return <p className="muted">No trips yet. Book your first ride from the Ride tab.</p>;

  return (
    <ul className="trip-list">
      {trips.map((t) => (
        <li key={t.id} className={open === t.id ? "open" : ""}>
          <button className="trip-row" onClick={() => setOpen(open === t.id ? null : t.id)} aria-expanded={open === t.id}>
            <div>
              <b>
                {t.pickup_place} → {t.drop_place}
              </b>
              <span>
                {when(t.requested_at)} · {km(t.distance_m)}
              </span>
            </div>
            <div className="right">
              <b>{rupees(t.fare_paise)}</b>
              <span className={`pill ${t.status}`}>{STATUS_LABEL[t.status]}</span>
            </div>
          </button>
          {open === t.id && (
            <div className="timeline">
              {!timeline ? (
                <span className="muted">Loading timeline…</span>
              ) : !timeline.available ? (
                <span className="muted">Trip history storage is not available on this deployment.</span>
              ) : timeline.events.length === 0 ? (
                <span className="muted">No recorded events.</span>
              ) : (
                <>
                  <ol>
                    {timeline.events.map((e, i) => (
                      <li key={i}>
                        <b>{EVENT_LABEL[e.type] ?? e.type}</b>
                        <span>{when(e.ts)}</span>
                      </li>
                    ))}
                  </ol>
                  <span className="muted">{timeline.path.length} GPS points recorded</span>
                </>
              )}
            </div>
          )}
        </li>
      ))}
    </ul>
  );
}

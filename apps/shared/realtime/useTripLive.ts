/**
 * React hook for the passenger trip screen: live status + live car position.
 * Usage: const { status, driverPos } = useTripLive(rt, tripId, api);
 */
import { useEffect, useState } from "react";
import type { SarathiSocket } from "./sarathiSocket";

export interface DriverPos {
  lat: number;
  lng: number;
  heading: number;
  speedKmh: number;
}

export function useTripLive(
  rt: SarathiSocket,
  tripId: string | null,
  fetchTrip: (id: string) => Promise<{ status: string }>,
) {
  const [status, setStatus] = useState<string>("loading");
  const [driverPos, setDriverPos] = useState<DriverPos | null>(null);

  useEffect(() => {
    if (!tripId) return;
    const topic = `trip:${tripId}`;

    const offStatus = rt.on<{ tripId: string; status: string }>("trip.status", (d) => {
      if (d.tripId === tripId) setStatus(d.status);
    });
    const offLoc = rt.on<DriverPos & { tripId: string }>("driver.location", (d) => {
      if (d.tripId === tripId) setDriverPos(d);
    });

    // 1) subscribe, 2) THEN read current state over REST: anything published in
    // between is either in the REST answer or arrives on the socket.
    rt.subscribe(topic)
      .catch(() => {}) // retried automatically on reconnect
      .finally(() => fetchTrip(tripId).then((t) => setStatus(t.status)).catch(() => {}));

    return () => {
      offStatus();
      offLoc();
      void rt.unsubscribe(topic).catch(() => {});
    };
  }, [rt, tripId, fetchTrip]);

  return { status, driverPos };
}

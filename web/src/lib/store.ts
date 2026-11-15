import { create } from "zustand";
import { api, ApiError, type Cab, type Trip, type User, refreshSession, setAccessToken, setSessionLostHandler } from "./api";
import type { City } from "./geo";

/**
 * Live cab poses are kept outside React state: they change every second for every
 * cab and are read inside the render loop, so pushing them through React would
 * re-render the whole tree for nothing.
 */
export interface CabPose {
  lat: number;
  lng: number;
  heading: number;
  status: string;
}
export const cabPoses = new Map<string, CabPose>();

export type Tab = "ride" | "trips" | "wifi";
export type Link = "offline" | "connecting" | "live";

interface State {
  booting: boolean;
  user: User | null;
  city: City | null;
  cabs: Record<string, Cab>;
  trip: Trip | null; // the active (or just-finished) trip shown on the ride tab
  quotaBytes: number | null;
  quotaVersion: number; // bumped on every quota event so panels can refetch history
  pickupId: string | null;
  dropId: string | null;
  tab: Tab;
  link: Link;
  followCab: boolean;
  toast: string | null;

  boot: () => Promise<void>;
  signedIn: (user: User, accessToken: string) => Promise<void>;
  signOut: () => Promise<void>;
  resync: () => Promise<void>;
  selectPlace: (id: string) => void;
  set: (p: Partial<State>) => void;
  notify: (msg: string) => void;
  applyFleet: (cabs: { id: string; lat: number; lng: number; heading: number; status: string }[]) => void;
  applyTrip: (trip: Trip) => void;
}

const ACTIVE = new Set(["requested", "assigned", "in_progress"]);
export const isActive = (t: Trip | null) => !!t && ACTIVE.has(t.status);

let toastTimer: ReturnType<typeof setTimeout> | undefined;

export const useApp = create<State>((set, get) => ({
  booting: true,
  user: null,
  city: null,
  cabs: {},
  trip: null,
  quotaBytes: null,
  quotaVersion: 0,
  pickupId: null,
  dropId: null,
  tab: "ride",
  link: "offline",
  followCab: true,
  toast: null,

  set: (p) => set(p),

  notify: (msg) => {
    clearTimeout(toastTimer);
    set({ toast: msg });
    toastTimer = setTimeout(() => set({ toast: null }), 5000);
  },

  boot: async () => {
    setSessionLostHandler(() => {
      if (get().user) get().notify("Your session expired. Please sign in again.");
      setAccessToken(null);
      cabPoses.clear();
      set({ user: null, cabs: {}, trip: null, quotaBytes: null });
    });
    try {
      const [city, session] = await Promise.all([api.city(), refreshSession().catch(() => null)]);
      set({ city });
      if (session) {
        set({ user: session.user });
        await get().resync();
      }
    } catch (e) {
      get().notify(e instanceof ApiError ? e.message : "Could not load the city.");
    } finally {
      set({ booting: false });
    }
  },

  signedIn: async (user, accessToken) => {
    setAccessToken(accessToken);
    set({ user });
    await get().resync();
  },

  signOut: async () => {
    await api.logout().catch(() => undefined);
    setAccessToken(null);
    cabPoses.clear();
    set({ user: null, cabs: {}, trip: null, quotaBytes: null, pickupId: null, dropId: null, tab: "ride" });
  },

  // Pulls authoritative state; called after sign-in and after every WebSocket reconnect,
  // because events published while disconnected are not replayed.
  resync: async () => {
    const [cabs, trips, quota] = await Promise.all([api.cabs(), api.trips(), api.quota()]);
    const map: Record<string, Cab> = {};
    for (const c of cabs) {
      map[c.id] = c;
      if (c.lat != null && c.lng != null) {
        cabPoses.set(c.id, { lat: c.lat, lng: c.lng, heading: c.heading, status: c.status });
      }
    }
    for (const id of cabPoses.keys()) if (!map[id]) cabPoses.delete(id);
    const active = trips.find((t) => ACTIVE.has(t.status)) ?? null;
    const shown = get().trip;
    set({
      cabs: map,
      quotaBytes: quota.remaining_bytes,
      // Keep a just-finished trip on screen (with its final state) until dismissed.
      trip: active ?? (shown ? (trips.find((t) => t.id === shown.id) ?? shown) : null),
    });
  },

  selectPlace: (id) => {
    const { pickupId, dropId, trip } = get();
    if (isActive(trip)) return;
    if (!pickupId || (pickupId && dropId)) set({ pickupId: id, dropId: null });
    else if (id !== pickupId) set({ dropId: id });
  },

  applyFleet: (updates) => {
    let unknown = false;
    let statusChanged = false;
    const cabs = get().cabs;
    for (const u of updates) {
      if (u.status === "offline") {
        cabPoses.delete(u.id);
        statusChanged ||= !!cabs[u.id];
        continue;
      }
      cabPoses.set(u.id, { lat: u.lat, lng: u.lng, heading: u.heading, status: u.status });
      if (!cabs[u.id]) unknown = true;
      else if (cabs[u.id].status !== u.status) statusChanged = true;
    }
    if (unknown) {
      // A cab came online since the last snapshot: fetch its details.
      void api.cabs().then((list) => set({ cabs: Object.fromEntries(list.map((c) => [c.id, c])) })).catch(() => undefined);
    } else if (statusChanged) {
      const next = { ...cabs };
      for (const u of updates) {
        if (u.status === "offline") delete next[u.id];
        else if (next[u.id]) next[u.id] = { ...next[u.id], status: u.status as Cab["status"] };
      }
      set({ cabs: next });
    }
  },

  applyTrip: (trip) => {
    const prev = get().trip;
    if (prev && prev.id !== trip.id && isActive(prev)) return; // stale event for an older trip
    set({ trip });
    if (prev?.status !== trip.status) {
      if (trip.status === "in_progress") get().notify("Boarding verified. Enjoy the ride — Wi-Fi is on.");
      if (trip.status === "completed") get().notify(`You have arrived at ${trip.drop_place}.`);
    }
  },
}));

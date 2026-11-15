// Thin client for the SARATHI API. The access token lives in memory only; the refresh
// token is an HttpOnly cookie the browser sends to /api/v1/auth/* and scripts cannot read.

export interface User {
  id: string;
  phone: string;
  name: string;
  role: "passenger" | "driver" | "admin";
}

export interface Cab {
  id: string;
  plate: string;
  model: string;
  seats: number;
  electric: boolean;
  driver_name: string;
  simulated: boolean;
  status: "offline" | "available" | "on_trip";
  wifi_ssid: string;
  lat: number | null;
  lng: number | null;
  heading: number;
}

export interface Trip {
  id: string;
  status: "requested" | "assigned" | "in_progress" | "completed" | "cancelled";
  cab_id: string | null;
  pickup_place: string;
  drop_place: string;
  pickup_lat: number;
  pickup_lng: number;
  drop_lat: number;
  drop_lng: number;
  distance_m: number;
  duration_s: number;
  fare_paise: number;
  requested_at: string;
  started_at: string | null;
  completed_at: string | null;
  cab?: Cab;
  boarding_code?: string;
}

export interface Estimate {
  distance_m: number;
  duration_s: number;
  fare_paise: number;
}

export interface Plan {
  id: string;
  name: string;
  quota_bytes: number;
  price_paise: number;
}

export interface LedgerEntry {
  id: number;
  delta_bytes: number;
  reason: string;
  trip_id: string | null;
  created_at: string;
}

export interface Session {
  access_token: string;
  expires_in: number;
  user: User;
}

export class ApiError extends Error {
  constructor(
    public status: number,
    public code: string,
    message: string,
  ) {
    super(message);
  }
}

let accessToken: string | null = null;
let onSessionLost: (() => void) | null = null;
let refreshing: Promise<Session | null> | null = null;

export function setAccessToken(t: string | null) {
  accessToken = t;
}

export function setSessionLostHandler(fn: () => void) {
  onSessionLost = fn;
}

async function raw<T>(method: string, path: string, body?: unknown, headers: Record<string, string> = {}): Promise<T> {
  const h: Record<string, string> = { ...headers };
  if (body !== undefined) h["Content-Type"] = "application/json";
  if (accessToken) h.Authorization = `Bearer ${accessToken}`;
  let res: Response;
  try {
    res = await fetch(path, {
      method,
      headers: h,
      body: body === undefined ? undefined : JSON.stringify(body),
      credentials: "same-origin",
    });
  } catch {
    throw new ApiError(0, "network_error", "Cannot reach the server. Check your connection.");
  }
  const json = await res.json().catch(() => null);
  if (!res.ok) {
    const e = json?.error;
    throw new ApiError(res.status, e?.code ?? "http_error", e?.message ?? `Request failed (${res.status})`);
  }
  return json.data as T;
}

/** Exchanges the refresh cookie for a new access token. Concurrent callers share one request. */
export function refreshSession(): Promise<Session | null> {
  refreshing ??= raw<Session>("POST", "/api/v1/auth/refresh")
    .then((s) => {
      accessToken = s.access_token;
      return s;
    })
    .catch((e: unknown) => {
      // Only a definite rejection ends the session; a network blip must not sign the user out.
      if (e instanceof ApiError && e.status === 401) {
        accessToken = null;
        return null;
      }
      throw e;
    })
    .finally(() => {
      refreshing = null;
    });
  return refreshing;
}

/** Authenticated call that transparently refreshes an expired access token once. */
async function call<T>(method: string, path: string, body?: unknown, headers?: Record<string, string>): Promise<T> {
  try {
    return await raw<T>(method, path, body, headers);
  } catch (e) {
    if (!(e instanceof ApiError) || e.status !== 401) throw e;
    const s = await refreshSession();
    if (!s) {
      onSessionLost?.();
      throw e;
    }
    return raw<T>(method, path, body, headers);
  }
}

export const api = {
  city: () => raw<import("./geo").City>("GET", "/api/v1/city"),
  requestOtp: (phone: string) =>
    raw<{ expires_in: number; dev_code?: string }>("POST", "/api/v1/auth/otp/request", { phone }),
  verifyOtp: (phone: string, code: string, name: string) =>
    raw<Session>("POST", "/api/v1/auth/otp/verify", { phone, code, name }),
  logout: () => raw<unknown>("POST", "/api/v1/auth/logout"),

  cabs: () => call<Cab[]>("GET", "/api/v1/cabs"),
  estimate: (pickup: string, drop: string) =>
    call<Estimate>("POST", "/api/v1/trips/estimate", { pickup_place_id: pickup, drop_place_id: drop }),
  book: (pickup: string, drop: string, idempotencyKey: string) =>
    call<Trip>("POST", "/api/v1/trips", { pickup_place_id: pickup, drop_place_id: drop }, { "Idempotency-Key": idempotencyKey }),
  trips: () => call<Trip[]>("GET", "/api/v1/trips"),
  cancel: (id: string) => call<Trip>("POST", `/api/v1/trips/${id}/cancel`),
  timeline: (id: string) =>
    call<{ available: boolean; events: { type: string; ts: string }[]; path: { lat: number; lng: number; ts: string }[] }>(
      "GET",
      `/api/v1/trips/${id}/timeline`,
    ),

  plans: () => call<{ plans: Plan[]; purchases_enabled: boolean }>("GET", "/api/v1/wifi/plans"),
  quota: () => call<{ remaining_bytes: number; history: LedgerEntry[] }>("GET", "/api/v1/wifi/quota"),
  purchase: (planId: string) => call<{ remaining_bytes: number }>("POST", "/api/v1/wifi/purchase", { plan_id: planId }),

  wsTicket: () => call<{ ticket: string }>("POST", "/api/v1/ws/ticket"),
};

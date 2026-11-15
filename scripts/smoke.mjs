#!/usr/bin/env node
// End-to-end smoke test against a running LOCAL stack (needs the simulator, the log
// OTP sender and the mock payment provider, i.e. SARATHI_ENV=local).
//
//   node scripts/smoke.mjs                      # via the web container (nginx proxy)
//   API_BASE=http://127.0.0.1:8080 SARATHI_GATEWAY_TOKEN=... node scripts/smoke.mjs   # + gateway checks
//
// Requires Node 22+ (global fetch and WebSocket). Exits non-zero on the first failure.

const BASE = process.env.BASE ?? "http://127.0.0.1:3000";
// The gateway endpoint is not proxied by the web tier; give the API address to test it.
const API_BASE = process.env.API_BASE ?? "";
const GATEWAY_TOKEN = process.env.SARATHI_GATEWAY_TOKEN ?? "";
const gateway = (body, token) =>
  fetch(`${API_BASE}/internal/v1/gateway/usage`, {
    method: "POST",
    headers: { "Content-Type": "application/json", "X-Gateway-Token": token },
    body: JSON.stringify(body),
  }).then((r) => r.status);

let passed = 0;
function ok(cond, name, detail = "") {
  if (!cond) {
    console.error(`FAIL  ${name} ${detail}`);
    process.exit(1);
  }
  passed += 1;
  console.log(`ok    ${name}`);
}

class Client {
  token = null;
  cookie = null;
  async call(method, path, body, headers = {}) {
    const h = { ...headers };
    if (body !== undefined) h["Content-Type"] = "application/json";
    if (this.token) h.Authorization = `Bearer ${this.token}`;
    if (this.cookie && path.startsWith("/api/v1/auth")) h.Cookie = this.cookie;
    const res = await fetch(BASE + path, { method, headers: h, body: body === undefined ? undefined : JSON.stringify(body) });
    const set = res.headers.getSetCookie?.().find((c) => c.startsWith("sarathi_rt="));
    if (set) this.cookie = set.split(";")[0];
    const json = await res.json().catch(() => null);
    return { status: res.status, data: json?.data, error: json?.error, headers: res.headers, setCookie: set };
  }
  async signIn(phone, name) {
    const req = await this.call("POST", "/api/v1/auth/otp/request", { phone });
    if (req.status !== 200 || !req.data.dev_code) throw new Error(`otp request failed: ${JSON.stringify(req)}`);
    const ver = await this.call("POST", "/api/v1/auth/otp/verify", { phone, code: req.data.dev_code, name });
    if (ver.status !== 200) throw new Error(`otp verify failed: ${JSON.stringify(ver)}`);
    this.token = ver.data.access_token;
    return { req, ver };
  }
}

const rnd = () => String(Math.floor(Math.random() * 1e9)).padStart(9, "0");
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

// ---------- health & public ----------
const anon = new Client();
const ready = await anon.call("GET", "/api/v1/version");
ok(ready.status === 200, "API reachable", JSON.stringify(ready));
const city = await anon.call("GET", "/api/v1/city");
ok(city.status === 200 && city.data.places.length >= 10, "city has places");
ok((await anon.call("GET", "/api/v1/cabs")).status === 401, "cabs require authentication");
ok((await anon.call("GET", "/api/v1/nope")).error?.code === "not_found", "unknown route uses the error envelope");

// ---------- auth ----------
const alice = new Client();
const phoneA = `+919${rnd()}`;
const bad = await alice.call("POST", "/api/v1/auth/otp/request", { phone: "12345" });
ok(bad.status === 400 && bad.error.code === "invalid_phone", "malformed phone rejected");

const reqA = await alice.call("POST", "/api/v1/auth/otp/request", { phone: phoneA });
const wrongCode = reqA.data.dev_code === "000000" ? "111111" : "000000";
const wrong = await alice.call("POST", "/api/v1/auth/otp/verify", { phone: phoneA, code: wrongCode, name: "" });
ok(wrong.status === 401 && wrong.error.code === "otp_invalid", "wrong OTP rejected");
const verA = await alice.call("POST", "/api/v1/auth/otp/verify", { phone: phoneA, code: reqA.data.dev_code, name: "Alice" });
ok(verA.status === 200 && verA.data.user.role === "passenger", "correct OTP signs in");
ok(/HttpOnly/i.test(verA.setCookie) && /SameSite=Strict/i.test(verA.setCookie), "refresh cookie is HttpOnly + SameSite=Strict");
ok(verA.data.refresh_token === undefined, "refresh token is not exposed to browser clients");
alice.token = verA.data.access_token;
const replay = await alice.call("POST", "/api/v1/auth/otp/verify", { phone: phoneA, code: reqA.data.dev_code, name: "" });
ok(replay.status === 401, "OTP is single-use");

const oldCookie = alice.cookie;
const ref = await alice.call("POST", "/api/v1/auth/refresh");
ok(ref.status === 200 && ref.data.access_token, "refresh rotates the session");
alice.token = ref.data.access_token;
const stolen = new Client();
stolen.cookie = oldCookie;
ok((await stolen.call("POST", "/api/v1/auth/refresh")).status === 401, "reused refresh token is rejected");
ok((await alice.call("POST", "/api/v1/auth/refresh")).status === 401, "token reuse revokes the whole session family");
await alice.signIn(phoneA, "Alice");

const tampered = new Client();
tampered.token = alice.token.slice(0, -3) + "abc";
ok((await tampered.call("GET", "/api/v1/me")).status === 401, "tampered access token rejected");
ok((await alice.call("GET", "/api/v1/admin/stats")).status === 403, "passenger cannot reach admin endpoints");

// ---------- quota & plans ----------
const q0 = await alice.call("GET", "/api/v1/wifi/quota");
ok(q0.data.remaining_bytes === 500 * 2 ** 20, "signup grants 500 MiB", String(q0.data?.remaining_bytes));
const plans = await alice.call("GET", "/api/v1/wifi/plans");
ok(plans.data.plans.length === 3 && plans.data.purchases_enabled, "plans listed");
const buy = await alice.call("POST", "/api/v1/wifi/purchase", { plan_id: "ride-1g" });
ok(buy.status === 201 && buy.data.remaining_bytes === 500 * 2 ** 20 + 2 ** 30, "plan purchase credits the ledger exactly");
ok((await alice.call("POST", "/api/v1/wifi/purchase", { plan_id: "nope" })).status === 404, "unknown plan rejected");

// ---------- fleet & realtime ----------
const cabs = await alice.call("GET", "/api/v1/cabs");
ok(cabs.data.length >= 5 && cabs.data.every((c) => c.lat !== null), "simulated fleet is online with positions", String(cabs.data?.length));

const ticket = await alice.call("POST", "/api/v1/ws/ticket");
const wsUrl = BASE.replace(/^http/, "ws") + "/api/v1/ws?ticket=";
const events = [];
const ws = new WebSocket(wsUrl + ticket.data.ticket);
await new Promise((res, rej) => {
  ws.onopen = res;
  ws.onerror = () => rej(new Error("websocket failed to open"));
});
ws.onmessage = (e) => events.push(JSON.parse(e.data));
ok(true, "WebSocket connects with a ticket");
const reuse = new WebSocket(wsUrl + ticket.data.ticket);
const reuseFailed = await new Promise((res) => {
  reuse.onopen = () => res(false);
  reuse.onerror = () => res(true);
});
ok(reuseFailed, "WebSocket ticket is single-use");
await sleep(2500);
ok(events.some((e) => e.type === "fleet" && e.cabs.length > 0), "fleet positions stream over WebSocket");

// ---------- trips ----------
const est = await alice.call("POST", "/api/v1/trips/estimate", { pickup_place_id: "mg-road", drop_place_id: "cubbon-park" });
ok(est.status === 200 && est.data.distance_m === 450 && est.data.fare_paise > 0, "estimate priced by the server");
ok((await alice.call("POST", "/api/v1/trips/estimate", { pickup_place_id: "mg-road", drop_place_id: "mg-road" })).status === 400, "same pickup and drop rejected");
ok((await alice.call("POST", "/api/v1/trips", { pickup_place_id: "mg-road", drop_place_id: "cubbon-park", fare_paise: 1 })).status === 400, "client cannot supply a fare (unknown fields rejected)");

const idem = `smoke-${rnd()}`;
const body = { pickup_place_id: "mg-road", drop_place_id: "cubbon-park" };
const [b1, b2] = await Promise.all([
  alice.call("POST", "/api/v1/trips", body, { "Idempotency-Key": idem }),
  alice.call("POST", "/api/v1/trips", body, { "Idempotency-Key": `${idem}-other` }),
]);
const created = [b1, b2].filter((r) => r.status === 201);
const conflicts = [b1, b2].filter((r) => r.status === 409);
ok(created.length === 1 && conflicts.length === 1, "concurrent double booking yields exactly one trip", `${b1.status}/${b2.status}`);
const trip = created[0].data;
ok(trip.status === "assigned" && trip.cab && /^\d{6}$/.test(trip.boarding_code), "trip assigned with cab and boarding code");
ok(trip.fare_paise === est.data.fare_paise, "booked fare equals the estimate");
if (b1.status === 201) {
  const again = await alice.call("POST", "/api/v1/trips", body, { "Idempotency-Key": idem });
  ok(again.status === 200 && again.data.id === trip.id, "idempotent replay returns the same trip");
}

// Another user must not see or cancel this trip.
const bob = new Client();
await bob.signIn(`+919${rnd()}`, "Bob");
ok((await bob.call("GET", `/api/v1/trips/${trip.id}`)).status === 404, "other users cannot read the trip");
ok((await bob.call("POST", `/api/v1/trips/${trip.id}/cancel`)).status === 404, "other users cannot cancel the trip");
ok((await bob.call("GET", "/api/v1/driver/cab")).status === 403, "passenger cannot use driver endpoints");

// Gateway endpoint is closed without the shared secret.
const gwBody = { report_id: `smoke-${rnd()}`, trip_id: trip.id, bytes: 1024 };
ok((await anon.call("POST", "/internal/v1/gateway/usage", gwBody, { "X-Gateway-Token": GATEWAY_TOKEN })).status !== 200 || BASE === API_BASE, "gateway endpoint is not reachable through the web tier");
if (API_BASE) ok((await gateway(gwBody, "wrong")) === 401, "gateway usage rejects a bad token");

// The simulator drives the cab: assigned -> in_progress -> completed.
console.log("      waiting for the simulated cab to complete the trip…");
const deadline = Date.now() + 180_000;
let final = null;
while (Date.now() < deadline) {
  const t = events.find((e) => e.type === "trip" && e.trip.id === trip.id && e.trip.status === "completed");
  if (t) {
    final = t.trip;
    break;
  }
  await sleep(1000);
}
ok(final, "trip completed (pushed over WebSocket)");
const statuses = events.filter((e) => e.type === "trip" && e.trip.id === trip.id).map((e) => e.trip.status);
ok(statuses.includes("in_progress"), "in_progress was pushed before completion", statuses.join(","));
ok(events.some((e) => e.type === "quota"), "quota updates pushed during the trip");

if (API_BASE && GATEWAY_TOKEN) {
  ok((await gateway(gwBody, GATEWAY_TOKEN)) === 409, "gateway cannot meter a finished trip");
}

const q1 = await alice.call("GET", "/api/v1/wifi/quota");
const used = buy.data.remaining_bytes - q1.data.remaining_bytes;
ok(used > 0, `usage was metered against the ledger (${(used / 2 ** 20).toFixed(1)} MiB)`);
const usageSum = -q1.data.history.filter((h) => h.reason === "usage").reduce((a, h) => a + h.delta_bytes, 0);
ok(usageSum === used, "ledger usage rows sum to the balance change byte-for-byte", `${usageSum} vs ${used}`);

await sleep(1500); // telemetry is flushed to MongoDB once a second
const tl = await alice.call("GET", `/api/v1/trips/${trip.id}/timeline`);
ok(tl.data.available, "MongoDB telemetry is attached");
ok(["assigned", "in_progress", "completed"].every((s) => tl.data.events.some((e) => e.type === s)), "trip timeline stored in MongoDB");
ok(tl.data.path.length > 3, `cab path stored in MongoDB (${tl.data.path.length} points)`);

ok((await alice.call("POST", `/api/v1/trips/${trip.id}/cancel`)).status === 409, "completed trip cannot be cancelled");
const hist = await alice.call("GET", "/api/v1/trips");
ok(hist.data[0].id === trip.id && hist.data[0].status === "completed", "trip appears in history");

// A second trip can be booked and cancelled; the cab is released.
const t2 = await alice.call("POST", "/api/v1/trips", { pickup_place_id: "hebbal", drop_place_id: "hsr-layout" });
ok(t2.status === 201, "second trip can be booked after the first completes");
const cancelled = await alice.call("POST", `/api/v1/trips/${t2.data.id}/cancel`);
ok(cancelled.status === 200 && cancelled.data.status === "cancelled" && cancelled.data.cab.status === "available", "cancel releases the cab");

// ---------- sign out ----------
ws.close();
const logout = await alice.call("POST", "/api/v1/auth/logout");
ok(logout.status === 200, "logout succeeds");
ok((await alice.call("POST", "/api/v1/auth/refresh")).status === 401, "refresh fails after logout");

console.log(`\nAll ${passed} checks passed against ${BASE}`);
process.exit(0);

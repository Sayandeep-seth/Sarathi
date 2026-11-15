import { useEffect, useState } from "react";
import { api, ApiError, type LedgerEntry, type Plan } from "../lib/api";
import { bytes, rupees, when } from "../lib/format";
import { useApp } from "../lib/store";

const REASON: Record<string, string> = {
  signup_grant: "Welcome bonus",
  plan_purchase: "Plan purchased",
  usage: "In-ride usage",
  adjustment: "Adjustment",
};

/** Consecutive usage rows are metered every few seconds; show them as one line. */
function condense(history: LedgerEntry[]): LedgerEntry[] {
  const out: LedgerEntry[] = [];
  for (const e of history) {
    const last = out[out.length - 1];
    if (last && e.reason === "usage" && last.reason === "usage" && last.trip_id === e.trip_id) {
      last.delta_bytes += e.delta_bytes;
    } else {
      out.push({ ...e });
    }
  }
  return out;
}

export function WifiPanel() {
  const quota = useApp((s) => s.quotaBytes);
  const version = useApp((s) => s.quotaVersion);
  const [plans, setPlans] = useState<Plan[]>([]);
  const [enabled, setEnabled] = useState(true);
  const [history, setHistory] = useState<LedgerEntry[]>([]);
  const [buying, setBuying] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    api
      .plans()
      .then((r) => {
        setPlans(r.plans);
        setEnabled(r.purchases_enabled);
      })
      .catch((e: unknown) => setError(e instanceof ApiError ? e.message : "Could not load plans."));
  }, []);

  // Usage events arrive every second during a trip; refresh history at most every 5 s.
  const bucket = Math.floor(version / 5);
  useEffect(() => {
    let stale = false;
    api
      .quota()
      .then((q) => {
        if (stale) return;
        setHistory(q.history);
        useApp.getState().set({ quotaBytes: q.remaining_bytes });
      })
      .catch(() => undefined);
    return () => {
      stale = true;
    };
  }, [bucket]);

  const buy = async (p: Plan) => {
    setBuying(p.id);
    setError(null);
    try {
      const r = await api.purchase(p.id);
      useApp.getState().set({ quotaBytes: r.remaining_bytes, quotaVersion: useApp.getState().quotaVersion + 5 });
      useApp.getState().notify(`${p.name} added to your account.`);
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "Purchase failed.");
    } finally {
      setBuying(null);
    }
  };

  const largest = Math.max(quota ?? 0, 2 ** 30);
  const pct = quota === null ? 0 : Math.min(100, (quota / largest) * 100);

  return (
    <div className="stack">
      <div className="card quota">
        <span>Wi-Fi quota remaining</span>
        <b className={quota === 0 ? "bad" : ""}>{quota === null ? "—" : bytes(quota)}</b>
        <div className="bar">
          <i style={{ width: `${pct}%` }} />
        </div>
        <p className="fine">Usage is metered by the cab's gateway while you are on a trip.</p>
      </div>

      <h3>Plans</h3>
      {!enabled && <p className="fine">Online payments are not enabled on this deployment, so plans cannot be purchased here.</p>}
      {enabled && <p className="fine">Demo payment provider: no money is charged.</p>}
      <ul className="plans">
        {plans.map((p) => (
          <li key={p.id} className="card">
            <div>
              <b>{p.name}</b>
              <span>{bytes(p.quota_bytes)}</span>
            </div>
            <button className="primary small" disabled={!enabled || buying !== null} onClick={() => void buy(p)}>
              {buying === p.id ? "Adding…" : rupees(p.price_paise)}
            </button>
          </li>
        ))}
      </ul>
      {error && (
        <p className="error" role="alert">
          {error}
        </p>
      )}

      <h3>Recent activity</h3>
      <ul className="ledger">
        {condense(history).map((e) => (
          <li key={e.id}>
            <div>
              <b>{REASON[e.reason] ?? e.reason}</b>
              <span>{when(e.created_at)}</span>
            </div>
            <b className={e.delta_bytes > 0 ? "good" : "muted"}>
              {e.delta_bytes > 0 ? "+" : "−"}
              {bytes(Math.abs(e.delta_bytes))}
            </b>
          </li>
        ))}
        {history.length === 0 && <li className="muted">No activity yet.</li>}
      </ul>
    </div>
  );
}

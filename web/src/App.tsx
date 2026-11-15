import { Component, type ReactNode, Suspense, lazy, useEffect } from "react";
import { startRealtime } from "./lib/realtime";
import { useApp } from "./lib/store";
import { AuthPanel } from "./ui/AuthPanel";
import { RidePanel } from "./ui/RidePanel";
import { TripsPanel } from "./ui/TripsPanel";
import { WifiPanel } from "./ui/WifiPanel";

// three.js is large: load the scene separately so the shell paints first.
const Scene = lazy(() => import("./scene/Scene").then((m) => ({ default: m.Scene })));

/** If WebGL is unavailable the panels must keep working without the map. */
class SceneBoundary extends Component<{ children: ReactNode }, { failed: boolean }> {
  state = { failed: false };
  static getDerivedStateFromError() {
    return { failed: true };
  }
  render() {
    return this.state.failed ? (
      <div className="scene-fallback">3D view is unavailable on this device. Booking still works from the panel.</div>
    ) : (
      this.props.children
    );
  }
}

export function App() {
  const booting = useApp((s) => s.booting);
  const user = useApp((s) => s.user);
  const tab = useApp((s) => s.tab);
  const link = useApp((s) => s.link);
  const toast = useApp((s) => s.toast);
  const cabs = useApp((s) => s.cabs);
  const set = useApp((s) => s.set);

  useEffect(() => {
    void useApp.getState().boot();
  }, []);

  const userId = user?.id;
  useEffect(() => (userId ? startRealtime() : undefined), [userId]);

  const list = Object.values(cabs);
  const available = list.filter((c) => c.status === "available").length;

  return (
    <div className="app">
      <div className="scene">
        <SceneBoundary>
          <Suspense fallback={null}>
            <Scene />
          </Suspense>
        </SceneBoundary>
      </div>

      <aside className="panel" aria-label="SARATHI">
        <header className="brand">
          <div className="logo" aria-hidden>
            <svg viewBox="0 0 64 64" width="22" height="22">
              <path
                d="M14 40l6-16a6 6 0 0 1 5.6-4h12.8a6 6 0 0 1 5.6 4l6 16v6a3 3 0 0 1-3 3h-2a3 3 0 0 1-3-3v-1H22v1a3 3 0 0 1-3 3h-2a3 3 0 0 1-3-3z"
                fill="currentColor"
              />
            </svg>
          </div>
          <div>
            <h1>SARATHI</h1>
            <p>Connected cabs · in-ride Wi-Fi</p>
          </div>
          {user && (
            <button className="ghost small" onClick={() => void useApp.getState().signOut()}>
              Sign out
            </button>
          )}
        </header>

        {booting ? (
          <div className="loading">Loading the city…</div>
        ) : !user ? (
          <AuthPanel />
        ) : (
          <>
            <nav className="tabs" role="tablist">
              {(["ride", "trips", "wifi"] as const).map((t) => (
                <button key={t} role="tab" aria-selected={tab === t} className={tab === t ? "active" : ""} onClick={() => set({ tab: t })}>
                  {t === "ride" ? "Ride" : t === "trips" ? "My trips" : "Wi-Fi"}
                </button>
              ))}
            </nav>
            <div className="panel-body">
              {tab === "ride" && <RidePanel />}
              {tab === "trips" && <TripsPanel />}
              {tab === "wifi" && <WifiPanel />}
            </div>
          </>
        )}
      </aside>

      {user && (
        <div className="hud">
          <span className={`chip conn ${link}`}>
            <i />
            {link === "live" ? "Live" : link === "connecting" ? "Reconnecting…" : "Offline"}
          </span>
          <span className="chip">
            <b>{available}</b> available
          </span>
          <span className="chip">
            <b>{list.length - available}</b> on trip
          </span>
        </div>
      )}

      <div className="legend" aria-hidden>
        <span>
          <i style={{ background: "#f6b21b" }} /> Available
        </span>
        <span>
          <i style={{ background: "#22c7b8" }} /> On trip
        </span>
        <span>
          <i style={{ background: "#ffffff" }} /> Your cab
        </span>
        <span className="hint">Drag to orbit · scroll to zoom · right-drag to pan</span>
      </div>

      {toast && (
        <div className="toast" role="status">
          {toast}
        </div>
      )}
    </div>
  );
}

import { api } from "./api";
import { useApp } from "./store";

/**
 * Maintains one WebSocket while a user is signed in. Each connection is authenticated
 * with a fresh single-use ticket. On every (re)connect the app resyncs over HTTP,
 * since the server does not replay events missed while offline.
 */
export function startRealtime(): () => void {
  let socket: WebSocket | null = null;
  let retry: ReturnType<typeof setTimeout> | undefined;
  let attempt = 0;
  let stopped = false;

  const schedule = () => {
    if (stopped) return;
    useApp.getState().set({ link: "connecting" });
    // Exponential backoff with jitter, capped at 15 s.
    const delay = Math.min(15_000, 500 * 2 ** attempt) * (0.5 + Math.random() / 2);
    attempt += 1;
    retry = setTimeout(connect, delay);
  };

  const connect = async () => {
    if (stopped) return;
    useApp.getState().set({ link: "connecting" });
    let ticket: string;
    try {
      ticket = (await api.wsTicket()).ticket;
    } catch {
      schedule();
      return;
    }
    if (stopped) return;
    const proto = location.protocol === "https:" ? "wss:" : "ws:";
    const ws = new WebSocket(`${proto}//${location.host}/api/v1/ws?ticket=${ticket}`);
    socket = ws;

    ws.onopen = () => {
      attempt = 0;
      useApp.getState().set({ link: "live" });
      void useApp.getState().resync().catch(() => undefined);
    };
    ws.onmessage = (ev) => {
      let msg: any;
      try {
        msg = JSON.parse(ev.data as string);
      } catch {
        return;
      }
      const app = useApp.getState();
      switch (msg.type) {
        case "fleet":
          app.applyFleet(msg.cabs);
          break;
        case "trip":
          app.applyTrip(msg.trip);
          break;
        case "quota":
          app.set({ quotaBytes: msg.remaining_bytes, quotaVersion: app.quotaVersion + 1 });
          break;
      }
    };
    ws.onclose = () => {
      if (socket === ws) socket = null;
      schedule();
    };
    ws.onerror = () => ws.close();
  };

  void connect();

  return () => {
    stopped = true;
    clearTimeout(retry);
    socket?.close();
    useApp.getState().set({ link: "offline" });
  };
}

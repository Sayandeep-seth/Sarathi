/**
 * SarathiSocket: one reconnecting WebSocket per app (passenger, driver, bus).
 * Works in browsers and React Native (both have a global WebSocket).
 *
 *   const rt = new SarathiSocket({
 *     wsUrl: "wss://rt.sarathi.in/ws",
 *     getTicket: async () => (await api.post("/v1/realtime/ticket")).data.ticket,
 *   });
 *   rt.on("trip.status", (data) => setTrip(data));
 *   rt.connect();
 *   await rt.subscribe("trip:7f3a");
 */

export interface Envelope<T = unknown> {
  type: string;
  ref?: string;
  topic?: string;
  data?: T;
  ts?: number;
}

export type ConnectionState = "idle" | "connecting" | "open" | "reconnecting" | "closed";

export interface SarathiSocketOptions {
  wsUrl: string;
  /** Calls POST /v1/realtime/ticket with the user's normal JWT and returns the ticket. */
  getTicket: () => Promise<string>;
  /** Called after every (re)connect: re-fetch trip/booking state over REST here. */
  onResync?: () => void;
  onStateChange?: (s: ConnectionState) => void;
  requestTimeoutMs?: number;
}

type Handler = (data: any, env: Envelope) => void;

interface Pending {
  resolve: () => void;
  reject: (e: Error) => void;
  timer: ReturnType<typeof setTimeout>;
}

export class SarathiSocket {
  private ws: WebSocket | null = null;
  private state: ConnectionState = "idle";
  private attempt = 0;
  private refSeq = 0;
  private handlers = new Map<string, Set<Handler>>();
  private pending = new Map<string, Pending>();
  private topics = new Set<string>(); // re-subscribed after every reconnect
  private heartbeat: ReturnType<typeof setInterval> | null = null;
  private lastPongAt = 0;
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null;
  private manualClose = false;

  constructor(private opts: SarathiSocketOptions) {}

  // ---------------------------------------------------------------- public

  connect(): void {
    this.manualClose = false;
    void this.open();
  }

  close(): void {
    this.manualClose = true;
    this.clearTimers();
    this.ws?.close(1000, "bye");
    this.setState("closed");
  }

  /** Listen to a server message type, e.g. "driver.location". Returns an unsubscribe fn. */
  on<T = any>(type: string, fn: (data: T, env: Envelope<T>) => void): () => void {
    if (!this.handlers.has(type)) this.handlers.set(type, new Set());
    this.handlers.get(type)!.add(fn as Handler);
    return () => this.handlers.get(type)?.delete(fn as Handler);
  }

  async subscribe(topic: string): Promise<void> {
    this.topics.add(topic);
    if (this.state === "open") await this.request({ type: "subscribe", topic });
  }

  async unsubscribe(topic: string): Promise<void> {
    this.topics.delete(topic);
    if (this.state === "open") await this.request({ type: "unsubscribe", topic });
  }

  /** Send and wait for ack / error (uses `ref`). */
  request(env: Omit<Envelope, "ref" | "ts">): Promise<void> {
    if (!this.ws || this.state !== "open") return Promise.reject(new Error("socket not open"));
    const ref = `c${++this.refSeq}`;
    return new Promise<void>((resolve, reject) => {
      const timer = setTimeout(() => {
        this.pending.delete(ref);
        reject(new Error(`timeout waiting for ${env.type}`));
      }, this.opts.requestTimeoutMs ?? 8000);
      this.pending.set(ref, { resolve, reject, timer });
      this.ws!.send(JSON.stringify({ ...env, ref }));
    });
  }

  /** Fire-and-forget (GPS frames). Silently dropped while offline: stale GPS is useless. */
  send(env: Omit<Envelope, "ts">): boolean {
    if (!this.ws || this.state !== "open") return false;
    this.ws.send(JSON.stringify(env));
    return true;
  }

  // -------------------------------------------------------------- internal

  private async open(): Promise<void> {
    this.setState(this.attempt === 0 ? "connecting" : "reconnecting");
    let ticket: string;
    try {
      ticket = await this.opts.getTicket(); // fresh one-time ticket every attempt
    } catch {
      return this.scheduleReconnect();
    }

    const ws = new WebSocket(`${this.opts.wsUrl}?ticket=${encodeURIComponent(ticket)}`);
    this.ws = ws;

    ws.onopen = () => {
      this.attempt = 0;
      this.setState("open");
      this.lastPongAt = Date.now();
      this.startHeartbeat();
      // Restore subscriptions, then let the app re-read state it may have missed.
      Promise.allSettled([...this.topics].map((t) => this.request({ type: "subscribe", topic: t }))).then(
        () => this.opts.onResync?.(),
      );
    };

    ws.onmessage = (ev) => {
      let env: Envelope;
      try {
        env = JSON.parse(typeof ev.data === "string" ? ev.data : "");
      } catch {
        return;
      }
      this.dispatch(env);
    };

    ws.onclose = (ev) => {
      this.clearTimers();
      this.failPending(new Error("socket closed"));
      if (this.manualClose) return;
      // 1008 policy violation / 4401 = our auth problems: retrying immediately is pointless.
      if (ev.code === 1008) this.attempt = Math.max(this.attempt, 5);
      this.scheduleReconnect();
    };

    ws.onerror = () => {
      /* onclose always follows; handled there */
    };
  }

  private dispatch(env: Envelope): void {
    if (env.type === "pong") this.lastPongAt = Date.now();

    if (env.ref && this.pending.has(env.ref)) {
      const p = this.pending.get(env.ref)!;
      clearTimeout(p.timer);
      this.pending.delete(env.ref);
      if (env.type === "error") {
        const d = env.data as { code?: string; message?: string } | undefined;
        p.reject(Object.assign(new Error(d?.message ?? "error"), { code: d?.code }));
      } else {
        p.resolve();
      }
    }

    this.handlers.get(env.type)?.forEach((fn) => {
      try {
        fn(env.data, env);
      } catch (e) {
        console.error("[rt] handler error", env.type, e);
      }
    });
    this.handlers.get("*")?.forEach((fn) => fn(env.data, env));
  }

  /** App-level ping every 25s; if no pong for 60s the socket is dead (common on mobile). */
  private startHeartbeat(): void {
    this.heartbeat = setInterval(() => {
      if (Date.now() - this.lastPongAt > 60_000) {
        this.ws?.close(4000, "heartbeat timeout");
        return;
      }
      this.send({ type: "ping" });
    }, 25_000);
  }

  /** Exponential back-off with full jitter: 0.5s, 1s, 2s ... capped at 30s, randomised
   *  so 10,000 phones do not all reconnect in the same second after a deploy. */
  private scheduleReconnect(): void {
    if (this.manualClose) return;
    this.setState("reconnecting");
    const base = Math.min(30_000, 500 * 2 ** this.attempt);
    const delay = Math.random() * base;
    this.attempt++;
    this.reconnectTimer = setTimeout(() => void this.open(), delay);
  }

  private failPending(err: Error): void {
    this.pending.forEach((p) => {
      clearTimeout(p.timer);
      p.reject(err);
    });
    this.pending.clear();
  }

  private clearTimers(): void {
    if (this.heartbeat) clearInterval(this.heartbeat);
    if (this.reconnectTimer) clearTimeout(this.reconnectTimer);
    this.heartbeat = null;
    this.reconnectTimer = null;
  }

  private setState(s: ConnectionState): void {
    if (this.state === s) return;
    this.state = s;
    this.opts.onStateChange?.(s);
  }
}

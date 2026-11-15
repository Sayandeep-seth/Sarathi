import { type FormEvent, useState } from "react";
import { api, ApiError } from "../lib/api";
import { useApp } from "../lib/store";

export function AuthPanel() {
  const [step, setStep] = useState<"phone" | "code">("phone");
  const [local, setLocal] = useState("");
  const [name, setName] = useState("");
  const [code, setCode] = useState("");
  const [devCode, setDevCode] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const phone = `+91${local}`;
  const phoneOk = /^[6-9]\d{9}$/.test(local);

  const run = async (fn: () => Promise<void>) => {
    setBusy(true);
    setError(null);
    try {
      await fn();
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "Something went wrong. Please try again.");
    } finally {
      setBusy(false);
    }
  };

  const request = (e?: FormEvent) => {
    e?.preventDefault();
    if (!phoneOk) return;
    void run(async () => {
      const r = await api.requestOtp(phone);
      setDevCode(r.dev_code ?? null);
      setCode("");
      setStep("code");
    });
  };

  const verify = (e: FormEvent) => {
    e.preventDefault();
    if (code.length !== 6) return;
    void run(async () => {
      const s = await api.verifyOtp(phone, code, name.trim());
      await useApp.getState().signedIn(s.user, s.access_token);
    });
  };

  return (
    <div className="panel-body auth">
      <h2>Ride the connected city</h2>
      <p className="muted">Book a cab, watch it arrive live in 3D, and stay online with metered in-ride Wi-Fi.</p>

      {step === "phone" ? (
        <form onSubmit={request}>
          <label htmlFor="phone">Mobile number</label>
          <div className="phone-row">
            <span>+91</span>
            <input
              id="phone"
              inputMode="numeric"
              autoComplete="tel-national"
              placeholder="98765 43210"
              maxLength={10}
              value={local}
              onChange={(e) => setLocal(e.target.value.replace(/\D/g, ""))}
              autoFocus
            />
          </div>
          <label htmlFor="name">
            Your name <span className="muted">(optional)</span>
          </label>
          <input id="name" autoComplete="name" maxLength={80} value={name} onChange={(e) => setName(e.target.value)} />
          <button className="primary" disabled={!phoneOk || busy}>
            {busy ? "Sending…" : "Send verification code"}
          </button>
        </form>
      ) : (
        <form onSubmit={verify}>
          <label htmlFor="code">Enter the 6-digit code sent to {phone}</label>
          <input
            id="code"
            className="code"
            inputMode="numeric"
            autoComplete="one-time-code"
            maxLength={6}
            value={code}
            onChange={(e) => setCode(e.target.value.replace(/\D/g, ""))}
            autoFocus
          />
          {devCode && (
            <p className="dev-note">
              Local development: no SMS is sent. Your code is{" "}
              <button type="button" className="link" onClick={() => setCode(devCode)}>
                {devCode}
              </button>
            </p>
          )}
          <button className="primary" disabled={code.length !== 6 || busy}>
            {busy ? "Verifying…" : "Verify and continue"}
          </button>
          <div className="row">
            <button type="button" className="ghost" onClick={() => setStep("phone")} disabled={busy}>
              Change number
            </button>
            <button type="button" className="ghost" onClick={() => request()} disabled={busy}>
              Resend code
            </button>
          </div>
        </form>
      )}
      {error && (
        <p className="error" role="alert">
          {error}
        </p>
      )}
    </div>
  );
}

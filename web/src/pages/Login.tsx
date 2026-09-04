import { FormEvent, useState } from "react";
import { useNavigate } from "react-router-dom";
import { gql, getDeviceId, getDeviceName, setToken, User } from "../api";

export default function LoginPage() {
  const nav = useNavigate();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [mfaCode, setMfaCode] = useState("");
  const [mfaRequired, setMfaRequired] = useState(false);
  const [deviceOtp, setDeviceOtp] = useState("");
  const [newDevice, setNewDevice] = useState(false);
  const [ephemeral, setEphemeral] = useState("");
  const [mode, setMode] = useState<"login" | "register">("login");
  const [consent, setConsent] = useState(false);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    setError("");
    setBusy(true);
    try {
      if (mode === "register") {
        const data = await gql<{ register: { token: string; user: User } }>(
          `mutation ($email: String!, $password: String!, $deviceId: String, $deviceName: String) {
            register(email: $email, password: $password, deviceId: $deviceId, deviceName: $deviceName) { token user { id email mfaEnabled } }
          }`,
          { email, password, deviceId: getDeviceId(), deviceName: getDeviceName() }
        );
        if (!data.register.token) throw new Error("Registration did not return a session");
        setToken(data.register.token);
        await gql(
          `mutation { recordConsent(purpose: "processing", accepted: true) { purpose accepted createdAt } }`
        );
        nav("/");
        return;
      }
      const data = await gql<{
        login: {
          token: string | null;
          mfaRequired: boolean;
          newDevice: boolean;
          ephemeralCode?: string | null;
          user: User | null;
        };
      }>(
        `mutation ($email: String!, $password: String!, $mfaCode: String, $deviceId: String!, $deviceName: String!, $deviceOtp: String) {
          login(email: $email, password: $password, mfaCode: $mfaCode, deviceId: $deviceId, deviceName: $deviceName, deviceOtp: $deviceOtp) {
            token mfaRequired newDevice ephemeralCode user { id email mfaEnabled }
          }
        }`,
        {
          email,
          password,
          mfaCode: mfaCode || null,
          deviceId: getDeviceId(),
          deviceName: getDeviceName(),
          deviceOtp: deviceOtp || null,
        }
      );
      if (data.login.mfaRequired && !data.login.token) {
        setMfaRequired(true);
        return;
      }
      if (data.login.newDevice && !data.login.token) {
        setNewDevice(true);
        setEphemeral(data.login.ephemeralCode || "");
        return;
      }
      if (!data.login.token) throw new Error("Login failed");
      setToken(data.login.token);
      nav("/");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="auth">
      <div className="panel auth-card">
        <p className="eyebrow">azula</p>
        <h1>your pipeline failed. let&apos;s find out why.</h1>
        <p className="muted">Investigate logs, configs, and code — then debate the root cause across two models. Each browser or desktop app is a separate trusted device.</p>
        <form onSubmit={onSubmit}>
          <label>
            Email
            <input value={email} onChange={(e) => setEmail(e.target.value)} type="email" required autoComplete="username" />
          </label>
          <label>
            Password
            <input
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              type="password"
              required
              minLength={8}
              autoComplete={mode === "login" ? "current-password" : "new-password"}
            />
          </label>
          {mfaRequired && (
            <label>
              Authenticator code
              <input
                value={mfaCode}
                onChange={(e) => setMfaCode(e.target.value)}
                inputMode="numeric"
                autoComplete="one-time-code"
                placeholder="6-digit TOTP"
                required
              />
            </label>
          )}
          {newDevice && (
            <label>
              New device code
              <input
                value={deviceOtp}
                onChange={(e) => setDeviceOtp(e.target.value)}
                inputMode="numeric"
                placeholder="6-digit code from email"
                required
              />
            </label>
          )}
          {newDevice && (
            <p className="muted">
              {ephemeral
                ? `Demo echo (DEVICE_OTP_ECHO): ${ephemeral}`
                : "This device is not on your trusted list yet. Check email or data/outbox for the 6-digit code. Other trusted devices stay signed in."}
            </p>
          )}
          {mode === "register" && (
            <label className="legal consent-check">
              <input type="checkbox" checked={consent} onChange={(e) => setConsent(e.target.checked)} required />
              I agree to processing of project files and prompts for incident investigation (KVKK / GDPR). I can export or delete my data later.
            </label>
          )}
          {error && <p className="error">{error}</p>}
          <button type="submit" disabled={busy || (mode === "register" && !consent)}>
            {busy ? "Working…" : mode === "login" ? "Sign in" : "Create account"}
          </button>
        </form>
        <button className="linkish" type="button" onClick={() => setMode(mode === "login" ? "register" : "login")}>
          {mode === "login" ? "Need an account? Register" : "Already registered? Sign in"}
        </button>
      </div>
    </div>
  );
}

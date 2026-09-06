import { FormEvent, useEffect, useState } from "react";
import { useNavigate, useSearchParams } from "react-router-dom";
import { apiOrigin, formatApiError, gql, getDeviceId, getDeviceName, hasSession, setToken, User } from "../api";
import { LanguageToggle, useI18n } from "../i18n";

export default function LoginPage() {
  const { t } = useI18n();
  const nav = useNavigate();
  const [params] = useSearchParams();
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
  const [sso, setSso] = useState(false);

  useEffect(() => {
    const token = params.get("ssoToken");
    if (token) {
      setToken(token);
      nav("/", { replace: true });
      return;
    }
    if (params.get("sso") === "1" && hasSession()) {
      nav("/", { replace: true });
    }
  }, [params, nav]);

  useEffect(() => {
    gql<{ authFeatures: { ssoEnabled: boolean } }>(`query { authFeatures { ssoEnabled } }`)
      .then((d) => setSso(d.authFeatures.ssoEnabled))
      .catch(() => setSso(false));
  }, []);

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
        if (!data.register.token) throw new Error(t("registerNoSession"));
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
      if (!data.login.token) throw new Error(t("loginFailed"));
      setToken(data.login.token);
      nav("/");
    } catch (err) {
      setError(formatApiError(err, t));
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="auth">
      <div className="panel auth-card">
        <div className="row bar">
          <p className="eyebrow">azula</p>
          <LanguageToggle />
        </div>
        <h1>{t("loginHeadline")}</h1>
        <p className="muted">{t("loginLead")}</p>
        <form onSubmit={onSubmit}>
          <label>
            {t("email")}
            <input value={email} onChange={(e) => setEmail(e.target.value)} type="email" required autoComplete="username" />
          </label>
          <label>
            {t("password")}
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
              {t("authenticatorCode")}
              <input
                value={mfaCode}
                onChange={(e) => setMfaCode(e.target.value)}
                inputMode="numeric"
                autoComplete="one-time-code"
                placeholder={t("totpPlaceholder")}
                required
              />
            </label>
          )}
          {newDevice && (
            <label>
              {t("newDeviceCode")}
              <input
                value={deviceOtp}
                onChange={(e) => setDeviceOtp(e.target.value)}
                inputMode="numeric"
                placeholder={t("emailCodePlaceholder")}
                required
              />
            </label>
          )}
          {newDevice && (
            <p className="muted">
              {ephemeral ? t("deviceOtpDemo", { code: ephemeral }) : t("deviceOtpHint")}
            </p>
          )}
          {mode === "register" && (
            <label className="legal consent-check">
              <input type="checkbox" checked={consent} onChange={(e) => setConsent(e.target.checked)} required />
              {t("registerConsent")}
            </label>
          )}
          {error && <p className="error">{error}</p>}
          <button type="submit" disabled={busy || (mode === "register" && !consent)}>
            {busy ? t("working") : mode === "login" ? t("signIn") : t("createAccount")}
          </button>
        </form>
        {sso && mode === "login" && (
          <button
            type="button"
            className="primary"
            style={{ marginTop: 12 }}
            onClick={() => {
              const q = new URLSearchParams({
                deviceId: getDeviceId(),
                deviceName: getDeviceName(),
              });
              window.location.href = `${apiOrigin()}/auth/oidc/start?${q.toString()}`;
            }}
          >
            {t("sso")}
          </button>
        )}
        <button className="linkish" type="button" onClick={() => setMode(mode === "login" ? "register" : "login")}>
          {mode === "login" ? t("needAccount") : t("haveAccount")}
        </button>
      </div>
    </div>
  );
}

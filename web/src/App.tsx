import { useEffect, useState } from "react";
import { Navigate, NavLink, Route, Routes } from "react-router-dom";
import { ConsentRecord, gql, goLogin, hasSession, setToken } from "./api";
import { LanguageToggle, useI18n } from "./i18n";
import LoginPage from "./pages/Login";
import HomePage from "./pages/Home";
import InvestigationPage from "./pages/Investigation";
import DashboardPage from "./pages/Dashboard";
import SecurityPage from "./pages/Security";
import LoopPage from "./pages/Loop";
import TrustPage from "./pages/Trust";

function Guard({ children }: { children: React.ReactNode }) {
  if (!hasSession()) return <Navigate to="/login" replace />;
  return children;
}

function Shell({ children }: { children: React.ReactNode }) {
  const { t } = useI18n();
  const [consent, setConsent] = useState<ConsentRecord | null | undefined>(undefined);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    gql<{ myConsent: ConsentRecord | null }>(`query { myConsent { purpose accepted createdAt } }`)
      .then((d) => setConsent(d.myConsent))
      .catch(() => setConsent(null));
  }, []);

  async function accept() {
    setBusy(true);
    setError("");
    try {
      const data = await gql<{ recordConsent: ConsentRecord }>(
        `mutation { recordConsent(purpose: "processing", accepted: true) { purpose accepted createdAt } }`
      );
      setConsent(data.recordConsent);
    } catch (e) {
      setError(e instanceof Error ? e.message : t("consentFail"));
    } finally {
      setBusy(false);
    }
  }

  const needsConsent = consent !== undefined && (!consent || !consent.accepted);

  return (
    <div className="app">
      <header className="topbar">
        <NavLink to="/" className="brand">
          azula
        </NavLink>
        <nav>
          <NavLink to="/" end>
            {t("navProjects")}
          </NavLink>
          <NavLink to="/dashboard">{t("navThink")}</NavLink>
          <NavLink to="/loop">{t("navNext")}</NavLink>
          <NavLink to="/security">{t("navAccount")}</NavLink>
        </nav>
        <div className="top-meta">
          <NavLink to="/trust" className="quiet-link">
            {t("navTrust")}
          </NavLink>
          <LanguageToggle />
          <button
            type="button"
            className="ghost"
            onClick={() => {
              void gql(`mutation { logout }`)
                .catch(() => undefined)
                .finally(() => {
                  setToken(null);
                  goLogin();
                });
            }}
          >
            {t("signOut")}
          </button>
        </div>
      </header>
      {needsConsent && (
        <div className="consent-bar legal">
          <p>
            {t("consentBar")}
          </p>
          {error && <p className="error">{error}</p>}
          <button type="button" className="primary" disabled={busy} onClick={() => void accept()}>
            {busy ? t("consentSaving") : t("consentAccept")}
          </button>
        </div>
      )}
      <main>{children}</main>
    </div>
  );
}

export default function App() {
  return (
    <Routes>
      <Route path="/login" element={<LoginPage />} />
      <Route
        path="/trust"
        element={
          hasSession() ? (
            <Guard>
              <Shell>
                <TrustPage />
              </Shell>
            </Guard>
          ) : (
            <TrustPage />
          )
        }
      />
      <Route
        path="/"
        element={
          <Guard>
            <Shell>
              <HomePage />
            </Shell>
          </Guard>
        }
      />
      <Route
        path="/investigation/:id"
        element={
          <Guard>
            <Shell>
              <InvestigationPage />
            </Shell>
          </Guard>
        }
      />
      <Route
        path="/loop/:projectId"
        element={
          <Guard>
            <Shell>
              <LoopPage />
            </Shell>
          </Guard>
        }
      />
      <Route
        path="/loop"
        element={
          <Guard>
            <Shell>
              <LoopPage />
            </Shell>
          </Guard>
        }
      />
      <Route
        path="/dashboard"
        element={
          <Guard>
            <Shell>
              <DashboardPage />
            </Shell>
          </Guard>
        }
      />
      <Route
        path="/security"
        element={
          <Guard>
            <Shell>
              <SecurityPage />
            </Shell>
          </Guard>
        }
      />
      <Route path="*" element={hasSession() ? <Navigate to="/" replace /> : <Navigate to="/login" replace />} />
    </Routes>
  );
}

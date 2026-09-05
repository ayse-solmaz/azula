import { FormEvent, useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { QRCodeSVG } from "qrcode.react";
import {
  ConsentRecord,
  Entitlements,
  forgetLocalDevice,
  getDeviceId,
  goLogin,
  gql,
  Organization,
  User,
  deviceShortLabel,
  setToken,
} from "../api";
import { EmptyState, formatWhen, UpgradeBanner } from "../ui";
import { LanguageToggle, useI18n } from "../i18n";

const USER_FIELDS = `
  id email displayName mfaEnabled tier orgId orgName orgRole ssoLinked createdAt
  notifyEmail notifyInvestigations notifyMarketing shareUsage
  trustedDevices { id name createdAt lastSeenAt }
`;

const ME_CORE = `
  me { id email mfaEnabled tier orgId orgName orgRole }
  myConsent { purpose accepted createdAt }
  myOrganization { id name members { email role userId } }
`;

const ME_QUERY = `query {
  me { ${USER_FIELDS} }
  entitlements {
    tier maxProjects maxInvestigationsPerMonth investigationsUsed
    deepAnalysis council generate evaluate gitMcp modelSelection
    billingConfigured ssoEnabled demoUpgrade
  }
  myConsent { purpose accepted createdAt }
  myOrganization { id name members { email role userId } }
}`;

type Section =
  | "profile"
  | "security"
  | "notifications"
  | "privacy"
  | "billing"
  | "org"
  | "prefs"
  | "support"
  | "manage";

function initials(user: User) {
  const src = (user.displayName || user.email || "?").trim();
  const parts = src.split(/[\s@._-]+/).filter(Boolean);
  const a = parts[0]?.[0] || "?";
  const b = parts[1]?.[0] || parts[0]?.[1] || "";
  return (a + b).toUpperCase();
}

function sectionFromHash(): Section {
  const h = window.location.hash.replace("#", "") as Section;
  const ok: Section[] = [
    "profile",
    "security",
    "notifications",
    "privacy",
    "billing",
    "org",
    "prefs",
    "support",
    "manage",
  ];
  return ok.includes(h) ? h : "profile";
}

export default function SecurityPage() {
  const { t, locale, theme, setTheme, textSize, setTextSize } = useI18n();
  const [section, setSection] = useState<Section>(sectionFromHash);
  const [user, setUser] = useState<User | null>(null);
  const [org, setOrg] = useState<Organization | null>(null);
  const [secret, setSecret] = useState("");
  const [otpauth, setOtpauth] = useState("");
  const [code, setCode] = useState("");
  const [error, setError] = useState("");
  const [ok, setOk] = useState("");
  const [orgName, setOrgName] = useState("");
  const [inviteEmail, setInviteEmail] = useState("");
  const [inviteRole, setInviteRole] = useState("engineer");
  const [consent, setConsent] = useState<ConsentRecord | null>(null);
  const [ent, setEnt] = useState<Entitlements | null>(null);
  const [displayName, setDisplayName] = useState("");
  const [currentPw, setCurrentPw] = useState("");
  const [newPw, setNewPw] = useState("");
  const [notifyEmail, setNotifyEmail] = useState(true);
  const [notifyInv, setNotifyInv] = useState(true);
  const [notifyMkt, setNotifyMkt] = useState(false);
  const [shareUsage, setShareUsage] = useState(false);

  const nav: { id: Section; label: Parameters<typeof t>[0] }[] = [
    { id: "profile", label: "accNavProfile" },
    { id: "security", label: "accNavSecurity" },
    { id: "notifications", label: "accNavNotify" },
    { id: "privacy", label: "accNavPrivacy" },
    { id: "billing", label: "accNavBilling" },
    { id: "org", label: "accNavOrg" },
    { id: "prefs", label: "accNavPrefs" },
    { id: "support", label: "accNavSupport" },
    { id: "manage", label: "accNavManage" },
  ];

  useEffect(() => {
    const onHash = () => setSection(sectionFromHash());
    window.addEventListener("hashchange", onHash);
    return () => window.removeEventListener("hashchange", onHash);
  }, []);

  async function load() {
    setError("");
    type Payload = {
      me: User;
      entitlements?: Entitlements;
      myConsent: ConsentRecord | null;
      myOrganization: Organization | null;
    };
    try {
      const data = await gql<Payload>(ME_QUERY);
      setUser(data.me);
      setDisplayName(data.me.displayName || "");
      setNotifyEmail(data.me.notifyEmail !== false);
      setNotifyInv(data.me.notifyInvestigations !== false);
      setNotifyMkt(!!data.me.notifyMarketing);
      setShareUsage(!!data.me.shareUsage);
      setEnt(data.entitlements || null);
      setConsent(data.myConsent);
      setOrg(data.myOrganization);
      return;
    } catch (first) {
      const data = await gql<Payload>(`query { ${ME_CORE} }`);
      setUser({ ...data.me, trustedDevices: data.me.trustedDevices || [] });
      setConsent(data.myConsent);
      setOrg(data.myOrganization);
      setError(first instanceof Error ? first.message : t("partialLoad"));
    }
  }

  useEffect(() => {
    load().catch((e) => setError(e.message));
  }, []);

  async function enroll() {
    setError("");
    setOk("");
    try {
      const data = await gql<{ enrollMfa: { secret: string; otpauthUrl: string } }>(
        `mutation { enrollMfa { secret otpauthUrl } }`
      );
      setSecret(data.enrollMfa.secret);
      setOtpauth(data.enrollMfa.otpauthUrl);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed");
    }
  }

  async function enable(e: FormEvent) {
    e.preventDefault();
    setError("");
    try {
      const data = await gql<{ enableMfa: User }>(
        `mutation ($code: String!) { enableMfa(code: $code) { ${USER_FIELDS} } }`,
        { code }
      );
      setUser(data.enableMfa);
      setOk(t("mfaOnMsg"));
      setSecret("");
      setOtpauth("");
      setCode("");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed");
    }
  }

  async function disable(e: FormEvent) {
    e.preventDefault();
    setError("");
    try {
      const data = await gql<{ disableMfa: User }>(
        `mutation ($code: String!) { disableMfa(code: $code) { ${USER_FIELDS} } }`,
        { code }
      );
      setUser(data.disableMfa);
      setOk(t("mfaOffMsg"));
      setCode("");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed");
    }
  }

  async function exportData() {
    setError("");
    setOk("");
    try {
      const data = await gql<{ exportMyData: string }>(`mutation { exportMyData }`);
      const blob = new Blob([data.exportMyData], { type: "application/json" });
      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      a.download = "azula-export.json";
      a.click();
      URL.revokeObjectURL(url);
      setOk(t("exportOk"));
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed");
    }
  }

  async function revokeDevice(id: string) {
    setError("");
    setOk("");
    try {
      const data = await gql<{ revokeTrustedDevice: User }>(
        `mutation ($id: ID!) { revokeTrustedDevice(deviceId: $id) { ${USER_FIELDS} } }`,
        { id }
      );
      setUser(data.revokeTrustedDevice);
      if (id === getDeviceId()) {
        forgetLocalDevice();
        goLogin();
        return;
      }
      setOk(t("revoked"));
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed");
    }
  }

  function simulateNewDevice() {
    forgetLocalDevice();
    goLogin();
  }

  async function wipe() {
    if (!confirm(t("wipeConfirm"))) return;
    setError("");
    try {
      await gql(`mutation { deleteAccount }`);
      setToken(null);
      goLogin();
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed");
    }
  }

  async function deactivate() {
    if (!confirm(t("deactivateConfirm"))) return;
    setError("");
    try {
      await gql(`mutation { deactivateAccount }`);
      setToken(null);
      goLogin();
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed");
    }
  }

  async function saveProfile(e: FormEvent) {
    e.preventDefault();
    setError("");
    try {
      const data = await gql<{ updateProfile: User }>(
        `mutation ($n: String!) { updateProfile(displayName: $n) { ${USER_FIELDS} } }`,
        { n: displayName }
      );
      setUser(data.updateProfile);
      setOk(t("profileSaved"));
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed");
    }
  }

  async function savePassword(e: FormEvent) {
    e.preventDefault();
    setError("");
    try {
      await gql(`mutation ($c: String!, $n: String!) { changePassword(currentPassword: $c, newPassword: $n) }`, {
        c: currentPw,
        n: newPw,
      });
      setCurrentPw("");
      setNewPw("");
      setOk(t("passwordSaved"));
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed");
    }
  }

  async function saveNotif(e: FormEvent) {
    e.preventDefault();
    setError("");
    try {
      const data = await gql<{ updateAccountPrefs: User }>(
        `mutation ($e: Boolean, $i: Boolean, $m: Boolean, $s: Boolean) {
          updateAccountPrefs(notifyEmail: $e, notifyInvestigations: $i, notifyMarketing: $m, shareUsage: $s) { ${USER_FIELDS} }
        }`,
        { e: notifyEmail, i: notifyInv, m: notifyMkt, s: shareUsage }
      );
      setUser(data.updateAccountPrefs);
      setOk(t("prefsSaved"));
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed");
    }
  }

  function signOut() {
    void gql(`mutation { logout }`)
      .catch(() => undefined)
      .finally(() => {
        setToken(null);
        goLogin();
      });
  }

  if (!user) {
    return (
      <div className="page">
        <section className="panel">
          <h2>{t("account")}</h2>
          <p className="feed-lead">{t("accountLoadLead")}</p>
          {error ? <p className="error">{error}</p> : <p className="muted">{t("loading")}</p>}
          <button type="button" onClick={() => void load().catch((e) => setError(e.message))}>
            {t("retry")}
          </button>
        </section>
      </div>
    );
  }

  const show = (id: Section) => section === id;

  return (
    <div className="page">
      <div className="account-layout">
        <nav className="account-nav" aria-label={t("account")}>
          {nav.map((item) => (
            <a
              key={item.id}
              href={`#${item.id}`}
              className={section === item.id ? "active" : ""}
              onClick={() => setSection(item.id)}
            >
              {t(item.label)}
            </a>
          ))}
        </nav>
        <div>
          {error && <p className="error">{error}</p>}
          {ok && <p className="ok">{ok}</p>}

          {show("profile") && (
            <section className="panel">
              <div className="feed-head">
                <h2>{t("profileTitle")}</h2>
                <p className="feed-lead">{t("profileLead")}</p>
              </div>
              <div className="row bar" style={{ marginBottom: 16 }}>
                <div className="avatar-lg" aria-hidden>
                  {initials(user)}
                </div>
                <div>
                  <p>
                    <strong>{user.displayName || user.email}</strong>
                  </p>
                  <p className="muted">
                    {user.ssoLinked ? t("ssoLinked") : t("ssoNotLinked")} · {user.tier}
                  </p>
                  {user.createdAt ? <p className="hint">{t("memberSince", { when: formatWhen(user.createdAt, locale) })}</p> : null}
                </div>
              </div>
              <p className="hint">{t("avatarHint")}</p>
              <form className="stack-form" onSubmit={saveProfile}>
                <label>
                  {t("displayName")}
                  <input value={displayName} onChange={(e) => setDisplayName(e.target.value)} placeholder={t("displayNamePh")} maxLength={80} />
                </label>
                <label>
                  {t("emailIdentity")}
                  <input value={user.email} readOnly />
                </label>
                <button type="submit" className="primary">
                  {t("saveProfile")}
                </button>
              </form>
            </section>
          )}

          {show("security") && (
            <>
              <section className="panel">
                <div className="feed-head">
                  <h2>{t("accountTitle")}</h2>
                  <p className="feed-lead">{t("accountLead", { email: user.email })}</p>
                </div>
                <p>
                  Status:{" "}
                  {user.mfaEnabled ? (
                    <span className="badge ok">{t("mfaOn")}</span>
                  ) : (
                    <span className="badge fail">{t("mfaOff")}</span>
                  )}{" "}
                  <span className="badge">{user.tier}</span>
                </p>
                {!user.mfaEnabled && !otpauth && (
                  <div className="project-actions">
                    <button type="button" className="primary" onClick={enroll}>
                      {t("enrollMfa")}
                    </button>
                  </div>
                )}
                {otpauth && (
                  <div className="mfa-enroll">
                    <QRCodeSVG value={otpauth} size={180} includeMargin />
                    <p>
                      {t("secret")} <code>{secret}</code>
                    </p>
                    <form className="stack-form" onSubmit={enable}>
                      <label>
                        {t("confirmTotp")}
                        <input value={code} onChange={(e) => setCode(e.target.value)} required inputMode="numeric" />
                      </label>
                      <button type="submit" className="primary">
                        {t("enableMfa")}
                      </button>
                    </form>
                  </div>
                )}
                {user.mfaEnabled && (
                  <form className="stack-form" onSubmit={disable}>
                    <label>
                      {t("disableCode")}
                      <input value={code} onChange={(e) => setCode(e.target.value)} required inputMode="numeric" />
                    </label>
                    <button type="submit" className="danger">
                      {t("disableMfa")}
                    </button>
                  </form>
                )}
              </section>
              <section className="panel">
                <h2>{t("passwordTitle")}</h2>
                <form className="stack-form" onSubmit={savePassword}>
                  <label>
                    {t("currentPassword")}
                    <input type="password" value={currentPw} onChange={(e) => setCurrentPw(e.target.value)} required autoComplete="current-password" />
                  </label>
                  <label>
                    {t("newPassword")}
                    <input type="password" value={newPw} onChange={(e) => setNewPw(e.target.value)} required minLength={8} autoComplete="new-password" />
                  </label>
                  <button type="submit" className="primary">
                    {t("savePassword")}
                  </button>
                </form>
              </section>
              <section className="panel">
                <h2>{t("devicesTitle")}</h2>
                <p className="feed-lead">{t("devicesLead")}</p>
                {(user.trustedDevices || []).length === 0 ? (
                  <EmptyState title={t("noDevices")} text={t("noDevicesText")} />
                ) : (
                  <table className="members">
                    <thead>
                      <tr>
                        <th>{t("device")}</th>
                        <th>{t("lastSeen")}</th>
                        <th></th>
                      </tr>
                    </thead>
                    <tbody>
                      {(user.trustedDevices || []).map((d) => {
                        const current = d.id === getDeviceId();
                        const label = deviceShortLabel(d.name);
                        return (
                          <tr key={d.id}>
                            <td>
                              <div className="device-cell">
                                <span className="device-ico" title={d.id}>
                                  {label.slice(0, 2).toUpperCase()}
                                </span>
                                <span title={d.id}>
                                  {label} {current ? <span className="badge accent">{t("thisDevice")}</span> : null}
                                </span>
                              </div>
                            </td>
                            <td>{formatWhen(d.lastSeenAt || d.createdAt, locale)}</td>
                            <td>
                              <button type="button" className="linkish" onClick={() => void revokeDevice(d.id)}>
                                {t("revoke")}
                              </button>
                            </td>
                          </tr>
                        );
                      })}
                    </tbody>
                  </table>
                )}
                <div className="project-actions">
                  <button type="button" onClick={simulateNewDevice}>
                    {t("simulateDevice")}
                  </button>
                </div>
                <p className="hint">{t("simulateHint")}</p>
              </section>
            </>
          )}

          {show("notifications") && (
            <section className="panel">
              <h2>{t("notifTitle")}</h2>
              <p className="feed-lead">{t("notifLead")}</p>
              <form onSubmit={saveNotif}>
                <label className="pref-row">
                  <input type="checkbox" checked={notifyEmail} onChange={(e) => setNotifyEmail(e.target.checked)} />
                  <span>{t("notifEmail")}</span>
                </label>
                <label className="pref-row">
                  <input type="checkbox" checked={notifyInv} onChange={(e) => setNotifyInv(e.target.checked)} />
                  <span>{t("notifInv")}</span>
                </label>
                <label className="pref-row">
                  <input type="checkbox" checked={notifyMkt} onChange={(e) => setNotifyMkt(e.target.checked)} />
                  <span>{t("notifMkt")}</span>
                </label>
                <button type="submit" className="primary">
                  {t("saveNotif")}
                </button>
              </form>
            </section>
          )}

          {show("privacy") && (
            <section className="panel">
              <h2>{t("privacyTitle")}</h2>
              <p className="feed-lead">{t("privacyLead")}</p>
              <p>{t("noPublicProfile")}</p>
              <h3>{t("consentTitle")}</h3>
              <p className="feed-lead legal">
                {consent?.accepted
                  ? t("consentAccepted", { when: consent.createdAt.slice(0, 19).replace("T", " ") })
                  : t("consentMissing")}
              </p>
              <form onSubmit={saveNotif}>
                <label className="pref-row">
                  <input type="checkbox" checked={shareUsage} onChange={(e) => setShareUsage(e.target.checked)} />
                  <span>{t("shareUsage")}</span>
                </label>
                <button type="submit" className="primary">
                  {t("saveNotif")}
                </button>
              </form>
              <p>
                <Link to="/trust">{t("trustPage")}</Link>
              </p>
            </section>
          )}

          {show("billing") && (
            <section className="panel">
              <h2>{t("planBilling")}</h2>
              <p className="feed-lead">{t("billingLead")}</p>
              <p className="feed-lead">
                {ent
                  ? t("entitlementsLine", {
                      tier: ent.tier,
                      used: ent.investigationsUsed,
                      max: ent.maxInvestigationsPerMonth || "∞",
                    })
                  : t("loadingEnt")}
              </p>
              {ent && ent.tier.toLowerCase() === "free" && (
                <UpgradeBanner title={t("upgradeTitle")} text={t("upgradeText")} demo={ent.demoUpgrade} />
              )}
              {ent && ent.tier.toLowerCase() !== "free" && <p className="ok">{t("proActive")}</p>}
              <p className="hint">{t("noCards")}</p>
              <p className="hint">{t("autoRenew")}</p>
              <p className="hint">{t("invoicesHint")}</p>
            </section>
          )}

          {show("org") && (
            <section className="panel">
              <h2>{t("orgTitle")}</h2>
              <p className="hint">{t("communitySkip")}</p>
              {!org && (
                <>
                  <p className="feed-lead">{t("noOrg")}</p>
                  <form
                    className="stack-form"
                    onSubmit={async (e) => {
                      e.preventDefault();
                      const name = orgName.trim();
                      if (!name) return;
                      setError("");
                      try {
                        const data = await gql<{ createOrganization: Organization }>(
                          `mutation ($name: String!) { createOrganization(name: $name) { id name members { email role } } }`,
                          { name }
                        );
                        setOrg(data.createOrganization);
                        setOk(t("orgCreated"));
                        setOrgName("");
                        await load();
                      } catch (err) {
                        setError(err instanceof Error ? err.message : "Failed");
                      }
                    }}
                  >
                    <label>
                      {t("orgName")}
                      <input value={orgName} onChange={(e) => setOrgName(e.target.value)} placeholder={t("orgName")} />
                    </label>
                    <button type="submit" className="primary">
                      {t("createOrg")}
                    </button>
                  </form>
                </>
              )}
              {org && (
                <>
                  <div className="project-header">
                    <h3 className="project-title">{org.name}</h3>
                    {user.orgRole ? <span className="badge">{user.orgRole}</span> : null}
                  </div>
                  <table className="members">
                    <thead>
                      <tr>
                        <th>{t("colEmail")}</th>
                        <th>{t("colRole")}</th>
                        <th>{t("colJoined")}</th>
                        <th></th>
                      </tr>
                    </thead>
                    <tbody>
                      {org.members.map((m) => (
                        <tr key={m.email}>
                          <td>{m.email}</td>
                          <td>
                            {user.orgRole === "admin" ? (
                              <select
                                value={m.role}
                                onChange={(e) => {
                                  const role = e.target.value;
                                  void gql<{ updateOrgMemberRole: Organization }>(
                                    `mutation ($email: String!, $role: String!) {
                                      updateOrgMemberRole(email: $email, role: $role) { id name members { email role userId } }
                                    }`,
                                    { email: m.email, role }
                                  )
                                    .then((d) => setOrg(d.updateOrgMemberRole))
                                    .catch((err) => setError(err instanceof Error ? err.message : "Failed"));
                                }}
                              >
                                <option value="admin">admin</option>
                                <option value="engineer">engineer</option>
                                <option value="viewer">viewer</option>
                              </select>
                            ) : (
                              m.role
                            )}
                          </td>
                          <td>{m.userId ? t("joinedYes") : t("invited")}</td>
                          <td>
                            {user.orgRole === "admin" && m.email !== user.email && (
                              <button
                                type="button"
                                className="linkish"
                                onClick={() => {
                                  void gql<{ removeOrgMember: Organization }>(
                                    `mutation ($email: String!) {
                                      removeOrgMember(email: $email) { id name members { email role userId } }
                                    }`,
                                    { email: m.email }
                                  )
                                    .then((d) => setOrg(d.removeOrgMember))
                                    .catch((err) => setError(err instanceof Error ? err.message : "Failed"));
                                }}
                              >
                                {t("remove")}
                              </button>
                            )}
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                  {user.orgRole === "admin" && (
                    <form
                      className="inline-form"
                      style={{ marginTop: "0.8rem" }}
                      onSubmit={async (e) => {
                        e.preventDefault();
                        const email = inviteEmail.trim();
                        if (!email) return;
                        setError("");
                        try {
                          const data = await gql<{ inviteOrgMember: Organization }>(
                            `mutation ($email: String!, $role: String!) {
                              inviteOrgMember(email: $email, role: $role) { id name members { email role userId } }
                            }`,
                            { email, role: inviteRole }
                          );
                          setOrg(data.inviteOrgMember);
                          setInviteEmail("");
                          setOk(t("invitedOk", { email, role: inviteRole }));
                        } catch (err) {
                          setError(err instanceof Error ? err.message : "Failed");
                        }
                      }}
                    >
                      <input
                        value={inviteEmail}
                        onChange={(e) => setInviteEmail(e.target.value)}
                        type="email"
                        placeholder="member@company.com"
                        required
                      />
                      <select value={inviteRole} onChange={(e) => setInviteRole(e.target.value)}>
                        <option value="admin">admin</option>
                        <option value="engineer">engineer</option>
                        <option value="viewer">viewer</option>
                      </select>
                      <button type="submit" className="primary">
                        {t("invite")}
                      </button>
                    </form>
                  )}
                </>
              )}
            </section>
          )}

          {show("prefs") && (
            <section className="panel">
              <h2>{t("prefsTitle")}</h2>
              <p className="feed-lead">{t("prefsLead")}</p>
              <p>{t("langLabel")}</p>
              <LanguageToggle />
              <p style={{ marginTop: 16 }}>{t("themeLabel")}</p>
              <div className="toggle-row mode-toggle" role="group">
                <button type="button" className={theme === "dark" ? "active" : ""} onClick={() => setTheme("dark")}>
                  {t("themeDark")}
                </button>
                <button type="button" className={theme === "light" ? "active" : ""} onClick={() => setTheme("light")}>
                  {t("themeLight")}
                </button>
              </div>
              <p style={{ marginTop: 16 }}>{t("textSize")}</p>
              <div className="toggle-row mode-toggle" role="group">
                <button type="button" className={textSize === "normal" ? "active" : ""} onClick={() => setTextSize("normal")}>
                  {t("textNormal")}
                </button>
                <button type="button" className={textSize === "large" ? "active" : ""} onClick={() => setTextSize("large")}>
                  {t("textLarge")}
                </button>
              </div>
              <p className="hint">{t("regionHint")}</p>
            </section>
          )}

          {show("support") && (
            <section className="panel">
              <h2>{t("supportTitle")}</h2>
              <ul className="faq-list">
                <li>
                  <strong>{t("faq1q")}</strong> {t("faq1a")}
                </li>
                <li>
                  <strong>{t("faq2q")}</strong> {t("faq2a")}
                </li>
                <li>
                  <strong>{t("faq3q")}</strong> {t("faq3a")}
                </li>
              </ul>
              <div className="project-actions">
                <a className="primary" href="mailto:hello@azula.local?subject=Azula%20feedback">
                  {t("sendFeedback")}
                </a>
                <Link to="/trust">{t("reportVuln")}</Link>
              </div>
              <h3>{t("aboutApp")}</h3>
              <p className="feed-lead">{t("aboutAppText")}</p>
              <p>
                <Link to="/trust">{t("trustPage")}</Link>
              </p>
            </section>
          )}

          {show("manage") && (
            <section className="panel">
              <h2>{t("manageTitle")}</h2>
              <p className="feed-lead">{t("yourDataLead")}</p>
              <div className="row-actions">
                <button type="button" onClick={exportData}>
                  {t("exportData")}
                </button>
                <button type="button" onClick={deactivate}>
                  {t("deactivate")}
                </button>
                <button type="button" className="danger" onClick={wipe}>
                  {t("deleteAccount")}
                </button>
              </div>
              <p className="hint">{t("logoutHint")}</p>
              <div className="project-actions">
                <button type="button" onClick={signOut}>
                  {t("signOut")}
                </button>
              </div>
            </section>
          )}
        </div>
      </div>
    </div>
  );
}

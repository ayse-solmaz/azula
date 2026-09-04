import { FormEvent, useEffect, useState } from "react";
import { QRCodeSVG } from "qrcode.react";
import { AuditLog, ConsentRecord, forgetLocalDevice, getDeviceId, gql, Organization, User } from "../api";

const ME_QUERY = `query {
  me { id email mfaEnabled tier orgId orgName orgRole trustedDevices { id name createdAt lastSeenAt } }
  myConsent { purpose accepted createdAt }
  myOrganization { id name members { email role userId } }
  auditLogs { id action resource createdAt }
}`;

export default function SecurityPage() {
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
  const [logs, setLogs] = useState<AuditLog[]>([]);
  const [consent, setConsent] = useState<ConsentRecord | null>(null);

  async function load() {
    const data = await gql<{
      me: User;
      myConsent: ConsentRecord | null;
      myOrganization: Organization | null;
      auditLogs: AuditLog[];
    }>(ME_QUERY);
    setUser(data.me);
    setConsent(data.myConsent);
    setOrg(data.myOrganization);
    setLogs(data.auditLogs || []);
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
        `mutation ($code: String!) { enableMfa(code: $code) { id email mfaEnabled trustedDevices { id name createdAt lastSeenAt } } }`,
        { code }
      );
      setUser(data.enableMfa);
      setOk("MFA is on. Sign-in will ask for a TOTP code.");
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
        `mutation ($code: String!) { disableMfa(code: $code) { id email mfaEnabled trustedDevices { id name createdAt lastSeenAt } } }`,
        { code }
      );
      setUser(data.disableMfa);
      setOk("MFA disabled.");
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
      setOk("Export downloaded.");
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed");
    }
  }

  async function revokeDevice(id: string) {
    setError("");
    setOk("");
    try {
      const data = await gql<{ revokeTrustedDevice: User }>(
        `mutation ($id: ID!) {
          revokeTrustedDevice(deviceId: $id) {
            id email mfaEnabled trustedDevices { id name createdAt lastSeenAt }
          }
        }`,
        { id }
      );
      setUser(data.revokeTrustedDevice);
      if (id === getDeviceId()) {
        forgetLocalDevice();
        window.location.href = "/login";
        return;
      }
      setOk("device revoked. that machine will need a new otp on next sign-in.");
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed");
    }
  }

  function simulateNewDevice() {
    forgetLocalDevice();
    window.location.href = "/login";
  }

  async function wipe() {
    if (!confirm("Permanently delete your account and all projects?")) return;
    setError("");
    try {
      await gql(`mutation { deleteAccount }`);
      localStorage.removeItem("azula_token");
      window.location.href = "/login";
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed");
    }
  }

  if (!user) return <p className="page">{error || "loading…"}</p>;

  return (
    <div className="page">
      <section className="panel">
        <div className="feed-head">
          <h2>security</h2>
          <p className="feed-lead">
            signed in as {user.email}. mfa uses totp (google authenticator, 1password, authy).
          </p>
        </div>
        <p>
          status: {user.mfaEnabled ? <span className="badge">mfa enabled</span> : <span className="badge">mfa off</span>}
          {" "}
          <span className="badge">{user.tier}</span>
        </p>
        {!user.mfaEnabled && !otpauth && (
          <div className="project-actions">
            <button type="button" onClick={enroll}>
              enroll authenticator
            </button>
          </div>
        )}
        {otpauth && (
          <div className="mfa-enroll">
            <QRCodeSVG value={otpauth} size={180} includeMargin />
            <p>
              secret: <code>{secret}</code>
            </p>
            <form className="stack-form" onSubmit={enable}>
              <label>
                confirm with a 6-digit code
                <input value={code} onChange={(e) => setCode(e.target.value)} required inputMode="numeric" />
              </label>
              <button type="submit">enable mfa</button>
            </form>
          </div>
        )}
        {user.mfaEnabled && (
          <form className="stack-form" onSubmit={disable}>
            <label>
              code to disable mfa
              <input value={code} onChange={(e) => setCode(e.target.value)} required inputMode="numeric" />
            </label>
            <button type="submit" className="danger">
              disable mfa
            </button>
          </form>
        )}
        {error && <p className="error">{error}</p>}
        {ok && <p className="ok">{ok}</p>}
      </section>

      <section className="panel">
        <h2>kvkk / gdpr consent</h2>
        <p className="feed-lead legal">
          processing of logs, configs, and prompts for investigation. {consent?.accepted
            ? `accepted ${consent.createdAt.slice(0, 19).replace("T", " ")}.`
            : "not recorded yet — accept the banner or register with the consent checkbox."}
        </p>
      </section>

      <section className="panel">
        <h2>trusted devices</h2>
        <p className="feed-lead">
          one account, many devices (browser, electron, phone). 5 concurrent investigations are a worker-pool limit, not a device cap.
          a new device needs a one-time email code; existing devices stay trusted.
        </p>
        {(user.trustedDevices || []).length === 0 ? (
          <div className="empty-state">
            <p className="empty-title">no devices stored</p>
            <p className="empty-text">sign in from this browser to register it as trusted.</p>
          </div>
        ) : (
          <table className="members">
            <thead>
              <tr>
                <th>device</th>
                <th>last seen</th>
                <th></th>
              </tr>
            </thead>
            <tbody>
              {(user.trustedDevices || []).map((d) => {
                const current = d.id === getDeviceId();
                return (
                  <tr key={d.id}>
                    <td className="legal">
                      {d.name} {current ? <span className="badge">this device</span> : null}
                    </td>
                    <td>{(d.lastSeenAt || d.createdAt).slice(0, 19).replace("T", " ")}</td>
                    <td>
                      <button
                        type="button"
                        className="linkish"
                        onClick={() => void revokeDevice(d.id)}
                      >
                        revoke
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
            simulate another device
          </button>
        </div>
        <p className="hint">
          simulate drops this browser&apos;s id and signs out. sign in again to get a device otp while other machines stay listed.
        </p>
      </section>

      <section className="panel">
        <h2>your data</h2>
        <p className="feed-lead">download a json copy of your account or permanently delete it and related projects.</p>
        <div className="row-actions">
          <button type="button" onClick={exportData}>
            export my data
          </button>
          <button type="button" className="danger" onClick={wipe}>
            delete account
          </button>
        </div>
      </section>

      <section className="panel">
        <h2>organization</h2>
        {!org && (
          <>
            <p className="feed-lead">no org yet. create one to share workspaces. roles: admin, engineer, viewer.</p>
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
                  setOk("organization created. members share workspaces; roles: admin, engineer, viewer.");
                  setOrgName("");
                  await load();
                } catch (err) {
                  setError(err instanceof Error ? err.message : "Failed");
                }
              }}
            >
              <label>
                organization name
                <input value={orgName} onChange={(e) => setOrgName(e.target.value)} placeholder="organization name" />
              </label>
              <button type="submit">create org</button>
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
                  <th>email</th>
                  <th>role</th>
                  <th>joined</th>
                </tr>
              </thead>
              <tbody>
                {org.members.map((m) => (
                  <tr key={m.email}>
                    <td>{m.email}</td>
                    <td>{m.role}</td>
                    <td>{m.userId ? "yes" : "invited"}</td>
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
                    setOk(`invited ${email} as ${inviteRole}. they join on next register/login.`);
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
                <button type="submit">invite</button>
              </form>
            )}
          </>
        )}
      </section>

      <section className="panel">
        <h2>activity log</h2>
        <p className="feed-lead">account actions recorded for this user.</p>
        {logs.length === 0 ? (
          <div className="empty-state">
            <p className="empty-title">no events yet</p>
            <p className="empty-text">sign-in, export, and org changes show up here.</p>
          </div>
        ) : (
          <table className="members">
            <thead>
              <tr>
                <th>when</th>
                <th>action</th>
                <th>resource</th>
              </tr>
            </thead>
            <tbody>
              {logs.map((log) => (
                <tr key={log.id}>
                  <td>{log.createdAt.slice(0, 19).replace("T", " ")}</td>
                  <td>{log.action}</td>
                  <td>{log.resource}</td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </section>
    </div>
  );
}

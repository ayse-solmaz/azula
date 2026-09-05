import { Link } from "react-router-dom";
import { hasSession } from "../api";
import { LanguageToggle, useI18n } from "../i18n";

export default function TrustPage() {
  const { t } = useI18n();
  const principles = [
    { title: t("trustP1t"), text: t("trustP1") },
    { title: t("trustP2t"), text: t("trustP2") },
    { title: t("trustP3t"), text: t("trustP3") },
    { title: t("trustP4t"), text: t("trustP4") },
    { title: t("trustP5t"), text: t("trustP5") },
    { title: t("trustP6t"), text: t("trustP6") },
  ];
  return (
    <div className="page">
      <section className="panel">
        <div className="row bar">
          <p className="eyebrow">azula</p>
          <LanguageToggle />
        </div>
        <h2>{t("trustTitle")}</h2>
        <p className="feed-lead">
          {t("trustLead")}{" "}
          <a href="https://github.com/gurkanfikretgunak/cursor-security" target="_blank" rel="noreferrer noopener">
            Agentic AI Security
          </a>
          .
        </p>
        <div className="project-actions">
          {hasSession() ? (
            <Link className="primary" to="/">
              {t("backHome")}
            </Link>
          ) : (
            <Link className="primary" to="/login">
              {t("signIn")}
            </Link>
          )}
        </div>
      </section>
      {principles.map((p) => (
        <section className="panel" key={p.title}>
          <h3 className="project-title">{p.title}</h3>
          <p className="feed-lead">{p.text}</p>
        </section>
      ))}
      <section className="panel">
        <h3 className="project-title">{t("reportVuln")}</h3>
        <p className="feed-lead">{t("reportVulnText")}</p>
      </section>
    </div>
  );
}

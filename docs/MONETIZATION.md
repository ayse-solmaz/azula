# Azula — Monetization Specification

**Model:** B2B SaaS with tiered plans  
**Principle:** Revenue comes from depth of analysis, team features, and enterprise compliance — not just seat count.

---

## 1. Pricing Tiers

### Free / Developer

**Target:** Individual ML engineers evaluating the product.

| Feature | Limit |
|---------|-------|
| Projects | 3 |
| Investigations / month | 10 |
| Models | Fast only |
| MCP connectors | Files (upload) |
| Investigation history | Last 10 |
| Council | Not available |
| Deep analysis | Not available |
| Analytics | Basic (count, avg time) |
| Support | Community / docs |

**Price:** $0

---

### Pro

**Target:** Active ML engineers and small teams.

| Feature | Limit |
|---------|-------|
| Projects | Unlimited |
| Investigations / month | 100 |
| Models | Fast + Deep + Council |
| MCP connectors | Files + Git |
| Investigation history | Unlimited |
| Council | Investigator + Challenger + Judge |
| Deep analysis | Yes |
| Analytics | Full dashboard |
| Model selection | Choose Fast/Deep/Judge models |
| Versioning | File version history |
| Support | Email |

**Price:** TBD (suggested: $29–49/month)

---

### Enterprise

**Target:** Organizations with compliance, security, and scale requirements.

| Feature | Limit |
|---------|-------|
| Projects | Unlimited |
| Investigations / month | Custom |
| Models | Custom / LoRA adapters |
| MCP connectors | Private (internal repos, databases) |
| Investigation history | Unlimited + export |
| Council | Custom model lineup (2–8 models) |
| Team workspace | Yes |
| Role-based access | Admin, Engineer, Viewer |
| MFA | Yes |
| Trusted devices | Yes |
| Audit logs | Full activity log |
| GDPR / KVKK | Data residency, deletion, export |
| API access | Scoped tokens |
| Concurrent users | Higher limits |
| Support | Dedicated / SLA |

**Price:** Custom (annual contract)

---

## 2. Feature Gating Matrix

| Feature | Free | Pro | Enterprise |
|---------|------|-----|------------|
| Fast classification | ✓ | ✓ | ✓ |
| Deep analysis | ✗ | ✓ | ✓ |
| AI Council | ✗ | ✓ | ✓ |
| Generate (synthetic data) | ✗ | ✓ | ✓ |
| Evaluate mode | ✗ | ✓ | ✓ |
| Git MCP | ✗ | ✓ | ✓ |
| Private MCP | ✗ | ✗ | ✓ |
| Custom models | ✗ | ✗ | ✓ |
| Team workspace | ✗ | ✗ | ✓ |
| RBAC | ✗ | ✗ | ✓ |
| MFA | ✗ | ✗ | ✓ |
| Audit logs | ✗ | ✗ | ✓ |
| API | ✗ | ✗ | ✓ |
| Model selection | ✗ | ✓ | ✓ |

---

## 3. Enforceable Config Keys

These keys are used in application config and tier checks:

```typescript
// config/tiers.ts
export const TIER_LIMITS = {
  free: {
    maxProjects: 3,
    maxInvestigationsPerMonth: 10,
    allowedModels: ['fast'],
    allowedMCPConnectors: ['files'],
    councilEnabled: false,
    deepAnalysisEnabled: false,
    historyLimit: 10,
    analyticsLevel: 'basic',
  },
  pro: {
    maxProjects: Infinity,
    maxInvestigationsPerMonth: 100,
    allowedModels: ['fast', 'deep', 'judge'],
    allowedMCPConnectors: ['files', 'git'],
    councilEnabled: true,
    deepAnalysisEnabled: true,
    historyLimit: Infinity,
    analyticsLevel: 'full',
    modelSelectionEnabled: true,
    versioningEnabled: true,
  },
  enterprise: {
    maxProjects: Infinity,
    maxInvestigationsPerMonth: Infinity, // or custom per contract
    allowedModels: ['fast', 'deep', 'judge', 'custom'],
    allowedMCPConnectors: ['files', 'git', 'database', 'private'],
    councilEnabled: true,
    deepAnalysisEnabled: true,
    historyLimit: Infinity,
    analyticsLevel: 'full',
    modelSelectionEnabled: true,
    versioningEnabled: true,
    teamWorkspaceEnabled: true,
    rbacEnabled: true,
    mfaEnabled: true,
    auditLogsEnabled: true,
    apiEnabled: true,
    customModelsEnabled: true,
    maxConcurrentUsers: 50, // configurable
  },
} as const;
```

---

## 4. Tier Check Points

| Action | Check |
|--------|-------|
| `createProject` | `projectCount < maxProjects` |
| `startInvestigation` | `monthlyCount < maxInvestigationsPerMonth` |
| `escalateToDeep` | `deepAnalysisEnabled` |
| `runCouncil` | `councilEnabled` |
| `connectGitMCP` | `'git' in allowedMCPConnectors` |
| `selectModel` | `modelSelectionEnabled` |
| `inviteTeamMember` | `teamWorkspaceEnabled` |

**UX on limit hit:** Show upgrade prompt with specific feature name, not generic "upgrade to Pro".

Example:
> You've used 10/10 investigations this month. Upgrade to Pro for 100 investigations and AI Council access.

---

## 5. Upgrade Triggers (In-App)

Show contextual upgrade prompts when user hits a gated feature:

| Trigger | Message |
|---------|---------|
| Deep analysis on Free | "Deep analysis requires Pro. Upgrade to investigate with evidence-backed root causes." |
| Council on Free | "AI Council debates hypotheses across two models. Available on Pro." |
| Project limit | "You've reached 3 projects. Upgrade to Pro for unlimited projects." |
| Investigation limit | "10/10 investigations used this month. Resets on [date]." |
| Git MCP | "Connect your Git repository. Available on Pro." |

---

## 6. MVP Implementation

Tier limits are enforced in `internal/billing`. All users start on `free`.

- Deep / Council / Generate / Evaluate / Git MCP require Pro
- Monthly investigation cap (`FREE_TIER_MAX_INVESTIGATIONS`, default 10)
- Stripe Checkout + webhook when `STRIPE_SECRET_KEY` and `STRIPE_PRICE_ID` are set
- Without Stripe keys, `BILLING_DEMO` (default true) exposes `activateProDemo` for jury / local use
- UI shows feature-specific upgrade prompts, not a generic lock

---

## 7. Enterprise Sales Motion

Enterprise features are sold through:

1. **Security questionnaire** — MFA, audit logs, GDPR
2. **Custom model requirements** — LoRA, private inference
3. **Team scale** — RBAC, concurrent users, API
4. **Private MCP** — internal repos, databases, custom connectors

Document these in sales collateral, not in self-serve UI.

---

## 8. Revenue Metrics to Track

| Metric | Description |
|--------|-------------|
| Free → Pro conversion rate | % of free users upgrading within 30 days |
| Investigation limit hit rate | % of free users hitting 10/month cap |
| Council feature interest | Clicks on gated Council CTA |
| ARPU | Average revenue per paying user |
| Enterprise pipeline | Qualified leads requesting enterprise features |

---

## 9. Related Documents

- [PRD.md](PRD.md) — Pillar 2: Monetization
- [MVP.md](MVP.md) — Free tier limits for MVP
- [ANALYTICS.md](ANALYTICS.md) — conversion metrics

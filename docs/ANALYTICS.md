# Azula — Analytics Specification

**Principle:** Measure problem resolution and product value — not vanity metrics like login count.

---

## 1. Dashboard Overview

The analytics dashboard answers: *"What kinds of problems is my team facing, and how well is Azula solving them?"*

**Not shown:**
- Login count
- Page views
- Session duration (unless tied to investigation)

---

## 2. Primary Metrics

### Incident Analytics Panel

```
INCIDENT ANALYTICS
──────────────────────────────────────
Total Investigations          248
Resolved by AI                 71%
Avg Investigation Time       38 sec
Avg Human Resolution*        42 min
──────────────────────────────────────
* Estimated baseline; user-configurable
```

| Metric | Definition | Calculation |
|--------|------------|-------------|
| Total Investigations | Completed investigations in workspace | `count(status = 'completed')` |
| Resolved by AI | Investigations where user marked fix as applied or did not reopen | `count(resolved = true) / total` |
| Avg Investigation Time | Time from `startInvestigation` to `completed` | `avg(completedAt - startedAt)` |
| Avg Human Resolution | User-provided baseline for manual debugging time | Configurable per workspace |

---

## 3. Top Root Causes

```
TOP ROOT CAUSES
──────────────────────────────────────
1. Schema mismatch          32%
2. Data drift               21%
3. Memory / GPU             17%
4. Dependency failure       12%
5. Configuration error       9%
6. Other                     9%
```

| Field | Source |
|-------|--------|
| Root cause category | `investigation.deepResult.rootCause` or `councilResult.finalJudgment.mostLikelyCause` |
| Classification | Map free-text to predefined categories via keyword rules or LLM tagger |
| Percentage | `count(category) / total completed investigations` |

**Predefined categories:**
- `schema_mismatch`
- `data_drift`
- `data_quality`
- `memory_gpu`
- `dependency_failure`
- `configuration_error`
- `data_leakage`
- `other`

---

## 4. Model Performance Comparison

```
FAST MODEL vs DEEP MODEL
──────────────────────────────────────
                FAST          DEEP
Avg response    3.2 sec      14.8 sec
Accuracy        84%          93%
Usage           248          186
──────────────────────────────────────
```

| Metric | Fast Model | Deep Model |
|--------|------------|------------|
| Avg response time | `avg(fastResult.completedAt - startedAt)` | `avg(deepResult.completedAt - fastResult.completedAt)` |
| Accuracy | User feedback or auto-validation rate | User feedback or evidence coverage rate |
| Usage count | Investigations that ran fast classify | Investigations that escalated to deep |

**Accuracy definition (MVP):** % of investigations where user did not reopen or dispute the root cause within 7 days.

---

## 5. AI Council Metrics

```
AI COUNCIL
──────────────────────────────────────
Council runs                  142
Agreement rate                78%
Model A (Investigator) wins   42%
Model B (Challenger) wins     38%
Council overturn rate         20%
Avg final confidence          91%
──────────────────────────────────────
```

| Metric | Definition |
|--------|------------|
| Council runs | Investigations that completed council stage |
| Agreement rate | % of council runs with ≥1 agreement and no disagreements on root cause |
| Model A win rate | % where final judgment matched Investigator hypothesis |
| Model B win rate | % where final judgment matched Challenger hypothesis |
| Council overturn rate | % where Judge chose a cause neither model proposed |
| Avg final confidence | `avg(councilResult.finalJudgment.confidence)` |

---

## 6. Generate & Evaluate Metrics (MVP+1)

```
DATA & EVALUATION
──────────────────────────────────────
Synthetic datasets generated     34
Evaluations run                  28
Fix adoption rate                64%
Avg metric improvement           +4.2%
──────────────────────────────────────
```

Deferred to MVP+1. Document now for dashboard layout planning.

---

## 7. Onboarding Funnel Metrics

| Event | Description |
|-------|-------------|
| `onboarding_started` | User sees welcome screen |
| `workspace_created` | Step 1 complete |
| `project_connected` | Step 2 complete |
| `first_investigation_started` | Step 3 begins |
| `first_investigation_completed` | Root cause shown |
| `onboarding_completed` | User views full report or uploads own project |

**Funnel target:** `started` → `completed` > 70%

---

## 8. Aggregation Queries (MongoDB)

### Total investigations

```javascript
db.investigations.countDocuments({
  projectId: { $in: projectIds },
  status: 'completed'
})
```

### Avg investigation time

```javascript
db.investigations.aggregate([
  { $match: { status: 'completed', projectId: { $in: projectIds } } },
  { $project: { duration: { $subtract: ['$completedAt', '$startedAt'] } } },
  { $group: { _id: null, avgMs: { $avg: '$duration' } } }
])
```

### Top root causes

```javascript
db.investigations.aggregate([
  { $match: { status: 'completed' } },
  { $group: { _id: '$incidentType', count: { $sum: 1 } } },
  { $sort: { count: -1 } },
  { $limit: 5 }
])
```

### Council agreement rate

```javascript
db.investigations.aggregate([
  { $match: { 'councilResult': { $exists: true } } },
  { $project: {
      hasAgreement: { $gt: [{ $size: '$councilResult.agreements' }, 0] },
      hasDisagreement: { $gt: [{ $size: '$councilResult.disagreements' }, 0] }
  }},
  { $group: {
      _id: null,
      total: { $sum: 1 },
      agreed: { $sum: { $cond: ['$hasAgreement', 1, 0] } }
  }}
])
```

---

## 9. Dashboard Wireframe (ASCII)

```
┌─────────────────────────────────────────────────────────────┐
│  AZULA ANALYTICS                          [Last 30 days ▼] │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐    │
│  │   248    │ │   71%    │ │  38 sec  │ │  42 min  │    │
│  │Investig. │ │Resolved  │ │ Avg Time │ │ Human    │    │
│  └──────────┘ └──────────┘ └──────────┘ └──────────┘    │
│                                                             │
│  TOP ROOT CAUSES                    MODEL COMPARISON        │
│  ┌─────────────────────┐           ┌─────────────────────┐ │
│  │ Schema mismatch 32% │           │ Fast  │ Deep        │ │
│  │ Data drift      21% │           │ 3.2s  │ 14.8s       │ │
│  │ Memory/GPU      17% │           │ 84%   │ 93%         │ │
│  │ Dependency      12% │           └─────────────────────┘ │
│  │ Config error     9% │                                   │
│  └─────────────────────┘           AI COUNCIL              │
│                                     ┌─────────────────────┐ │
│  RECENT INVESTIGATIONS              │ Agreement:    78%   │ │
│  ┌─────────────────────┐           │ Avg conf:       91%  │ │
│  │ schema_drift  2h ago │           │ Overturn:       20%  │ │
│  │ oom_error     1d ago │           └─────────────────────┘ │
│  │ config_err    3d ago │                                   │
│  └─────────────────────┘                                   │
└─────────────────────────────────────────────────────────────┘
```

---

## 10. Tier-Based Analytics Access

| Feature | Free | Pro | Enterprise |
|---------|------|-----|------------|
| Total investigations | ✓ | ✓ | ✓ |
| Avg investigation time | ✓ | ✓ | ✓ |
| Top root causes | Top 3 | All | All + export |
| Model comparison | ✗ | ✓ | ✓ |
| Council metrics | ✗ | ✓ | ✓ |
| Onboarding funnel | ✗ | ✗ | ✓ |
| Custom date range | ✗ | ✓ | ✓ |
| Export (CSV/PDF) | ✗ | ✗ | ✓ |

---

## 11. Event Tracking Schema

```typescript
interface AnalyticsEvent {
  event: string;
  userId: string;
  workspaceId: string;
  projectId?: string;
  investigationId?: string;
  metadata?: Record<string, unknown>;
  timestamp: Date;
}
```

**Core events:**
- `investigation_started`
- `investigation_completed`
- `investigation_failed`
- `council_completed`
- `fix_marked_applied`
- `investigation_reopened`
- `onboarding_step_completed`

---

## 12. Related Documents

- [PRD.md](PRD.md) — Pillar 3: Analytics
- [MVP.md](MVP.md) — US-8 acceptance criteria
- [ONBOARDING.md](ONBOARDING.md) — funnel events
- [MONETIZATION.md](MONETIZATION.md) — tier-gated analytics

# Security

Azula follows [agentic AI security](https://github.com/gurkanfikretgunak/cursor-security) controls: least agency for MCP tools, identity before action, and policy in code rather than in prompts.

## Reporting a vulnerability

Email the maintainers or open a **private** report. Do not attach working exploits to public issues.

Include: affected version or commit, impact, and steps that stay high-level.

## Runtime kill switch

Set `AZULA_KILL_SWITCH=true` and restart the API to refuse new investigations, generate, and evaluate runs. In-flight runs can be stopped with `cancelInvestigation`.

## Production checklist

- `AZULA_ENV=production`
- Strong `JWT_SECRET` (the default `change-me-in-production` is rejected)
- Leave `AZULA_GRAPHQL_PLAYGROUND` unset (introspection and playground stay off)
- Serve web and API on one origin (see `deploy/nginx.conf`) so HttpOnly session cookies work

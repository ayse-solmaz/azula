# Free public URL on Fly.io

Stable `https://….fly.dev` on the free tier (no rotating tunnels).

## Needs (all free)
1. Fly.io account — `fly auth login`
2. MongoDB Atlas M0 — connection string
3. `OPENAI_API_KEY` (Ollama does not run on Fly free VMs; keep Ollama local)

## Deploy
```bash
fly auth login
fly apps create azula-demo
# edit fly.toml `app =` if the name is taken
fly secrets set MONGODB_URI="mongodb+srv://USER:PASS@cluster/azula"
fly secrets set OPENAI_API_KEY="sk-..."
fly deploy
fly open
```

Health: `https://<app>.fly.dev/health` → `{"status":"ok"}`.

The root Dockerfile ships the Go API + `samples/`. For a browser UI, host `web/dist` on Cloudflare Pages (free) pointed at this API, or add static serving later.

Local jury path unchanged: API `:8080` + Vite `:3001` + Ollama.

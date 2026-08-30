# PulseLog dashboard

Next.js App Router UI for the authenticated Query API. The browser never talks to query-api directly and never sees the JWT.

## Stack

- Next.js 15 (App Router) + React 19 + TypeScript (strict)
- Tailwind CSS
- Recharts
- Vitest + Testing Library

## Local start

```powershell
# from repo root, with query-api already on :8082
cd apps/dashboard
copy .env.example .env.local
npm install
npm run dev
```

Open http://127.0.0.1:3000/login

## Environment

| Variable | Where | Purpose |
| --- | --- | --- |
| `QUERY_API_URL` | server only | Base URL of query-api (default `http://127.0.0.1:8082`) |

Do not put `JWT_SECRET` or API keys in `NEXT_PUBLIC_*` variables.

## Tests

```powershell
npm test
```

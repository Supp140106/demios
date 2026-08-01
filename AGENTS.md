# Demios — AGENTS.md

## Dev commands
- `wails dev` — live dev (hot-reloads frontend, Wails backend rebuilds on Go changes)
- `wails build` — production build
- Go: `go build ./...`, `go test ./...`, `go mod tidy`
- Frontend: `npm run dev`, `npm run build`, `npm run lint`, `npm run format`, `npm run typecheck`

## Architecture
- **Entrypoints:** `main.go` (Wails bootstrap), `app.go` (App struct bound to Wails), `frontend/src/main.tsx` (React root)
- **Server:** HTTP server starts on `127.0.0.1:0` (random port) at startup; frontend fetches port via Wails `GetServerPort()` binding
- **API:** `POST /api/chat/stream` — SSE streaming endpoint; events: `think`, `tool-call`, `tool-result`, `token`, `error`, `done`
- **Agent protocol:** Native OpenAI function calling via OpenRouter API. Tools defined as JSON schemas, model returns structured `tool_calls`. Max 15 iterations per turn.

## Backend
- **LLM:** OpenRouter (`nvidia/nemotron-3-super-120b-a12b:free`) via direct HTTP to `https://openrouter.ai/api/v1/chat/completions`. API key from `OPENROUTER_API_KEY` env var. Uses native OpenAI function calling format.
- **Tools:** All execute in PowerShell syntax. Bash tool timeout: default 120s, max 600s. Workspace-relative paths resolved via `WithWorkspace` context.
- **Packages:** `core/` (agent loop + client wrapper), `llm/` (OpenRouter HTTP client, types), `internal/server/` (SSE handler), `tools/` (each tool is a standalone file)
- **Tool schemas:** Each tool has a `Schema` field (`jsonschema.Schema`). `tools.AllToolDefs()` converts them to OpenAI function definitions sent in the API `tools` parameter. Tool IDs: `Read`, `Write`, `Edit`, `Grep`, `Glob`, `Bash`.
- **Streaming:** Text deltas arrive as `StreamEvent{Type: "text"}`. Tool calls arrive as `StreamEvent{Type: "tool_call"}` after full accumulation. Text is streamed to frontend in real-time; tool calls are parsed from structured API response (no XML parsing).

## Frontend
- **Framework:** React 19 + TypeScript ~6 + Vite 8 + Tailwind CSS v4
- **UI library:** shadcn/ui with `@base-ui/react` (NOT Radix). Components in `@/components/ui/`.
- **Alias:** `@/` maps to `src/` (configured in `vite.config.ts` + `tsconfig.json`)
- **Code style:** ESLint (flat config) + Prettier (with `prettier-plugin-tailwindcss`)
- **State:** Agent chat state in `useChatState` hook (SSE reader), workspace persistence in `useWorkspace` hook (localStorage)
- **shadcn config:** `components.json` — style `base-nova`, icon library `lucide`
- **prompt-kit components:** Tool display, Message, Reasoning, Chain of Thought — compatible with the structured events from the backend

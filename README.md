# Demios

An autonomous AI coding agent with a full desktop UI, powered by Wails (Go) + React. Demios pairs a tool-calling LLM agent loop with a real terminal, browser automation, project introspection, and a live visual topology canvas of everything it does.

## Features

- **Tool-calling agent** — native OpenAI function-calling loop. Reads, writes, greps, globs, edits files via unified diffs, and runs shell commands across a workspace (PowerShell on Windows, bash on Linux/macOS).
- **Real terminal** — interactive PTY sessions in the UI that the agent can operate.
- **Browser agent** — a sub-agent that drives a real Chromium window to test websites: navigate, click, fill forms, screenshot, extract content, and report back.
- **Dev-server orchestrator** — starts background dev servers, auto-detects their bound port, streams their output, and hands the authoritative URL to the browser agent.
- **Live topology canvas** — a graph (via React Flow) visualizing the agent, tools, servers, and sub-agents as they execute.
- **Human-in-the-loop** — permission prompts (allow / confirm-write / confirm-all) and an `AskUser` tool with an in-app dialog.
- **Sessions & history** — SQLite-backed chat sessions per workspace with message persistence.
- **Bring-your-own model** — built-in presets for OpenRouter, NVIDIA NIM, Mistral, Groq, GitHub Models, Gemini, PoolSide, and DeepSeek, plus user-defined providers with custom base URLs and headers.
- **Sub-agent delegation** — the `Task` tool spawns isolated sub-agents with their own context.
- **Context management** — automatic summarization and pruning when conversation history gets large.
- **Skills** — discoverable skill files loaded on demand via `ListSkills` / `ReadSkill`.

## Tech stack

| Layer    | Technology |
|----------|------------|
| Desktop  | [Wails v2](https://wails.io) — Go backend + WebView2/WebKit frontend |
| Backend  | Go 1.25 — custom agent loop, OpenAI SDK, `modernc.org/sqlite` |
| Frontend | React 19 + TypeScript + Vite + Tailwind CSS v4 |
| UI       | shadcn/ui on `@base-ui/react`, `@xyflow/react`, xterm.js, shiki |
| LLM API  | OpenAI-compatible `/v1/chat/completions` (SSE), plus Gemini (`genai`) backend |
| Browser  | Playwright (Chromium) |

## Architecture

```
┌─────────────────────────────────────────────────────────────────────────────────────────┐
│                              WAILS v2 DESKTOP APP                                        │
│                                                                                         │
│  ┌──────────────┐    ┌──────────────┐                                                   │
│  │  main.go     │───▶│  app.go      │  App struct bound to Wails                        │
│  │  Bootstrap   │    │  Wails Bind  │  (workspace, sessions, terminals, providers)       │
│  └──────────────┘    └──────┬───────┘                                                   │
│                             │                                                           │
│           ┌─────────────────┼─────────────────────┐                                     │
│           │                 │                     │                                     │
│           ▼                 ▼                     ▼                                     │
│  ┌─────────────────┐ ┌─────────────┐  ┌─────────────────────┐                          │
│  │  Wails Bindings  │ │  HTTP Srv   │  │  SQLite DB          │                          │
│  │  (Go ↔ JS RPC)  │ │  127.0.0.1  │  │  sessions & msgs    │                          │
│  └────────┬────────┘ │  :0 (rand)  │  └─────────────────────┘                          │
│           │          └──────┬──────┘                                                    │
│           │                 │                                                           │
│           │    ┌────────────┴──────────────────────────────────────────┐                │
│           │    │  POST /api/chat/stream  (SSE)                        │                │
│           │    │  POST /api/permission/respond                        │                │
│           │    │  POST /api/human-input/respond                       │                │
│           │    └────────────┬──────────────────────────────────────────┘                │
│           │                 │                                                           │
│           │                 ▼                                                           │
│           │    ┌────────────────────────────────────────────────────────────────┐       │
│           │    │                     CORE / AGENT LOOP                         │       │
│           │    │                                                                │       │
│           │    │  ┌──────────┐  ┌──────────┐  ┌────────────────────────────┐   │       │
│           │    │  │ agent.go │  │client.go │  │ browser_agent.go           │   │       │
│           │    │  │          │  │ LLM HTTP │  │ Playwright Chromium        │   │       │
│           │    │  │ Dispatch │  │ SSE resp │  │ navigate / click / fill /  │   │       │
│           │    │  │ loop     │  │ stream   │  │ screenshot / extract       │   │       │
│           │    │  │ (15 iter)│  │ parser   │  │                            │   │       │
│           │    │  └────┬─────┘  └──────────┘  └────────────┬───────────────┘   │       │
│           │    │       │                                    │                   │       │
│           │    │       ▼                                    ▼                   │       │
│           │    │  ┌──────────────────────────────────────────────────────┐     │       │
│           │    │  │              TOOLS LAYER  (tools/)                   │     │       │
│           │    │  │                                                      │     │       │
│           │    │  │  ┌────────┐ ┌────────┐ ┌────────┐ ┌──────────────┐  │     │       │
│           │    │  │  │ Read   │ │ Write  │ │ Edit   │ │ Bash         │  │     │       │
│           │    │  │  │        │ │        │ │ (diff) │ │ (PTY exec)  │  │     │       │
│           │    │  │  └────────┘ └────────┘ └────────┘ └──────────────┘  │     │       │
│           │    │  │  ┌────────┐ ┌────────┐ ┌────────┐ ┌──────────────┐  │     │       │
│           │    │  │  │ Grep   │ │ Glob   │ │ Undo   │ │ ReadRelated  │  │     │       │
│           │    │  │  └────────┘ └────────┘ └────────┘ └──────────────┘  │     │       │
│           │    │  │  ┌────────────────┐ ┌────────────────────────────┐  │     │       │
│           │    │  │  │ ProjectStruct  │ │ Task (sub-agent spawn)     │  │     │       │
│           │    │  │  └────────────────┘ └────────────────────────────┘  │     │       │
│           │    │  │  ┌────────────────┐ ┌────────────────────────────┐  │     │       │
│           │    │  │  │ ListSkills     │ │ AskUser                    │  │     │       │
│           │    │  │  │ ReadSkill      │ │ (human-in-loop dialog)     │  │     │       │
│           │    │  │  └────────────────┘ └────────────────────────────┘  │     │       │
│           │    │  │  ┌──────────────────────────────────────────────┐  │     │       │
│           │    │  │  │ SERVER TOOLS                                 │  │     │       │
│           │    │  │  │ StartServer  StopServer  RestartServer       │  │     │       │
│           │    │  │  │ GetServerStatus  ListServers  TestWebsite   │  │     │       │
│           │    │  │  └──────────────────────┬───────────────────────┘  │     │       │
│           │    │  └─────────────────────────┼──────────────────────────┘     │       │
│           │    │                            │                              │       │
│           │    │                            ▼                              │       │
│           │    │              ┌──────────────────────────────┐            │       │
│           │    │              │  ServerManager               │            │       │
│           │    │              │  (port detection, exclusion) │            │       │
│           │    │              └──────────────────────────────┘            │       │
│           │    └────────────────────────────────────────────────────────────┘       │
│           │                                                                         │
│           │    ┌────────────────────────────────────────────────────────────┐       │
│           │    │                    LLM LAYER  (llm/)                       │       │
│           │    │                                                            │       │
│           │    │  ┌───────────┐  ┌───────────┐  ┌──────────┐              │       │
│           │    │  │ openrouter│  │ genai      │  │ presets  │              │       │
│           │    │  │ .go       │  │ (Gemini)   │  │ .go      │              │       │
│           │    │  └─────┬─────┘  └─────┬─────┘  └──────────┘              │       │
│           │    │        │              │                                    │       │
│           │    └────────┼──────────────┼────────────────────────────────────┘       │
│           │             │              │                                            │
│           │             ▼              ▼                                            │
│           │      ┌───────────┐  ┌───────────┐                                     │
│           │      │ OpenRouter│  │ Gemini    │                                     │
│           │      │ API       │  │ API       │                                     │
│           │      └───────────┘  └───────────┘                                     │
│           │                                                                        │
│           │                                                                        │
│  ┌────────┴──────────────────────────────────────────────────────────────────────┐ │
│  │                           FRONTEND (React 19 + TS + Vite)                     │ │
│  │                                                                               │ │
│  │  ┌─────────────────┐ ┌──────────────┐ ┌───────────────────────────────────┐  │ │
│  │  │ use-chat-state  │ │ use-model    │ │  use-permission                   │  │ │
│  │  │ SSE reader      │ │ provider cfg │ │  allow / deny / confirm-write     │  │ │
│  │  └────────┬────────┘ └──────────────┘ └───────────────────────────────────┘  │ │
│  │           │                                                                   │ │
│  │           ▼                                                                   │ │
│  │  ┌──────────────────────────────────────────────────────────────────────┐     │ │
│  │  │                     COMPONENTS                                       │     │ │
│  │  │                                                                      │     │ │
│  │  │  ┌────────────────┐ ┌──────────────────┐ ┌──────────────────────┐  │     │ │
│  │  │  │ agent-chat     │ │ agent-topology   │ │ canvas-terminal      │  │     │ │
│  │  │  │ (messages)     │ │ (React Flow)     │ │ (xterm.js)           │  │     │ │
│  │  │  └────────────────┘ └──────────────────┘ └──────────────────────┘  │     │ │
│  │  │  ┌────────────────┐ ┌──────────────────┐ ┌──────────────────────┐  │     │ │
│  │  │  │ canvas-website │ │ prompt-input     │ │ session-sidebar      │  │     │ │
│  │  │  │ (Playwright)   │ │ (actions bar)    │ │ (history)            │  │     │ │
│  │  │  └────────────────┘ └──────────────────┘ └──────────────────────┘  │     │ │
│  │  │  ┌────────────────┐ ┌──────────────────┐ ┌──────────────────────┐  │     │ │
│  │  │  │ diff-viewer    │ │ permission-dlg   │ │ human-input-dialog   │  │     │ │
│  │  │  │ (unified diff) │ │ (allow / deny)   │ │ (free text input)    │  │     │ │
│  │  │  └────────────────┘ └──────────────────┘ └──────────────────────┘  │     │ │
│  │  │  ┌────────────────┐ ┌──────────────────┐                           │     │ │
│  │  │  │ model-selector │ │ providers-settings│                           │     │ │
│  │  │  └────────────────┘ └──────────────────┘                           │     │ │
│  │  └──────────────────────────────────────────────────────────────────────┘     │ │
│  └───────────────────────────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────────────────────────────┘
```

### Data flow

```
User types prompt
        │
        ▼
┌───────────────┐  Wails binding   ┌──────────────┐  POST /api/chat/stream  ┌──────────────┐
│ React UI      │ ────────────────▶ │  app.go      │ ──────────────────────▶ │ agent.go     │
│ prompt-input  │                   │  (Go→JS RPC) │                         │  Loop        │
└───────────────┘                   └──────────────┘                         └──────┬───────┘
       │                                                                            │
       │  SSE stream                                                                │  tool_calls
       │◀───────────────────────────────────────────────────────────────────────────┤
       │                                                                            ▼
       │                                                                     ┌──────────────┐
       │                                                                     │  tools/      │
       │                                                                     │  Read, Write │
       │                                                                     │  Edit, Bash  │
       │                                                                     │  Grep, Glob  │
       │                                                                     └──────┬───────┘
       │                                                                            │
       │                                                                     ┌──────┴───────┐
       │                                                                     │ Execute tool │
       │                                                                     │ (on system)  │
       │                                                                     └──────┬───────┘
       │                                                                            │
       │  SSE events                                                                │ result
       │◀───────────────────────────────────────────────────────────────────────────┘
       │
       ▼
┌───────────────────────────────────────┐
│            SSE Event Types            │
├───────────────────────────────────────┤
│  think      — model reasoning chain  │
│  iteration  — loop step number       │
│  token      — streamed text delta    │
│  tool-call  — structured call args   │
│  tool-result — tool output           │
│  subagent-event — delegated task     │
│  permission-request — allow/deny     │
│  human-input-request — free text     │
│  error      — failure message        │
│  done       — turn complete          │
└───────────────────────────────────────┘
```

### Tool execution graph

```
                        ┌─────────────────┐
                        │   agent.go      │
                        │  Dispatch tool  │
                        └────────┬────────┘
                                 │
            ┌────────────────────┼────────────────────┐
            │                    │                    │
            ▼                    ▼                    ▼
     ┌─────────────┐    ┌─────────────┐    ┌─────────────┐
     │  FILE I/O   │    │  SHELL      │    │  NETWORK    │
     │             │    │             │    │             │
     │  Read       │    │  Bash       │    │  StartServer│
     │  Write      │    │  (PTY exec) │    │  StopServer │
     │  Edit       │    │             │    │  TestWebsite│
     │  Grep       │    │             │    │  (Playwright│
     │  Glob       │    │             │    │   Chromium) │
     │  Undo       │    │             │    │             │
     └─────────────┘    └─────────────┘    └──────┬──────┘
                                                   │
                                                  ╱╲
                                                 ╱  ╲
                                                ╱    ╲
                                               ▼      ▼
                                    ┌──────────┐  ┌──────────┐
                                    │ Dev Srv  │  │ Browser  │
                                    │ (user    │  │ Agent    │
                                    │  project)│  │ (Playwrt)│
                                    └──────────┘  └──────────┘
                                        │              │
                                        │   ┌──────────┘
                                        │   │
                                        ▼   ▼
                                   ┌─────────────┐
                                   │  Navigate   │
                                   │  Click      │
                                   │  Fill form  │
                                   │  Screenshot │
                                   │  Extract    │
                                   └─────────────┘
```

### Sub-agent delegation

```
┌─────────────────┐     Task tool      ┌─────────────────┐
│  Main Agent     │ ──────────────────▶ │  Sub-Agent      │
│  (agent.go)     │                     │  (isolated ctx) │
│                 │◀──────────────────  │                 │
│  Iteration loop │  tool-result        │  Own tool loop  │
│  (15 iter max)  │                     │  (own history)  │
└─────────────────┘                     └─────────────────┘
```

### SSE events

`POST /api/chat/stream` emits: `think`, `iteration`, `token`, `tool-call`, `tool-result`, `subagent-event`, `permission-request`, `human-input-request`, `error`, `done`.

### Built-in tools

`Read`, `Write`, `Edit` (diff patch), `Grep`, `Glob`, `Bash`, `Undo`, `ReadRelated`, `ProjectStructure`, `Task`, `AskUser`, `ListSkills`, `ReadSkill`, plus `StartServer` / `StopServer` / `GetServerStatus` / `RestartServer` / `ListServers` / `TestWebsite`.

## Prerequisites

- Go 1.25+
- Node.js + npm
- A [Wails](https://wails.io/docs/gettingstarted/installation) environment for your OS
- An LLM provider API key (e.g. `OPENROUTER_API_KEY`) in a `.env` file, or configured in-app under **Providers**

## Getting started

```bash
# 1. Install frontend dependencies
cd frontend && npm install && cd ..

# 2. Live development (hot-reload frontend + rebuild backend on Go changes)
wails dev

# 3. Production build
wails build
```

### Frontend-only development

```bash
cd frontend
npm run dev        # Vite on http://localhost:5173
npm run build      # typecheck + production build
npm run lint       # ESLint
npm run typecheck  # TypeScript only
npm run format     # Prettier
```

### Backend-only checks

```bash
go build ./...
go test ./...
go mod tidy
```

## Configuration

Provider API keys are read from the environment (per provider `EnvVarName`) or managed in-app under **Providers** (user-defined base URLs, auth headers, extra body fields). Set a workspace directory in the UI to let the agent operate on your code.

## Project layout reference

- `AGENTS.md` — development commands & architecture notes
- `cmd/` — CLI entrypoints
- `embed/`, `build/` — Wails build assets
- `Skills/`, `.agents/skills/` — skill definitions
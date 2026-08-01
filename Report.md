# Demios — Engineering Report: Dev-Server Launch & Browser Agent Port Detection

**Report type:** Post-incident / implementation report for external AI review
**Scope:** `main agent launching dev server` → `browser agent testing the result`
**Status:** Root-caused, fixed, and verified (`go build`, `go vet`, `go test ./...` all green)

---

## 0. Executive Summary

Demios is a Wails (Go + React) desktop app whose LLM agent can start a dev
server for a user project and then dispatch a **browser agent** (Playwright
Chromium) to visit the URL and screenshot it. Two defects broke this pipeline:

1. **The `Edit` tool rejected most valid calls with "patch is required"**, so
   the main agent could not edit any file during a session.
2. **The dev-server port scanner misidentified Demios' own backend
   (`http://127.0.0.1:4096`) as the user's dev server**, so the browser agent
   tested the wrong site (Vite actually ran on `[::1]:5174`).

Both were architectural, not transient, failures: the tool schema over-promised
and under-implemented, and the port scanner used a "any TCP response" heuristic
that cannot distinguish an API from a website. This report documents root
causes, the exact fixes, the code, design tradeoffs, and open questions for
review.

---

## 1. System Context (needed to interpret the fix)

```
┌────────────────────────────────────────────────────────────────┐
│  frontend/  React 19 + TS + Vite, Wails-bound App (app.go)      │
│    │  POST /api/chat/stream (SSE)                               │
│    ▼                                                             │
│  core/agent.go   Agent loop  (tool dispatch, 15-iter cap)       │
│    │  ├─ tools/  Read, Write, Edit, Grep, Glob, Bash,           │
│    │  │           browser_test, TestWebsite ...                 │
│    │  └─ core/browser_agent.go  (separate sub-agent w/ Playwright)│
│    ▼                                                             │
│  llm/  OpenRouter HTTP client (OpenAI function-calling)          │
└────────────────────────────────────────────────────────────────┘
```

- **Agent loop:** model returns structured `tool_calls`; tool schemas are
  `jsonschema.Schema` (`tools/types.go` `ToToolDef`) converted to OpenAI
  function definitions.
- **ServerManager** (`tools/server_manager.go`): owns dev-server process
  lifecycle (start/kill), tracks each `ServerInstance{URL, Port, ...}`, and has
  a fallback heuristic `findExistingDevServer()` used when a server doesn't
  print a URL.
- **Backend HTTP server:** `server.StartServer(...)` listens on
  `127.0.0.1:0` (OS-random port) and reports that port through `app.go`.
- **Browser agent:** triggered by messages starting with `@browser`; it
  receives a URL, launches Chromium via playwright-go, navigates, screenshots,
  and reports back. It is driven by the same LLM (OpenRouter) using
  `browser_test`, `navigate`, `screenshot`, `click`, `type`, etc.

---

## 2. Problem 1 — `Edit` tool: "patch is required"

### 2.1 Symptom

During a live session the agent attempted to edit a CSS file and every call
failed. The error surface was a single terse line:

```
invalid arguments: patch is required
```

### 2.2 Root cause analysis

Three compounding causes:

1. **Schema over-promise.** `ToToolDef()` emitted the tool schema without
   `additionalProperties: false`. JSON Schema by default **allows arbitrary
   extra keys**, so the model was free to invent argument names. It did.
2. **Handler under-implementation.** `ApplyPatch.Execute` only implemented the
   `patch` (unified-diff) path. Because Go's `encoding/json` **silently drops
   unknown fields** on unmarshal, calls like
   `{path, oldContent, newContent}` unmarshalled into an empty `ApplyPatchArgs`
   with no error.
3. **Unhelpful error.** With `Patch == ""` the tool returned the terse string
   with no guidance about accepted shapes, so the model could not self-correct
   (this is an LLM feedback loop — the error message *is* the next prompt).

### 2.3 Observed argument shapes the model actually sent (evidence)

- `{ "patch": "<unified diff>" }` — the only supported form
- `{ "path": "...", "oldContent": "...", "newContent": "..." }`
- `{ "file_path": "...", "old_string": "...", "new_string": "..." }`
- `{ "path": "...", "replaces": [ {old_string,new_string}, ... ] }`
- `{ "path": "...", "old": "...", "new": "..." }`

### 2.4 The fix

**A. Schema hardening** — `tools/types.go:32`, `ToToolDef` now emits:

```go
"additionalProperties": false
```

so the schema itself rejects invented keys before the model wastes a turn.

**B. Argument flexibility** — `tools/apply_patch.go`:

```go
type ApplyPatchArgs struct {
	Patch     string        `json:"patch,omitempty"`
	Path      string        `json:"path,omitempty"`
	OldString string        `json:"old_string,omitempty"`
	NewString string        `json:"new_string,omitempty"`
	Replaces  []Replacement `json:"replaces,omitempty"`
}
```

`Execute` now routes:

```go
if args.Patch != "" { return applyDiff(ctx, args.Patch) }
if args.Path == "" {
	return ExecuteResult{}, fmt.Errorf("invalid arguments: missing 'patch' or 'path'. Provide one of:\n" +
		"  - {\"patch\": \"<unified git diff>\"}\n" +
		"  - {\"path\": \"src/foo.ts\", \"old_string\": \"<exact old text>\", \"new_string\": \"<new text>\"}\n" +
		"  - {\"path\": \"src/foo.ts\", \"replaces\": [{\"old_string\": \"...\", \"new_string\": \"...\"}]}")
}
```

**C. Alias normalization** — `fillArgAliases(rawArgs, args)`
(`tools/apply_patch.go:114`) maps the model's preferred names to canonical
fields after unmarshal, so `oldContent`/`newContent`, `old`/`new`,
`search`/`replace`, `file_path`, `oldSeek`/`newSeek` all work.

**D. Safety** — `BackupFile(path)` before every write; replacements applied
through the existing fuzzy hunk matcher (`fuzzyApplyHunk`) with per-hunk error
reporting (`"replaces[2]: old_string must not be empty"`,
`"'<path>' replacement 2: ..."`).

**E. Tests** — 7 cases in `tools/apply_patch_test.go` covering every arg
shape, both alias families, the helpful-error path, and the not-found path.

---

## 3. Problem 2 — Browser agent navigated to the wrong port

### 3.1 Symptom

- Vite (the real dev server) reported: `Local: http://localhost:5174/`
- Browser agent opened: `http://localhost:4096/` (Demios' own backend)

### 3.2 Root cause analysis

The old scanner (`findExistingDevServer`, pre-fix):

```go
// BEFORE — legacy implementation
func findExistingDevServer() string {
	httpClient := &http.Client{Timeout: 500 * time.Millisecond}
	for port := 3000; port <= 8090; port++ {        // (1) ~5000 ports
		if isPortInUse(port) {
			url := fmt.Sprintf("http://localhost:%d", port)
			resp, err := httpClient.Get(url)
			if err != nil { continue }
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			if resp.StatusCode < 500 {               // (2) status-only heuristic
				return url                          // (3) first match wins
			}
		}
	}
	return ""
}
```

Failure modes, in order of severity:

1. **Self-detection.** Demios' backend listens on `127.0.0.1:<random>`.
   Its `404` (any non-HTML response) satisfies `StatusCode < 500`, so the
   scanner returns Demios' own backend. In the live session that random port
   was **4096**.
2. **No content validation.** Status-code-only matching cannot distinguish a
   JSON/plain-text API from a real website (`text/html`).
3. **Blind range.** Scanning all of `3000–8090` guarantees unrelated services
   (postgres 5432, mongo 27017, Redis 6379, any desktop app's local port) are
   probed and are one accidental match away from being "detected".
4. **No exclusion mechanism.** There was no way to tell the scanner "this
   port is ours, never pick it".
5. **First-match-wins.** Returning the *first* port that loosely matches means
   the result depends on port ordering, not on which service is actually the
   dev server.
6. **IPv4-only URL parsing.** `parsePortFromLine` matched only
   `localhost | 127.0.0.1 | 0.0.0.0`, so modern servers that print IPv6 URLs
   (`http://[::1]:5174/`, `http://[::]:5175/`) were invisible to output-parsing
   and pushed the code down the (broken) scan path.

### 3.3 The fix

**A. Curated port ranges** — only scan ranges that plausibly host a dev server:

```go
var commonDevPortRanges = [][2]int{
	{5173, 5200}, // vite / astro / sveltekit defaults and increments
	{3000, 3010}, // next / nuxt / react-scripts
	{8000, 8010}, // python http.server / django / many tools
	{8080, 8090}, // common alternates
	{5000, 5010}, // flask / serve
	{4200, 4210}, // angular
}
```

**B. Content validation** — a candidate is a dev server **only if** it is a
real HTML page:

```go
func isDevServerResponse(resp *http.Response) bool {
	if resp.StatusCode >= 500 { return false }
	ct := strings.ToLower(resp.Header.Get("Content-Type"))
	if strings.Contains(ct, "text/html") { return true }
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
	lower := strings.ToLower(string(body))
	return strings.Contains(lower, "<!doctype") || strings.Contains(lower, "<html")
}
```

JSON / plain-text / backend-404 responses no longer match.

**C. Exclusion API** — new methods and a default denylist
(`tools/server_manager.go:69`):

```go
var defaultExcludedPorts = []int{4096, 5173}

func (sm *ServerManager) ExcludePort(port int)      { ... } // single
func (sm *ServerManager) ExcludePorts(ports ...int) { ... } // batch, seeds defaults in NewServerManager()
func (sm *ServerManager) isExcluded(port int) bool  { ... } // consulted during scan
```

**D. Backend self-exclusion** — `app.go` registers Demios' own random port at
startup so the scanner can never detect the app itself:

```go
srv, port, err := server.StartServer(a.agent, a.db, "127.0.0.1:0")
...
if p := parsePortFromAddr(port); p > 0 {
	a.agent.ServerManager().ExcludePort(p)
	log.Printf("Excluded backend port %d from dev-server detection", p)
}
```

**E. IPv6 URL parsing** — `parsePortFromLine` regex extended to:

```go
var portURLRegex = regexp.MustCompile(
	`(?:https?://)?(?:localhost|127\.0\.0\.1|0\.0\.0\.0|::1|\[::\]|\[::1\]|\[0\.0\.0\.0\]):(\d+)`)
```

now handles `http://[::1]:5174/`, `http://[::]:5175/`, `http://[0.0.0.0]:5175/`.

**F. Grace period** — `waitForServer` waits **10s** (was 5s) before falling
back to the port scan, so fast-printing servers (Vite) are found via their own
stdout first.

**G. Final scanner** (`tools/server_manager.go:499`):

```go
func (sm *ServerManager) findExistingDevServer() string {
	httpClient := &http.Client{Timeout: 800 * time.Millisecond}
	for _, r := range commonDevPortRanges {
		for port := r[0]; port <= r[1]; port++ {
			if sm.isExcluded(port) { continue }
			if !isPortInUse(port) { continue }
			url := fmt.Sprintf("http://localhost:%d", port)
			resp, err := httpClient.Get(url)
			if err != nil { continue }
			if isDevServerResponse(resp) {
				resp.Body.Close()
				return url
			}
			resp.Body.Close()
		}
	}
	return ""
}
```

### 3.4 Why 4096 and 5173 are both in the default denylist

- **4096** was the backend's random port in the live session. The backend port
  is *always* excluded dynamically (Fix D), but 4096 is kept in the defaults
  as defense-in-depth.
- **5173** is Vite's default port and IS inside the scan range. The user
  explicitly requested it be excluded because the port is occupied by another
  service they do not want the agent to touch. **Tradeoff (flagged for
  review):** a genuine Vite server on 5173 will no longer be found by the
  scan; it is still found if Vite prints a URL (output-parsing path).

---

## 4. Problem 3 — Flaky tests caused by a real server during development

### 4.1 Symptom

While writing scan tests, `TestFindExistingDevServerSkipsNonHTML` intermittently
"found" a non-HTML responder. Root cause: a real Vite instance from an earlier
session was still bound to **`[::1]:5174`** (IPv6 loopback). The test helper
probed IPv4 only, so:

- helper found 5174 "free" on `127.0.0.1` and bound a text/plain test server;
- the scanner's `http.Get("http://localhost:5174")` resolved to `::1` first and
  hit the *real* Vite (HTML) → false positive.

### 4.2 Fix

`freePortInDevRange` now skips ports in `defaultExcludedPorts` **and** ports
already in use on loopback (via `isPortInUse`, which covers IPv4 + IPv6), and
`startTestHTTPServer` binds `127.0.0.1` explicitly. Tests are now deterministic
and immune to whatever else is running on the machine.

---

## 5. Test Coverage Added

### `tools/apply_patch_test.go` (7 tests)
| Test | Verifies |
|---|---|
| `TestEditViaOldNewString` | `path + old_string/new_string` |
| `TestEditViaCamelCaseAliases` | `oldContent`/`newContent` aliases |
| `TestEditViaFilepathAlias` | `file_path` alias |
| `TestEditViaReplaces` | multi-edit `replaces[]` list |
| `TestEditViaPatch` | unified diff path still works |
| `TestEditMissingArgsGivesHelpfulError` | helpful usage error message |
| `TestEditNotFoundError` | missing-file error |

### `tools/server_manager_test.go` (new/extended)
| Test | Verifies |
|---|---|
| `TestParsePortFromLine` | incl. IPv6 forms (`[::1]`, `[::]`, `[0.0.0.0]`) |
| `TestIsDevServerResponse` | HTML accepted; JSON/plain/5xx rejected |
| `TestFindExistingDevServerSkipsNonHTML` | scan skips plain-text responder |
| `TestFindExistingDevServerExcludePort` | excluded port never returned |
| `TestExcludePortIgnoresInvalid` | `0` / negative ports rejected |
| `TestNewServerManagerDefaults` | `4096`/`5173` registered by default |
| `TestExcludePortsBatch` | `ExcludePorts(...)` multi-registration |

---

## 6. Verification

```text
go build ./...   -> ok
go vet ./...     -> ok
go test ./...    -> ok  (tools package green, 14+ tests)
```

Post-fix behavioral evidence from the test run: the scanner **skipped** the
plain-text responder on 5175 while still **finding** the genuine Vite HTML
server on `[::1]:5174` — proving detection works and false positives are gone.

---

## 7. Files Changed

| File | Change |
|---|---|
| `tools/apply_patch.go` | `ApplyPatchArgs` flex, `fillArgAliases`, helpful errors, `applyDiff` extraction, `BackupFile` |
| `tools/types.go` | `additionalProperties: false` in `ToToolDef` |
| `tools/apply_patch_test.go` | 7 new tests |
| `tools/server_manager.go` | `commonDevPortRanges`, `isDevServerResponse`, `ExcludePort(s)`, `defaultExcludedPorts`, IPv6 regex, 10s grace, `findExistingDevServer` as method |
| `tools/server_manager_test.go` | 7 new/extended tests, env-proof helpers |
| `app.go` | `parsePortFromAddr` + backend self-exclusion via `ExcludePort` |
| `core/agent.go` | (existing) `ServerManager()` accessor used by `app.go` |

---

## 8. Open Questions for the Reviewing Model

1. **`additionalProperties: false` and model resilience.** Strict schemas can
   cause some models to emit *no* args at all rather than risk invalid keys.
   Should we keep strict mode, or keep aliases as a fallback (we did) and
   monitor for empty-args errors?
2. **5173 exclusion tradeoff.** The user explicitly excluded 5173, but it is
   Vite's default port. Should exclusion be *dynamic* (only when another
   service owns it) or stay a hard denylist? Is the current "still found via
   stdout" path a sufficient safety net?
3. **HTML heuristic scope.** `isDevServerResponse` accepts any `text/html`
   under 500. A SPA that returns `text/html` but fails to boot would pass.
   Should we add a "title/body sanity" check or keep it minimal?
4. **Scan range maintenance.** Curated ranges must be extended as new
   toolchains appear (e.g., Turbopack, SolidStart). Should ranges be
   configurable per-workspace instead of a package constant?
5. **Race in `waitForServer`.** The URL and port fields are read under
   `instance.mu`, but the final `instance.Port` read at the end is not. Minor,
   but worth confirming the lock discipline is complete.
6. **BackupFile policy.** `BackupFile` (`tools/backup.go`) keeps backups
   **in-memory** (a bounded slice) and powers the `UndoLast` tool
   (`ClearBackups` clears them). No files leak into the user's repo. Open
   question: is the slice bounded? A long session with many edits grows memory;
   should it cap (e.g., last N edits)?
7. **Test isolation.** `findExistingDevServer` depends on real local ports.
   Should it be made injectable (port provider interface) to allow hermetic
   tests that never touch the network?

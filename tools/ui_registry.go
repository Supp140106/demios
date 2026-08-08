package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/invopop/jsonschema"
)

type SearchUIComponentsArgs struct {
	Query    string `json:"query" jsonschema:"title=Query,description=What to search for (e.g. 'animated card', 'gradient background', 'login form', 'button')"`
	Registry string `json:"registry,omitempty" jsonschema:"title=Registry,description=Filter by registry: 'shadcn', 'reactbits', 'magicui', 'aceternity', or 'all' (default),enum=shadcn,enum=reactbits,enum=magicui,enum=aceternity,enum=all"`
}

// registry describes an installable UI component registry and its LLM-friendly index.
type registry struct {
	id          string
	label       string
	source      string
	install     func(uic *uiComponent) string
	description string
}

func (r registry) installLine(uic *uiComponent) string {
	line := "[" + r.label + "] " + uic.Name
	if cmd := r.install(uic); cmd != "" {
		line += " — " + cmd
	}
	return line
}

type uiComponent struct {
	Name        string
	Description string
	URL         string
	Install     string
	Path        string
}

var uiRegistries = []registry{
	{
		id:    "shadcn",
		label: "shadcn",
		source: "https://ui.shadcn.com/llms.txt",
		install: func(uic *uiComponent) string {
			return "npx shadcn@latest add " + uic.Name
		},
		description: "Base components (Button, Card, Dialog, Input, Select).",
	},
	{
		id:    "reactbits",
		label: "ReactBits",
		source: "https://reactbits.dev/llms.txt",
		install: func(uic *uiComponent) string {
			if uic.Install != "" {
				return "npx shadcn@latest add https://reactbits.dev/r/" + uic.Install
			}
			return "npx shadcn@latest add https://reactbits.dev" + uic.Path
		},
		description: "135+ animated components (Aurora, Particles, BlurText).",
	},
	{
		id:    "magicui",
		label: "MagicUI",
		source: "https://magicui.design/llms.txt",
		install: func(uic *uiComponent) string {
			return "npx @magicuidesign/cli@latest add " + uic.Name
		},
		description: "Animated effects (Marquee, Bento Grid, Globe, Dock).",
	},
	{
		id:    "aceternity",
		label: "Aceternity",
		source: "https://ui.aceternity.com/llms.txt",
		install: func(uic *uiComponent) string {
			seg := strings.TrimPrefix(uic.Path, "/")
			if i := strings.LastIndex(seg, "/"); i >= 0 {
				seg = seg[i+1:]
			}
			return "npx @aceternity/cli@latest add " + seg
		},
		description: "Motion components (Background Beams, Sparkles, CardHover).",
	},
}

const (
	registryCacheTTL = 6 * time.Hour
	maxPerRegistry   = 5
)

var (
	registryCache   = map[string]*cachedIndex{}
	registryCacheMu sync.Mutex
)

type cachedIndex struct {
	components []uiComponent
	fetchedAt  time.Time
}

var (
	llmsNameRe   = regexp.MustCompile(`\[([^\]]+)\]\((https?://[^\)]+)\)(?::\s*(.*))?`)
	llmsCLIRe    = regexp.MustCompile(`CLI:\s*\` + "`" + `([^` + "`" + `]+)` + "`" + ``)
	llmsRouteRe  = regexp.MustCompile(`(?:https?://[^/]+)(/[\w\-\/]+)`)
)

// loadRegistryIndex fetches and caches the component list for a registry from its
// llms.txt file. Fetching is cached in-memory for registryCacheTTL to keep context
// and bandwidth low across repeated searches.
func loadRegistryIndex(reg registry, client *http.Client) ([]uiComponent, error) {
	registryCacheMu.Lock()
	if cached, ok := registryCache[reg.id]; ok && time.Since(cached.fetchedAt) < registryCacheTTL {
		registryCacheMu.Unlock()
		return cached.components, nil
	}
	registryCacheMu.Unlock()

	resp, err := client.Get(reg.source)
	if err != nil {
		return nil, fmt.Errorf("fetch %s: %w", reg.source, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("fetch %s: HTTP %d", reg.source, resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", reg.source, err)
	}

	var components []uiComponent
	for _, line := range strings.Split(string(body), "\n") {
		line = strings.TrimSpace(line)
		m := llmsNameRe.FindStringSubmatch(line)
		if len(m) < 3 {
			continue
		}
		comp := uiComponent{
			Name:        m[1],
			URL:         m[2],
			Description: strings.TrimSpace(m[3]),
		}
		if cli := llmsCLIRe.FindStringSubmatch(line); len(cli) > 1 {
			comp.Install = cli[1]
		}
		if route := llmsRouteRe.FindStringSubmatch(m[2]); len(route) > 1 {
			comp.Path = route[1]
		}
		components = append(components, comp)
	}

	registryCacheMu.Lock()
	registryCache[reg.id] = &cachedIndex{components: components, fetchedAt: time.Now()}
	registryCacheMu.Unlock()

	return components, nil
}

var SearchUIComponents = Tool{
	ID:          "SearchUIComponents",
	Description: "Search for UI components from shadcn/ui, ReactBits, Magic UI, and Aceternity UI registries. Returns component names and installation commands. ALWAYS use this before building any UI from scratch — never reinvent what already exists in these registries.",
	Schema:      jsonschema.Reflect(&SearchUIComponentsArgs{}),
	Execute: func(ctx context.Context, rawArgs json.RawMessage) (ExecuteResult, error) {
		var args SearchUIComponentsArgs
		if err := json.Unmarshal(rawArgs, &args); err != nil {
			return ExecuteResult{}, fmt.Errorf("invalid arguments: %w", err)
		}
		if args.Query == "" {
			return ExecuteResult{}, fmt.Errorf("query is required")
		}
		registryFilter := strings.ToLower(strings.TrimSpace(args.Registry))
		if registryFilter == "" {
			registryFilter = "all"
		}

		client := &http.Client{Timeout: 15 * time.Second}

		type scored struct {
			score int
			line  string
		}
		var matches []scored

		for _, reg := range uiRegistries {
			if registryFilter != "all" && registryFilter != reg.id {
				continue
			}
			components, err := loadRegistryIndex(reg, client)
			if err != nil {
				// Skip failing registries; never fail the whole search.
				continue
			}
			top := rankComponents(components, args.Query, maxPerRegistry)
			for _, c := range top {
				matches = append(matches, scored{score: c.score, line: reg.installLine(&c.component)})
			}
		}

		if len(matches) == 0 {
			return ExecuteResult{
				Title:  "No components found",
				Output: fmt.Sprintf("No UI components found matching '%s' in %s. Try a different query or browse the registries directly.", args.Query, registryFilter),
			}, nil
		}

		sort.Slice(matches, func(i, j int) bool { return matches[i].score > matches[j].score })

		var b strings.Builder
		fmt.Fprintf(&b, "UI components matching '%s' (%d results):\n", args.Query, len(matches))
		for _, m := range matches {
			b.WriteString("- ")
			b.WriteString(m.line)
			b.WriteByte('\n')
		}

		return ExecuteResult{
			Title:  fmt.Sprintf("Found %d UI components", len(matches)),
			Output: b.String(),
		}, nil
	},
}

type scoredComponent struct {
	component uiComponent
	score     int
}

// rankComponents scores components by how well they match the query and returns the
// top n. Matching name tokens outweigh description matches, keeping output tight and
// relevant.
func rankComponents(components []uiComponent, query string, n int) []scoredComponent {
	qWords := strings.Fields(strings.ToLower(query))
	scored := make([]scoredComponent, 0, len(components))
	for _, c := range components {
		name := strings.ToLower(c.Name)
		desc := strings.ToLower(c.Description)
		score := 0
		for _, w := range qWords {
			if strings.Contains(name, w) {
				score += 3
			}
			if strings.Contains(desc, w) {
				score += 1
			}
		}
		if score > 0 {
			scored = append(scored, scoredComponent{component: c, score: score})
		}
	}
	sort.SliceStable(scored, func(i, j int) bool { return scored[i].score > scored[j].score })
	if len(scored) > n {
		scored = scored[:n]
	}
	return scored
}

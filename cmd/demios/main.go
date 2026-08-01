// Command demios is a headless harness for the Demios agent. Any AI coding
// agent (Claude Code, OpenCode, Antigravity, ...) can drive the browser agent
// from a shell:
//
//	demios test-website --url http://localhost:5173 --prompt "..."
//	demios start-server --workdir ./my-app
//	demios list-servers
//	demios stop-server --id server-1
//
// Servers started here are managed persistently across invocations, exactly
// like the in-app harness, so subsequent runs reuse them.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"demios/core"
	"demios/llm"

	"github.com/joho/godotenv"
)

func main() {
	godotenv.Load()

	fs := flag.NewFlagSet("demios", flag.ExitOnError)
	fs.Usage = func() { usage(os.Stderr) }
	fs.Parse(os.Args[1:])

	args := fs.Args()
	if len(args) == 0 {
		usage(os.Stderr)
		os.Exit(2)
	}

	switch args[0] {
	case "test-website":
		cmdTestWebsite(args[1:])
	case "start-server":
		cmdStartServer(args[1:])
	case "stop-server":
		cmdStopServer(args[1:])
	case "list-servers":
		cmdListServers(args[1:])
	case "models":
		cmdModels(args[1:])
	case "help", "-h", "--help":
		usage(os.Stdout)
	default:
		fmt.Fprintf(os.Stderr, "demios: unknown command %q\n\n", args[0])
		usage(os.Stderr)
		os.Exit(2)
	}
}

func usage(w *os.File) {
	fmt.Fprint(w, `demios — headless agent harness for browser testing and dev-server management.

USAGE:
  demios <command> [flags]

COMMANDS:
  test-website   Run the Browser Agent against a URL (or auto-resolved server)
                 and print its report. Exits 0 on success, 1 on failure.
  start-server   Start a dev server, detect its port, and print id/url/port.
  list-servers   List servers currently managed by the harness.
  stop-server    Stop a running server by id.
  models         List available model IDs.

GLOBAL NOTES:
  - OPENROUTER_API_KEY (or the chosen model's key env) must be set.
  - Servers persist across invocations; use stop-server to clean them up.

Run 'demios <command> --help' for command-specific flags.
`)
}

func newAgent(workdir, model string) (*core.Agent, error) {
	a := core.NewAgent("demios-cli")
	if workdir != "" {
		a.SetWorkspace(workdir)
	} else if wd, err := os.Getwd(); err == nil {
		a.SetWorkspace(wd)
	}
	if model != "" {
		if err := a.SetModel(model); err != nil {
			return nil, fmt.Errorf("unknown model %q (run 'demios models')", model)
		}
	}
	if err := a.ServerManager().EnablePersistence(registryPath()); err != nil {
		return nil, fmt.Errorf("load server registry: %w", err)
	}
	return a, nil
}

// registryPath returns the on-disk location of the persistent server registry.
func registryPath() string {
	base := os.Getenv("LOCALAPPDATA")
	if base == "" {
		if dir, err := os.UserConfigDir(); err == nil {
			base = dir
		}
	}
	return filepath.Join(base, "demios", "servers.json")
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "demios:", err)
	os.Exit(1)
}

func cmdTestWebsite(args []string) {
	fs := flag.NewFlagSet("test-website", flag.ExitOnError)
	url := fs.String("url", "", "URL to test (e.g. http://localhost:5173). If empty, a running server is used or one is auto-started.")
	prompt := fs.String("prompt", "", "Detailed instructions for the Browser Agent (what to click, fill, verify, report).")
	workdir := fs.String("workdir", "", "Workspace directory (default: current directory).")
	model := fs.String("model", "", "Model ID to use (default: current model). See 'demios models'.")
	timeout := fs.Int("timeout", 300, "Max seconds to wait for the Browser Agent (default 300).")
	fs.Parse(args)

	agent, err := newAgent(*workdir, *model)
	if err != nil {
		fatal(err)
	}

	if *prompt == "" {
		if *url == "" {
			*prompt = "Navigate to the page, wait for it to load, take a screenshot, extract the visible text, and report what you see including the title and key content. Verify it renders correctly."
		} else {
			*prompt = fmt.Sprintf("Test the website at %s: navigate to it, wait for the page to load, take a screenshot, extract the visible text content, and report what you see including the page title and key content. Verify the page renders correctly.", *url)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(*timeout)*time.Second)
	defer cancel()

	report, err := agent.TestWebsite(ctx, *url, *prompt)
	if err != nil {
		fatal(err)
	}
	fmt.Println(report)
}

func cmdStartServer(args []string) {
	fs := flag.NewFlagSet("start-server", flag.ExitOnError)
	workdir := fs.String("workdir", "", "Working directory (default: current directory).")
	command := fs.String("command", "", "Command to start the server (default: auto-detected from package.json).")
	port := fs.Int("port", 0, "Preferred port (default: auto-detect from server output).")
	model := fs.String("model", "", "Model ID for LLM URL resolution (default: current model). See 'demios models'.")
	fs.Parse(args)

	ws := *workdir
	if ws == "" {
		ws, _ = os.Getwd()
	}
	agent, err := newAgent(ws, *model)
	if err != nil {
		fatal(err)
	}

	inst, err := agent.StartDevServer(context.Background(), ws, *command, *port)
	if err != nil {
		fatal(err)
	}
	fmt.Printf("id=%s\nurl=%s\nport=%d\npid=%d\nproject=%s\ncommand=%s\nstatus=%s\n",
		inst.ID, inst.URL, inst.Port, inst.PID, inst.ProjectDir, inst.Command, inst.Status)
}

func cmdStopServer(args []string) {
	fs := flag.NewFlagSet("stop-server", flag.ExitOnError)
	id := fs.String("id", "", "Server ID to stop (from start-server or list-servers).")
	fs.Parse(args)

	if *id == "" {
		fatal(errors.New("--id is required"))
	}
	agent, err := newAgent("", "")
	if err != nil {
		fatal(err)
	}
	if err := agent.StopDevServer(*id); err != nil {
		fatal(err)
	}
	fmt.Printf("stopped %s\n", *id)
}

func cmdListServers(args []string) {
	fs := flag.NewFlagSet("list-servers", flag.ExitOnError)
	fs.Parse(args)

	agent, err := newAgent("", "")
	if err != nil {
		fatal(err)
	}
	servers := agent.ListDevServers()
	if len(servers) == 0 {
		fmt.Println("No servers running.")
		return
	}
	for _, s := range servers {
		fmt.Printf("%s\tstatus=%s\turl=%s\tpid=%d\tproject=%s\n",
			s.ID, s.Status, s.URL, s.PID, s.ProjectDir)
	}
}

func cmdModels(args []string) {
	fs := flag.NewFlagSet("models", flag.ExitOnError)
	fs.Parse(args)

	for _, m := range llm.AvailableModels {
		fmt.Printf("%s\t(%s, model=%s, env=%s)\n", m.ID, m.Label, m.Model, m.APIKey)
	}
}

package tools

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/invopop/jsonschema"
)

// browserURLBlocked reports whether a URL should never be navigated to by the
// browser agent. The excluded set comes from the active session context (the
// Demios backend's own random port, ports the user excluded via
// DEMIOS_EXCLUDE_PORTS, etc.) — nothing is hardcoded.
func browserURLBlocked(ctx context.Context, rawURL string) bool {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	if parsed.Port() == "" {
		return false
	}
	p, err := strconv.Atoi(parsed.Port())
	if err != nil {
		return false
	}
	for _, ep := range BrowserExcludedPortsFrom(ctx) {
		if ep == p {
			return true
		}
	}
	return false
}

type BrowserNavigateArgs struct {
	URL string `json:"url" jsonschema:"title=Url,description=Full URL to navigate to (e.g. https://google.com)"`
}

var BrowserNavigate = Tool{
	ID:          "browser_navigate",
	Description: "Navigate the browser to a URL.",
	Schema:      jsonschema.Reflect(&BrowserNavigateArgs{}),
	Execute: func(ctx context.Context, rawArgs json.RawMessage) (ExecuteResult, error) {
		var args BrowserNavigateArgs
		if err := json.Unmarshal(rawArgs, &args); err != nil {
			return ExecuteResult{}, fmt.Errorf("invalid arguments: %w", err)
		}
		if args.URL == "" {
			return ExecuteResult{}, fmt.Errorf("url is required")
		}
		if browserURLBlocked(ctx, args.URL) {
			return ExecuteResult{}, fmt.Errorf("navigation to %s is blocked (port is on the excluded list). Only navigate to the TARGET URL provided in the task.", args.URL)
		}
		sess := BrowserSessionFrom(ctx)
		if sess == nil {
			return ExecuteResult{}, fmt.Errorf("browser session not available")
		}
		if err := sess.Navigate(args.URL); err != nil {
			return ExecuteResult{}, fmt.Errorf("navigate failed: %w", err)
		}
		url, _ := sess.CurrentURL()
		title, _ := sess.CurrentTitle()
		return ExecuteResult{
			Title:  fmt.Sprintf("Navigated to %s", url),
			Output: fmt.Sprintf("Navigated to %s — %s", url, title),
			Metadata: map[string]any{
				"url":   url,
				"title": title,
				"action": "navigate",
			},
		}, nil
	},
}

type BrowserClickArgs struct {
	Selector string `json:"selector" jsonschema:"title=Selector,description=CSS selector or text to click (e.g. 'Search button' or '#search-btn')"`
}

var BrowserClick = Tool{
	ID:          "browser_click",
	Description: "Click an element on the page by CSS selector or text.",
	Schema:      jsonschema.Reflect(&BrowserClickArgs{}),
	Execute: func(ctx context.Context, rawArgs json.RawMessage) (ExecuteResult, error) {
		var args BrowserClickArgs
		if err := json.Unmarshal(rawArgs, &args); err != nil {
			return ExecuteResult{}, fmt.Errorf("invalid arguments: %w", err)
		}
		if args.Selector == "" {
			return ExecuteResult{}, fmt.Errorf("selector is required")
		}
		sess := BrowserSessionFrom(ctx)
		if sess == nil {
			return ExecuteResult{}, fmt.Errorf("browser session not available")
		}
		if err := sess.Click(args.Selector); err != nil {
			return ExecuteResult{}, fmt.Errorf("click failed: %w", err)
		}
		return ExecuteResult{
			Title:  fmt.Sprintf("Clicked %s", args.Selector),
			Output: fmt.Sprintf("Clicked element matching \"%s\"", args.Selector),
			Metadata: map[string]any{
				"selector": args.Selector,
				"action":   "click",
			},
		}, nil
	},
}

type BrowserTypeArgs struct {
	Selector string `json:"selector,omitempty" jsonschema:"title=Selector,description=CSS selector of the element to type into (optional, types into the first visible input if omitted)"`
	Text     string `json:"text" jsonschema:"title=Text,description=Text to type into the element"`
}

var BrowserType = Tool{
	ID:          "browser_type",
	Description: "Type text into an input field. If a selector is provided, types into that specific element. Otherwise types into the first visible input on the page.",
	Schema:      jsonschema.Reflect(&BrowserTypeArgs{}),
	Execute: func(ctx context.Context, rawArgs json.RawMessage) (ExecuteResult, error) {
		var args BrowserTypeArgs
		if err := json.Unmarshal(rawArgs, &args); err != nil {
			return ExecuteResult{}, fmt.Errorf("invalid arguments: %w", err)
		}
		if args.Text == "" {
			return ExecuteResult{}, fmt.Errorf("text is required")
		}
		sess := BrowserSessionFrom(ctx)
		if sess == nil {
			return ExecuteResult{}, fmt.Errorf("browser session not available")
		}
		selector := args.Selector
		if selector == "" {
			selector = "input,textarea,[contenteditable]"
		}
		url, _ := sess.CurrentURL()
		if err := sess.Type(selector, args.Text); err != nil {
			return ExecuteResult{}, fmt.Errorf("type failed: %w", err)
		}
		return ExecuteResult{
			Title:  fmt.Sprintf("Typed into %s", selector),
			Output: fmt.Sprintf("Typed \"%s\" into \"%s\" on %s", args.Text, selector, url),
			Metadata: map[string]any{
				"text":     args.Text,
				"selector": selector,
				"url":      url,
				"action":   "type",
			},
		}, nil
	},
}

type BrowserFillArgs struct {
	Selector string `json:"selector" jsonschema:"title=Selector,description=CSS selector of the input to fill (e.g. '#username')"`
	Value    string `json:"value" jsonschema:"title=Value,description=Text to fill into the input, replacing any existing value"`
}

var BrowserFill = Tool{
	ID:          "browser_fill",
	Description: "Fill an input field by CSS selector with a value (replaces existing text). Use this for form fields like login inputs.",
	Schema:      jsonschema.Reflect(&BrowserFillArgs{}),
	Execute: func(ctx context.Context, rawArgs json.RawMessage) (ExecuteResult, error) {
		var args BrowserFillArgs
		if err := json.Unmarshal(rawArgs, &args); err != nil {
			return ExecuteResult{}, fmt.Errorf("invalid arguments: %w", err)
		}
		if args.Selector == "" {
			return ExecuteResult{}, fmt.Errorf("selector is required")
		}
		if args.Value == "" {
			return ExecuteResult{}, fmt.Errorf("value is required")
		}
		sess := BrowserSessionFrom(ctx)
		if sess == nil {
			return ExecuteResult{}, fmt.Errorf("browser session not available")
		}
		url, _ := sess.CurrentURL()
		if err := sess.Fill(args.Selector, args.Value); err != nil {
			return ExecuteResult{}, fmt.Errorf("fill failed: %w", err)
		}
		return ExecuteResult{
			Title:  fmt.Sprintf("Filled %s", args.Selector),
			Output: fmt.Sprintf("Filled input \"%s\" with \"%s\" on %s", args.Selector, args.Value, url),
			Metadata: map[string]any{
				"selector": args.Selector,
				"value":    args.Value,
				"url":      url,
				"action":   "fill",
			},
		}, nil
	},
}

type BrowserScreenshotArgs struct{}

var BrowserScreenshot = Tool{
	ID:          "browser_screenshot",
	Description: "Take a screenshot of the current browser viewport. Returns a base64-encoded PNG image.",
	Schema:      jsonschema.Reflect(&BrowserScreenshotArgs{}),
	Execute: func(ctx context.Context, rawArgs json.RawMessage) (ExecuteResult, error) {
		sess := BrowserSessionFrom(ctx)
		if sess == nil {
			return ExecuteResult{}, fmt.Errorf("browser session not available")
		}
		imgBytes, err := sess.Screenshot()
		if err != nil {
			return ExecuteResult{}, fmt.Errorf("screenshot failed: %w", err)
		}
		b64 := base64.StdEncoding.EncodeToString(imgBytes)
		url, _ := sess.CurrentURL()
		return ExecuteResult{
			Title:  "Screenshot captured",
			Output: fmt.Sprintf("Screenshot of %s captured (%d bytes, base64)", url, len(imgBytes)),
			Metadata: map[string]any{
				"screenshot": b64,
				"url":        url,
				"action":     "screenshot",
			},
		}, nil
	},
}

type BrowserExtractArgs struct {
	Selector string `json:"selector,omitempty" jsonschema:"title=Selector,description=CSS selector to extract text from (optional, extracts full page if omitted)"`
}

var BrowserExtract = Tool{
	ID:          "browser_extract",
	Description: "Extract visible text from the page or a specific element.",
	Schema:      jsonschema.Reflect(&BrowserExtractArgs{}),
	Execute: func(ctx context.Context, rawArgs json.RawMessage) (ExecuteResult, error) {
		var args BrowserExtractArgs
		if err := json.Unmarshal(rawArgs, &args); err != nil {
			return ExecuteResult{}, fmt.Errorf("invalid arguments: %w", err)
		}
		sess := BrowserSessionFrom(ctx)
		if sess == nil {
			return ExecuteResult{}, fmt.Errorf("browser session not available")
		}
		var text string
		var err error
		if args.Selector != "" {
			text, err = sess.Page().TextContent(args.Selector)
			if err != nil {
				return ExecuteResult{}, fmt.Errorf("extract failed: %w", err)
			}
		} else {
			text, err = sess.Page().TextContent("body")
			if err != nil {
				return ExecuteResult{}, fmt.Errorf("extract failed: %w", err)
			}
		}
		text = strings.TrimSpace(text)
		url, _ := sess.CurrentURL()
		return ExecuteResult{
			Title:  "Extracted content",
			Output: fmt.Sprintf("Extracted from %s: %s", url, truncateText(text, 2000)),
			Metadata: map[string]any{
				"text": text,
				"url":  url,
				"action": "extract",
			},
		}, nil
	},
}

type BrowserScrollArgs struct {
	DeltaX int `json:"delta_x,omitempty" jsonschema:"title=DeltaX,description=Horizontal scroll pixels (positive=right)"`
	DeltaY int `json:"delta_y" jsonschema:"title=DeltaY,description=Vertical scroll pixels (positive=down, negative=up)"`
}

var BrowserScroll = Tool{
	ID:          "browser_scroll",
	Description: "Scroll the page by the given pixel amounts.",
	Schema:      jsonschema.Reflect(&BrowserScrollArgs{}),
	Execute: func(ctx context.Context, rawArgs json.RawMessage) (ExecuteResult, error) {
		var args BrowserScrollArgs
		if err := json.Unmarshal(rawArgs, &args); err != nil {
			return ExecuteResult{}, fmt.Errorf("invalid arguments: %w", err)
		}
		sess := BrowserSessionFrom(ctx)
		if sess == nil {
			return ExecuteResult{}, fmt.Errorf("browser session not available")
		}
		if args.DeltaY < 0 {
			_ = sess.ScrollUp(float64(-args.DeltaY))
		} else if args.DeltaY > 0 {
			_ = sess.ScrollDown(float64(args.DeltaY))
		}
		if args.DeltaX != 0 {
			_ = sess.ScrollHorizontal(float64(args.DeltaX))
		}
		return ExecuteResult{
			Title:  fmt.Sprintf("Scrolled by (%d, %d)", args.DeltaX, args.DeltaY),
			Output: fmt.Sprintf("Scrolled page by (%d px horizontal, %d px vertical)", args.DeltaX, args.DeltaY),
			Metadata: map[string]any{
				"delta_x": args.DeltaX,
				"delta_y": args.DeltaY,
				"action":  "scroll",
			},
		}, nil
	},
}

type BrowserPressArgs struct {
	Key string `json:"key" jsonschema:"title=Key,description=Key to press (e.g. Enter, Tab, Escape, Backspace)"`
}

var BrowserPress = Tool{
	ID:          "browser_press",
	Description: "Press a keyboard key on the page (Enter, Tab, Escape, etc.).",
	Schema:      jsonschema.Reflect(&BrowserPressArgs{}),
	Execute: func(ctx context.Context, rawArgs json.RawMessage) (ExecuteResult, error) {
		var args BrowserPressArgs
		if err := json.Unmarshal(rawArgs, &args); err != nil {
			return ExecuteResult{}, fmt.Errorf("invalid arguments: %w", err)
		}
		if args.Key == "" {
			return ExecuteResult{}, fmt.Errorf("key is required")
		}
		sess := BrowserSessionFrom(ctx)
		if sess == nil {
			return ExecuteResult{}, fmt.Errorf("browser session not available")
		}
		url, _ := sess.CurrentURL()
		if err := sess.Press("body", args.Key); err != nil {
			return ExecuteResult{}, fmt.Errorf("press failed: %w", err)
		}
		return ExecuteResult{
			Title:  fmt.Sprintf("Pressed %s", args.Key),
			Output: fmt.Sprintf("Pressed key \"%s\" on page %s", args.Key, url),
			Metadata: map[string]any{
				"key":   args.Key,
				"url":   url,
				"action": "press",
			},
		}, nil
	},
}

type BrowserBackArgs struct{}

var BrowserBack = Tool{
	ID:          "browser_back",
	Description: "Navigate the browser back one step in history.",
	Schema:      jsonschema.Reflect(&BrowserBackArgs{}),
	Execute: func(ctx context.Context, rawArgs json.RawMessage) (ExecuteResult, error) {
		sess := BrowserSessionFrom(ctx)
		if sess == nil {
			return ExecuteResult{}, fmt.Errorf("browser session not available")
		}
		if err := sess.Back(); err != nil {
			return ExecuteResult{}, fmt.Errorf("navigate back failed: %w", err)
		}
		url, _ := sess.CurrentURL()
		return ExecuteResult{
			Title:  "Navigated back",
			Output: fmt.Sprintf("Navigated back to %s", url),
			Metadata: map[string]any{
				"url":   url,
				"action": "back",
			},
		}, nil
	},
}

type BrowserReloadArgs struct{}

var BrowserReload = Tool{
	ID:          "browser_reload",
	Description: "Reload the current page.",
	Schema:      jsonschema.Reflect(&BrowserReloadArgs{}),
	Execute: func(ctx context.Context, rawArgs json.RawMessage) (ExecuteResult, error) {
		sess := BrowserSessionFrom(ctx)
		if sess == nil {
			return ExecuteResult{}, fmt.Errorf("browser session not available")
		}
		if err := sess.Reload(); err != nil {
			return ExecuteResult{}, fmt.Errorf("reload failed: %w", err)
		}
		url, _ := sess.CurrentURL()
		title, _ := sess.CurrentTitle()
		return ExecuteResult{
			Title:  "Page reloaded",
			Output: fmt.Sprintf("Reloaded %s — %s", url, title),
			Metadata: map[string]any{
				"url":   url,
				"title": title,
				"action": "reload",
			},
		}, nil
	},
}

type BrowserWaitArgs struct {
	Selector string `json:"selector,omitempty" jsonschema:"title=Selector,description=CSS selector to wait for (optional)"`
	MS       int    `json:"ms,omitempty" jsonschema:"title=Milliseconds,description=Time to wait in milliseconds (default 1000)"`
}

var BrowserWait = Tool{
	ID:          "browser_wait",
	Description: "Wait for a selector to appear on the page, or wait a fixed number of milliseconds.",
	Schema:      jsonschema.Reflect(&BrowserWaitArgs{}),
	Execute: func(ctx context.Context, rawArgs json.RawMessage) (ExecuteResult, error) {
		var args BrowserWaitArgs
		if err := json.Unmarshal(rawArgs, &args); err != nil {
			return ExecuteResult{}, fmt.Errorf("invalid arguments: %w", err)
		}
		ms := args.MS
		if ms <= 0 {
			ms = 1000
		}
		sess := BrowserSessionFrom(ctx)
		if sess == nil {
			return ExecuteResult{}, fmt.Errorf("browser session not available")
		}
		if args.Selector != "" {
			if err := sess.WaitForSelector(args.Selector); err != nil {
				return ExecuteResult{}, fmt.Errorf("wait for selector failed: %w", err)
			}
		} else {
			sess.Wait(float64(ms))
		}
		url, _ := sess.CurrentURL()
		return ExecuteResult{
			Title:  fmt.Sprintf("Waited (%dms, selector=%s)", ms, args.Selector),
			Output: fmt.Sprintf("Waited %dms on %s", ms, url),
			Metadata: map[string]any{
				"selector": args.Selector,
				"ms":       ms,
				"url":      url,
				"action":   "wait",
			},
		}, nil
	},
}

type BrowserTestArgs struct {
	URL string `json:"url" jsonschema:"title=Url,description=URL of the website to test (e.g. http://localhost:3000)"`
}

var BrowserTest = Tool{
	ID:          "browser_test",
	Description: "Test a website by navigating to it, taking a screenshot, and reporting what you see. Use this to verify a running dev server or check a website's appearance.",
	Schema:      jsonschema.Reflect(&BrowserTestArgs{}),
	Execute: func(ctx context.Context, rawArgs json.RawMessage) (ExecuteResult, error) {
		var args BrowserTestArgs
		if err := json.Unmarshal(rawArgs, &args); err != nil {
			return ExecuteResult{}, fmt.Errorf("invalid arguments: %w", err)
		}
		if args.URL == "" {
			return ExecuteResult{}, fmt.Errorf("url is required")
		}
		sess := BrowserSessionFrom(ctx)
		if sess == nil {
			return ExecuteResult{}, fmt.Errorf("browser session not available")
		}
		if err := sess.Navigate(args.URL); err != nil {
			return ExecuteResult{}, fmt.Errorf("navigate failed: %w", err)
		}
		sess.Wait(2000)
		imgBytes, err := sess.Screenshot()
		if err != nil {
			return ExecuteResult{}, fmt.Errorf("screenshot failed: %w", err)
		}
		b64 := base64.StdEncoding.EncodeToString(imgBytes)
		url, _ := sess.CurrentURL()
		title, _ := sess.CurrentTitle()
		text, _ := sess.Page().TextContent("body")
		text = strings.TrimSpace(text)
		return ExecuteResult{
			Title:  fmt.Sprintf("Tested %s", url),
			Output: fmt.Sprintf("Tested %s — Title: %s\n\nContent preview:\n%s", url, title, truncateText(text, 3000)),
			Metadata: map[string]any{
				"url":        url,
				"title":      title,
				"screenshot": b64,
				"action":     "test",
			},
		}, nil
	},
}

type BrowserStopArgs struct {
	Message string `json:"message,omitempty" jsonschema:"title=Message,description=Reason for stopping the browser session"`
}

var BrowserStop = Tool{
	ID:          "browser_stop",
	Description: "Stop the browser session and close Chromium.",
	Schema:      jsonschema.Reflect(&BrowserStopArgs{}),
	Execute: func(ctx context.Context, rawArgs json.RawMessage) (ExecuteResult, error) {
		var args BrowserStopArgs
		if err := json.Unmarshal(rawArgs, &args); err != nil {
			return ExecuteResult{}, fmt.Errorf("invalid arguments: %w", err)
		}
		sess := BrowserSessionFrom(ctx)
		if sess == nil {
			return ExecuteResult{}, fmt.Errorf("no active browser session")
		}
		_ = sess.Stop(ctx)
		ClearBrowserSession(ctx)
		msg := "Browser session stopped"
		if args.Message != "" {
			msg += ": " + args.Message
		}
		return ExecuteResult{
			Title:  "Browser stopped",
			Output: msg,
			Metadata: map[string]any{
				"action": "stop",
			},
		}, nil
	},
}

func truncateText(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
package tools

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/mxschmitt/playwright-go"
)

type browserKey struct{}

func WithBrowserSession(ctx context.Context, sess *BrowserSession) context.Context {
	return context.WithValue(ctx, browserKey{}, sess)
}

func BrowserSessionFrom(ctx context.Context) *BrowserSession {
	sess, _ := ctx.Value(browserKey{}).(*BrowserSession)
	return sess
}

type excludedPortsKey struct{}

// WithBrowserExcludedPorts attaches a list of local ports the browser agent
// must never navigate to. The agent seeds this with the Demios backend's own
// random port plus any user-configured exclusions so nothing is hardcoded.
func WithBrowserExcludedPorts(ctx context.Context, ports []int) context.Context {
	return context.WithValue(ctx, excludedPortsKey{}, ports)
}

// BrowserExcludedPortsFrom returns the excluded ports attached to ctx.
func BrowserExcludedPortsFrom(ctx context.Context) []int {
	ports, _ := ctx.Value(excludedPortsKey{}).([]int)
	return ports
}

func ClearBrowserSession(ctx context.Context) {
	_ = ctx
}

type BrowserSession struct {
	pw      *playwright.Playwright
	browser playwright.Browser
	context playwright.BrowserContext
	page    playwright.Page
	open    bool
}

func NewBrowserSession() *BrowserSession {
	return &BrowserSession{}
}

func (s *BrowserSession) Start(ctx context.Context) error {
	if s.open {
		return nil
	}

	userDataDir := filepath.Join(os.Getenv("LOCALAPPDATA"), "demios", "browser-profile")
	if err := os.MkdirAll(userDataDir, 0755); err != nil {
		return fmt.Errorf("create profile dir: %w", err)
	}

	// playwright.Install() downloads browsers into %LOCALAPPDATA%\ms-playwright.
	// Check that location so we don't re-"download" on every browser start.
	playwrightInstallDir := filepath.Join(os.Getenv("LOCALAPPDATA"), "ms-playwright")
	if _, err := os.Stat(playwrightInstallDir); os.IsNotExist(err) {
		log.Println("[browser] installing playwright browsers (this is a one-time step)...")
		if err := playwright.Install(); err != nil {
			return fmt.Errorf("install playwright: %w", err)
		}
		log.Println("[browser] playwright browsers installed")
	}

	pw, err := playwright.Run()
	if err != nil {
		return fmt.Errorf("start playwright: %w", err)
	}
	s.pw = pw

	browser, err := pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
		Headless: playwright.Bool(false),
	})
	if err != nil {
		// The installed browser revision may not match what this playwright-go
		// version expects. (Re)install the correct revision and retry once so
		// the harness "just works" without manual setup.
		log.Printf("[browser] chromium launch failed: %v; (re)installing playwright browsers...", err)
		if ierr := playwright.Install(); ierr != nil {
			pw.Stop()
			return fmt.Errorf("launch chromium: %v (browser reinstall also failed: %v)", err, ierr)
		}
		browser, err = pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
			Headless: playwright.Bool(false),
		})
		if err != nil {
			pw.Stop()
			return fmt.Errorf("launch chromium after reinstall: %w", err)
		}
	}
	s.browser = browser

	bc, err := browser.NewContext()
	if err != nil {
		browser.Close()
		pw.Stop()
		return fmt.Errorf("create context: %w", err)
	}
	s.context = bc

	page, err := bc.NewPage()
	if err != nil {
		bc.Close()
		browser.Close()
		pw.Stop()
		return fmt.Errorf("create page: %w", err)
	}
	s.page = page

	s.open = true
	log.Println("[browser] chromium opened")
	return nil
}

func (s *BrowserSession) Stop(ctx context.Context) error {
	if !s.open {
		return nil
	}
	var lastErr error
	if s.page != nil {
		if err := s.page.Close(); err != nil && lastErr == nil {
			lastErr = err
		}
	}
	if s.context != nil {
		if err := s.context.Close(); err != nil && lastErr == nil {
			lastErr = err
		}
	}
	if s.browser != nil {
		if err := s.browser.Close(); err != nil && lastErr == nil {
			lastErr = err
		}
	}
	if s.pw != nil {
		if err := s.pw.Stop(); err != nil && lastErr == nil {
			lastErr = err
		}
	}
	s.open = false
	log.Println("[browser] chromium closed")
	return lastErr
}

func (s *BrowserSession) Navigate(url string) error {
	if !s.open || s.page == nil {
		return fmt.Errorf("browser not started")
	}
	_, err := s.page.Goto(url, playwright.PageGotoOptions{
		WaitUntil: playwright.WaitUntilStateNetworkidle,
	})
	return err
}

func (s *BrowserSession) CurrentURL() (string, error) {
	if !s.open || s.page == nil {
		return "", fmt.Errorf("browser not started")
	}
	return s.page.URL(), nil
}

func (s *BrowserSession) CurrentTitle() (string, error) {
	if !s.open || s.page == nil {
		return "", fmt.Errorf("browser not started")
	}
	return s.page.Title()
}

func (s *BrowserSession) Screenshot() ([]byte, error) {
	if !s.open || s.page == nil {
		return nil, fmt.Errorf("browser not started")
	}
	return s.page.Screenshot()
}

func (s *BrowserSession) Content() (string, error) {
	if !s.open || s.page == nil {
		return "", fmt.Errorf("browser not started")
	}
	return s.page.Content()
}

func (s *BrowserSession) Click(selector string) error {
	if !s.open || s.page == nil {
		return fmt.Errorf("browser not started")
	}
	return s.page.Click(selector, playwright.PageClickOptions{
		Timeout: playwright.Float(10000),
	})
}

func (s *BrowserSession) Fill(selector, text string) error {
	if !s.open || s.page == nil {
		return fmt.Errorf("browser not started")
	}
	return s.page.Fill(selector, text, playwright.PageFillOptions{
		Timeout: playwright.Float(10000),
	})
}

func (s *BrowserSession) Type(selector, text string) error {
	if !s.open || s.page == nil {
		return fmt.Errorf("browser not started")
	}
	return s.page.Type(selector, text, playwright.PageTypeOptions{
		Timeout: playwright.Float(10000),
	})
}

func (s *BrowserSession) Press(selector, key string) error {
	if !s.open || s.page == nil {
		return fmt.Errorf("browser not started")
	}
	return s.page.Press(selector, key, playwright.PagePressOptions{
		Timeout: playwright.Float(10000),
	})
}

func (s *BrowserSession) WaitForSelector(selector string) error {
	if !s.open || s.page == nil {
		return fmt.Errorf("browser not started")
	}
	_, err := s.page.WaitForSelector(selector, playwright.PageWaitForSelectorOptions{
		State:  playwright.WaitForSelectorStateVisible,
		Timeout: playwright.Float(15000),
	})
	return err
}

func (s *BrowserSession) Wait(ms float64) {
	if !s.open || s.page == nil {
		return
	}
	s.page.WaitForTimeout(ms)
}

func (s *BrowserSession) ScrollDown(pixels float64) error {
	if !s.open || s.page == nil {
		return fmt.Errorf("browser not started")
	}
	_, _ = s.page.EvalOnSelector("html", "window.scrollBy(0, arguments[0])", pixels)
	return nil
}

func (s *BrowserSession) ScrollUp(pixels float64) error {
	return s.ScrollDown(-pixels)
}

func (s *BrowserSession) Back() error {
	if !s.open || s.page == nil {
		return fmt.Errorf("browser not started")
	}
	_, err := s.page.GoBack()
	return err
}

func (s *BrowserSession) IsOpen() bool {
	return s.open
}

func (s *BrowserSession) Page() playwright.Page {
	return s.page
}
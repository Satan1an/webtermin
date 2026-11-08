// Package notify pushes short alerts to a Telegram chat. Sending is
// fire-and-forget through a small queue, so a slow or unreachable Telegram API
// never holds up the request (or audit write) that triggered the alert.
package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"log/slog"
	"net/http"
	"os"
	"slices"
	"time"
)

// Event names an alert category; the config can enable a subset.
const (
	EventLogin         = "login"          // successful sign-in from an IP not seen before for that user
	EventLockout       = "lockout"        // sign-in locked out after repeated failures
	EventDenied        = "denied"         // RBAC / safety check refused an action
	EventReboot        = "reboot"         // host reboot requested from the panel
	EventSetup         = "setup"          // first-run admin created
	EventServiceFailed = "service_failed" // a systemd unit entered (or left) the failed state
	EventDisk          = "disk"           // a filesystem crossed the usage threshold
	EventUpdates       = "updates"        // new security updates became available
)

var AllEvents = []string{EventLogin, EventLockout, EventDenied, EventReboot, EventSetup,
	EventServiceFailed, EventDisk, EventUpdates}

type Config struct {
	BotToken string
	ChatID   string
	Events   []string // empty = all
	DiskPct  int
}

type Notifier struct {
	cfg    Config
	host   string
	log    *slog.Logger
	client *http.Client
	q      chan string
	api    string // Telegram API base, overridable in tests
}

// New returns nil when Telegram isn't configured; every method is nil-safe,
// so callers don't need to check.
func New(cfg Config, log *slog.Logger) *Notifier {
	if cfg.BotToken == "" || cfg.ChatID == "" {
		return nil
	}
	if cfg.DiskPct <= 0 || cfg.DiskPct > 100 {
		cfg.DiskPct = 90
	}
	host, _ := os.Hostname()
	return &Notifier{
		cfg: cfg, host: host, log: log,
		client: &http.Client{Timeout: 10 * time.Second},
		q:      make(chan string, 64),
		api:    "https://api.telegram.org",
	}
}

func (n *Notifier) Enabled() bool { return n != nil }

func (n *Notifier) DiskPct() int {
	if n == nil {
		return 0
	}
	return n.cfg.DiskPct
}

// Wants reports whether alerts of this kind should be sent.
func (n *Notifier) Wants(event string) bool {
	return n != nil && (len(n.cfg.Events) == 0 || slices.Contains(n.cfg.Events, event))
}

// Events returns the enabled event names.
func (n *Notifier) Events() []string {
	if n == nil {
		return nil
	}
	if len(n.cfg.Events) == 0 {
		return AllEvents
	}
	return n.cfg.Events
}

// Notify queues an alert of the given kind. title is plain text; lines are
// plain text too and get HTML-escaped. Drops the message if the queue is full
// rather than blocking the caller.
func (n *Notifier) Notify(event, title string, lines ...string) {
	if !n.Wants(event) {
		return
	}
	select {
	case n.q <- n.format(title, lines):
	default:
		n.log.Warn("notify queue full, dropping alert", "event", event)
	}
}

func (n *Notifier) format(title string, lines []string) string {
	var b bytes.Buffer
	fmt.Fprintf(&b, "<b>%s</b> · %s", html.EscapeString(n.host), html.EscapeString(title))
	for _, l := range lines {
		if l != "" {
			b.WriteString("\n" + html.EscapeString(l))
		}
	}
	return b.String()
}

// Run drains the queue until ctx ends, spacing messages a second apart to
// stay inside Telegram's per-chat rate limit.
func (n *Notifier) Run(ctx context.Context) {
	if n == nil {
		return
	}
	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-n.q:
			if err := n.send(ctx, msg); err != nil {
				n.log.Warn("telegram send failed", "err", err)
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Second):
			}
		}
	}
}

// Test sends a message synchronously so the admin sees errors (bad token,
// wrong chat id) right away.
func (n *Notifier) Test(ctx context.Context, who string) error {
	if n == nil {
		return errors.New("telegram is not configured (notify.telegram in config.yaml)")
	}
	return n.send(ctx, n.format("test alert", []string{"Sent from the panel by " + who + "."}))
}

func (n *Notifier) send(ctx context.Context, text string) error {
	body, _ := json.Marshal(map[string]any{
		"chat_id": n.cfg.ChatID, "text": text,
		"parse_mode": "HTML", "disable_web_page_preview": true,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, n.api+"/bot"+n.cfg.BotToken+"/sendMessage", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := n.client.Do(req)
	if err != nil {
		// The URL carries the bot token; never let it reach logs.
		var ue interface{ Unwrap() error }
		if errors.As(err, &ue) {
			return fmt.Errorf("telegram request: %w", ue.Unwrap())
		}
		return errors.New("telegram request failed")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		var r struct {
			Description string `json:"description"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&r)
		return fmt.Errorf("telegram: %d %s", resp.StatusCode, r.Description)
	}
	return nil
}

// Drain empties the queue and returns how many alerts were waiting. Tests use
// it to assert what would have been sent without a network round trip.
func Drain(n *Notifier) int {
	if n == nil {
		return 0
	}
	c := 0
	for {
		select {
		case <-n.q:
			c++
		default:
			return c
		}
	}
}

package notify

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewDisabledWithoutCredentials(t *testing.T) {
	n := New(Config{BotToken: "x"}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if n.Enabled() || n.Wants(EventLogin) {
		t.Fatal("notifier without chat id must be disabled")
	}
	n.Notify(EventLogin, "ignored") // nil-safe
	if err := n.Test(context.Background(), "admin"); err == nil {
		t.Fatal("Test on a disabled notifier must explain it is not configured")
	}
}

func TestEventFilterAndEscaping(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/botTOKEN/sendMessage") {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		_ = json.NewDecoder(r.Body).Decode(&got)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	n := New(Config{BotToken: "TOKEN", ChatID: "42", Events: []string{EventDisk}},
		slog.New(slog.NewTextHandler(io.Discard, nil)))
	n.api = srv.URL
	n.host = "pi"
	if n.Wants(EventLogin) || !n.Wants(EventDisk) {
		t.Fatal("event filter not applied")
	}
	if n.DiskPct() != 90 {
		t.Errorf("default disk threshold = %d", n.DiskPct())
	}
	if err := n.send(context.Background(), n.format("disk <full>", []string{"/ at 95% & rising"})); err != nil {
		t.Fatal(err)
	}
	want := "<b>pi</b> · disk &lt;full&gt;\n/ at 95% &amp; rising"
	if got["text"] != want || got["chat_id"] != "42" || got["parse_mode"] != "HTML" {
		t.Errorf("payload = %+v", got)
	}
}

func TestSendReportsTelegramError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"ok":false,"description":"Bad Request: chat not found"}`))
	}))
	defer srv.Close()
	n := New(Config{BotToken: "T", ChatID: "1"}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	n.api = srv.URL
	err := n.Test(context.Background(), "admin")
	if err == nil || !strings.Contains(err.Error(), "chat not found") {
		t.Fatalf("want telegram description in error, got %v", err)
	}
}

package greenhouse

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"jobclaw/internal/discovery"
)

func TestNewMultiBoardClientIgnoresEmptyBoardTokens(t *testing.T) {
	client := NewMultiBoardClient([]string{
		"",
		"  ",
		"stripe",
		"  test-board  ",
	})

	if len(client.clients) != 2 {
		t.Fatalf(
			"expected 2 clients, got %d",
			len(client.clients),
		)
	}

	if client.clients[0].boardToken != "stripe" {
		t.Fatalf(
			"expected first board token stripe, got %q",
			client.clients[0].boardToken,
		)
	}

	if client.clients[1].boardToken != "test-board" {
		t.Fatalf(
			"expected second board token test-board, got %q",
			client.clients[1].boardToken,
		)
	}
}

func TestMultiBoardClientName(t *testing.T) {
	client := NewMultiBoardClient([]string{"stripe"})

	if client.Name() != "greenhouse" {
		t.Fatalf(
			"expected greenhouse, got %q",
			client.Name(),
		)
	}
}

// TestMultiBoardClientIsolatesFailingBoard covers the regression that left
// Greenhouse discovery silently dead in production: one retired board token
// returned 404, the loop returned on the first error, and every healthy board's
// jobs were discarded. Discovery reported "Fetched: 0" for eleven working
// employers.
func TestMultiBoardClientIsolatesFailingBoard(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/boards/good":
				_, _ = w.Write([]byte(`{"name":"Good Co"}`))

			case "/boards/good/jobs":
				_, _ = w.Write([]byte(`{"jobs":[
					{"id":1,"title":"Backend Engineer","absolute_url":"https://example.test/1"},
					{"id":2,"title":"Platform Engineer","absolute_url":"https://example.test/2"}
				]}`))

			default:
				// Stands in for a board token that is not a Greenhouse board.
				w.WriteHeader(http.StatusNotFound)
			}
		},
	))
	defer server.Close()

	// "dead" is listed first so the test fails if the loop still aborts early.
	client := NewMultiBoardClient([]string{"dead", "good"})

	for _, boardClient := range client.clients {
		boardClient.baseURL = server.URL
	}

	jobs, err := client.Discover(
		context.Background(),
		discovery.Request{},
	)

	// The failure must still be reported: silently swallowing it would leave a
	// broken board token undiagnosable.
	if err == nil {
		t.Fatal("expected the failing board to be reported as an error")
	}

	if !strings.Contains(err.Error(), `board "dead"`) {
		t.Fatalf(
			"expected the error to name the failing board, got %q",
			err.Error(),
		)
	}

	if strings.Contains(err.Error(), `board "good"`) {
		t.Fatalf(
			"healthy board must not appear in the error, got %q",
			err.Error(),
		)
	}

	// The point of the fix: the healthy board's jobs survive.
	if len(jobs) != 2 {
		t.Fatalf(
			"expected 2 jobs from the healthy board, got %d",
			len(jobs),
		)
	}

	for _, discovered := range jobs {
		if discovered.Company != "Good Co" {
			t.Fatalf(
				"expected company Good Co, got %q",
				discovered.Company,
			)
		}

		if discovered.BoardToken != "good" {
			t.Fatalf(
				"expected board token good, got %q",
				discovered.BoardToken,
			)
		}
	}
}

func TestMultiBoardClientReportsEveryFailingBoard(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		},
	))
	defer server.Close()

	client := NewMultiBoardClient([]string{"one", "two"})

	for _, boardClient := range client.clients {
		boardClient.baseURL = server.URL
	}

	jobs, err := client.Discover(
		context.Background(),
		discovery.Request{},
	)

	if err == nil {
		t.Fatal("expected an error when every board fails")
	}

	if len(jobs) != 0 {
		t.Fatalf("expected no jobs, got %d", len(jobs))
	}

	// Every board is attempted, so a wholly misconfigured list reports each
	// token rather than only the first.
	for _, token := range []string{`board "one"`, `board "two"`} {
		if !strings.Contains(err.Error(), token) {
			t.Fatalf(
				"expected %s in the error, got %q",
				token,
				err.Error(),
			)
		}
	}
}

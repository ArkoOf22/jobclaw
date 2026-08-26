package greenhouse

import "testing"

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

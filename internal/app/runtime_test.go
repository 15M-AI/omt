package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestInstalledModelsFiltersFilesAndSortsNames(t *testing.T) {
	root := t.TempDir()
	modelDir := filepath.Join(root, "models")
	if err := os.MkdirAll(modelDir, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"zeta.omf", "Alpha.omf", "skip.gguf"} {
		if err := os.WriteFile(filepath.Join(modelDir, name), []byte("fixture"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(modelDir, "folder.omf"), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("OPEN_MIND_HOME", root)

	models, err := installedModels()
	if err != nil {
		t.Fatal(err)
	}
	if len(models) != 2 || models[0].Name != "Alpha.omf" || models[1].Name != "zeta.omf" {
		t.Fatalf("unexpected model list: %#v", models)
	}
}

func TestInstalledModelsAllowsMissingDirectory(t *testing.T) {
	t.Setenv("OPEN_MIND_HOME", t.TempDir())
	models, err := installedModels()
	if err != nil {
		t.Fatal(err)
	}
	if len(models) != 0 {
		t.Fatalf("expected an empty model library, got %#v", models)
	}
}

func TestGenerateUsesOMAAndParsesCompletion(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/generate" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode request: %v", err)
		}
		if body["model"] != "sample.omf" || body["prompt"] != "Hello" {
			t.Errorf("unexpected request body: %#v", body)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"choices":[{"text":" world"}]}`)
	}))
	defer server.Close()

	host, portText, err := net.SplitHostPort(strings.TrimPrefix(server.URL, "http://"))
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		t.Fatal(err)
	}
	got, err := generate(context.Background(), host, port, "sample.omf", "Hello", 16)
	if err != nil {
		t.Fatal(err)
	}
	if got != " world" {
		t.Fatalf("completion = %q, want %q", got, " world")
	}
}

func TestBuildPromptRetainsConversation(t *testing.T) {
	m := model{chat: []chatMessage{
		{role: "user", text: "Hi"},
		{role: "assistant", text: "Hello"},
		{role: "user", text: "How are you?"},
	}}
	want := "User: Hi\nAssistant: Hello\nUser: How are you?\nAssistant:"
	if got := m.buildPrompt(); got != want {
		t.Fatalf("buildPrompt() = %q, want %q", got, want)
	}
}

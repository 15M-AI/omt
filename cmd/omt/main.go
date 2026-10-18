package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"open-mind-tui/internal/app"
)

func main() {
	runtime := flag.String("runtime", "", "path to the Open Mind `om` runtime")
	source := flag.String("source", "", "path to the Open Mind source repository")
	host := flag.String("host", "127.0.0.1", "local OMA server bind address")
	port := flag.Int("port", 8080, "local OMA server port")
	version := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *version {
		fmt.Println("omt 0.2.0")
		return
	}
	if *port < 1 || *port > 65535 {
		fmt.Fprintln(os.Stderr, "error: --port must be between 1 and 65535")
		os.Exit(2)
	}

	if *source == "" {
		*source = os.Getenv("OPEN_MIND_SOURCE")
	}
	if *source == "" {
		home, _ := os.UserHomeDir()
		candidate := filepath.Join(home, "OpenMind")
		if _, err := os.Stat(filepath.Join(candidate, "Cargo.toml")); err == nil {
			*source = candidate
		}
	}
	if *runtime == "" {
		*runtime = os.Getenv("OPEN_MIND_RUNTIME")
	}
	if *runtime == "" {
		if *source != "" {
			candidate := filepath.Join(*source, "target", "release", "om")
			if _, err := os.Stat(candidate); err == nil {
				*runtime = candidate
			}
		}
		if *runtime == "" {
			*runtime, _ = lookPath("om")
		}
	}

	if err := app.Run(app.Config{
		Runtime: *runtime,
		Source:  *source,
		Host:    *host,
		Port:    *port,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "omt: %v\n", err)
		os.Exit(1)
	}
}

func lookPath(name string) (string, error) {
	return exec.LookPath(name)
}

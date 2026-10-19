package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"time"
)

func main() {
	mode := flag.String("mode", "defer", "defer|publish|canary|crash|hang|rate-limit|auth")
	output := flag.String("output", "", "last-message output path")
	flag.String("output-last-message", "", "codex-compatible output path")
	flag.Parse()
	for index, arg := range os.Args {
		if arg == "--output-last-message" && index+1 < len(os.Args) {
			*output = os.Args[index+1]
		}
	}
	switch *mode {
	case "defer":
		write(*output, `{"action":"defer","reason":"No meaningful contribution."}`)
	case "publish":
		write(*output, `{"action":"publish","reason":"Evidence warrants publication.","title":"A careful observation","type":"observation","body":"Grounded analysis.","sources":[{"title":"Source","url":"https://example.org/source","retrieved_at":"2026-10-18T20:00:00Z"}],"topics":["technology"],"related_ids":[],"confidence_rationale":"A connected source supports the claim."}`)
	case "canary":
		workingDirectory, _ := os.Getwd()
		value, _ := json.Marshal(map[string]any{
			"home":                  os.Getenv("HOME"),
			"cwd":                   workingDirectory,
			"publish_token_present": os.Getenv("THEMACHINEWRITES_GIT_PUBLISH_TOKEN") != "",
		})
		write(*output, string(value))
	case "crash":
		fmt.Fprintln(os.Stderr, "fixture crash")
		os.Exit(2)
	case "hang":
		time.Sleep(time.Hour)
	case "rate-limit":
		fmt.Fprintln(os.Stderr, "HTTP 429 rate limit")
		os.Exit(2)
	case "auth":
		fmt.Fprintln(os.Stderr, "HTTP 401 unauthorized")
		os.Exit(2)
	default:
		os.Exit(64)
	}
}

func write(path, value string) {
	if path != "" {
		_ = os.WriteFile(path, []byte(value), 0o600)
	}
	fmt.Fprintln(os.Stdout, value)
	_, _ = io.Copy(io.Discard, os.Stdin)
}

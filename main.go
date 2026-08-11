// kutt — minimal CLI for self-hosted Kutt (v2 API).
// Commands: setup, submit, list, delete
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const version = "1.0.0"

// --- config ---------------------------------------------------------------

type Config struct {
	Host   string
	APIKey string
}

func configPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".kutt")
}

// loadConfig reads ~/.kutt. Format: lines of key=value (host, apikey).
// Backward compatible: a file with no '=' is treated as a bare API key.
// Env vars KUTT_HOST and KUTT_API_KEY override the file.
func loadConfig() (*Config, error) {
	cfg := &Config{Host: "https://kutt.it"}
	data, err := os.ReadFile(configPath())
	if err == nil {
		content := strings.TrimSpace(string(data))
		if strings.Contains(content, "=") {
			for _, line := range strings.Split(content, "\n") {
				line = strings.TrimSpace(line)
				if line == "" || strings.HasPrefix(line, "#") {
					continue
				}
				k, v, ok := strings.Cut(line, "=")
				if ok {
					switch strings.TrimSpace(k) {
					case "host":
						cfg.Host = strings.TrimSpace(v)
					case "apikey":
						cfg.APIKey = strings.TrimSpace(v)
					}
				}
			}
		} else if content != "" {
			cfg.APIKey = content
		}
	}
	if h := os.Getenv("KUTT_HOST"); h != "" {
		cfg.Host = strings.TrimRight(h, "/")
	}
	if k := os.Getenv("KUTT_API_KEY"); k != "" {
		cfg.APIKey = k
	}
	if cfg.Host == "" || cfg.APIKey == "" {
		return nil, fmt.Errorf("missing config: run 'kutt setup' or set KUTT_HOST / KUTT_API_KEY")
	}
	return cfg, nil
}

func saveConfig(cfg *Config) error {
	content := fmt.Sprintf("host=%s\napikey=%s\n", cfg.Host, cfg.APIKey)
	return os.WriteFile(configPath(), []byte(content), 0600)
}

// --- api ------------------------------------------------------------------

type Link struct {
	ID         string    `json:"id"`
	Address    string    `json:"address"`
	Target     string    `json:"target"`
	Link       string    `json:"link"`
	Password   bool      `json:"password"`
	ExpireIn   any       `json:"expire_in"`
	VisitCount int       `json:"visit_count"`
	Domain     string    `json:"domain"`
	CreatedAt  time.Time `json:"created_at"`
}

type listResponse struct {
	Total int    `json:"total"`
	Data  []Link `json:"data"`
}

func doJSON(cfg *Config, method, path string, body io.Reader, out any) (int, error) {
	req, err := http.NewRequest(method, cfg.Host+path, body)
	if err != nil {
		return 0, err
	}
	req.Header.Set("X-API-Key", cfg.APIKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "kutt-cli/"+version)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return resp.StatusCode, fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	if out != nil {
		return resp.StatusCode, json.NewDecoder(resp.Body).Decode(out)
	}
	return resp.StatusCode, nil
}

// --- commands -------------------------------------------------------------

func cmdSetup(args []string) error {
	var host, key string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--host":
			i++
			if i >= len(args) {
				return fmt.Errorf("--host requires a value")
			}
			host = strings.TrimRight(args[i], "/")
		case "--apikey":
			i++
			if i >= len(args) {
				return fmt.Errorf("--apikey requires a value")
			}
			key = args[i]
		default:
			return fmt.Errorf("unknown flag: %s", args[i])
		}
	}
	if host == "" || key == "" {
		return fmt.Errorf("usage: kutt setup --host https://kutt.example --apikey <key>")
	}
	if err := saveConfig(&Config{Host: host, APIKey: key}); err != nil {
		return err
	}
	fmt.Printf("Saved config to %s (host=%s)\n", configPath(), host)
	return nil
}

func cmdSubmit(cfg *Config, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: kutt submit <url> [-c custom] [-p password] [-r]")
	}
	target := args[0]

	payload := map[string]any{"target": target, "reuse": false}
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "-c", "--custom":
			i++
			if i >= len(args) {
				return fmt.Errorf("%s requires a value", args[i-1])
			}
			payload["customurl"] = args[i]
		case "-p", "--password":
			i++
			if i >= len(args) {
				return fmt.Errorf("%s requires a value", args[i-1])
			}
			payload["password"] = args[i]
		case "-r", "--reuse":
			payload["reuse"] = true
		default:
			return fmt.Errorf("unknown flag: %s", args[i])
		}
	}

	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	var l Link
	code, err := doJSON(cfg, http.MethodPost, "/api/links", strings.NewReader(string(b)), &l)
	if err != nil {
		return err
	}
	if code != http.StatusCreated {
		return fmt.Errorf("unexpected status %d", code)
	}
	if l.Link != "" {
		fmt.Println(l.Link)
	} else {
		fmt.Printf("%s/%s\n", strings.TrimRight(cfg.Host, "/"), l.Address)
	}
	return nil
}

func cmdList(cfg *Config, args []string) error {
	limit := 10
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-n", "--limit":
			i++
			if i >= len(args) {
				return fmt.Errorf("%s requires a value", args[i-1])
			}
			n, err := strconv.Atoi(args[i])
			if err != nil || n < 1 {
				return fmt.Errorf("invalid limit: %s", args[i])
			}
			limit = n
		default:
			return fmt.Errorf("unknown flag: %s", args[i])
		}
	}

	var r listResponse
	if _, err := doJSON(cfg, http.MethodGet, "/api/links?limit="+strconv.Itoa(limit), nil, &r); err != nil {
		return err
	}
	if len(r.Data) == 0 {
		fmt.Println("No links.")
		return nil
	}
	// plain columns — stable in Telegram
	fmt.Printf("%-38s %-10s %-6s %s\n", "ID", "VISITS", "PASS", "LINK")
	for _, l := range r.Data {
		pass := "no"
		if l.Password {
			pass = "yes"
		}
		fmt.Printf("%-38s %-10d %-6s %s\n", l.ID, l.VisitCount, pass, l.Link)
	}
	return nil
}

func cmdDelete(cfg *Config, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: kutt delete <id>")
	}
	id := args[0]
	_, err := doJSON(cfg, http.MethodDelete, "/api/links/"+id, nil, nil)
	if err != nil {
		return err
	}
	fmt.Printf("Deleted %s\n", id)
	return nil
}

// --- main -----------------------------------------------------------------

func usage() {
	fmt.Printf(`kutt %s — CLI for self-hosted Kutt (v2 API)

Usage:
  kutt setup --host <url> --apikey <key>   save config to ~/.kutt
  kutt submit <url> [-c custom] [-p password] [-r]   shorten a URL
  kutt list [-n limit]                     list recent links
  kutt delete <id>                         delete a link

Config: ~/.kutt (host= / apikey=) or env KUTT_HOST, KUTT_API_KEY.
`, version)
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}
	cmd := os.Args[1]
	args := os.Args[2:]

	switch cmd {
	case "setup":
		if err := cmdSetup(args); err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			os.Exit(1)
		}
	case "submit", "list", "delete":
		cfg, err := loadConfig()
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			os.Exit(1)
		}
		var runErr error
		switch cmd {
		case "submit":
			runErr = cmdSubmit(cfg, args)
		case "list":
			runErr = cmdList(cfg, args)
		case "delete":
			runErr = cmdDelete(cfg, args)
		}
		if runErr != nil {
			fmt.Fprintln(os.Stderr, "Error:", runErr)
			os.Exit(1)
		}
	case "help", "-h", "--help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", cmd)
		usage()
		os.Exit(1)
	}
}

// Command agent runs on a VPN node and applies peer/client changes requested by
// the platform's provisioner (see internal/vpn/provisioner.go, "agent" mode).
//
// It manages WireGuard peers via the `wg` tool and, optionally, VLESS clients via
// configurable Xray management commands. It is intended to be deployed by the
// Ansible playbook in deploy/ansible and reached over a private network / TLS.
package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

type agent struct {
	token       string
	wgInterface string
	log         *slog.Logger
}

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	a := &agent{
		token:       os.Getenv("AGENT_TOKEN"),
		wgInterface: getenv("WG_INTERFACE", "wg0"),
		log:         log,
	}
	if a.token == "" {
		log.Error("AGENT_TOKEN is required")
		os.Exit(1)
	}
	addr := getenv("AGENT_ADDR", ":8090")

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.Handle("POST /wg/peers", a.auth(http.HandlerFunc(a.addWGPeer)))
	mux.Handle("POST /wg/peers/remove", a.auth(http.HandlerFunc(a.removeWGPeer)))
	mux.Handle("POST /vless/clients", a.auth(http.HandlerFunc(a.addVLESS)))
	mux.Handle("POST /vless/clients/remove", a.auth(http.HandlerFunc(a.removeVLESS)))

	srv := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	log.Info("agent listening", "addr", addr, "wg_interface", a.wgInterface)
	if err := srv.ListenAndServe(); err != nil {
		log.Error("server stopped", "error", err)
		os.Exit(1)
	}
}

func (a *agent) auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+a.token {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (a *agent) addWGPeer(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PublicKey    string `json:"public_key"`
		PresharedKey string `json:"preshared_key"`
		AllowedIP    string `json:"allowed_ip"`
	}
	if !decode(w, r, &req) {
		return
	}
	args := []string{"set", a.wgInterface, "peer", req.PublicKey, "allowed-ips", req.AllowedIP}

	// `wg` reads the preshared key from a file path; feed it via a temp file.
	if req.PresharedKey != "" {
		f, err := os.CreateTemp("", "psk-*")
		if err != nil {
			a.fail(w, "create psk file", err)
			return
		}
		defer func() { _ = os.Remove(f.Name()) }()
		if _, err := f.WriteString(req.PresharedKey + "\n"); err != nil {
			a.fail(w, "write psk", err)
			return
		}
		_ = f.Close()
		args = append(args, "preshared-key", f.Name())
	}

	if err := a.run("wg", args...); err != nil {
		a.fail(w, "wg set peer", err)
		return
	}
	a.persistWG()
	writeJSON(w, http.StatusOK, map[string]string{"status": "added"})
}

func (a *agent) removeWGPeer(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PublicKey string `json:"public_key"`
	}
	if !decode(w, r, &req) {
		return
	}
	if err := a.run("wg", "set", a.wgInterface, "peer", req.PublicKey, "remove"); err != nil {
		a.fail(w, "wg remove peer", err)
		return
	}
	a.persistWG()
	writeJSON(w, http.StatusOK, map[string]string{"status": "removed"})
}

// addVLESS / removeVLESS shell out to configurable Xray management commands.
// Set XRAY_ADD_CMD / XRAY_REMOVE_CMD to a template containing {uuid}; e.g. a
// wrapper that calls the Xray gRPC HandlerService. Returns 501 if unconfigured.
func (a *agent) addVLESS(w http.ResponseWriter, r *http.Request) {
	a.vlessOp(w, r, os.Getenv("XRAY_ADD_CMD"))
}

func (a *agent) removeVLESS(w http.ResponseWriter, r *http.Request) {
	a.vlessOp(w, r, os.Getenv("XRAY_REMOVE_CMD"))
}

func (a *agent) vlessOp(w http.ResponseWriter, r *http.Request, tmpl string) {
	var req struct {
		UUID string `json:"uuid"`
	}
	if !decode(w, r, &req) {
		return
	}
	if tmpl == "" {
		writeJSON(w, http.StatusNotImplemented,
			map[string]string{"error": "VLESS management command not configured (set XRAY_ADD_CMD/XRAY_REMOVE_CMD)"})
		return
	}
	cmdline := strings.ReplaceAll(tmpl, "{uuid}", req.UUID)
	if err := a.run("sh", "-c", cmdline); err != nil {
		a.fail(w, "xray command", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (a *agent) run(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %s: %w (output: %s)", name, strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}

// persistWG best-effort saves the running config so peers survive a restart.
func (a *agent) persistWG() {
	if err := a.run("sh", "-c", "wg-quick save "+a.wgInterface); err != nil {
		a.log.Warn("wg persist failed", "error", err)
	}
}

func (a *agent) fail(w http.ResponseWriter, ctx string, err error) {
	a.log.Error(ctx, "error", err)
	writeJSON(w, http.StatusInternalServerError, map[string]string{"error": ctx})
}

func decode(w http.ResponseWriter, r *http.Request, dst any) bool {
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body"})
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, code int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(body)
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

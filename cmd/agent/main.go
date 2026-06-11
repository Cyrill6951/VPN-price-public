// Command agent runs on a VPN node and applies peer/client changes requested by
// the platform's provisioner (see internal/vpn/provisioner.go, "agent" mode).
//
// WireGuard peers are managed via the `wg` tool. VLESS clients are managed by
// editing the Xray config's clients list and reloading Xray.
//
// It is intended to be deployed by the Ansible playbook in deploy/ansible and
// reached over a private network / TLS.
package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

type agent struct {
	token       string
	wgInterface string

	// VLESS / Xray
	xrayConfig string
	xrayTag    string
	xrayFlow   string
	reloadCmd  string
	xrayMu     sync.Mutex

	log *slog.Logger
}

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	a := &agent{
		token:       os.Getenv("AGENT_TOKEN"),
		wgInterface: getenv("WG_INTERFACE", "wg0"),
		xrayConfig:  getenv("XRAY_CONFIG", "/opt/vpn-node/xray.json"),
		xrayTag:     getenv("XRAY_INBOUND_TAG", "vless-reality"),
		xrayFlow:    getenv("XRAY_FLOW", "xtls-rprx-vision"),
		reloadCmd:   getenv("XRAY_RELOAD_CMD", "docker restart vpn-node-xray-1"),
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
	log.Info("agent listening", "addr", addr, "wg_interface", a.wgInterface, "xray_tag", a.xrayTag)
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

// --- WireGuard ---

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

// --- VLESS (Xray) ---

func (a *agent) addVLESS(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UUID string `json:"uuid"`
	}
	if !decode(w, r, &req) {
		return
	}
	if err := a.editVLESS(req.UUID, true); err != nil {
		a.fail(w, "add vless client", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "added"})
}

func (a *agent) removeVLESS(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UUID string `json:"uuid"`
	}
	if !decode(w, r, &req) {
		return
	}
	if err := a.editVLESS(req.UUID, false); err != nil {
		a.fail(w, "remove vless client", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "removed"})
}

// editVLESS adds or removes a client UUID in the Xray config's target inbound and
// reloads Xray. Access is serialised to avoid racing config writes.
func (a *agent) editVLESS(uuid string, add bool) error {
	if uuid == "" {
		return fmt.Errorf("uuid is required")
	}
	a.xrayMu.Lock()
	defer a.xrayMu.Unlock()

	raw, err := os.ReadFile(a.xrayConfig)
	if err != nil {
		return fmt.Errorf("read xray config: %w", err)
	}
	var cfg map[string]any
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return fmt.Errorf("parse xray config: %w", err)
	}

	settings, err := a.inboundSettings(cfg)
	if err != nil {
		return err
	}

	clients, _ := settings["clients"].([]any)
	filtered := make([]any, 0, len(clients)+1)
	exists := false
	for _, c := range clients {
		cm, ok := c.(map[string]any)
		if ok && cm["id"] == uuid {
			exists = true
			if !add {
				continue // drop on remove
			}
		}
		filtered = append(filtered, c)
	}
	if add && !exists {
		filtered = append(filtered, map[string]any{"id": uuid, "flow": a.xrayFlow})
	}
	settings["clients"] = filtered

	out, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal xray config: %w", err)
	}
	if err := os.WriteFile(a.xrayConfig, out, 0o644); err != nil {
		return fmt.Errorf("write xray config: %w", err)
	}
	if err := a.run("sh", "-c", a.reloadCmd); err != nil {
		return fmt.Errorf("reload xray: %w", err)
	}
	return nil
}

// inboundSettings locates the settings object of the configured VLESS inbound.
func (a *agent) inboundSettings(cfg map[string]any) (map[string]any, error) {
	inbounds, ok := cfg["inbounds"].([]any)
	if !ok {
		return nil, fmt.Errorf("xray config has no inbounds array")
	}
	for _, ib := range inbounds {
		ibm, ok := ib.(map[string]any)
		if !ok || ibm["tag"] != a.xrayTag {
			continue
		}
		settings, ok := ibm["settings"].(map[string]any)
		if !ok {
			settings = map[string]any{}
			ibm["settings"] = settings
		}
		return settings, nil
	}
	return nil, fmt.Errorf("inbound with tag %q not found", a.xrayTag)
}

// --- helpers ---

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

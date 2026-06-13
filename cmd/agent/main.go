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
	"runtime"
	"strconv"
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
	xraySSTag  string
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
		xraySSTag:   getenv("XRAY_SS_INBOUND_TAG", "shadowsocks"),
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
	mux.Handle("POST /ss/clients", a.auth(http.HandlerFunc(a.addSS)))
	mux.Handle("POST /ss/clients/remove", a.auth(http.HandlerFunc(a.removeSS)))
	mux.Handle("GET /status", a.auth(http.HandlerFunc(a.status)))

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

func (a *agent) editVLESS(uuid string, add bool) error {
	if uuid == "" {
		return fmt.Errorf("uuid is required")
	}
	return a.editXrayClients(a.xrayTag, "id", uuid, add, map[string]any{"id": uuid, "flow": a.xrayFlow})
}

// addSS / removeSS manage Shadowsocks-2022 per-user keys on the SS inbound.
func (a *agent) addSS(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Password string `json:"password"`
		Email    string `json:"email"`
	}
	if !decode(w, r, &req) {
		return
	}
	if err := a.editXrayClients(a.xraySSTag, "email", req.Email, true,
		map[string]any{"password": req.Password, "email": req.Email}); err != nil {
		a.fail(w, "add ss client", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "added"})
}

func (a *agent) removeSS(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email string `json:"email"`
	}
	if !decode(w, r, &req) {
		return
	}
	if err := a.editXrayClients(a.xraySSTag, "email", req.Email, false, nil); err != nil {
		a.fail(w, "remove ss client", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "removed"})
}

// editXrayClients adds or removes a client (matched by matchField==matchValue) in
// the given inbound's clients list and reloads Xray. Serialised against races.
func (a *agent) editXrayClients(tag, matchField, matchValue string, add bool, newClient map[string]any) error {
	if matchValue == "" {
		return fmt.Errorf("%s is required", matchField)
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

	settings, err := a.inboundSettings(cfg, tag)
	if err != nil {
		return err
	}

	clients, _ := settings["clients"].([]any)
	filtered := make([]any, 0, len(clients)+1)
	exists := false
	for _, c := range clients {
		cm, ok := c.(map[string]any)
		if ok && cm[matchField] == matchValue {
			exists = true
			if !add {
				continue
			}
		}
		filtered = append(filtered, c)
	}
	if add && !exists {
		filtered = append(filtered, newClient)
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

// inboundSettings locates the settings object of the inbound with the given tag.
func (a *agent) inboundSettings(cfg map[string]any, tag string) (map[string]any, error) {
	inbounds, ok := cfg["inbounds"].([]any)
	if !ok {
		return nil, fmt.Errorf("xray config has no inbounds array")
	}
	for _, ib := range inbounds {
		ibm, ok := ib.(map[string]any)
		if !ok || ibm["tag"] != tag {
			continue
		}
		settings, ok := ibm["settings"].(map[string]any)
		if !ok {
			settings = map[string]any{}
			ibm["settings"] = settings
		}
		return settings, nil
	}
	return nil, fmt.Errorf("inbound with tag %q not found", tag)
}

// --- node status / health ---

// blockTargets are the sites probed from the node to detect egress blocking.
func blockTargets() map[string]string {
	targets := map[string]string{
		"google":     "https://www.google.com/generate_204",
		"youtube":    "https://www.youtube.com/favicon.ico",
		"telegram":   "https://api.telegram.org",
		"cloudflare": "https://www.cloudflare.com/cdn-cgi/trace",
	}
	if env := os.Getenv("BLOCK_CHECK_TARGETS"); env != "" {
		targets = map[string]string{}
		for _, pair := range strings.Split(env, ",") {
			if name, url, ok := strings.Cut(pair, "="); ok {
				targets[strings.TrimSpace(name)] = strings.TrimSpace(url)
			}
		}
	}
	return targets
}

func (a *agent) status(w http.ResponseWriter, _ *http.Request) {
	st := map[string]any{
		"cpu_load":     cpuLoad(),
		"mem_used_pct": memUsedPct(),
		"wg_peers":     a.wgPeerCount(),
		"xray_up":      a.xrayUp(),
		"blocks":       a.blockCheck(),
	}
	writeJSON(w, http.StatusOK, st)
}

func cpuLoad() float64 {
	b, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return 0
	}
	fields := strings.Fields(string(b))
	if len(fields) == 0 {
		return 0
	}
	load, _ := strconv.ParseFloat(fields[0], 64)
	n := runtime.NumCPU()
	if n == 0 {
		n = 1
	}
	return load / float64(n)
}

func memUsedPct() float64 {
	b, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0
	}
	var total, avail float64
	for _, line := range strings.Split(string(b), "\n") {
		f := strings.Fields(line)
		if len(f) < 2 {
			continue
		}
		switch f[0] {
		case "MemTotal:":
			total, _ = strconv.ParseFloat(f[1], 64)
		case "MemAvailable:":
			avail, _ = strconv.ParseFloat(f[1], 64)
		}
	}
	if total == 0 {
		return 0
	}
	return (1 - avail/total) * 100
}

func (a *agent) wgPeerCount() int {
	cmd := exec.Command("wg", "show", a.wgInterface, "peers")
	out, err := cmd.Output()
	if err != nil {
		return 0
	}
	n := 0
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if strings.TrimSpace(line) != "" {
			n++
		}
	}
	return n
}

func (a *agent) xrayUp() bool {
	out, err := exec.Command("sh", "-c", "docker ps --filter name=xray --filter status=running -q").Output()
	return err == nil && strings.TrimSpace(string(out)) != ""
}

func (a *agent) blockCheck() map[string]bool {
	client := &http.Client{Timeout: 6 * time.Second}
	res := map[string]bool{}
	var mu sync.Mutex
	var wg sync.WaitGroup
	for name, url := range blockTargets() {
		wg.Add(1)
		go func(name, url string) {
			defer wg.Done()
			ok := false
			if req, err := http.NewRequest(http.MethodGet, url, nil); err == nil {
				if resp, err := client.Do(req); err == nil {
					ok = resp.StatusCode < 500
					_ = resp.Body.Close()
				}
			}
			mu.Lock()
			res[name] = ok
			mu.Unlock()
		}(name, url)
	}
	wg.Wait()
	return res
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

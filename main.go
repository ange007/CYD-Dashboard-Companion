package main

import (
	"embed"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"cyd-companion/internal/collector"
	"cyd-companion/internal/config"
	"cyd-companion/internal/discovery"
	"cyd-companion/internal/focus"
	"cyd-companion/internal/rules"
	"cyd-companion/internal/tray"
	"cyd-companion/internal/ws"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed all:frontend/dist
var assets embed.FS

const defaultConfigPath = "config.json"
const metricsPort = 9800 // HTTP-only endpoint for CYD Web UI cross-origin access

func main() {
	startHidden := false
	for _, a := range os.Args[1:] {
		if a == "--minimized" {
			startHidden = true
		}
	}

	// Always log to file (no console when built with -H windowsgui)
	logFile, err := os.OpenFile("companion.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err == nil {
		log.SetOutput(logFile)
		defer logFile.Close()
	}
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	cfg, err := config.Load(defaultConfigPath)
	if err != nil {
		cfg = config.Default()
		if err := cfg.Save(defaultConfigPath); err != nil {
			log.Printf("[Config] could not save defaults: %v", err)
		}
	}

	deviceURL := discovery.Discover(cfg.DeviceURL, cfg.MdnsHostHint)
	cfg.DeviceURL = deviceURL
	log.Printf("[Config] device_url=%s", cfg.DeviceURL)

	col := collector.New()
	wsClient := ws.NewClient(cfg, col)

	focusTracker := focus.New()
	rulesEngine := rules.New(cfg.FocusRules,
		func(profileID string) error {
			if err := wsClient.SwitchProfile(profileID); err != nil {
				log.Printf("[Rules] profile switch failed: %v", err)
				return err
			}
			return nil
		},
		func() string {
			return wsClient.GetState().ActiveProfile
		},
	)

	// Forward every device-side profile change to the rules engine so that
	// manual profile switches (touch UI, web UI) update the restore target
	// even while a focus rule is active.
	wsClient.SetProfileChangedHandler(func(id string) {
		rulesEngine.NotifyProfileChange(id)
	})

	app := NewApp(cfg, defaultConfigPath, wsClient, col, rulesEngine)

	go wsClient.Run()

	// Focus tracking ticker
	go func() {
		ticker := time.NewTicker(time.Duration(cfg.PushIntervalMs) * time.Millisecond)
		defer ticker.Stop()
		for range ticker.C {
			w := focusTracker.Get()
			if w != nil {
				wsClient.SetFocusInfo(w.Title, w.Process)
				rulesEngine.Evaluate(w)
			}
		}
	}()

	// Lightweight HTTP server — exposes /api/metrics with CORS for the CYD Web UI.
	// Wails already serves the frontend; this port is for cross-origin access only.
	go startMetricsHTTP(col, metricsPort)

	// System tray — runs alongside Wails window.
	// Wrapped in a restart loop: systray's nativeLoop() exits on any
	// GetMessage error or unexpected WM_QUIT; we restart it automatically so the
	// tray icon stays alive for the lifetime of the app.
	trayQuit := make(chan struct{}) // closed when the user explicitly chooses Quit
	go func() {
		cb := tray.Callbacks{
			OnOpen: func() {
				if app.ctx != nil {
					wailsruntime.WindowShow(app.ctx)
					wailsruntime.WindowSetAlwaysOnTop(app.ctx, true)
					wailsruntime.WindowSetAlwaysOnTop(app.ctx, false)
				}
			},
			OnOpenDashboard: func() {
				st := wsClient.GetState()
				httpURL := wsURLToHTTP(st.DeviceURL)
				if httpURL == "" {
					httpURL = fmt.Sprintf("http://localhost:%d", metricsPort)
				}
				openBrowser(httpURL)
			},
			OnQuit: func() {
				log.Println("[Companion] quit via tray")
				close(trayQuit)
				wsClient.Stop()
				os.Exit(0)
			},
			StatusFunc: func() string {
				st := wsClient.GetState()
				if st.Connected {
					url := wsURLToHTTP(st.DeviceURL)
					if url != "" {
						return "Connected — " + url
					}
					return "Connected"
				}
				return "Disconnected"
			},
			GetAutoStart: app.GetAutoStart,
			SetAutoStart: func(on bool) {
				if err := app.SetAutoStart(on); err != nil {
					log.Printf("[Autostart] set %v failed: %v", on, err)
				}
			},
		}
		for {
			func() {
				defer func() {
					if r := recover(); r != nil {
						log.Printf("[Tray] recovered from panic: %v", r)
					}
				}()
				tray.Run(cb)
			}()

			// If the user explicitly quit, don't restart.
			select {
			case <-trayQuit:
				return
			default:
			}
			log.Println("[Tray] exited unexpectedly, restarting in 1 s…")
			time.Sleep(time.Second)
		}
	}()

	err = wails.Run(&options.App{
		Title:            "CYD Companion",
		Width:            860,
		Height:           620,
		MinWidth:         700,
		MinHeight:        500,
		BackgroundColour: &options.RGBA{R: 15, G: 23, B: 42, A: 1}, // slate-900
		AssetServer:      &assetserver.Options{Assets: assets},
		OnStartup:        app.startup,
		// When launched via autostart entry (--minimized), go straight to tray.
		// Manual launch always shows the window.
		StartHidden:       startHidden,
		HideWindowOnClose: true,
		Bind:              []interface{}{app},
	})
	if err != nil {
		log.Printf("[Wails] error: %v", err)
	}
}

// openBrowser opens url in the default system browser.
func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	if err := cmd.Start(); err != nil {
		log.Printf("[Tray] openBrowser error: %v", err)
	}
}

// wsURLToHTTP converts ws://host/ws → http://host
func wsURLToHTTP(wsURL string) string {
	s := wsURL
	if strings.HasPrefix(s, "ws://") {
		s = "http://" + s[5:]
	}
	if strings.HasSuffix(s, "/ws") {
		s = s[:len(s)-3]
	}
	return s
}

// startMetricsHTTP runs a minimal HTTP server that exposes /api/metrics with CORS.
// This lets the CYD Web UI (served from the device) discover available metrics.
func startMetricsHTTP(col *collector.Collector, port int) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/metrics", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodOptions {
			w.Header().Set("Access-Control-Allow-Methods", "GET")
			w.WriteHeader(http.StatusNoContent)
			return
		}
		metrics := col.Catalog()
		enc := make([]byte, 0, 512)
		enc = append(enc, '[')
		for i, m := range metrics {
			if i > 0 {
				enc = append(enc, ',')
			}
			enc = append(enc, []byte(fmt.Sprintf(
				`{"id":%q,"label":%q,"category":%q,"unit":%q,"value":%q}`,
				m.ID, m.Label, m.Category, m.Unit, m.Value,
			))...)
		}
		enc = append(enc, ']')
		w.Write(enc)
	})
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	log.Printf("[MetricsHTTP] listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Printf("[MetricsHTTP] error: %v", err)
	}
}

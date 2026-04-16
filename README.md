# CYD Dashboard Companion

Desktop companion app for [CYD Dashboard](https://github.com/ange007/CYD-Dashboard) — a touchscreen macro & widget dashboard for Sunton ESP32 "Cheap Yellow Display" boards.

## Features

- **Background tray daemon** — hides to the system tray on close; stays alive as a persistent proxy even when no window is open. Tray menu: Open, Open dashboard, Start at login, Quit.
- **Start at login** — toggle in the tray menu or Settings page; on Windows writes an HKCU Run key, on macOS writes a LaunchAgent plist, on Linux writes an XDG autostart `.desktop` file. Autostart entry launches with `--minimized` so the app goes straight to the tray.
- **HID relay** — injects keyboard shortcuts and text typed on the CYD device into the host PC. Windows only (full SendInput); macOS/Linux return an error.
- **Fetch proxy** — handles `get_url` requests from the firmware: performs the HTTP GET on the device's behalf (no CORS restrictions, no 64 KB limit), parses the response with regex or JSON path, and returns `return_url_response`. Works on all platforms.
- **System metrics** — supplies CPU, memory, disk, network, and temperature data to CYD widgets.
- **Device telemetry** — displays firmware heap, uptime, Wi-Fi RSSI/IP, HTTP and TCP counters.
- **Profile switching** — shows the active macro profile and lets you switch from the desktop.
- **Auto-discovery** — finds the device via mDNS (configurable host hint) or a fixed URL.

## Prerequisites

- [Go 1.22+](https://golang.org/dl/)
- [Node.js 18+](https://nodejs.org/)
- [Wails v2](https://wails.io/docs/gettingstarted/installation): `go install github.com/wailsapp/wails/v2/cmd/wails@latest`

## Development

```bash
wails dev
```

Opens a hot-reload dev window and a dev server at `http://localhost:34115`.

## Building

```bash
wails build
```

Produces `build/bin/companion.exe` (Windows) or the platform equivalent.

## Configuration

On first launch the app creates `config.json` in the working directory (or alongside the binary in production). Use `config.example.json` as a reference:

```json
{
  "device_url": "ws://cyd-dashboard.local:80/ws",
  "mdns_host_hint": "cyd",
  "push_interval_ms": 2000,
  "local_port": 9800,
  "hid_relay_enabled": true,
  "commands_allowlist": [],
  "focus_rules": []
}
```

Leave `device_url` empty to rely on mDNS auto-discovery (looks for a host containing the `mdns_host_hint` string).

## Profiles

The companion can switch the device's active macro profile two ways:

- **Manual** — the **Rules** page shows a button per profile; click to switch. Sent over the existing WebSocket (`switch_profile`), so it works on **all platforms**.
- **Focus-based auto-switch** — define rules that switch the profile automatically based on the focused application, then restore the previous profile when you leave it.

### Focus rules

Each rule is a regular expression matched (case-insensitive) against `process|title` of the active window — e.g. `Code\.exe|cursor`. The first matching rule's profile becomes active; when no rule matches, the profile that was active before the rule fired is restored. A manual switch while a rule is active becomes the new restore target.

Edit rules on the **Rules** page (regex + profile dropdown, Add / Remove / Save) or directly in `config.json`:

```json
"focus_rules": [
  { "match": "Code\\.exe|cursor",  "profile_id": "dev" },
  { "match": "photoshop|gimp",     "profile_id": "design" }
]
```

> `profile_id` is the device profile ID, not its display name. The Rules page dropdown picks the right ID for you.

### Platform support

Auto-switch depends on detecting the focused window, which varies per OS. The `switch_profile` command itself works everywhere — only the focus trigger is limited:

| Platform | Focus source | Matchable fields | Notes |
|----------|--------------|------------------|-------|
| Windows | Win32 `GetForegroundWindow` | process + window title | Full support |
| macOS | `osascript` (System Events) | app name only | `process` and `title` both hold the frontmost app name — match on the app, not per-window title |
| Linux (X11) | `xdotool` | process + window title | Requires `xdotool` |
| Linux (Wayland) | `hyprctl` | window class + title | **Hyprland only** — GNOME/KDE Wayland have no supported source, so rules never fire |

## Installation

### Windows
Download `companion-installer.exe` from the [latest release](https://github.com/ange007/CYD-Dashboard/releases/latest) and run it. Or use the bare `companion.exe` — no install needed, just place it anywhere and run. Toggle **Start at login** in Settings or the tray menu to register the Run key.

### macOS
Download `companion-macos.zip`, unzip, and drag `companion.app` to Applications. First launch: right-click → Open (to bypass Gatekeeper — the app is unsigned). Toggle **Start at login** to install a LaunchAgent.

> Linux focus tracking requires `xdotool` (X11) or `hyprctl` (Hyprland/Wayland).

### Linux
Download `companion-linux-amd64.tar.gz`, extract, and run the binary. For focus-based profile rules install `xdotool` (`apt install xdotool`) or use Hyprland which is auto-detected. Toggle **Start at login** to write an XDG autostart entry.

## Platforms

| Feature | Windows | macOS | Linux |
|---------|---------|-------|-------|
| Fetch proxy (`get_url`) | ✅ | ✅ | ✅ |
| System metrics | ✅ | ✅ | ✅ |
| Device telemetry | ✅ | ✅ | ✅ |
| Auto-discovery (mDNS) | ✅ | ✅ | ✅ |
| Focus tracking | ✅ Win32 | ✅ osascript | ✅ xdotool/hyprctl |
| Start at login | ✅ Registry | ✅ LaunchAgent | ✅ XDG autostart |
| **HID relay** | ✅ SendInput | ❌ stub | ❌ stub |

## License

[MIT](LICENSE)

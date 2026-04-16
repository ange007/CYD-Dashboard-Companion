// Wails exposes Go methods via window.go.main.App.*
// Bindings are auto-generated into wailsjs/ on first `wails build` / `wails dev`.

export interface Status {
  connected: boolean
  device_url: string
  os: string
  hostname: string
  active_title: string
  active_process: string
  active_profile: string
}

export interface Metric {
  id: string
  label: string
  category: string
  unit: string
  value: string
}

export interface DeviceDiag {
  uptime_ms: number
  wifi: { status: number; rssi: number; ip: string; disconnects: number; reconnects: number }
  http: { enter: number; ok: number; '503_heap': number; '503_busy': number }
  tcp: { opens: number; closes: number; reject_lowheap: number }
  gw: { ping_ok: number; ping_fail: number }
  heap: { free: number; min: number; psram: number }
}

export interface Config {
  device_url: string
  mdns_host_hint: string
  push_interval_ms: number
  local_port: number
  commands_allowlist: string[]
  focus_rules: FocusRule[]
}

export interface FocusRule {
  match: string
  profile_id: string
}

export interface Profile {
  id: string
  name: string
}

// Lazy-loaded Wails bindings (generated at build time into wailsjs/)
let _App: typeof import('../../wailsjs/go/main/App') | null = null
async function App() {
  if (!_App) _App = await import('../../wailsjs/go/main/App')
  return _App
}

export const api = {
  async status():               Promise<Status>                           { return (await App()).GetStatus() },
  async metrics():              Promise<Metric[]>                         { return (await App()).GetMetrics() },
  async config():               Promise<Config>                           { return (await App()).GetConfig() },
  async saveConfig(cfg: Config):Promise<void>                            { return (await App()).SaveConfig(cfg as any) },
  async profiles():             Promise<{ list: Profile[]; active_id: string }> { return (await App()).GetProfiles() as any },
  async switchProfile(id: string): Promise<void>                         { return (await App()).SwitchProfile(id) },
  async deviceDiag():              Promise<DeviceDiag | null>             { return (await App()).GetDeviceDiag() as any },
  async getAutoStart():         Promise<boolean>                          { return (await App()).GetAutoStart() },
  async setAutoStart(on: boolean): Promise<void>                         { return (await App()).SetAutoStart(on) },
}

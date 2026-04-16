<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { api, type Status, type Metric, type DeviceDiag } from '@/api'

const status  = ref<Status | null>(null)
const metrics = ref<Metric[]>([])
const diag    = ref<DeviceDiag | null>(null)
const error   = ref('')
let timer: ReturnType<typeof setInterval>

async function load() {
  try {
    status.value = await api.status()
    error.value = ''
  } catch (e) {
    error.value = String(e)
  }
  try {
    const m = await api.metrics()
    if (m?.length) metrics.value = m
  } catch {
    // keep last known metrics on transient errors
  }
  try {
    const d = await api.deviceDiag()
    if (d) diag.value = d
  } catch {
    // keep last known diag on transient errors
  }
}

const grouped = computed(() => {
  const map = new Map<string, Metric[]>()
  for (const m of metrics.value) {
    const list = map.get(m.category) ?? []
    list.push(m)
    map.set(m.category, list)
  }
  return map
})

function fmtUptime(ms: number): string {
  const s = Math.floor(ms / 1000)
  const h = Math.floor(s / 3600)
  const m = Math.floor((s % 3600) / 60)
  return h > 0 ? `${h}h ${m}m` : `${m}m ${s % 60}s`
}

function fmtBytes(n: number): string {
  if (n >= 1048576) return (n / 1048576).toFixed(1) + ' MB'
  if (n >= 1024)    return (n / 1024).toFixed(1) + ' KB'
  return n + ' B'
}

onMounted(() => { load(); timer = setInterval(load, 2000) })
onUnmounted(() => clearInterval(timer))
</script>

<template>
  <div class="space-y-6">
    <h1 class="text-2xl font-bold text-white">Dashboard</h1>

    <!-- Connection status -->
    <div class="rounded-xl border p-4"
         :class="status?.connected ? 'border-green-500/40 bg-green-500/5' : 'border-red-500/40 bg-red-500/5'">
      <div class="flex items-center gap-3">
        <span class="size-3 rounded-full"
              :class="status?.connected ? 'bg-green-400 shadow-[0_0_8px_#4ade80]' : 'bg-red-400'"></span>
        <span class="font-medium">
          {{ status?.connected ? 'Connected' : 'Disconnected' }}
        </span>
        <span v-if="status?.device_url" class="ml-auto text-xs text-slate-400 font-mono">
          {{ status.device_url }}
        </span>
      </div>
      <div class="mt-3 grid grid-cols-2 gap-2 text-xs text-slate-400">
        <div>
          <span class="text-slate-500">Window: </span>
          <span class="text-slate-300 font-mono truncate inline-block max-w-[180px] align-bottom">
            {{ status?.active_title || '—' }}
          </span>
        </div>
        <div>
          <span class="text-slate-500">Process: </span>
          <span class="text-slate-300 font-mono">{{ status?.active_process || '—' }}</span>
        </div>
        <div>
          <span class="text-slate-500">Profile: </span>
          <span class="text-slate-300">{{ status?.active_profile || '—' }}</span>
        </div>
      </div>
    </div>

    <!-- Device telemetry -->
    <div v-if="diag" class="space-y-2">
      <h2 class="text-xs font-semibold text-slate-400 uppercase tracking-wider px-1">Device</h2>
      <div class="rounded-xl border border-slate-700 bg-slate-800/50 overflow-hidden">
        <table class="w-full text-sm">
          <tbody>
            <tr class="border-b border-slate-700/60 hover:bg-slate-700/30 transition-colors">
              <td class="px-4 py-2.5 text-slate-300">Uptime</td>
              <td class="px-4 py-2.5 text-right font-mono font-semibold text-white tabular-nums">{{ fmtUptime(diag.uptime_ms) }}</td>
            </tr>
            <tr class="border-b border-slate-700/60 hover:bg-slate-700/30 transition-colors">
              <td class="px-4 py-2.5 text-slate-300">Wi-Fi IP</td>
              <td class="px-4 py-2.5 text-right font-mono font-semibold text-white tabular-nums">{{ diag.wifi.ip || '—' }}</td>
            </tr>
            <tr class="border-b border-slate-700/60 hover:bg-slate-700/30 transition-colors">
              <td class="px-4 py-2.5 text-slate-300">Wi-Fi RSSI</td>
              <td class="px-4 py-2.5 text-right font-mono font-semibold text-white tabular-nums">{{ diag.wifi.rssi }} dBm</td>
            </tr>
            <tr class="border-b border-slate-700/60 hover:bg-slate-700/30 transition-colors">
              <td class="px-4 py-2.5 text-slate-300">Reconnects</td>
              <td class="px-4 py-2.5 text-right font-mono font-semibold text-white tabular-nums">{{ diag.wifi.reconnects }}</td>
            </tr>
            <tr class="border-b border-slate-700/60 hover:bg-slate-700/30 transition-colors">
              <td class="px-4 py-2.5 text-slate-300">Free heap</td>
              <td class="px-4 py-2.5 text-right font-mono font-semibold text-white tabular-nums">{{ fmtBytes(diag.heap.free) }}</td>
            </tr>
            <tr class="border-b border-slate-700/60 hover:bg-slate-700/30 transition-colors">
              <td class="px-4 py-2.5 text-slate-300">Heap min</td>
              <td class="px-4 py-2.5 text-right font-mono font-semibold text-white tabular-nums">{{ fmtBytes(diag.heap.min) }}</td>
            </tr>
            <tr v-if="diag.heap.psram > 0" class="border-b border-slate-700/60 hover:bg-slate-700/30 transition-colors">
              <td class="px-4 py-2.5 text-slate-300">Free PSRAM</td>
              <td class="px-4 py-2.5 text-right font-mono font-semibold text-white tabular-nums">{{ fmtBytes(diag.heap.psram) }}</td>
            </tr>
            <tr class="border-b border-slate-700/60 hover:bg-slate-700/30 transition-colors">
              <td class="px-4 py-2.5 text-slate-300">HTTP ok / 503</td>
              <td class="px-4 py-2.5 text-right font-mono font-semibold text-white tabular-nums">{{ diag.http.ok }} / {{ diag.http['503_heap'] + diag.http['503_busy'] }}</td>
            </tr>
            <tr class="border-b border-slate-700/60 hover:bg-slate-700/30 transition-colors">
              <td class="px-4 py-2.5 text-slate-300">TCP opens/closes</td>
              <td class="px-4 py-2.5 text-right font-mono font-semibold text-white tabular-nums">{{ diag.tcp.opens }} / {{ diag.tcp.closes }}</td>
            </tr>
            <tr class="hover:bg-slate-700/30 transition-colors">
              <td class="px-4 py-2.5 text-slate-300">GW ping ok/fail</td>
              <td class="px-4 py-2.5 text-right font-mono font-semibold text-white tabular-nums">{{ diag.gw.ping_ok }} / {{ diag.gw.ping_fail }}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <!-- Metrics by category -->
    <div v-for="[category, items] in grouped" :key="category" class="space-y-2">
      <h2 class="text-xs font-semibold text-slate-400 uppercase tracking-wider px-1">{{ category }}</h2>
      <div class="rounded-xl border border-slate-700 bg-slate-800/50 overflow-hidden">
        <table class="w-full text-sm">
          <tbody>
            <tr v-for="m in items" :key="m.id"
                class="border-b border-slate-700/60 last:border-0 hover:bg-slate-700/30 transition-colors">
              <td class="px-4 py-2.5 text-slate-300">{{ m.label }}</td>
              <td class="px-4 py-2.5 font-mono text-slate-500 text-xs">{{ m.id }}</td>
              <td class="px-4 py-2.5 text-right font-mono font-semibold text-white tabular-nums">
                {{ m.value }}<span class="text-slate-400 font-normal ml-1 text-xs">{{ m.unit }}</span>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <p v-if="!metrics.length && !error" class="text-slate-500 text-sm">Loading metrics…</p>
    <p v-if="error" class="text-red-400 text-sm">{{ error }}</p>
  </div>
</template>

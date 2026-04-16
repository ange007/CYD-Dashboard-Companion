<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { api, type Config } from '@/api'

const cfg = ref<Config | null>(null)
const saving = ref(false)
const error = ref('')
const success = ref('')
const autoStart = ref(false)

async function load() {
  try {
    cfg.value = await api.config()
    autoStart.value = await api.getAutoStart()
  } catch (e) {
    error.value = String(e)
  }
}

async function toggleAutoStart() {
  try {
    await api.setAutoStart(autoStart.value)
  } catch (e) {
    error.value = String(e)
    autoStart.value = !autoStart.value // revert on error
  }
}

async function save() {
  if (!cfg.value) return
  saving.value = true
  error.value = ''
  success.value = ''
  try {
    await api.saveConfig(cfg.value)
    success.value = 'Saved!'
    setTimeout(() => success.value = '', 4000)
  } catch (e) {
    error.value = String(e)
  } finally {
    saving.value = false
  }
}

function addCommand() {
  cfg.value?.commands_allowlist.push('')
}

function removeCommand(i: number) {
  cfg.value?.commands_allowlist.splice(i, 1)
}

onMounted(load)
</script>

<template>
  <div class="space-y-6">
    <h1 class="text-2xl font-bold text-white">Settings</h1>

    <div v-if="cfg" class="space-y-5">

      <!-- Connection -->
      <section class="rounded-xl border border-slate-700 bg-slate-800/50 p-5 space-y-4">
        <h2 class="text-sm font-semibold text-slate-300 uppercase tracking-wider">Connection</h2>

        <div>
          <label class="block text-xs text-slate-400 mb-1">
            Device URL
            <span class="text-slate-500 ml-1">(leave empty for auto-discovery)</span>
          </label>
          <input
            v-model="cfg.device_url"
            placeholder="ws://192.168.1.100/ws"
            class="w-full bg-slate-900 border border-slate-600 rounded-lg px-3 py-2 text-white font-mono text-sm focus:outline-none focus:border-blue-500"
          />
        </div>

        <div>
          <label class="block text-xs text-slate-400 mb-1">mDNS hostname hint</label>
          <input
            v-model="cfg.mdns_host_hint"
            placeholder="cyd"
            class="w-full bg-slate-900 border border-slate-600 rounded-lg px-3 py-2 text-white font-mono text-sm focus:outline-none focus:border-blue-500"
          />
        </div>

        <div>
          <label class="block text-xs text-slate-400 mb-1">
            Push interval (ms)
            <span class="text-slate-500 ml-1">how often system info is pushed to device</span>
          </label>
          <input
            v-model.number="cfg.push_interval_ms"
            type="number" min="500" max="30000" step="500"
            class="w-full bg-slate-900 border border-slate-600 rounded-lg px-3 py-2 text-white text-sm focus:outline-none focus:border-blue-500"
          />
        </div>
      </section>

      <!-- Command allowlist -->
      <section class="rounded-xl border border-slate-700 bg-slate-800/50 p-5 space-y-3">
        <h2 class="text-sm font-semibold text-slate-300 uppercase tracking-wider">Command Allowlist</h2>
        <p class="text-xs text-slate-500">Only these named commands can be executed when requested by the device.</p>

        <div v-for="(cmd, i) in cfg.commands_allowlist" :key="i" class="flex gap-2">
          <input
            v-model="cfg.commands_allowlist[i]"
            placeholder="spotify:play_pause"
            class="flex-1 bg-slate-900 border border-slate-600 rounded-lg px-3 py-2 text-white font-mono text-sm focus:outline-none focus:border-blue-500"
          />
          <button @click="removeCommand(i)" class="px-3 py-2 text-red-400 hover:text-red-300 text-sm">✕</button>
        </div>

        <button
          @click="addCommand"
          class="w-full py-2 rounded-lg border border-dashed border-slate-600 text-slate-400 hover:border-blue-500 hover:text-blue-400 text-sm transition-colors"
        >
          + Add command
        </button>
      </section>

      <!-- Startup -->
      <section class="rounded-xl border border-slate-700 bg-slate-800/50 p-5 space-y-3">
        <h2 class="text-sm font-semibold text-slate-300 uppercase tracking-wider">Startup</h2>
        <label class="flex items-center gap-3 cursor-pointer select-none">
          <input
            type="checkbox"
            v-model="autoStart"
            @change="toggleAutoStart"
            class="w-4 h-4 rounded accent-blue-500"
          />
          <span class="text-sm text-slate-300">Start at login</span>
          <span class="text-xs text-slate-500">(launches minimised to system tray)</span>
        </label>
      </section>

      <!-- Save -->
      <div class="flex items-center gap-3">
        <button
          @click="save"
          :disabled="saving"
          class="px-6 py-2 bg-blue-600 hover:bg-blue-500 disabled:opacity-50 rounded-lg text-white font-medium text-sm transition-colors"
        >
          {{ saving ? 'Saving…' : 'Save settings' }}
        </button>
        <span v-if="success" class="text-green-400 text-sm">{{ success }}</span>
        <span v-if="error" class="text-red-400 text-sm">{{ error }}</span>
      </div>
    </div>
  </div>
</template>

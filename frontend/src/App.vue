<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import { RouterLink, RouterView } from 'vue-router'
import { api } from '@/api'

const connected = ref<boolean | null>(null)
const deviceUrl = ref('')
let statusTimer: ReturnType<typeof setInterval>

async function refreshStatus() {
  try {
    const st = await api.status()
    connected.value = st.connected
    deviceUrl.value = st.device_url ?? ''
  } catch {
    connected.value = false
  }
}

onMounted(() => { refreshStatus(); statusTimer = setInterval(refreshStatus, 2000) })
onUnmounted(() => clearInterval(statusTimer))
</script>

<template>
  <div class="flex h-screen bg-slate-900 text-white overflow-hidden">
    <!-- Sidebar -->
    <nav class="w-48 shrink-0 bg-slate-800/60 border-r border-slate-700 flex flex-col py-4 gap-1">
      <div class="px-4 pb-4 mb-2 border-b border-slate-700">
        <span class="text-sm font-semibold tracking-wide text-slate-300">CYD Companion</span>
        <!-- Connection status indicator -->
        <div class="mt-2 flex items-center gap-1.5">
          <span class="size-2 rounded-full shrink-0"
                :class="connected === null ? 'bg-slate-500'
                       : connected        ? 'bg-green-400 shadow-[0_0_6px_#4ade80]'
                       :                    'bg-red-400'"></span>
          <span class="text-xs truncate"
                :class="connected ? 'text-green-400' : 'text-slate-500'">
            {{ connected === null ? 'Checking…'
             : connected         ? (deviceUrl || 'Connected')
             :                     'Disconnected' }}
          </span>
        </div>
      </div>
      <RouterLink to="/"
        class="mx-2 px-3 py-2 rounded-lg text-sm text-slate-300 hover:bg-slate-700 hover:text-white transition-colors"
        active-class="bg-blue-600/30 text-blue-300 hover:bg-blue-600/40">
        Dashboard
      </RouterLink>
      <RouterLink to="/rules"
        class="mx-2 px-3 py-2 rounded-lg text-sm text-slate-300 hover:bg-slate-700 hover:text-white transition-colors"
        active-class="bg-blue-600/30 text-blue-300 hover:bg-blue-600/40">
        Profile Rules
      </RouterLink>
      <RouterLink to="/settings"
        class="mx-2 px-3 py-2 rounded-lg text-sm text-slate-300 hover:bg-slate-700 hover:text-white transition-colors"
        active-class="bg-blue-600/30 text-blue-300 hover:bg-blue-600/40">
        Settings
      </RouterLink>
    </nav>

    <!-- Main content -->
    <main class="flex-1 overflow-y-auto p-6">
      <RouterView />
    </main>
  </div>
</template>

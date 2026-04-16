<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { api, type Config, type FocusRule, type Profile } from '@/api'

const cfg = ref<Config | null>(null)
const profiles = ref<Profile[]>([])
const activeProfileId = ref('')
const saving = ref(false)
const error = ref('')
const success = ref('')

async function load() {
  try {
    cfg.value = await api.config()
  } catch (e) {
    error.value = String(e)
  }
  try {
    const p = await api.profiles()
    if (p.list?.length) {
      profiles.value = p.list
      activeProfileId.value = p.active_id
    }
  } catch {
    // device offline — keep whatever was cached; don't overwrite existing list
  }
}

function addRule() {
  cfg.value?.focus_rules.push({ match: '', profile_id: '' })
}

function removeRule(i: number) {
  cfg.value?.focus_rules.splice(i, 1)
}

async function save() {
  if (!cfg.value) return
  saving.value = true
  error.value = ''
  success.value = ''
  try {
    await api.saveConfig(cfg.value)
    success.value = 'Saved!'
    setTimeout(() => success.value = '', 2000)
  } catch (e) {
    error.value = String(e)
  } finally {
    saving.value = false
  }
}

async function switchProfile(id: string) {
  try {
    await api.switchProfile(id)
    activeProfileId.value = id
  } catch (e) {
    error.value = String(e)
  }
}

onMounted(load)
</script>

<template>
  <div class="space-y-6">
    <h1 class="text-2xl font-bold text-white">Profile Rules</h1>
    <p class="text-slate-400 text-sm">
      Auto-switch the active profile based on the focused window or process name.
      Match uses a regular expression against <code class="text-blue-400">process|title</code>.
    </p>

    <!-- Manual profile switch -->
    <div v-if="profiles.length" class="rounded-xl border border-slate-700 bg-slate-800/50 p-4">
      <p class="text-xs text-slate-400 mb-3 uppercase tracking-wider">Manual switch</p>
      <div class="flex flex-wrap gap-2">
        <button
          v-for="p in profiles" :key="p.id"
          @click="switchProfile(p.id)"
          class="px-3 py-1.5 rounded-lg text-sm font-medium transition-colors"
          :class="activeProfileId === p.id
            ? 'bg-blue-600 text-white'
            : 'bg-slate-700 text-slate-300 hover:bg-slate-600'"
        >
          {{ p.name }}
        </button>
      </div>
    </div>

    <!-- Rules editor -->
    <div v-if="cfg" class="space-y-3">
      <div
        v-for="(rule, i) in cfg.focus_rules" :key="i"
        class="rounded-xl border border-slate-700 bg-slate-800/50 p-4 space-y-3"
      >
        <div class="flex items-center justify-between">
          <span class="text-xs text-slate-400 uppercase tracking-wider">Rule {{ i + 1 }}</span>
          <button @click="removeRule(i)" class="text-red-400 hover:text-red-300 text-sm">Remove</button>
        </div>
        <div class="grid grid-cols-2 gap-3">
          <div>
            <label class="block text-xs text-slate-400 mb-1">Regex match</label>
            <input
              v-model="rule.match"
              placeholder="e\.g\. Code\.exe|cursor"
              class="w-full bg-slate-900 border border-slate-600 rounded-lg px-3 py-2 text-white text-sm font-mono focus:outline-none focus:border-blue-500"
            />
          </div>
          <div>
            <label class="block text-xs text-slate-400 mb-1">Profile</label>
            <select
              v-model="rule.profile_id"
              class="w-full bg-slate-900 border border-slate-600 rounded-lg px-3 py-2 text-white text-sm focus:outline-none focus:border-blue-500"
            >
              <option value="">— select —</option>
              <option v-for="p in profiles" :key="p.id" :value="p.id">{{ p.name }}</option>
            </select>
          </div>
        </div>
      </div>

      <button
        @click="addRule"
        class="w-full py-2 rounded-lg border border-dashed border-slate-600 text-slate-400 hover:border-blue-500 hover:text-blue-400 text-sm transition-colors"
      >
        + Add rule
      </button>

      <div class="flex items-center gap-3 pt-2">
        <button
          @click="save"
          :disabled="saving"
          class="px-6 py-2 bg-blue-600 hover:bg-blue-500 disabled:opacity-50 rounded-lg text-white font-medium text-sm transition-colors"
        >
          {{ saving ? 'Saving…' : 'Save rules' }}
        </button>
        <span v-if="success" class="text-green-400 text-sm">{{ success }}</span>
        <span v-if="error" class="text-red-400 text-sm">{{ error }}</span>
      </div>
    </div>
  </div>
</template>

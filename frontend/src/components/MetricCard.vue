<script setup lang="ts">
defineProps<{
  label: string
  value: string
  unit: string
  color: 'blue' | 'purple' | 'orange'
}>()

const colorMap = {
  blue:   { bg: 'bg-blue-500/10',   border: 'border-blue-500/30',   text: 'text-blue-400' },
  purple: { bg: 'bg-purple-500/10', border: 'border-purple-500/30', text: 'text-purple-400' },
  orange: { bg: 'bg-orange-500/10', border: 'border-orange-500/30', text: 'text-orange-400' },
}
</script>

<template>
  <div class="rounded-xl border p-4" :class="[colorMap[color].bg, colorMap[color].border]">
    <p class="text-xs uppercase tracking-wider mb-2" :class="colorMap[color].text">{{ label }}</p>
    <p class="text-3xl font-bold text-white tabular-nums">
      {{ value }}<span class="text-lg font-normal text-slate-400 ml-1">{{ unit }}</span>
    </p>
  </div>
</template>

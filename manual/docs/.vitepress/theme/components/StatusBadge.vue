<template>
  <span :style="styleObject">{{ label }}</span>
  <slot />
  </template>

<script setup lang="ts">
type Status = 'success' | 'warning' | 'danger' | 'info'

const props = defineProps<{
  status?: Status
  label?: string
}>()

const colorMap: Record<Status, string> = {
  success: '#10b981',
  warning: '#f59e0b',
  danger: '#ef4444',
  info: '#3b82f6'
}

const styleObject = {
  display: 'inline-block',
  padding: '2px 8px',
  borderRadius: '999px',
  fontSize: '12px',
  lineHeight: 1.6,
  color: '#fff',
  backgroundColor: colorMap[props.status ?? 'info']
}

const label = props.label ?? 'info'
</script>


<template>
  <div class="tabs">
    <div class="nav">
      <button
        v-for="(t,i) in tabs"
        :key="i"
        :class="['tab', i===activeIndex? 'active':'']"
        @click="setActive(i)"
        type="button"
      >{{ t }}</button>
    </div>
    <div class="panel">
      <slot :name="`panel-${activeIndex}`" />
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
const props = defineProps<{ tabs: string[]; activeIndex?: number }>()
const active = ref(props.activeIndex ?? 0)
function setActive(i: number) { active.value = i }
const activeIndex = active
const tabs = props.tabs
</script>

<style scoped>
.tabs { display: grid; gap: 12px; }
.nav { display: flex; gap: 8px; flex-wrap: wrap; }
.tab {
  padding: 6px 12px;
  border-radius: 6px;
  border: 1px solid var(--vp-c-divider);
  background: var(--vp-c-bg-soft);
  color: var(--vp-c-text-1);
}
.tab.active { border-color: var(--vp-c-brand-1); color: var(--vp-c-brand-1); }
.panel { border: 1px solid var(--vp-c-divider); border-radius: 8px; padding: 12px; }
</style>

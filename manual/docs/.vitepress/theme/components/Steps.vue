<template>
  <div class="steps">
    <div
      v-for="(s, i) in steps"
      :key="i"
      class="step"
      :class="{ active: i === activeIndex }"
    >
      <div class="index">{{ i + 1 }}</div>
      <div class="content">
        <div class="title">{{ s.title }}</div>
        <div v-if="s.description" class="desc">{{ s.description }}</div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
interface StepItem {
  title: string
  description?: string
}

const props = defineProps<{
  steps: StepItem[]
  activeIndex?: number
}>()

const activeIndex = props.activeIndex ?? 0
const steps = props.steps
</script>

<style scoped>
.steps {
  display: flex;
  gap: 12px;
  align-items: flex-start;
}
.step {
  display: flex;
  gap: 8px;
  padding: 8px 12px;
  border-radius: 8px;
  border: 1px solid var(--vp-c-divider);
}
.step.active {
  border-color: var(--vp-c-brand-1);
  background: var(--vp-c-bg-soft);
}
.index {
  width: 22px;
  height: 22px;
  border-radius: 999px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 12px;
  color: #fff;
  background: var(--vp-c-brand-1);
}
.title {
  font-weight: 600;
}
.desc {
  font-size: 12px;
  color: var(--vp-c-text-2);
}
</style>


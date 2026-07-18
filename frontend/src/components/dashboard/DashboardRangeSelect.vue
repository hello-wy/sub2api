<template>
  <div class="dashboard-range-select">
    <Icon name="calendar" size="sm" aria-hidden="true" />
    <Select
      :model-value="modelValue"
      :options="options"
      :searchable="false"
      aria-label="统计时间范围"
      @update:model-value="handleUpdate"
      @change="handleChange"
    />
  </div>
</template>

<script lang="ts">
export type DashboardTimeRange = '24h' | '7d' | '30d'
</script>

<script setup lang="ts">
import Select from '@/components/common/Select.vue'
import Icon from '@/components/icons/Icon.vue'

defineProps<{
  modelValue: DashboardTimeRange
  options: Array<{ value: DashboardTimeRange; label: string }>
}>()

const emit = defineEmits<{
  (event: 'update:modelValue', value: DashboardTimeRange): void
  (event: 'change', value: DashboardTimeRange): void
}>()

function isDashboardTimeRange(value: unknown): value is DashboardTimeRange {
  return value === '24h' || value === '7d' || value === '30d'
}

function handleUpdate(value: unknown) {
  if (isDashboardTimeRange(value)) emit('update:modelValue', value)
}

function handleChange(value: unknown) {
  if (isDashboardTimeRange(value)) emit('change', value)
}
</script>

<style scoped>
.dashboard-range-select {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  min-width: 154px;
  height: 40px;
  border: 1px solid var(--dashboard-line, var(--line, #dce7f5));
  border-radius: 10px;
  background: var(--dashboard-surface, #fff);
  padding: 0 7px 0 12px;
  color: #53627d;
  box-shadow: 0 4px 12px rgba(28, 56, 112, 0.04);
  font-size: 13px;
  font-weight: 650;
  transition: border-color 0.18s ease, box-shadow 0.18s ease;
}

.dashboard-range-select:focus-within {
  border-color: #8eb8ff;
  box-shadow: 0 0 0 3px rgba(47, 123, 255, 0.12), 0 5px 14px rgba(28, 56, 112, 0.06);
}

.dashboard-range-select > :deep(svg) {
  flex: 0 0 auto;
  color: #3979ec;
}

.dashboard-range-select > :deep(div) {
  flex: 1;
  min-width: 0;
}

.dashboard-range-select :deep(.select-trigger) {
  min-height: 38px;
  border: 0;
  border-radius: 7px;
  background: transparent;
  padding: 0 3px 0 0;
  color: inherit;
  box-shadow: none;
  font: inherit;
}

.dashboard-range-select :deep(.select-trigger:hover),
.dashboard-range-select :deep(.select-trigger-open),
.dashboard-range-select :deep(.select-trigger:focus) {
  border: 0;
  background: transparent;
  box-shadow: none;
}

.dashboard-range-select :deep(.select-icon) {
  color: #7183a2;
}

@media (max-width: 760px) {
  .dashboard-range-select {
    flex: 1;
  }
}
</style>

<style>
.dark .dashboard-range-select {
  border-color: rgba(152, 180, 224, 0.16);
  background: #0e192b;
  color: #dbe6f8;
  box-shadow: 0 5px 16px rgba(0, 0, 0, 0.16);
}
</style>

<template>
  <nav ref="navigation" class="business-panel-navigation" aria-label="经营分析子页">
    <div class="business-panel-navigation__track" :style="{ '--panel-count': businessTabs.length, '--panel-index': activeIndex }">
      <span class="business-panel-navigation__slider" aria-hidden="true"></span>
      <button
        v-for="item in businessTabs"
        :key="item.key"
        type="button"
        :class="{ 'is-active': modelValue === item.key }"
        :aria-current="modelValue === item.key ? 'page' : undefined"
        @click="$emit('update:modelValue', item.key)"
      >{{ item.label }}</button>
    </div>
  </nav>
</template>

<script lang="ts">
export const businessTabs = [
  { key: 'overview', label: '经营总览' },
  { key: 'profit', label: '盈利明细' },
  { key: 'ledger', label: '收支与成本台账' },
  { key: 'benefits', label: '福利与优惠' },
  { key: 'reconcile', label: '核对与配置' },
] as const

export type BusinessTab = typeof businessTabs[number]['key']
</script>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'

const props = defineProps<{ modelValue: BusinessTab }>()
defineEmits<{ 'update:modelValue': [value: BusinessTab] }>()
const activeIndex = computed(() => Math.max(0, businessTabs.findIndex(item => item.key === props.modelValue)))
const navigation = ref<HTMLElement | null>(null)

watch(activeIndex, () => {
  const container = navigation.value
  const selected = container?.querySelector<HTMLButtonElement>('[aria-current="page"]')
  if (container && selected) {
    container.scrollLeft = selected.offsetLeft - (container.clientWidth - selected.offsetWidth) / 2
  }
}, { flush: 'post' })
</script>

<style scoped>
.business-panel-navigation {
  @apply min-w-0 max-w-full overflow-x-auto;
  scrollbar-width: thin;
}

.business-panel-navigation__track {
  @apply relative isolate grid auto-cols-fr grid-flow-col rounded-xl bg-gray-200/70 p-1 dark:bg-dark-900;
  width: max-content;
  min-width: 100%;
}

.business-panel-navigation__slider {
  @apply pointer-events-none absolute bottom-1 left-1 top-1 rounded-lg bg-white shadow-sm transition-transform duration-200 ease-out dark:bg-dark-700;
  width: calc((100% - 8px) / var(--panel-count));
  transform: translateX(calc(var(--panel-index) * 100%));
}

.business-panel-navigation button {
  @apply relative z-10 flex h-9 items-center justify-center whitespace-nowrap rounded-lg px-3 text-[13px] font-medium text-gray-500 transition-colors hover:text-gray-900 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-primary-500 dark:text-gray-400 dark:hover:text-white;
}

.business-panel-navigation button.is-active {
  @apply text-primary-700 dark:text-primary-300;
}

@media (prefers-reduced-motion: reduce) {
  .business-panel-navigation__slider,
  .business-panel-navigation button { transition: none; }
}
</style>

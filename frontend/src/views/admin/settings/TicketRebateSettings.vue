<template>
  <section class="border-t border-gray-100 pt-6 dark:border-dark-700">
    <div class="flex items-center justify-between gap-4">
      <h3 class="text-base font-semibold text-gray-900 dark:text-white">抽奖券返利设置</h3>
      <div class="flex items-center gap-2 text-sm text-gray-700 dark:text-gray-200">
        <span>{{ enabled ? '开启' : '关闭' }}</span>
        <Toggle :model-value="enabled" aria-label="抽奖券返利" @update:model-value="emit('update:enabled', $event)" />
      </div>
    </div>
    <div class="mt-5 space-y-3">
      <div class="grid grid-cols-[minmax(0,1fr)_minmax(0,1fr)_32px] gap-3 text-xs text-gray-500 dark:text-gray-400">
        <span>消费金额</span><span>赠送抽奖券</span><span></span>
      </div>
      <div v-for="(rule, index) in rules" :key="index" class="grid grid-cols-[minmax(0,1fr)_minmax(0,1fr)_32px] items-center gap-3">
        <div class="relative min-w-0">
          <span class="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-gray-500">$</span>
          <input class="input w-full pl-7" type="number" min="0.01" step="0.01" :value="rule.amount_threshold" :aria-label="`第 ${index + 1} 档消费金额`" @input="updateRule(index, 'amount_threshold', $event)" />
        </div>
        <input class="input min-w-0 w-full" type="number" min="1" step="1" :value="rule.ticket_count" :aria-label="`第 ${index + 1} 档抽奖券数量`" @input="updateRule(index, 'ticket_count', $event)" />
        <button type="button" class="btn btn-ghost h-8 w-8 p-0" title="删除规则" aria-label="删除规则" :disabled="rules.length <= 1" @click="removeRule(index)"><Icon name="trash" size="sm" /></button>
      </div>
    </div>
    <button type="button" class="btn btn-secondary btn-sm mt-4" @click="addRule"><Icon name="plus" size="xs" />添加规则</button>
  </section>
</template>

<script setup lang="ts">
import Icon from '@/components/icons/Icon.vue'
import Toggle from '@/components/common/Toggle.vue'

export type TicketRebateRule = { amount_threshold: number; ticket_count: number }
const props = defineProps<{ enabled: boolean; rules: TicketRebateRule[] }>()
const emit = defineEmits<{
  'update:enabled': [value: boolean]
  'update:rules': [value: TicketRebateRule[]]
}>()

function updateRule(index: number, key: keyof TicketRebateRule, event: Event): void {
  const next = props.rules.map(rule => ({ ...rule }))
  next[index][key] = Number((event.target as HTMLInputElement).value)
  emit('update:rules', next)
}

function addRule(): void {
  const nextThreshold = Math.max(...props.rules.map(rule => rule.amount_threshold)) + 5
  emit('update:rules', [...props.rules, { amount_threshold: nextThreshold, ticket_count: 1 }])
}

function removeRule(index: number): void {
  emit('update:rules', props.rules.filter((_, row) => row !== index))
}
</script>

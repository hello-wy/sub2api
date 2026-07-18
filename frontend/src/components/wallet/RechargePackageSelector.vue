<template>
  <div class="space-y-4">
    <div class="flex flex-wrap items-end justify-between gap-2">
      <div>
        <h3 class="text-sm font-semibold text-gray-900 dark:text-white">
          {{ t('wallet.rechargePackages') }}
        </h3>
        <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
          {{ t('wallet.rechargePackagesHint') }}
        </p>
      </div>
      <span class="rounded-md bg-primary-50 px-2.5 py-1 text-xs font-medium text-primary-700 dark:bg-primary-900/30 dark:text-primary-300">
        {{ t('wallet.conversionRate') }} {{ formatMultiplier(multiplier) }}x
      </span>
    </div>

    <div class="grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
      <button
        v-for="item in packageOptions"
        :key="item.credit"
        type="button"
        :class="[
          'group relative min-h-[168px] overflow-hidden rounded-lg border p-4 text-left transition duration-200',
          isSelected(item.payment)
            ? 'border-primary-500 bg-primary-50/70 shadow-[0_10px_28px_rgba(37,99,235,0.12)] dark:border-primary-400 dark:bg-primary-950/35'
            : 'border-gray-200 bg-white hover:-translate-y-0.5 hover:border-primary-300 hover:shadow-md dark:border-dark-600 dark:bg-dark-800 dark:hover:border-primary-600',
        ]"
        @click="selectPackage(item.payment)"
      >
        <span class="text-2xl font-bold text-gray-950 dark:text-white">${{ formatCredit(item.credit) }}</span>
        <span class="ml-2 text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('wallet.generalBalance') }}</span>
        <span class="mt-2 block text-xs font-medium uppercase text-primary-600 dark:text-primary-400">
          {{ t('wallet.payAsYouGo') }}
        </span>
        <dl class="mt-4 grid grid-cols-2 gap-x-3 gap-y-2 border-t border-gray-100 pt-3 text-xs dark:border-dark-600">
          <div>
            <dt class="text-gray-400 dark:text-gray-500">{{ t('payment.paymentAmount') }}</dt>
            <dd class="mt-0.5 font-semibold text-gray-800 dark:text-gray-100">{{ formatAmount(item.payment) }}</dd>
          </div>
          <div>
            <dt class="text-gray-400 dark:text-gray-500">{{ t('payment.creditedBalance') }}</dt>
            <dd class="mt-0.5 font-semibold text-emerald-600 dark:text-emerald-400">${{ item.credit.toFixed(2) }}</dd>
          </div>
        </dl>
        <span v-if="isSelected(item.payment)" class="absolute right-3 top-3 flex h-5 w-5 items-center justify-center rounded-full bg-primary-600 text-white">
          <Icon name="check" size="xs" />
        </span>
      </button>

      <button
        type="button"
        :class="[
          'group flex min-h-[168px] flex-col items-center justify-center rounded-lg border border-dashed p-4 text-center transition duration-200',
          customOpen
            ? 'border-primary-500 bg-primary-50/70 dark:border-primary-400 dark:bg-primary-950/35'
            : 'border-gray-300 bg-gray-50/60 hover:border-primary-400 hover:bg-primary-50/50 dark:border-dark-500 dark:bg-dark-800/60 dark:hover:border-primary-600 dark:hover:bg-primary-950/25',
        ]"
        @click="openCustom"
      >
        <span class="flex h-10 w-10 items-center justify-center rounded-full bg-white text-primary-600 shadow-sm dark:bg-dark-700 dark:text-primary-400">
          <Icon name="plus" size="md" />
        </span>
        <span class="mt-3 text-sm font-semibold text-gray-900 dark:text-white">{{ t('wallet.customRecharge') }}</span>
        <span class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ t('wallet.customRechargeHint') }}</span>
      </button>
    </div>

    <div v-if="customOpen" class="rounded-lg border border-primary-200 bg-primary-50/50 p-4 dark:border-primary-800 dark:bg-primary-950/20">
      <label for="custom-wallet-credit" class="text-sm font-medium text-gray-800 dark:text-gray-200">
        {{ t('wallet.customCreditAmount') }}
      </label>
      <div class="mt-2 grid gap-3 sm:grid-cols-[minmax(0,1fr)_auto] sm:items-center">
        <div class="relative">
          <span class="absolute left-3 top-1/2 -translate-y-1/2 text-gray-400">$</span>
          <input
            id="custom-wallet-credit"
            :value="customText"
            type="text"
            inputmode="decimal"
            class="input w-full py-3 pl-8"
            :placeholder="t('wallet.customCreditPlaceholder')"
            @input="handleCustomInput"
          />
        </div>
        <div class="min-w-[160px] text-sm text-gray-500 dark:text-gray-400">
          {{ t('wallet.estimatedPayment') }}
          <strong class="ml-1 text-gray-900 dark:text-white">{{ customPaymentLabel }}</strong>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'

const props = withDefaults(defineProps<{
  modelValue: number | null
  credits?: number[]
  multiplier: number
  min?: number
  max?: number
  formatAmount: (value: number) => string
}>(), {
  credits: () => [10, 30, 50, 100, 200],
  min: 0,
  max: 0,
})

const emit = defineEmits<{
  'update:modelValue': [value: number | null]
}>()

const { t } = useI18n()
const customOpen = ref(false)
const customText = ref('')
const AMOUNT_PATTERN = /^\d*(\.\d{0,2})?$/

const safeMultiplier = computed(() => props.multiplier > 0 ? props.multiplier : 1)
const packageOptions = computed(() => props.credits
  .map((credit) => ({
    credit,
    payment: Math.round((credit / safeMultiplier.value) * 100) / 100,
  }))
  .filter(({ payment }) => (props.min <= 0 || payment >= props.min) && (props.max <= 0 || payment <= props.max)))

const customPayment = computed(() => {
  const credit = Number(customText.value)
  if (!Number.isFinite(credit) || credit <= 0) return 0
  return Math.round((credit / safeMultiplier.value) * 100) / 100
})

const customPaymentLabel = computed(() => customPayment.value > 0 ? props.formatAmount(customPayment.value) : '--')

function formatCredit(value: number): string {
  return Number.isInteger(value) ? String(value) : value.toFixed(2)
}

function formatMultiplier(value: number): string {
  return Number.isInteger(value) ? value.toFixed(1) : String(Number(value.toFixed(2)))
}

function isSelected(payment: number): boolean {
  return !customOpen.value && props.modelValue !== null && Math.abs(props.modelValue - payment) < 0.001
}

function selectPackage(payment: number) {
  customOpen.value = false
  customText.value = ''
  emit('update:modelValue', payment)
}

function openCustom() {
  customOpen.value = true
  if (customPayment.value <= 0) emit('update:modelValue', null)
}

function handleCustomInput(event: Event) {
  const input = event.target as HTMLInputElement
  const value = input.value
  if (!AMOUNT_PATTERN.test(value)) {
    input.value = customText.value
    return
  }
  customText.value = value
  emit('update:modelValue', customPayment.value > 0 ? customPayment.value : null)
}

watch(() => props.modelValue, (value) => {
  if (value === null || !customOpen.value) return
  const credit = Math.round((value * safeMultiplier.value) * 100) / 100
  if (Math.abs(credit - Number(customText.value)) > 0.001) customText.value = String(credit)
})
</script>

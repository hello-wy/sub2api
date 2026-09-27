<template>
  <div>
    <Select :model-value="modelValue" :options="options" :aria-label="label" :placeholder="label" searchable remote :clearable="!selectedIds" :loading="loading" @search="search" @update:model-value="choose($event)" />
    <p v-if="error" role="alert" class="mt-1 text-xs text-red-600">{{ error }}</p>
  </div>
</template>
<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import Select from '@/components/common/Select.vue'
import { businessAPI, type BusinessEntity } from '@/api/admin/business'
import { ledgerError } from '@/utils/business-ledger'
const props = defineProps<{ modelValue: number; kind: string; label: string; selectedIds?: number[]; disabledReasons?: Record<number, string>; optionHints?: Record<number, string> }>()
const emit = defineEmits<{ 'update:modelValue': [value: number]; select: [entity: BusinessEntity] }>()
const entities = ref<BusinessEntity[]>([])
const options = computed(() => entities.value.map(entity => {
  const selected = props.selectedIds?.includes(entity.id)
  const reason = selected ? '本次已选' : props.disabledReasons?.[entity.id]
  const hint = reason || props.optionHints?.[entity.id]
  return { value: entity.id, label: `${entity.name} · #${entity.id}${hint ? ' · ' + hint : ''}`, disabled: !!reason }
}))
const loading = ref(false), error = ref('')
let sequence = 0
async function search(q = '') {
  const request = ++sequence; loading.value = true; error.value = ''
  try { const data = await businessAPI.entities(props.kind, q); if (request === sequence) entities.value = data }
  catch (e) { if (request === sequence) error.value = ledgerError(e) }
  finally { if (request === sequence) loading.value = false }
}
function choose(value: string | number | boolean | null) {
  const id = Number(value) || 0
  if (props.selectedIds?.includes(id) || props.disabledReasons?.[id]) return
  emit('update:modelValue', id)
  const entity = entities.value.find(item => item.id === id)
  if (entity) emit('select', entity)
}
onMounted(() => search())
</script>

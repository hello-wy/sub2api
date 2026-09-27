<template>
  <div>
    <Select :model-value="modelValue" :options="options" :aria-label="label" :placeholder="label" searchable remote clearable :loading="loading" @search="search" @update:model-value="choose($event)" />
    <p v-if="error" role="alert" class="mt-1 text-xs text-red-600">{{ error }}</p>
  </div>
</template>
<script setup lang="ts">
import { onMounted, ref } from 'vue'
import Select from '@/components/common/Select.vue'
import { businessAPI, type BusinessEntity } from '@/api/admin/business'
import { ledgerError } from '@/utils/business-ledger'
const props = defineProps<{ modelValue: number; kind: string; label: string }>()
const emit = defineEmits<{ 'update:modelValue': [value: number]; select: [entity: BusinessEntity] }>()
const entities = ref<BusinessEntity[]>([])
const options = ref<{ value: number; label: string }[]>([])
const loading = ref(false), error = ref('')
let sequence = 0
async function search(q = '') {
  const request = ++sequence; loading.value = true; error.value = ''
  try { const data = await businessAPI.entities(props.kind, q); if (request === sequence) { entities.value = data; options.value = data.map(v => ({ value: v.id, label: `${v.name} · #${v.id}` })) } }
  catch (e) { if (request === sequence) error.value = ledgerError(e) }
  finally { if (request === sequence) loading.value = false }
}
function choose(value: string | number | boolean | null) {
  const id = Number(value) || 0
  emit('update:modelValue', id)
  const entity = entities.value.find(item => item.id === id)
  if (entity) emit('select', entity)
}
onMounted(() => search())
</script>

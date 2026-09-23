<template>
  <button type="button" class="badge badge-danger text-xs" :title="t('accountModelMismatch.viewDetails')" @click="emit('click')">
    {{ t('accountModelMismatch.label') }}
  </button>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
const { t } = useI18n()
const emit = defineEmits<{ click: [] }>()
</script>

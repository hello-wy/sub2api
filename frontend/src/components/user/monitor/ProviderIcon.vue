<template>
  <PlatformIcon :platform="provider" colored :style="{ width: size + 'px', height: size + 'px' }" aria-hidden="true" />
</template>

<script setup lang="ts">
import PlatformIcon from '@/components/common/PlatformIcon.vue'

withDefaults(defineProps<{
  provider: string
  size?: number
}>(), { size: 20 })
</script>

<template>
  <div class="flex h-full min-h-0 min-w-0 flex-col">
    <div :class="['min-h-0 min-w-0 flex-1 overflow-y-auto', contentClass]">
      <slot />
    </div>
  </div>
</template>

<script setup lang="ts">
withDefaults(defineProps<{
  contentClass?: string
}>(), {
  contentClass: 'p-4 md:p-6 lg:p-8'
})
</script>

<template>
  <section class="space-y-4" aria-label="本页鹈鹕图像预览">
    <div class="flex flex-wrap items-center justify-between gap-3">
      <button
        type="button"
        class="btn btn-secondary"
        :disabled="disabled || (!imageRecords.length && !open)"
        :aria-expanded="open"
        data-testid="toggle-test-image-gallery"
        @click="emit('update:open', !open)"
      >
        {{ open ? '收起本页预览' : `显示本页全部预览（${imageRecords.length}）` }}
      </button>
      <p v-if="open" class="text-xs text-gray-500">本页 {{ imageRecords.length }} 个图像结果 · 点击图像放大查看</p>
    </div>

    <div v-if="open" data-testid="test-image-gallery">
      <div v-if="imageRecords.length" class="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3 2xl:grid-cols-4">
        <article
          v-for="record in imageRecords"
          :key="record.id"
          class="min-w-0 overflow-hidden rounded-xl border border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-800"
          :data-record-id="record.id"
        >
          <button
            type="button"
            class="test-gallery-preview block h-64 w-full overflow-hidden text-left focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-primary-500 disabled:cursor-default"
            :aria-label="`放大查看账号 #${record.account_id} 的图像，记录 #${record.id}`"
            :disabled="isPending(record.status)"
            @click="emit('detail', record.id)"
          >
            <span v-if="isPending(record.status)" class="flex h-full items-center justify-center bg-gray-50 p-6 text-sm text-gray-500 dark:bg-dark-900" role="status">
              {{ record.status === 'queued' ? '正在排队生成图像…' : '正在生成图像…' }}
            </span>
            <TestGeneratedImage
              v-else
              :source="record.result_image"
              :record-id="record.id"
              :public-view="publicView"
            />
          </button>
          <div class="space-y-1.5 border-t border-gray-100 px-4 py-3 text-sm dark:border-dark-700">
            <p class="font-medium text-gray-900 dark:text-gray-100">账号 #{{ record.account_id }}</p>
            <p class="truncate text-gray-600 dark:text-gray-300" :title="record.model || '默认模型'">{{ record.model || '默认模型' }}</p>
            <time :datetime="record.created_at" class="block text-xs text-gray-500">{{ testTime(record.created_at) }}</time>
          </div>
        </article>
      </div>
      <p v-else class="rounded-xl border border-dashed border-gray-200 py-12 text-center text-sm text-gray-500 dark:border-dark-700">本页暂无鹈鹕图像结果</p>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { PublicTestRecord } from '@/api/intelligentTests'
import TestGeneratedImage from './TestGeneratedImage.vue'
import { isPending, testTime } from './display'

const props = defineProps<{ records: PublicTestRecord[]; open: boolean; publicView?: boolean; disabled?: boolean }>()
const emit = defineEmits<{ 'update:open': [value: boolean]; detail: [id: number] }>()
const imageRecords = computed(() => props.records.filter(record => record.test_type === 'pelican'))
</script>

<style scoped>
.test-gallery-preview :deep(img) {
  max-height: 100%;
}
</style>

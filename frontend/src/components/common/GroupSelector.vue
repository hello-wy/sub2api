<template>
  <div>
    <label class="input-label">
      {{ label ?? t('admin.users.groups') }}
      <span class="font-normal text-gray-400">{{ t('common.selectedCount', { count: modelValue.length }) }}</span>
    </label>
    <div
      v-if="tagSummaries.length > 0"
      class="rounded-t-lg border border-b-0 border-gray-200 bg-gray-50 px-3 py-2 dark:border-dark-600 dark:bg-dark-800"
    >
      <div class="mb-1.5 text-xs font-medium text-gray-500 dark:text-gray-400">
        {{ t('common.groupTags') }}
      </div>
      <div class="flex flex-wrap gap-1.5">
        <button
          v-for="summary in tagSummaries"
          :key="summary.tag"
          type="button"
          :aria-pressed="summary.allSelected"
          :data-group-tag="summary.tag"
          :class="[
            'inline-flex max-w-full items-center gap-1 rounded border px-2 py-1 text-xs font-medium transition-colors',
            summary.allSelected
              ? 'border-primary-500 bg-primary-50 text-primary-700 dark:bg-primary-900/30 dark:text-primary-300'
              : 'border-gray-200 bg-white text-gray-600 hover:border-primary-300 hover:text-primary-600 dark:border-dark-600 dark:bg-dark-700 dark:text-gray-300 dark:hover:border-primary-600'
          ]"
          @click="toggleTag(summary.ids, summary.allSelected)"
        >
          <span class="truncate">{{ summary.tag }}</span>
          <span class="shrink-0 text-[11px] opacity-70">
            {{ t('common.groupTagSelection', { selected: summary.selected, total: summary.ids.length }) }}
          </span>
        </button>
      </div>
    </div>
    <div
      v-if="isSearchable"
      :class="[
        'flex items-center gap-2 border border-b-0 border-gray-200 bg-gray-50 px-3 py-2 dark:border-dark-600 dark:bg-dark-800',
        tagSummaries.length === 0 ? 'rounded-t-lg' : ''
      ]"
    >
      <Icon name="search" size="sm" class="shrink-0 text-gray-400" />
      <input
        v-model="searchText"
        type="text"
        :placeholder="t('common.searchPlaceholder')"
        class="flex-1 bg-transparent text-sm text-gray-900 placeholder:text-gray-400 focus:outline-none dark:text-gray-100 dark:placeholder:text-dark-400"
      />
    </div>
    <div
      :class="[
        'grid max-h-32 grid-cols-1 gap-1 overflow-y-auto p-2 sm:grid-cols-2',
        isSearchable || tagSummaries.length > 0
          ? 'rounded-b-lg border border-t-0 border-gray-200 bg-gray-50 dark:border-dark-600 dark:bg-dark-800'
          : 'rounded-lg border border-gray-200 bg-gray-50 dark:border-dark-600 dark:bg-dark-800'
      ]"
    >
      <label
        v-for="group in filteredGroups"
        :key="group.id"
        class="flex cursor-pointer items-center gap-2 rounded px-2 py-1.5 transition-colors hover:bg-white dark:hover:bg-dark-700"
        :title="group.rate_multiplier == null ? group.name : t('admin.groups.rateAndAccounts', { rate: group.rate_multiplier, count: group.account_count || 0 })"
      >
        <input
          type="checkbox"
          :value="group.id"
          :checked="modelValue.includes(group.id)"
          @change="handleChange(group.id, ($event.target as HTMLInputElement).checked)"
          class="h-3.5 w-3.5 shrink-0 rounded border-gray-300 text-primary-500 focus:ring-primary-500 dark:border-dark-500"
        />
        <GroupBadge
          :name="group.name"
          :platform="group.platform"
          :subscription-type="group.subscription_type || undefined"
          :rate-multiplier="group.rate_multiplier == null ? undefined : group.rate_multiplier"
          class="min-w-0 flex-1"
        />
        <span
          v-if="group.tag"
          class="max-w-20 shrink-0 truncate rounded bg-gray-200 px-1.5 py-0.5 text-[10px] text-gray-600 dark:bg-dark-600 dark:text-gray-300"
        >
          {{ group.tag }}
        </span>
        <span class="shrink-0 text-xs text-gray-400">{{ group.account_count || 0 }}</span>
      </label>
      <div
        v-if="filteredGroups.length === 0"
        class="py-2 text-center text-sm text-gray-500 dark:text-gray-400 sm:col-span-2"
      >
        {{ t('common.noGroupsAvailable') }}
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import GroupBadge from './GroupBadge.vue'
import Icon from '@/components/icons/Icon.vue'
import type { Group, GroupPlatform } from '@/types'
import { useAuthStore } from '@/stores'

const { t } = useI18n()
const authStore = useAuthStore()

interface Props {
  modelValue: number[]
  groups: (Group & { account_count?: number })[]
  /** Field label; defaults to the generic "Groups". */
  label?: string
  platform?: GroupPlatform // Optional platform filter
  mixedScheduling?: boolean // For antigravity accounts: allow anthropic/gemini groups
  searchable?: boolean | 'auto'
}

const props = withDefaults(defineProps<Props>(), {
  searchable: 'auto'
})
const emit = defineEmits<{
  'update:modelValue': [value: number[]]
}>()

const searchText = ref('')

const isSearchable = computed(() => {
  if (props.searchable === 'auto') return props.groups.length > 5
  return props.searchable
})

// Filter by account compatibility before tag operations so a tag never selects
// groups that the current account cannot use.
const eligibleGroups = computed(() => {
  let result = authStore.isSimpleMode
    ? props.groups.filter((g) => g.platform !== 'composite')
    : props.groups
  if (props.platform) {
    // antigravity 账户启用混合调度后，可选择 anthropic/gemini 分组
    if (props.platform === 'antigravity' && props.mixedScheduling) {
      result = result.filter(
        (g) => g.platform === 'antigravity' || g.platform === 'anthropic' || g.platform === 'gemini' || g.platform === 'composite'
      )
    } else {
      // 默认：只能选择同 platform 的分组；composite 分组可接收任意具体平台账号
      result = result.filter((g) => g.platform === props.platform || g.platform === 'composite')
    }
  }
  return result
})

const filteredGroups = computed(() => {
  let result = eligibleGroups.value
  if (isSearchable.value && searchText.value) {
    const q = searchText.value.toLowerCase()
    result = result.filter(
      (g) =>
        g.name.toLowerCase().includes(q) ||
        g.description?.toLowerCase().includes(q) ||
        g.tag?.toLowerCase().includes(q)
    )
  }
  return result
})

const tagSummaries = computed(() => {
  const groupsByTag = new Map<string, number[]>()
  for (const group of eligibleGroups.value) {
    const tag = group.tag?.trim()
    if (!tag) continue
    const ids = groupsByTag.get(tag) || []
    ids.push(group.id)
    groupsByTag.set(tag, ids)
  }

  return [...groupsByTag.entries()]
    .map(([tag, ids]) => {
      const selected = ids.filter((id) => props.modelValue.includes(id)).length
      return { tag, ids, selected, allSelected: selected === ids.length }
    })
    .sort((a, b) => a.tag.localeCompare(b.tag))
})

const toggleTag = (ids: number[], allSelected: boolean) => {
  const selected = new Set(props.modelValue)
  ids.forEach((id) => (allSelected ? selected.delete(id) : selected.add(id)))
  emit('update:modelValue', [...selected])
}

watch(
  () => [authStore.isSimpleMode, props.groups, props.modelValue] as const,
  () => {
    if (!authStore.isSimpleMode || props.groups.length === 0) return
    const visibleIDs = new Set(props.groups.filter((group) => group.platform !== 'composite').map((group) => group.id))
    const cleaned = props.modelValue.filter((id) => visibleIDs.has(id))
    if (cleaned.length !== props.modelValue.length) emit('update:modelValue', cleaned)
  },
  { immediate: true, deep: true }
)

const handleChange = (groupId: number, checked: boolean) => {
  const newValue = checked
    ? [...props.modelValue, groupId]
    : props.modelValue.filter((id) => id !== groupId)
  emit('update:modelValue', newValue)
}
</script>

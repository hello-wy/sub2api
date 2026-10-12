<template>
  <section class="border-t border-gray-100 pt-6 dark:border-dark-700">
    <div class="flex items-center justify-between gap-4">
      <h3 class="text-base font-semibold text-gray-900 dark:text-white">{{ text('排除用户', 'Excluded users') }}</h3>
      <button type="button" class="btn btn-secondary btn-sm" data-testid="welfare-exclusions-open" @click="openSearch">
        {{ text('搜索用户', 'Search users') }}
      </button>
    </div>
    <p class="mt-2 text-xs text-gray-500 dark:text-gray-400">
      {{ text('排除的用户不发放每日消费返利（金额和抽奖券），可单独设置是否在排行榜展示，默认不展示。修改后请保存设置。', 'Excluded users receive no daily rebates (balance or tickets). Choose whether to show each user on the leaderboard; hidden by default. Save settings to apply changes.') }}
    </p>
    <p v-if="detailsError" role="alert" class="mt-3 text-sm text-red-600">{{ detailsError }}</p>
    <button v-if="detailsError" type="button" class="btn btn-secondary btn-sm mt-2" @click="loadDetails">
      {{ text('重新加载用户', 'Reload users') }}
    </button>
    <p v-if="modelValue.length === 0" class="mt-4 text-sm text-gray-500">{{ text('暂无排除用户', 'No excluded users') }}</p>
    <ul v-else class="mt-4 divide-y divide-gray-100 rounded-lg border border-gray-100 dark:divide-dark-700 dark:border-dark-700">
      <li v-for="id in modelValue" :key="id" class="flex items-center justify-between gap-3 p-3" :data-testid="`welfare-excluded-${id}`">
        <div class="min-w-0 text-sm">
          <p class="truncate font-medium text-gray-900 dark:text-white">{{ userDetails[id]?.email || `#${id}` }}</p>
          <p class="text-xs text-gray-500">#{{ id }}<span v-if="userDetails[id]?.username"> · {{ userDetails[id].username }}</span></p>
        </div>
        <div class="flex shrink-0 flex-wrap items-center justify-end gap-3">
          <label class="flex items-center gap-2 text-xs text-gray-500">
            {{ text('排行榜展示', 'Show on leaderboard') }}
            <Toggle :model-value="visibleUserIds.includes(id)" :aria-label="`${text('排行榜展示', 'Show on leaderboard')} #${id}`" :data-testid="`welfare-visible-${id}`" @update:model-value="setVisibility(id, $event)" />
          </label>
          <button type="button" class="btn btn-ghost btn-sm" :aria-label="`${text('移除', 'Remove')} #${id}`" @click="removeUser(id)">
            {{ text('移除', 'Remove') }}
          </button>
        </div>
      </li>
    </ul>
    <BaseDialog :show="showSearch" :title="text('搜索并选择排除用户', 'Select users to exclude')" @close="closeSearch">
      <div class="space-y-4">
        <div class="flex gap-2">
          <input v-model="query" class="input min-w-0 flex-1" :placeholder="text('搜索邮箱或用户名', 'Search email or username')" :aria-label="text('搜索用户', 'Search users')" data-testid="welfare-exclusions-query" @keyup.enter="searchUsers(1)" />
          <button type="button" class="btn btn-primary" :disabled="searching" data-testid="welfare-exclusions-search" @click="searchUsers(1)">{{ text('搜索', 'Search') }}</button>
        </div>
        <p v-if="searchError" role="alert" class="text-sm text-red-600">{{ searchError }}</p>
        <p v-if="searching" class="text-sm text-gray-500">{{ text('搜索中…', 'Searching…') }}</p>
        <p v-else-if="searched && results.length === 0" class="text-sm text-gray-500">{{ text('未找到用户', 'No users found') }}</p>
        <ul v-else class="space-y-2">
          <li v-for="user in results" :key="user.id">
            <label class="flex items-center gap-3 rounded-lg border border-gray-100 p-3 dark:border-dark-700">
              <input v-model="selectedIDs" type="checkbox" :value="user.id" :disabled="modelValue.includes(user.id)" :data-testid="`welfare-select-${user.id}`" />
              <span class="min-w-0 text-sm"><span class="block truncate">{{ user.email }}</span><span class="text-xs text-gray-500">#{{ user.id }} · {{ user.username }}</span></span>
              <span v-if="modelValue.includes(user.id)" class="ml-auto text-xs text-gray-500">{{ text('已排除', 'Excluded') }}</span>
            </label>
          </li>
        </ul>
        <div v-if="total > SEARCH_PAGE_SIZE" class="flex items-center justify-between">
          <button type="button" class="btn btn-secondary btn-sm" :disabled="searching || page === 1" @click="searchUsers(page - 1)">{{ text('上一页', 'Previous') }}</button>
          <span class="text-sm text-gray-500">{{ page }} / {{ Math.ceil(total / SEARCH_PAGE_SIZE) }}</span>
          <button type="button" class="btn btn-secondary btn-sm" :disabled="searching || page * SEARCH_PAGE_SIZE >= total" @click="searchUsers(page + 1)">{{ text('下一页', 'Next') }}</button>
        </div>
      </div>
      <template #footer>
        <button type="button" class="btn btn-secondary" @click="closeSearch">{{ text('取消', 'Cancel') }}</button>
        <button type="button" class="btn btn-primary" :disabled="selectedIDs.length === 0 || searching" data-testid="welfare-exclusions-confirm" @click="confirmSelection">{{ text('确认添加', 'Confirm') }}</button>
      </template>
    </BaseDialog>
  </section>
</template>

<script setup lang="ts">
import { onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api'
import type { AdminUser } from '@/types'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Toggle from '@/components/common/Toggle.vue'

const SEARCH_PAGE_SIZE = 20
const props = withDefaults(defineProps<{ modelValue: number[]; visibleUserIds?: number[] }>(), { visibleUserIds: () => [] })
const emit = defineEmits<{
  'update:modelValue': [value: number[]]
  'update:visibleUserIds': [value: number[]]
}>()
const { locale } = useI18n()
const text = (zh: string, en: string) => locale.value.startsWith('zh') ? zh : en
const userDetails = ref<Record<number, AdminUser>>({})
const detailsError = ref('')
const showSearch = ref(false)
const query = ref('')
const results = ref<AdminUser[]>([])
const selectedIDs = ref<number[]>([])
const searching = ref(false)
const searched = ref(false)
const searchError = ref('')
const page = ref(1)
const total = ref(0)
let searchController: AbortController | undefined

function errorMessage(error: unknown): string {
  const apiError = error as { response?: { data?: { message?: string } }; message?: string }
  return apiError.response?.data?.message || apiError.message || String(error)
}

async function loadDetails(): Promise<void> {
  detailsError.value = ''
  const missing = props.modelValue.filter(id => !userDetails.value[id])
  try {
    const users = await Promise.all(missing.map(id => adminAPI.users.getById(id, true)))
    userDetails.value = { ...userDetails.value, ...Object.fromEntries(users.map(user => [user.id, user])) }
  } catch (error) {
    detailsError.value = errorMessage(error)
  }
}

function openSearch(): void {
  searching.value = false
  query.value = ''
  results.value = []
  selectedIDs.value = []
  searched.value = false
  searchError.value = ''
  total.value = 0
  page.value = 1
  showSearch.value = true
}

function closeSearch(): void {
  searchController?.abort()
  showSearch.value = false
}

async function searchUsers(nextPage: number): Promise<void> {
  searchController?.abort()
  const controller = new AbortController()
  searchController = controller
  searching.value = true
  searchError.value = ''
  results.value = []
  try {
    const response = await adminAPI.users.list(nextPage, SEARCH_PAGE_SIZE,
      { search: query.value.trim(), role: 'user' }, { signal: controller.signal })
    if (controller.signal.aborted) return
    results.value = response.items
    total.value = response.total
    page.value = nextPage
    searched.value = true
    userDetails.value = { ...userDetails.value, ...Object.fromEntries(response.items.map(user => [user.id, user])) }
  } catch (error) {
    if (!controller.signal.aborted) searchError.value = errorMessage(error)
  } finally {
    if (searchController === controller) searching.value = false
  }
}

function confirmSelection(): void {
  emit('update:modelValue', [...new Set([...props.modelValue, ...selectedIDs.value])])
  closeSearch()
}

function setVisibility(id: number, visible: boolean): void {
  const ids = props.visibleUserIds.filter(userID => userID !== id)
  emit('update:visibleUserIds', visible ? [...ids, id] : ids)
}

function removeUser(id: number): void {
  setVisibility(id, false)
  emit('update:modelValue', props.modelValue.filter(userID => userID !== id))
}

watch(() => [...props.modelValue], loadDetails, { immediate: true })
onUnmounted(() => searchController?.abort())
</script>

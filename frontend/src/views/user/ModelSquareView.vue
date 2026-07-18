<template>
  <AppLayout>
    <div class="min-h-full overflow-hidden rounded-2xl bg-white text-gray-900 shadow-card dark:bg-dark-900 dark:text-gray-100">
      <div class="grid min-h-full lg:grid-cols-[258px_minmax(0,1fr)]">
        <aside
          data-test="filter-sidebar"
          class="self-stretch border-b border-gray-200 px-5 py-6 dark:border-dark-700 lg:border-b-0 lg:border-r lg:px-5 lg:py-8"
        >
          <div class="lg:sticky lg:top-6">
            <div class="flex items-center justify-between">
              <button
                type="button"
                class="filter-toggle"
                :aria-expanded="filtersOpen"
                aria-controls="model-square-filters"
                @click="filtersOpen = !filtersOpen"
              >
                <Icon name="filter" size="sm" class="lg:hidden" />
                {{ t('modelSquare.filters.title') }}
                <Icon
                  name="chevronDown"
                  size="sm"
                  class="ml-auto transition-transform lg:hidden"
                  :class="filtersOpen && 'rotate-180'"
                />
              </button>
              <button
                type="button"
                data-test="reset-filters"
                class="filter-reset"
                @click="resetFilters"
              >
                <Icon name="refresh" size="xs" />
                {{ t('modelSquare.filters.reset') }}
              </button>
            </div>

            <div id="model-square-filters" v-show="filtersOpen" class="mt-7 space-y-8 lg:block">
              <label class="block">
                <span class="filter-label">{{ t('modelSquare.filters.search') }}</span>
                <div class="relative mt-2.5">
                  <Icon name="search" size="sm" class="absolute left-3 top-1/2 -translate-y-1/2 text-gray-400" />
                  <input v-model="searchQuery" class="filter-control pl-9" :placeholder="t('modelSquare.searchPlaceholder')" />
                </div>
              </label>

              <div role="group" :aria-label="t('modelSquare.filters.platform')">
                <span class="filter-label">{{ t('modelSquare.filters.platform') }}</span>
                <div class="mt-3 flex flex-wrap gap-2">
                  <button
                    type="button"
                    class="filter-option"
                    :class="{ 'filter-option-active': !platformFilter }"
                    :aria-pressed="!platformFilter"
                    data-test="platform-filter-all"
                    @click="selectPlatform('')"
                  >
                    {{ t('modelSquare.filters.allPlatforms') }}
                  </button>
                  <button
                    v-for="platform in platforms"
                    :key="platform"
                    type="button"
                    class="filter-option max-w-full break-words"
                    :class="{ 'filter-option-active': platformFilter === platform }"
                    :aria-pressed="platformFilter === platform"
                    :data-test="`platform-filter-${platform}`"
                    @click="selectPlatform(platform)"
                  >
                    {{ platform }}
                  </button>
                </div>
              </div>

              <div role="group" :aria-label="t('modelSquare.filters.billing')">
                <span class="filter-label">{{ t('modelSquare.filters.billing') }}</span>
                <div class="mt-3 flex flex-wrap gap-2">
                  <button
                    v-for="option in billingOptions"
                    :key="option.value"
                    type="button"
                    class="filter-option"
                    :class="{ 'filter-option-active': billingFilter === option.value }"
                    :aria-pressed="billingFilter === option.value"
                    :data-test="`billing-filter-${option.value || 'all'}`"
                    @click="billingFilter = option.value"
                  >
                    {{ option.label }}
                  </button>
                </div>
              </div>

              <div role="group" :aria-label="t('modelSquare.filters.group')">
                <span class="filter-label">{{ t('modelSquare.filters.group') }}</span>
                <div class="mt-3 flex flex-wrap gap-2">
                  <button
                    type="button"
                    class="filter-option"
                    :class="{ 'filter-option-active': !groupFilter }"
                    :aria-pressed="!groupFilter"
                    data-test="group-filter-all"
                    @click="selectGroup('')"
                  >
                    {{ t('modelSquare.filters.allGroups') }}
                  </button>
                  <button
                    v-for="group in groupOptions"
                    :key="group.id"
                    type="button"
                    class="filter-option max-w-full break-words"
                    :class="{ 'filter-option-active': groupFilter === String(group.id) }"
                    :aria-pressed="groupFilter === String(group.id)"
                    :data-test="`group-filter-${group.id}`"
                    @click="selectGroup(String(group.id))"
                  >
                    {{ group.name }} · {{ formatPrice(group.rate) }}×
                  </button>
                </div>
              </div>
            </div>
          </div>
        </aside>

        <main class="min-w-0 px-5 py-6 sm:px-7 lg:px-7 lg:py-7">
          <header class="border-b border-gray-200 pb-5 dark:border-dark-700">
            <p class="text-xs font-semibold uppercase tracking-[0.24em] text-primary-600 dark:text-primary-400">{{ t('modelSquare.eyebrow') }}</p>
            <div class="mt-1 flex flex-wrap items-end justify-between gap-3">
              <div>
                <h1 class="text-3xl font-bold tracking-tight sm:text-4xl">{{ t('modelSquare.title') }}</h1>
                <p class="mt-1.5 text-sm text-gray-500 dark:text-gray-400">{{ t('modelSquare.description') }}</p>
              </div>
              <div class="pb-0.5 text-sm text-gray-400">{{ t('modelSquare.resultCount', { count: filteredCards.length }) }}</div>
            </div>
          </header>

          <div class="flex flex-wrap items-center justify-between gap-3 py-4">
            <div data-test="sort-control" class="w-36">
              <Select v-model="sortMode" :options="sortOptions" />
            </div>
            <div class="view-switcher" role="group" :aria-label="t('modelSquare.view.label')">
              <button
                type="button"
                data-test="grid-view"
                class="view-button"
                :class="viewMode === 'grid' && 'view-button-active'"
                :aria-label="t('modelSquare.view.grid')"
                :aria-pressed="viewMode === 'grid'"
                @click="viewMode = 'grid'"
              >
                <Icon name="grid" size="sm" />
              </button>
              <button
                type="button"
                data-test="list-view"
                class="view-button"
                :class="viewMode === 'list' && 'view-button-active'"
                :aria-label="t('modelSquare.view.list')"
                :aria-pressed="viewMode === 'list'"
                @click="viewMode = 'list'"
              >
                <Icon name="list" size="sm" />
              </button>
            </div>
          </div>

          <div v-if="loading" class="flex justify-center py-20"><div class="h-8 w-8 animate-spin rounded-full border-2 border-primary-500 border-t-transparent" /></div>
          <div v-else-if="filteredCards.length === 0" class="rounded-xl border border-dashed border-gray-200 py-20 text-center text-base text-gray-400 dark:border-dark-700">{{ t('modelSquare.empty') }}</div>
          <section
            v-else
            data-test="model-card-grid"
            :class="viewMode === 'grid' ? 'grid grid-cols-1 gap-3 md:grid-cols-2 xl:grid-cols-3' : 'grid grid-cols-1 gap-3'"
          >
            <ModelSquareCard
              v-for="item in paginatedCards"
              :key="item.card.key"
              :card="item.card"
              :active-group="item.activeGroup"
              :effective-rate="item.effectiveRate"
              :monitor="item.monitor"
              :view-mode="viewMode"
            />
          </section>

          <Pagination
            v-if="filteredCards.length > pageSize"
            v-model:page="page"
            class="mt-4"
            :total="filteredCards.length"
            :page-size="pageSize"
            :show-page-size-selector="false"
          />
        </main>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import Pagination from '@/components/common/Pagination.vue'
import Select from '@/components/common/Select.vue'
import ModelSquareCard, { type ModelSquareCardData } from '@/components/user/models/ModelSquareCard.vue'
import userChannelsAPI, { type UserAvailableGroup } from '@/api/channels'
import userGroupsAPI from '@/api/groups'
import channelMonitorUserAPI, { type UserMonitorView } from '@/api/channelMonitor'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import { formatPrice, resolveCardGroup, resolveEffectiveRate } from '@/utils/model-pricing'

type SortMode = 'default' | 'model-asc' | 'model-desc' | 'platform'
type ViewMode = 'grid' | 'list'

const { t } = useI18n()
const appStore = useAppStore()
const loading = ref(false)
const filtersOpen = ref(true)
const cards = ref<ModelSquareCardData[]>([])
const monitors = ref<UserMonitorView[]>([])
const userGroupRates = ref<Record<number, number>>({})
const searchQuery = ref('')
const platformFilter = ref('')
const billingFilter = ref('')
const groupFilter = ref('')
const sortMode = ref<SortMode>('default')
const viewMode = ref<ViewMode>('grid')
const page = ref(1)
const pageSize = 12

const billingOptions = computed(() => [
  { value: '', label: t('modelSquare.filters.allBillingModes') },
  { value: 'usage', label: t('modelSquare.billing.usageBased') },
  { value: 'request', label: t('modelSquare.billing.perImage') },
])
const sortOptions = computed(() => [
  { value: 'default', label: t('modelSquare.sort.default') },
  { value: 'model-asc', label: t('modelSquare.sort.modelAsc') },
  { value: 'model-desc', label: t('modelSquare.sort.modelDesc') },
  { value: 'platform', label: t('modelSquare.sort.platform') },
])
const platforms = computed(() => [...new Set(cards.value.map((item) => item.platform))].sort())
const groupOptions = computed(() => {
  const groups = new Map<number, { id: number; name: string; platform: string; rate: number }>()
  for (const card of cards.value) {
    for (const group of card.groups) {
      groups.set(group.id, {
        id: group.id,
        name: group.name,
        platform: card.platform,
        rate: resolveEffectiveRate({ groupId: group.id, groupRate: group.rate_multiplier, userGroupRates: userGroupRates.value }),
      })
    }
  }
  return [...groups.values()].sort((left, right) => left.platform.localeCompare(right.platform) || left.name.localeCompare(right.name))
})
const selectedGroupOption = computed(() => groupOptions.value.find((group) => String(group.id) === groupFilter.value))

watch(platformFilter, (platform) => {
  if (groupFilter.value && selectedGroupOption.value?.platform !== platform) groupFilter.value = ''
})
watch([searchQuery, platformFilter, billingFilter, groupFilter, sortMode], () => { page.value = 1 })

function selectPlatform(platform: string) {
  platformFilter.value = platform
}
function selectGroup(groupId: string) {
  groupFilter.value = groupId
  const group = selectedGroupOption.value
  if (group) platformFilter.value = group.platform
}
function normalizedIdentity(value: string): string {
  return value.trim().normalize('NFKC').toLowerCase().replace(/[\s_]+/g, '-')
}
function monitorFor(card: ModelSquareCardData, selectedGroup?: UserAvailableGroup): UserMonitorView | undefined {
  const candidates = monitors.value.filter((monitor) =>
    normalizedIdentity(monitor.provider) === normalizedIdentity(card.platform)
    && normalizedIdentity(monitor.name) === normalizedIdentity(card.channel)
    && normalizedIdentity(monitor.primary_model) === normalizedIdentity(card.model),
  )
  if (selectedGroup) {
    const groupMatches = candidates.filter((monitor) => monitor.group_name && normalizedIdentity(monitor.group_name) === normalizedIdentity(selectedGroup.name))
    return groupMatches.length === 1 ? groupMatches[0] : undefined
  }
  if (candidates.length === 1) return candidates[0]
  const ungrouped = candidates.filter((monitor) => !monitor.group_name)
  return ungrouped.length === 1 ? ungrouped[0] : undefined
}
function billingCategory(card: ModelSquareCardData): 'usage' | 'request' {
  return card.billingMode === 'token' ? 'usage' : 'request'
}

const resolvedCards = computed(() => cards.value.map((card, sourceIndex) => {
  const selectedGroup = groupFilter.value
    ? card.groups.find((group) => String(group.id) === groupFilter.value)
    : undefined
  const activeGroup = selectedGroup ?? resolveCardGroup({ groups: card.groups, userGroupRates: userGroupRates.value })
  const effectiveRate = activeGroup
    ? resolveEffectiveRate({ groupId: activeGroup.id, groupRate: activeGroup.rate_multiplier, userGroupRates: userGroupRates.value })
    : 1
  return { card, activeGroup, effectiveRate, monitor: monitorFor(card, activeGroup), sourceIndex }
}))
const filteredCards = computed(() => {
  const query = searchQuery.value.trim().toLowerCase()
  return resolvedCards.value.filter(({ card }) =>
    (!query || card.searchText.includes(query))
    && (!platformFilter.value || card.platform === platformFilter.value)
    && (!billingFilter.value || billingCategory(card) === billingFilter.value)
    && (!groupFilter.value || card.groups.some((group) => String(group.id) === groupFilter.value)),
  )
})
const sortedCards = computed(() => {
  if (sortMode.value === 'default') return filteredCards.value
  const direction = sortMode.value === 'model-desc' ? -1 : 1
  return [...filteredCards.value].sort((left, right) => {
    const primary = sortMode.value === 'platform'
      ? left.card.platform.localeCompare(right.card.platform)
      : left.card.model.localeCompare(right.card.model) * direction
    return primary || left.card.model.localeCompare(right.card.model) || left.sourceIndex - right.sourceIndex
  })
})
const paginatedCards = computed(() => sortedCards.value.slice((page.value - 1) * pageSize, page.value * pageSize))

watch(() => filteredCards.value.length, (count) => {
  const lastPage = Math.max(1, Math.ceil(count / pageSize))
  if (page.value > lastPage) page.value = lastPage
})

function resetFilters() {
  searchQuery.value = ''
  platformFilter.value = ''
  billingFilter.value = ''
  groupFilter.value = ''
  sortMode.value = 'default'
  page.value = 1
}

async function loadModels() {
  loading.value = true
  try {
    const [channels, rates] = await Promise.all([userChannelsAPI.getAvailable(), userGroupsAPI.getUserGroupRates()])
    userGroupRates.value = rates
    const [monitorResponse] = await Promise.allSettled([channelMonitorUserAPI.list()])
    monitors.value = monitorResponse.status === 'fulfilled' ? monitorResponse.value.items : []
    cards.value = channels.flatMap((channel) => channel.platforms.flatMap((section) => section.supported_models.map((model) => ({
      key: `${channel.name}:${section.platform}:${model.name}`,
      model: model.name,
      platform: section.platform,
      channel: channel.name,
      groups: section.groups,
      pricing: model.pricing,
      billingMode: model.pricing?.billing_mode ?? 'token',
      searchText: [channel.name, model.name, section.platform, ...section.groups.map((group) => group.name)].join(' ').toLowerCase(),
    }))))
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('modelSquare.loadFailed')))
  } finally {
    loading.value = false
  }
}

onMounted(loadModels)
</script>

<style scoped>
.filter-label { font-size: 0.75rem; font-weight: 600; color: rgb(107 114 128); }
.filter-control { width: 100%; border-radius: 0.5rem; border: 1px solid rgb(229 231 235); background: white; padding: 0.55rem 0.7rem 0.55rem 2.25rem; font-size: 0.75rem; outline: none; }
.filter-control:focus { border-color: #1677ff; box-shadow: 0 0 0 2px rgb(22 119 255 / 0.1); }
.filter-toggle { display: flex; min-width: 0; flex: 1 1 0%; align-items: center; gap: 0.5rem; border-radius: 0.5rem; color: rgb(17 24 39); font-size: 1.125rem; font-weight: 600; text-align: left; }
.filter-reset { display: inline-flex; flex-shrink: 0; align-items: center; gap: 0.35rem; margin-left: 0.75rem; border: 1px solid #bae0ff; border-radius: 0.5rem; background: rgb(230 244 255 / 0.72); padding: 0.35rem 0.55rem; color: #003eb3; font-size: 0.75rem; font-weight: 600; transition: background-color 150ms, border-color 150ms, color 150ms, box-shadow 150ms; }
.filter-reset:hover { border-color: #69b1ff; background: #bae0ff; color: #002c8c; box-shadow: 0 4px 12px rgb(22 119 255 / 0.12); }
.filter-option { min-width: 0; border-radius: 0.5rem; border: 1px solid #bae0ff; background: rgb(255 255 255 / 0.82); padding: 0.4rem 0.65rem; color: rgb(71 85 105); font-size: 0.75rem; line-height: 1rem; box-shadow: 0 1px 2px rgb(15 23 42 / 0.04); transition: background-color 150ms, border-color 150ms, color 150ms, box-shadow 150ms, transform 150ms; }
.filter-option:hover { border-color: #69b1ff; background: #e6f4ff; color: #003eb3; box-shadow: 0 4px 12px rgb(22 119 255 / 0.1); }
.filter-option:active { transform: scale(0.98); }
.filter-reset:focus-visible, .filter-option:focus-visible, .view-button:focus-visible, .filter-toggle:focus-visible { outline: 2px solid #1677ff; outline-offset: 2px; }
.filter-option-active { border-color: #0958d9; background: #0958d9; color: white; font-weight: 600; box-shadow: 0 5px 14px rgb(22 119 255 / 0.22); }
.filter-option-active:hover { border-color: #003eb3; background: #003eb3; color: white; }
.view-switcher { display: inline-flex; border: 1px solid #bae0ff; border-radius: 0.5rem; background: rgb(230 244 255 / 0.52); padding: 0.125rem; box-shadow: inset 0 1px 2px rgb(15 23 42 / 0.04); }
.view-button { display: grid; height: 2rem; width: 2rem; place-items: center; border-radius: 0.375rem; color: rgb(100 116 139); transition: color 150ms, background-color 150ms, box-shadow 150ms; }
.view-button:hover { background: rgb(186 224 255 / 0.72); color: #003eb3; }
.view-button-active { background: #0958d9; color: white; box-shadow: 0 2px 7px rgb(22 119 255 / 0.24); }
.view-button-active:hover { background: #003eb3; color: white; }
:global(.dark .filter-label) { color: rgb(156 163 175); }
:global(.dark .filter-control) { border-color: rgb(55 65 81); background: rgb(31 41 55); color: rgb(229 231 235); }
:global(.dark .filter-toggle) { color: rgb(243 244 246); }
:global(.dark .filter-reset) { border-color: rgb(22 119 255 / 0.28); background: rgb(22 119 255 / 0.1); color: #69b1ff; }
:global(.dark .filter-reset:hover) { border-color: rgb(64 150 255 / 0.55); background: rgb(22 119 255 / 0.18); color: #91caff; }
:global(.dark .filter-option) { border-color: rgb(22 119 255 / 0.22); background: rgb(31 41 55 / 0.72); color: rgb(209 213 219); box-shadow: none; }
:global(.dark .filter-option:hover) { border-color: rgb(64 150 255 / 0.48); background: rgb(22 119 255 / 0.12); color: #91caff; }
:global(.dark .filter-option-active) { border-color: #1677ff; background: #0958d9; color: white; box-shadow: 0 5px 16px rgb(0 0 0 / 0.2); }
:global(.dark .filter-option-active:hover) { border-color: #4096ff; background: #1677ff; color: white; }
:global(.dark .view-switcher) { border-color: rgb(22 119 255 / 0.24); background: rgb(22 119 255 / 0.08); }
:global(.dark .view-button:hover) { background: rgb(22 119 255 / 0.14); color: #91caff; }
:global(.dark .view-button-active) { background: #0958d9; color: white; }

@media (min-width: 1024px) {
  .filter-toggle { pointer-events: none; border-radius: 0; }
}
</style>

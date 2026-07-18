<template>
  <AppLayout>
    <div class="leaderboard-page">
      <!-- Animated background particles -->
      <div class="bg-particles">
        <div v-for="n in 20" :key="n" class="particle" :style="particleStyle(n)" />
      </div>

      <!-- Header -->
      <header class="lb-header">
        <div class="header-glow" />
        <h1 class="lb-title">
          <span class="title-icon">🏆</span>
          <span class="title-text">{{ selectedRangeLabel }}排行榜</span>
        </h1>
        <p class="lb-subtitle">{{ selectedDateFormatted }}</p>
        <div class="range-switcher" role="tablist" aria-label="排行榜日期">
          <button
            v-for="option in rangeOptions"
            :key="option.value"
            type="button"
            role="tab"
            class="range-option"
            :class="{ active: selectedRange === option.value }"
            :aria-selected="selectedRange === option.value"
            :data-testid="`leaderboard-range-${option.value}`"
            @click="selectRange(option.value)"
          >
            {{ option.label }}
          </button>
        </div>
      </header>

      <!-- Loading state -->
      <div v-if="loading" class="loading-container">
        <div class="loader">
          <div class="loader-ring" />
          <div class="loader-ring" />
          <div class="loader-ring" />
        </div>
        <p class="loading-text">正在加载排行榜...</p>
      </div>

      <!-- Error state -->
      <div v-else-if="error" class="error-container">
        <div class="error-icon">⚠️</div>
        <p class="error-text">{{ error }}</p>
        <button class="retry-btn" @click="fetchRanking">重试</button>
      </div>

      <!-- Empty state -->
      <div v-else-if="ranking.length === 0" class="empty-container">
        <div class="empty-icon">📊</div>
        <p class="empty-text">{{ selectedRangeLabel }}暂无数据</p>
      </div>

      <!-- Leaderboard content -->
      <div v-else class="lb-content">
        <!-- Podium: Top 3 -->
        <div class="podium-section">
          <!-- 2nd place (left) -->
          <div v-if="ranking.length >= 2" class="podium-card rank-2" :class="{ 'is-me': ranking[1].user_id === currentUserId }" @mouseenter="hoveredCard = 2" @mouseleave="hoveredCard = null">
            <div v-if="ranking[1].user_id === currentUserId" class="me-badge">我</div>
            <div class="rank-badge silver">2</div>
            <div class="avatar-wrapper silver-glow">
              <img
                :src="getAvatarUrl(ranking[1].email)"
                :alt="ranking[1].email"
                class="avatar-img"
                loading="lazy"
              />
            </div>
            <div class="user-email" :title="ranking[1].email">{{ maskEmail(ranking[1].email) }}</div>
            <div class="token-value">{{ formatCost(ranking[1].actual_cost) }}</div>
            <div class="podium-bar bar-2">
              <div class="bar-shine" />
            </div>
          </div>

          <!-- 1st place (center) -->
          <div v-if="ranking.length >= 1" class="podium-card rank-1" :class="{ 'is-me': ranking[0].user_id === currentUserId }" @mouseenter="hoveredCard = 1" @mouseleave="hoveredCard = null">
            <div v-if="ranking[0].user_id === currentUserId" class="me-badge">我</div>
            <div class="crown">👑</div>
            <div class="rank-badge gold">1</div>
            <div class="avatar-wrapper gold-glow">
              <img
                :src="getAvatarUrl(ranking[0].email)"
                :alt="ranking[0].email"
                class="avatar-img"
                loading="lazy"
              />
            </div>
            <div class="user-email" :title="ranking[0].email">{{ maskEmail(ranking[0].email) }}</div>
            <div class="token-value champion">{{ formatCost(ranking[0].actual_cost) }}</div>
            <div class="podium-bar bar-1">
              <div class="bar-shine" />
            </div>
          </div>

          <!-- 3rd place (right) -->
          <div v-if="ranking.length >= 3" class="podium-card rank-3" :class="{ 'is-me': ranking[2].user_id === currentUserId }" @mouseenter="hoveredCard = 3" @mouseleave="hoveredCard = null">
            <div v-if="ranking[2].user_id === currentUserId" class="me-badge">我</div>
            <div class="rank-badge bronze">3</div>
            <div class="avatar-wrapper bronze-glow">
              <img
                :src="getAvatarUrl(ranking[2].email)"
                :alt="ranking[2].email"
                class="avatar-img"
                loading="lazy"
              />
            </div>
            <div class="user-email" :title="ranking[2].email">{{ maskEmail(ranking[2].email) }}</div>
            <div class="token-value">{{ formatCost(ranking[2].actual_cost) }}</div>
            <div class="podium-bar bar-3">
              <div class="bar-shine" />
            </div>
          </div>
        </div>

        <!-- Rest of the list: rank 4-10 -->
        <div v-if="ranking.length > 3" class="list-section">
          <div
            v-for="(user, index) in ranking.slice(3)"
            :key="user.user_id"
            class="list-item"
            :class="{ 'is-me': user.user_id === currentUserId }"
            :style="{ animationDelay: `${(index + 3) * 0.08}s` }"
          >
            <div class="list-rank">#{{ index + 4 }}</div>
            <img
              :src="getAvatarUrl(user.email)"
              :alt="user.email"
              class="list-avatar"
              loading="lazy"
            />
            <div class="list-info">
              <div class="list-email-wrapper">
                <div class="list-email" :title="user.email">{{ maskEmail(user.email) }}</div>
                <span v-if="user.user_id === currentUserId" class="list-me">我</span>
              </div>
            </div>
            <div class="list-tokens">
              <span class="list-token-value">{{ formatCost(user.actual_cost) }}</span>
            </div>
          </div>
        </div>

        <!-- Current User Rank (if not in top list) -->
        <div v-if="showUserRankingAtBottom" class="user-rank-bottom">
          <div class="divider-line" />
          <div class="list-item is-me bottom-me-item">
            <div class="list-rank">#{{ userRanking?.rank }}</div>
            <img
              :src="getAvatarUrl(userRanking?.email || '')"
              :alt="userRanking?.email"
              class="list-avatar"
              loading="lazy"
            />
            <div class="list-info">
              <div class="list-email-wrapper">
                <div class="list-email" :title="userRanking?.email">{{ maskEmail(userRanking?.email || '') }}</div>
                <span class="list-me">我</span>
              </div>
            </div>
            <div class="list-tokens">
              <span class="list-token-value">{{ formatCost(userRanking?.actual_cost || 0) }}</span>
            </div>
          </div>
        </div>
      </div>

      <!-- Footer -->
      <footer class="lb-footer">
        <p>每 60 秒自动刷新</p>
      </footer>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { getUserSpendingRanking } from '@/api/admin/dashboard'
import type { UserSpendingRankingItem } from '@/types'
import AppLayout from '@/components/layout/AppLayout.vue'
import { useAuthStore } from '@/stores'
import { maskLeaderboardEmail } from '@/utils/leaderboardEmail'

type LeaderboardRange = 'today' | 'yesterday'

const LEADERBOARD_LIMIT = 10
const REFRESH_INTERVAL_MS = 60_000
const YESTERDAY_OFFSET_DAYS = -1

// State
const authStore = useAuthStore()
const currentUserId = computed(() => authStore.user?.id)
const ranking = ref<UserSpendingRankingItem[]>([])
const userRanking = ref<UserSpendingRankingItem | null>(null)
const loading = ref(true)
const error = ref<string | null>(null)
const hoveredCard = ref<number | null>(null)
const selectedRange = ref<LeaderboardRange>('today')
let refreshTimer: ReturnType<typeof setInterval> | null = null

const rangeOptions: ReadonlyArray<{ value: LeaderboardRange; label: string }> = [
  { value: 'today', label: '今日' },
  { value: 'yesterday', label: '昨日' }
]

const userInTopList = computed(() => {
  return ranking.value.some(item => item.user_id === currentUserId.value)
})

const showUserRankingAtBottom = computed(() => {
  return !!(userRanking.value && !userInTopList.value)
})

const selectedRangeLabel = computed(() => {
  return selectedRange.value === 'today' ? '今日' : '昨日'
})

const selectedDate = computed(() => {
  return selectedRange.value === 'today'
    ? getLocalDate()
    : getLocalDate(YESTERDAY_OFFSET_DAYS)
})

const selectedDateFormatted = computed(() => {
  return selectedDate.value.toLocaleDateString('zh-CN', {
    year: 'numeric',
    month: 'long',
    day: 'numeric',
    weekday: 'long'
  })
})

function getLocalDate(offsetDays = 0): Date {
  const date = new Date()
  date.setDate(date.getDate() + offsetDays)
  return date
}

function formatLocalDate(date: Date): string {
  const year = date.getFullYear()
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  return `${year}-${month}-${day}`
}

function selectRange(range: LeaderboardRange): void {
  if (selectedRange.value === range) return
  selectedRange.value = range
  fetchRanking()
}

// DiceBear Notionists avatar URL
function getAvatarUrl(email: string): string {
  const seed = encodeURIComponent(email)
  return `https://api.dicebear.com/9.x/notionists/svg?seed=${seed}&backgroundColor=transparent`
}

function maskEmail(email: string): string {
  return maskLeaderboardEmail(email)
}

// Format cost to 2 decimal places with $ prefix
function formatCost(cost: number): string {
  return `$${(cost || 0).toFixed(2)}`
}

// Generate random particle styles
function particleStyle(n: number) {
  const size = 2 + Math.random() * 4
  const left = (n * 5) % 100
  const delay = Math.random() * 20
  const duration = 15 + Math.random() * 20
  return {
    width: `${size}px`,
    height: `${size}px`,
    left: `${left}%`,
    animationDelay: `${delay}s`,
    animationDuration: `${duration}s`
  }
}

// Fetch ranking data
async function fetchRanking() {
  try {
    loading.value = true
    error.value = null
    const targetDate = formatLocalDate(selectedDate.value)
    const response = await getUserSpendingRanking({
      start_date: targetDate,
      end_date: targetDate,
      limit: LEADERBOARD_LIMIT
    })
    const list = response.ranking || []
    // Explicitly sort by actual_cost descending
    list.sort((a, b) => b.actual_cost - a.actual_cost)
    ranking.value = list
    userRanking.value = response.user_ranking || null
  } catch (err: any) {
    error.value = err?.message || 'Failed to load ranking data'
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  fetchRanking()
  // Auto-refresh every 60 seconds
  refreshTimer = setInterval(fetchRanking, REFRESH_INTERVAL_MS)
})

onUnmounted(() => {
  if (refreshTimer) {
    clearInterval(refreshTimer)
  }
})
</script>

<style scoped>
/* ==================== Base & Background ==================== */
.leaderboard-page {
  min-height: calc(100vh - 64px - 4rem);
  display: flex;
  flex-direction: column;
  border-radius: 1.5rem;
  background: linear-gradient(135deg, #0a0a1a 0%, #1a1035 30%, #0d1b2a 60%, #0a0a1a 100%);
  color: #e0e0e0;
  font-family: 'Inter', 'Segoe UI', system-ui, -apple-system, sans-serif;
  position: relative;
  overflow: visible;
  padding: 2rem;
  box-shadow: 0 20px 40px rgba(0, 0, 0, 0.4);
  border: 1px solid rgba(255, 255, 255, 0.05);
}

/* Animated particles */
.bg-particles {
  position: absolute;
  top: 0;
  left: 0;
  width: 100%;
  height: 100%;
  pointer-events: none;
  z-index: 0;
  overflow: hidden;
  border-radius: 1.5rem;
}

.particle {
  position: absolute;
  bottom: -10px;
  background: rgba(255, 255, 255, 0.15);
  border-radius: 50%;
  animation: float-up linear infinite;
}

@keyframes float-up {
  0% {
    transform: translateY(0) scale(1);
    opacity: 0;
  }
  10% {
    opacity: 0.6;
  }
  90% {
    opacity: 0.2;
  }
  100% {
    transform: translateY(-100vh) scale(0.5);
    opacity: 0;
  }
}

/* ==================== Header ==================== */
.lb-header {
  text-align: center;
  margin-bottom: 2rem;
  position: relative;
  z-index: 1;
}

.header-glow {
  position: absolute;
  top: -40px;
  left: 50%;
  transform: translateX(-50%);
  width: 300px;
  height: 300px;
  background: radial-gradient(circle, rgba(255, 215, 0, 0.08) 0%, transparent 70%);
  pointer-events: none;
}

.lb-title {
  font-size: 2rem;
  font-weight: 800;
  margin: 0 0 0.5rem;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 0.75rem;
}

.title-icon {
  font-size: 2.2rem;
  animation: trophy-bounce 2s ease-in-out infinite;
}

@keyframes trophy-bounce {
  0%, 100% { transform: translateY(0); }
  50% { transform: translateY(-6px); }
}

.title-text {
  background: linear-gradient(135deg, #ffd700, #ffaa00, #ffd700);
  -webkit-background-clip: text;
  background-clip: text;
  -webkit-text-fill-color: transparent;
  text-shadow: none;
}

.lb-subtitle {
  font-size: 0.9rem;
  color: rgba(255, 255, 255, 0.5);
  margin: 0;
  letter-spacing: 0.5px;
}

.range-switcher {
  display: inline-flex;
  align-items: center;
  gap: 0.25rem;
  margin-top: 1rem;
  padding: 0.25rem;
  background: rgba(255, 255, 255, 0.06);
  border: 1px solid rgba(255, 255, 255, 0.1);
  border-radius: 8px;
  backdrop-filter: blur(10px);
}

.range-option {
  min-width: 68px;
  height: 32px;
  padding: 0 0.9rem;
  border: none;
  border-radius: 6px;
  background: transparent;
  color: rgba(255, 255, 255, 0.62);
  font-size: 0.85rem;
  font-weight: 700;
  cursor: pointer;
  transition: background 0.2s ease, color 0.2s ease, box-shadow 0.2s ease;
}

.range-option:hover {
  color: rgba(255, 255, 255, 0.88);
  background: rgba(255, 255, 255, 0.07);
}

.range-option.active {
  color: #1a1a2e;
  background: linear-gradient(135deg, #ffd700, #ffaa00);
  box-shadow: 0 4px 16px rgba(255, 215, 0, 0.22);
}

/* ==================== Loading ==================== */
.loading-container {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 6rem 0;
  position: relative;
  z-index: 1;
  flex: 1;
}

.loader {
  position: relative;
  width: 60px;
  height: 60px;
}

.loader-ring {
  position: absolute;
  inset: 0;
  border: 3px solid transparent;
  border-radius: 50%;
  animation: spin 1.5s linear infinite;
}

.loader-ring:nth-child(1) {
  border-top-color: #ffd700;
  animation-delay: 0s;
}

.loader-ring:nth-child(2) {
  inset: 6px;
  border-right-color: #c0c0c0;
  animation-delay: 0.2s;
  animation-direction: reverse;
}

.loader-ring:nth-child(3) {
  inset: 12px;
  border-bottom-color: #cd7f32;
  animation-delay: 0.4s;
}

@keyframes spin {
  to { transform: rotate(360deg); }
}

.loading-text {
  margin-top: 1.5rem;
  color: rgba(255, 255, 255, 0.5);
  font-size: 0.95rem;
}

/* ==================== Error / Empty ==================== */
.error-container,
.empty-container {
  text-align: center;
  padding: 6rem 0;
  position: relative;
  z-index: 1;
  flex: 1;
}

.error-icon,
.empty-icon {
  font-size: 3rem;
  margin-bottom: 1rem;
}

.error-text,
.empty-text {
  color: rgba(255, 255, 255, 0.6);
  font-size: 1.1rem;
  margin-bottom: 1.5rem;
}

.retry-btn {
  padding: 0.6rem 2rem;
  background: linear-gradient(135deg, #ffd700, #ffaa00);
  color: #000;
  border: none;
  border-radius: 8px;
  font-size: 0.95rem;
  font-weight: 600;
  cursor: pointer;
  transition: transform 0.2s, box-shadow 0.2s;
}

.retry-btn:hover {
  transform: translateY(-2px);
  box-shadow: 0 4px 20px rgba(255, 215, 0, 0.3);
}

/* ==================== Leaderboard Content ==================== */
.lb-content {
  width: 100%;
  max-width: 800px;
  margin: 0 auto;
  position: relative;
  z-index: 1;
  flex: 1;
  display: flex;
  flex-direction: column;
}

/* ==================== Podium Section ==================== */
.podium-section {
  display: flex;
  align-items: flex-end;
  justify-content: center;
  gap: 0.75rem;
  margin-bottom: 2rem;
  padding: 0 1rem;
}

.podium-card {
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 1.25rem 0.75rem 0;
  border-radius: 16px 16px 0 0;
  position: relative;
  transition: transform 0.3s ease;
  animation: card-enter 0.6s ease backwards;
}

.podium-card:hover {
  transform: translateY(-4px);
}

/* Rank 1 - Gold (center, tallest) */
.rank-1 {
  background: linear-gradient(180deg, rgba(255, 215, 0, 0.08) 0%, rgba(255, 215, 0, 0.02) 100%);
  border: 1px solid rgba(255, 215, 0, 0.15);
  width: 170px;
  animation-delay: 0.1s;
}

/* Rank 2 - Silver (left) */
.rank-2 {
  background: linear-gradient(180deg, rgba(192, 192, 192, 0.08) 0%, rgba(192, 192, 192, 0.02) 100%);
  border: 1px solid rgba(192, 192, 192, 0.12);
  width: 145px;
  animation-delay: 0.2s;
}

/* Rank 3 - Bronze (right) */
.rank-3 {
  background: linear-gradient(180deg, rgba(205, 127, 50, 0.08) 0%, rgba(205, 127, 50, 0.02) 100%);
  border: 1px solid rgba(205, 127, 50, 0.12);
  width: 145px;
  animation-delay: 0.3s;
}

@keyframes card-enter {
  from {
    opacity: 0;
    transform: translateY(30px) scale(0.95);
  }
  to {
    opacity: 1;
    transform: translateY(0) scale(1);
  }
}

/* Crown for 1st place */
.crown {
  position: absolute;
  top: -15px;
  font-size: 1.7rem;
  animation: crown-float 3s ease-in-out infinite;
}

@keyframes crown-float {
  0%, 100% { transform: translateY(0) rotate(-5deg); }
  50% { transform: translateY(-4px) rotate(5deg); }
}

/* Rank badges */
.rank-badge {
  width: 28px;
  height: 28px;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  font-weight: 800;
  font-size: 0.85rem;
  margin-bottom: 0.5rem;
  box-shadow: 0 2px 10px rgba(0, 0, 0, 0.3);
}

.rank-badge.gold {
  background: linear-gradient(135deg, #ffd700, #f0a500);
  color: #1a1a2e;
}

.rank-badge.silver {
  background: linear-gradient(135deg, #e8e8e8, #a0a0a0);
  color: #1a1a2e;
}

.rank-badge.bronze {
  background: linear-gradient(135deg, #cd7f32, #a0522d);
  color: #fff;
}

/* Avatar */
.avatar-wrapper {
  width: 64px;
  height: 64px;
  border-radius: 50%;
  padding: 3px;
  margin-bottom: 0.5rem;
  position: relative;
}

.rank-1 .avatar-wrapper {
  width: 80px;
  height: 80px;
}

.gold-glow {
  background: linear-gradient(135deg, #ffd700, #f0a500, #ffd700);
  box-shadow: 0 0 15px rgba(255, 215, 0, 0.25);
}

.silver-glow {
  background: linear-gradient(135deg, #e8e8e8, #a0a0a0, #e8e8e8);
  box-shadow: 0 0 12px rgba(192, 192, 192, 0.15);
}

.bronze-glow {
  background: linear-gradient(135deg, #cd7f32, #a0522d, #cd7f32);
  box-shadow: 0 0 12px rgba(205, 127, 50, 0.15);
}

.avatar-img {
  width: 100%;
  height: 100%;
  border-radius: 50%;
  object-fit: cover;
  background: #1a1035;
}

/* User info */
.user-email {
  font-size: 0.75rem;
  color: rgba(255, 255, 255, 0.6);
  margin-bottom: 0.4rem;
  max-width: 130px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  text-align: center;
}

.token-value {
  font-size: 1.3rem;
  font-weight: 800;
  color: #ff4444;
  line-height: 1;
  text-shadow: 0 0 20px rgba(255, 68, 68, 0.3);
}

.token-value.champion {
  font-size: 1.6rem;
  background: linear-gradient(135deg, #ff4444, #ff6b6b);
  -webkit-background-clip: text;
  background-clip: text;
  -webkit-text-fill-color: transparent;
  text-shadow: none;
  filter: drop-shadow(0 0 10px rgba(255, 68, 68, 0.4));
}

/* Podium bars */
.podium-bar {
  width: 100%;
  border-radius: 0;
  position: relative;
  overflow: hidden;
}

.bar-1 {
  height: 80px;
  background: linear-gradient(180deg, rgba(255, 215, 0, 0.2), rgba(255, 215, 0, 0.05));
}

.bar-2 {
  height: 55px;
  background: linear-gradient(180deg, rgba(192, 192, 192, 0.15), rgba(192, 192, 192, 0.05));
}

.bar-3 {
  height: 38px;
  background: linear-gradient(180deg, rgba(205, 127, 50, 0.15), rgba(205, 127, 50, 0.05));
}

.bar-shine {
  position: absolute;
  top: 0;
  left: -100%;
  width: 100%;
  height: 100%;
  background: linear-gradient(90deg, transparent, rgba(255, 255, 255, 0.05), transparent);
  animation: shine 3s ease-in-out infinite;
}

@keyframes shine {
  0% { left: -100%; }
  50% { left: 100%; }
  100% { left: 100%; }
}

/* ==================== List Section (4-10) ==================== */
.list-section {
  display: flex;
  flex-direction: column;
  gap: 0.4rem;
}

.list-item {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  padding: 0.6rem 1rem;
  background: rgba(255, 255, 255, 0.03);
  border: 1px solid rgba(255, 255, 255, 0.06);
  border-radius: 10px;
  transition: all 0.25s ease;
  animation: list-enter 0.5s ease backwards;
  backdrop-filter: blur(10px);
}

.list-item:hover {
  background: rgba(255, 255, 255, 0.06);
  border-color: rgba(255, 255, 255, 0.1);
  transform: translateX(4px);
}

@keyframes list-enter {
  from {
    opacity: 0;
    transform: translateX(-20px);
  }
  to {
    opacity: 1;
    transform: translateX(0);
  }
}

.list-rank {
  font-size: 0.95rem;
  font-weight: 700;
  color: rgba(255, 255, 255, 0.35);
  min-width: 32px;
  text-align: center;
}

.list-avatar {
  width: 32px;
  height: 32px;
  border-radius: 50%;
  object-fit: cover;
  background: #1a1035;
  border: 2px solid rgba(255, 255, 255, 0.1);
  flex-shrink: 0;
}

.list-info {
  flex: 1;
  min-width: 0;
}

.list-email {
  font-size: 0.85rem;
  color: rgba(255, 255, 255, 0.7);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.list-tokens {
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  flex-shrink: 0;
}

.list-token-value {
  font-size: 1.1rem;
  font-weight: 800;
  color: #ff4444;
  line-height: 1.1;
}

/* ==================== Footer ==================== */
.lb-footer {
  text-align: center;
  margin-top: auto;
  padding-top: 1.5rem;
  position: relative;
  z-index: 1;
}

.lb-footer p {
  font-size: 0.75rem;
  color: rgba(255, 255, 255, 0.2);
}

/* ==================== Responsive ==================== */
@media (max-width: 640px) {
  .leaderboard-page {
    padding: 1rem 0.5rem;
    min-height: auto;
  }

  .lb-header {
    margin-bottom: 1.5rem;
  }

  .lb-title {
    font-size: 1.5rem;
  }

  .title-icon {
    font-size: 1.8rem;
  }

  .range-switcher {
    margin-top: 0.75rem;
  }

  .range-option {
    min-width: 60px;
    height: 30px;
    padding: 0 0.7rem;
    font-size: 0.8rem;
  }

  .podium-section {
    gap: 0.4rem;
    padding: 0 0.25rem;
    margin-bottom: 1.5rem;
  }

  .rank-1 {
    width: 120px;
  }

  .rank-2,
  .rank-3 {
    width: 95px;
  }

  .rank-1 .avatar-wrapper {
    width: 60px;
    height: 60px;
  }

  .avatar-wrapper {
    width: 48px;
    height: 48px;
  }

  .token-value {
    font-size: 1.05rem;
  }

  .token-value.champion {
    font-size: 1.2rem;
  }

  .user-email {
    font-size: 0.65rem;
    max-width: 80px;
  }

  .bar-1 { height: 60px; }
  .bar-2 { height: 40px; }
  .bar-3 { height: 28px; }

  .list-item {
    padding: 0.5rem 0.75rem;
    gap: 0.5rem;
  }

  .list-rank {
    min-width: 24px;
    font-size: 0.85rem;
  }

  .list-avatar {
    width: 28px;
    height: 28px;
  }

  .list-email {
    font-size: 0.75rem;
  }

  .list-token-value {
    font-size: 0.95rem;
  }
}

/* ==================== Highlight Current User Styles ==================== */
.me-badge {
  position: absolute;
  top: 10px;
  right: 10px;
  background: #007aff;
  color: #ffffff;
  font-size: 0.75rem;
  font-weight: 700;
  padding: 2px 8px;
  border-radius: 20px;
  box-shadow: 0 2px 8px rgba(0, 122, 255, 0.4);
  z-index: 10;
  border: 1px solid rgba(255, 255, 255, 0.2);
}

.podium-card.is-me {
  background: linear-gradient(180deg, rgba(0, 122, 255, 0.15) 0%, rgba(0, 122, 255, 0.04) 100%) !important;
  border: 1px solid rgba(0, 122, 255, 0.35) !important;
  box-shadow: 0 0 20px rgba(0, 122, 255, 0.25) !important;
}

.podium-card.is-me .avatar-wrapper {
  background: linear-gradient(135deg, #007aff, #00c6ff, #007aff) !important;
  box-shadow: 0 0 15px rgba(0, 122, 255, 0.4) !important;
}

.podium-card.is-me .podium-bar {
  background: linear-gradient(180deg, rgba(0, 122, 255, 0.3), rgba(0, 122, 255, 0.08)) !important;
}

.podium-card.is-me .token-value {
  color: #38bdf8 !important;
  text-shadow: 0 0 20px rgba(0, 122, 255, 0.4) !important;
  background: none !important;
  -webkit-background-clip: unset !important;
  -webkit-text-fill-color: unset !important;
}

.list-item.is-me {
  background: rgba(0, 122, 255, 0.08) !important;
  border-color: rgba(0, 122, 255, 0.3) !important;
  box-shadow: 0 0 15px rgba(0, 122, 255, 0.15) !important;
}

.list-item.is-me:hover {
  background: rgba(0, 122, 255, 0.12) !important;
  border-color: rgba(0, 122, 255, 0.45) !important;
}

.list-item.is-me .list-token-value {
  color: #38bdf8 !important;
}

.list-item.is-me .list-avatar {
  border-color: rgba(0, 122, 255, 0.4) !important;
}

.list-email-wrapper {
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.list-me {
  background: #007aff;
  color: #ffffff;
  font-size: 0.7rem;
  font-weight: 700;
  padding: 1px 6px;
  border-radius: 12px;
  flex-shrink: 0;
  box-shadow: 0 2px 6px rgba(0, 122, 255, 0.3);
  border: 1px solid rgba(255, 255, 255, 0.15);
  line-height: 1.2;
}

.user-rank-bottom {
  margin-top: 1.5rem;
  width: 100%;
}

.divider-line {
  height: 1px;
  background: linear-gradient(90deg, transparent, rgba(0, 122, 255, 0.4), transparent);
  margin-bottom: 1.5rem;
}

.bottom-me-item {
  animation: list-enter 0.5s ease backwards;
}

/* ==================== Light theme ==================== */
.leaderboard-page {
  background: linear-gradient(145deg, #ffffff 0%, #f4f8ff 52%, #f8fafc 100%);
  color: #0f172a;
  border-color: #dbe5f1;
  box-shadow: 0 18px 44px rgba(15, 23, 42, 0.1);
}

.particle {
  background: rgba(22, 119, 255, 0.14);
}

.header-glow {
  background: radial-gradient(circle, rgba(22, 119, 255, 0.11) 0%, transparent 70%);
}

.title-text {
  background: linear-gradient(135deg, #0f172a, #1677ff);
  -webkit-background-clip: text;
  background-clip: text;
  -webkit-text-fill-color: transparent;
}

.lb-subtitle,
.loading-text,
.error-text,
.empty-text {
  color: #64748b;
}

.range-switcher {
  background: rgba(255, 255, 255, 0.88);
  border-color: #dbe5f1;
  box-shadow: 0 5px 16px rgba(15, 23, 42, 0.06);
  backdrop-filter: blur(12px);
}

.range-option {
  color: #64748b;
}

.range-option:hover {
  color: #1677ff;
  background: #eff6ff;
}

.range-option:focus-visible,
.retry-btn:focus-visible {
  outline: 2px solid #1677ff;
  outline-offset: 2px;
}

.range-option.active,
.retry-btn {
  color: #ffffff;
  background: #1677ff;
  box-shadow: 0 5px 16px rgba(22, 119, 255, 0.22);
}

.retry-btn:hover {
  box-shadow: 0 7px 20px rgba(22, 119, 255, 0.26);
}

.rank-1 {
  background: linear-gradient(180deg, rgba(255, 215, 0, 0.16) 0%, rgba(255, 215, 0, 0.045) 100%);
  border-color: rgba(217, 164, 0, 0.28);
}

.rank-2 {
  background: linear-gradient(180deg, rgba(148, 163, 184, 0.15) 0%, rgba(148, 163, 184, 0.035) 100%);
  border-color: rgba(100, 116, 139, 0.22);
}

.rank-3 {
  background: linear-gradient(180deg, rgba(205, 127, 50, 0.14) 0%, rgba(205, 127, 50, 0.035) 100%);
  border-color: rgba(180, 105, 35, 0.24);
}

.avatar-img,
.list-avatar {
  background: #ffffff;
}

.user-email,
.list-email {
  color: #475569;
}

.list-section {
  gap: 0.5rem;
}

.list-item {
  background: rgba(255, 255, 255, 0.84);
  border-color: #dbe5f1;
  box-shadow: 0 4px 14px rgba(15, 23, 42, 0.045);
  backdrop-filter: blur(12px);
}

.list-item:hover {
  background: #ffffff;
  border-color: #b6d4fe;
  box-shadow: 0 7px 18px rgba(22, 119, 255, 0.1);
}

.list-rank {
  color: #94a3b8;
}

.list-avatar {
  border-color: #dbe5f1;
}

.lb-footer p {
  color: #94a3b8;
}

.podium-card.is-me {
  background: linear-gradient(180deg, rgba(22, 119, 255, 0.13) 0%, rgba(22, 119, 255, 0.035) 100%) !important;
  border-color: rgba(22, 119, 255, 0.34) !important;
  box-shadow: 0 8px 24px rgba(22, 119, 255, 0.14) !important;
}

.podium-card.is-me .token-value,
.list-item.is-me .list-token-value {
  color: #1677ff !important;
  text-shadow: none !important;
}

.list-item.is-me {
  background: #eff6ff !important;
  border-color: #91caff !important;
  box-shadow: 0 6px 18px rgba(22, 119, 255, 0.1) !important;
}

.list-item.is-me:hover {
  background: #e6f4ff !important;
  border-color: #69b1ff !important;
}

</style>

<style>
/* Preserve the existing high-contrast presentation in dark mode. */
.dark .leaderboard-page {
  background: linear-gradient(135deg, #0a0a1a 0%, #1a1035 30%, #0d1b2a 60%, #0a0a1a 100%);
  color: #e0e0e0;
  border-color: rgba(255, 255, 255, 0.05);
  box-shadow: 0 20px 40px rgba(0, 0, 0, 0.4);
}

.dark .leaderboard-page .particle {
  background: rgba(255, 255, 255, 0.15);
}

.dark .leaderboard-page .header-glow {
  background: radial-gradient(circle, rgba(255, 215, 0, 0.08) 0%, transparent 70%);
}

.dark .leaderboard-page .title-text {
  background: linear-gradient(135deg, #ffd700, #ffaa00, #ffd700);
  -webkit-background-clip: text;
  background-clip: text;
  -webkit-text-fill-color: transparent;
}

.dark .leaderboard-page .lb-subtitle,
.dark .leaderboard-page .loading-text {
  color: rgba(255, 255, 255, 0.5);
}

.dark .leaderboard-page .error-text,
.dark .leaderboard-page .empty-text {
  color: rgba(255, 255, 255, 0.6);
}

.dark .leaderboard-page .range-switcher {
  background: rgba(255, 255, 255, 0.06);
  border-color: rgba(255, 255, 255, 0.1);
  box-shadow: none;
}

.dark .leaderboard-page .range-option {
  color: rgba(255, 255, 255, 0.62);
}

.dark .leaderboard-page .range-option:hover {
  color: rgba(255, 255, 255, 0.88);
  background: rgba(255, 255, 255, 0.07);
}

.dark .leaderboard-page .range-option.active,
.dark .leaderboard-page .retry-btn {
  color: #1a1a2e;
  background: linear-gradient(135deg, #ffd700, #ffaa00);
  box-shadow: 0 4px 16px rgba(255, 215, 0, 0.22);
}

.dark .leaderboard-page .rank-1 {
  background: linear-gradient(180deg, rgba(255, 215, 0, 0.08) 0%, rgba(255, 215, 0, 0.02) 100%);
  border-color: rgba(255, 215, 0, 0.15);
}

.dark .leaderboard-page .rank-2 {
  background: linear-gradient(180deg, rgba(192, 192, 192, 0.08) 0%, rgba(192, 192, 192, 0.02) 100%);
  border-color: rgba(192, 192, 192, 0.12);
}

.dark .leaderboard-page .rank-3 {
  background: linear-gradient(180deg, rgba(205, 127, 50, 0.08) 0%, rgba(205, 127, 50, 0.02) 100%);
  border-color: rgba(205, 127, 50, 0.12);
}

.dark .leaderboard-page .avatar-img,
.dark .leaderboard-page .list-avatar {
  background: #1a1035;
}

.dark .leaderboard-page .user-email {
  color: rgba(255, 255, 255, 0.6);
}

.dark .leaderboard-page .list-email {
  color: rgba(255, 255, 255, 0.7);
}

.dark .leaderboard-page .list-section {
  gap: 0.4rem;
}

.dark .leaderboard-page .list-item {
  background: rgba(255, 255, 255, 0.03);
  border-color: rgba(255, 255, 255, 0.06);
  box-shadow: none;
}

.dark .leaderboard-page .list-item:hover {
  background: rgba(255, 255, 255, 0.06);
  border-color: rgba(255, 255, 255, 0.1);
  box-shadow: none;
}

.dark .leaderboard-page .list-rank {
  color: rgba(255, 255, 255, 0.35);
}

.dark .leaderboard-page .list-avatar {
  border-color: rgba(255, 255, 255, 0.1);
}

.dark .leaderboard-page .lb-footer p {
  color: rgba(255, 255, 255, 0.2);
}

.dark .leaderboard-page .podium-card.is-me {
  background: linear-gradient(180deg, rgba(0, 122, 255, 0.15) 0%, rgba(0, 122, 255, 0.04) 100%) !important;
  border-color: rgba(0, 122, 255, 0.35) !important;
  box-shadow: 0 0 20px rgba(0, 122, 255, 0.25) !important;
}

.dark .leaderboard-page .podium-card.is-me .token-value,
.dark .leaderboard-page .list-item.is-me .list-token-value {
  color: #38bdf8 !important;
  text-shadow: 0 0 20px rgba(0, 122, 255, 0.4) !important;
}

.dark .leaderboard-page .list-item.is-me {
  background: rgba(0, 122, 255, 0.08) !important;
  border-color: rgba(0, 122, 255, 0.3) !important;
  box-shadow: 0 0 15px rgba(0, 122, 255, 0.15) !important;
}

.dark .leaderboard-page .list-item.is-me:hover {
  background: rgba(0, 122, 255, 0.12) !important;
  border-color: rgba(0, 122, 255, 0.45) !important;
}

</style>

<style scoped>
@media (prefers-reduced-motion: reduce) {
  .particle,
  .title-icon,
  .crown,
  .bar-shine,
  .podium-card,
  .list-item,
  .loader-ring {
    animation: none !important;
  }

  .podium-card:hover,
  .list-item:hover,
  .retry-btn:hover {
    transform: none;
  }
}
</style>

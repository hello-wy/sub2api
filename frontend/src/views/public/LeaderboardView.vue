<template>
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
        <span class="title-text">今日排行榜</span>
      </h1>
      <p class="lb-subtitle">{{ todayFormatted }}</p>
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
      <p class="empty-text">今日暂无数据</p>
    </div>

    <!-- Leaderboard content -->
    <div v-else class="lb-content">
      <!-- Podium: Top 3 -->
      <div class="podium-section">
        <!-- 2nd place (left) -->
        <div v-if="ranking.length >= 2" class="podium-card rank-2" @mouseenter="hoveredCard = 2" @mouseleave="hoveredCard = null">
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
          <div class="token-value">{{ formatTokens(ranking[1].tokens) }}</div>
          <div class="token-label">令牌数</div>
          <div class="podium-bar bar-2">
            <div class="bar-shine" />
          </div>
        </div>

        <!-- 1st place (center) -->
        <div v-if="ranking.length >= 1" class="podium-card rank-1" @mouseenter="hoveredCard = 1" @mouseleave="hoveredCard = null">
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
          <div class="token-value champion">{{ formatTokens(ranking[0].tokens) }}</div>
          <div class="token-label">令牌数</div>
          <div class="podium-bar bar-1">
            <div class="bar-shine" />
          </div>
        </div>

        <!-- 3rd place (right) -->
        <div v-if="ranking.length >= 3" class="podium-card rank-3" @mouseenter="hoveredCard = 3" @mouseleave="hoveredCard = null">
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
          <div class="token-value">{{ formatTokens(ranking[2].tokens) }}</div>
          <div class="token-label">令牌数</div>
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
            <div class="list-email" :title="user.email">{{ maskEmail(user.email) }}</div>
          </div>
          <div class="list-tokens">
            <span class="list-token-value">{{ formatTokens(user.tokens) }}</span>
            <span class="list-token-label">令牌数</span>
          </div>
        </div>
      </div>
    </div>

    <!-- Footer -->
    <footer class="lb-footer">
      <p>每 60 秒自动刷新</p>
    </footer>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { getUserSpendingRanking } from '@/api/admin/dashboard'
import type { UserSpendingRankingItem } from '@/types'

// State
const ranking = ref<UserSpendingRankingItem[]>([])
const loading = ref(true)
const error = ref<string | null>(null)
const hoveredCard = ref<number | null>(null)
let refreshTimer: ReturnType<typeof setInterval> | null = null

// Today's date formatted
const today = computed(() => {
  const now = new Date()
  const year = now.getFullYear()
  const month = String(now.getMonth() + 1).padStart(2, '0')
  const day = String(now.getDate()).padStart(2, '0')
  return `${year}-${month}-${day}`
})

const todayFormatted = computed(() => {
  const now = new Date()
  return now.toLocaleDateString('zh-CN', {
    year: 'numeric',
    month: 'long',
    day: 'numeric',
    weekday: 'long'
  })
})

// DiceBear Notionists avatar URL
function getAvatarUrl(email: string): string {
  const seed = encodeURIComponent(email)
  return `https://api.dicebear.com/9.x/notionists/svg?seed=${seed}&backgroundColor=transparent`
}

// Mask email for privacy: show first 3 chars + ***@domain
function maskEmail(email: string): string {
  if (!email) return '***'
  const [local, domain] = email.split('@')
  if (!domain) return email
  const visible = local.slice(0, 3)
  return `${visible}***@${domain}`
}

// Format tokens with abbreviation: 22782478 → 22.8M
function formatTokens(tokens: number): string {
  if (tokens >= 1_000_000_000) {
    return (tokens / 1_000_000_000).toFixed(1).replace(/\.0$/, '') + 'B'
  }
  if (tokens >= 1_000_000) {
    return (tokens / 1_000_000).toFixed(1).replace(/\.0$/, '') + 'M'
  }
  if (tokens >= 1_000) {
    return (tokens / 1_000).toFixed(1).replace(/\.0$/, '') + 'K'
  }
  return tokens.toString()
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
    const response = await getUserSpendingRanking({
      start_date: today.value,
      end_date: today.value,
      limit: 10
    })
    ranking.value = response.ranking || []
  } catch (err: any) {
    error.value = err?.message || 'Failed to load ranking data'
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  fetchRanking()
  // Auto-refresh every 60 seconds
  refreshTimer = setInterval(fetchRanking, 60_000)
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
  min-height: 100vh;
  background: linear-gradient(135deg, #0a0a1a 0%, #1a1035 30%, #0d1b2a 60%, #0a0a1a 100%);
  color: #e0e0e0;
  font-family: 'Inter', 'Segoe UI', system-ui, -apple-system, sans-serif;
  position: relative;
  overflow-x: hidden;
  padding: 2rem 1rem;
}

/* Animated particles */
.bg-particles {
  position: fixed;
  top: 0;
  left: 0;
  width: 100%;
  height: 100%;
  pointer-events: none;
  z-index: 0;
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
  margin-bottom: 3rem;
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
  font-size: 2.5rem;
  font-weight: 800;
  margin: 0 0 0.5rem;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 0.75rem;
}

.title-icon {
  font-size: 2.8rem;
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
  font-size: 1rem;
  color: rgba(255, 255, 255, 0.5);
  margin: 0;
  letter-spacing: 0.5px;
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
  max-width: 900px;
  margin: 0 auto;
  position: relative;
  z-index: 1;
}

/* ==================== Podium Section ==================== */
.podium-section {
  display: flex;
  align-items: flex-end;
  justify-content: center;
  gap: 1rem;
  margin-bottom: 2.5rem;
  padding: 0 1rem;
}

.podium-card {
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 1.5rem 1rem 0;
  border-radius: 20px 20px 0 0;
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
  width: 200px;
  animation-delay: 0.1s;
}

/* Rank 2 - Silver (left) */
.rank-2 {
  background: linear-gradient(180deg, rgba(192, 192, 192, 0.08) 0%, rgba(192, 192, 192, 0.02) 100%);
  border: 1px solid rgba(192, 192, 192, 0.12);
  width: 170px;
  animation-delay: 0.2s;
}

/* Rank 3 - Bronze (right) */
.rank-3 {
  background: linear-gradient(180deg, rgba(205, 127, 50, 0.08) 0%, rgba(205, 127, 50, 0.02) 100%);
  border: 1px solid rgba(205, 127, 50, 0.12);
  width: 170px;
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
  top: -18px;
  font-size: 2rem;
  animation: crown-float 3s ease-in-out infinite;
}

@keyframes crown-float {
  0%, 100% { transform: translateY(0) rotate(-5deg); }
  50% { transform: translateY(-4px) rotate(5deg); }
}

/* Rank badges */
.rank-badge {
  width: 32px;
  height: 32px;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  font-weight: 800;
  font-size: 0.9rem;
  margin-bottom: 0.75rem;
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
  width: 80px;
  height: 80px;
  border-radius: 50%;
  padding: 3px;
  margin-bottom: 0.75rem;
  position: relative;
}

.rank-1 .avatar-wrapper {
  width: 100px;
  height: 100px;
}

.gold-glow {
  background: linear-gradient(135deg, #ffd700, #f0a500, #ffd700);
  box-shadow: 0 0 20px rgba(255, 215, 0, 0.3);
}

.silver-glow {
  background: linear-gradient(135deg, #e8e8e8, #a0a0a0, #e8e8e8);
  box-shadow: 0 0 15px rgba(192, 192, 192, 0.2);
}

.bronze-glow {
  background: linear-gradient(135deg, #cd7f32, #a0522d, #cd7f32);
  box-shadow: 0 0 15px rgba(205, 127, 50, 0.2);
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
  font-size: 0.8rem;
  color: rgba(255, 255, 255, 0.6);
  margin-bottom: 0.5rem;
  max-width: 160px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  text-align: center;
}

.token-value {
  font-size: 1.6rem;
  font-weight: 800;
  color: #ff4444;
  line-height: 1;
  text-shadow: 0 0 20px rgba(255, 68, 68, 0.3);
}

.token-value.champion {
  font-size: 2rem;
  background: linear-gradient(135deg, #ff4444, #ff6b6b);
  -webkit-background-clip: text;
  background-clip: text;
  -webkit-text-fill-color: transparent;
  text-shadow: none;
  filter: drop-shadow(0 0 10px rgba(255, 68, 68, 0.4));
}

.token-label {
  font-size: 0.7rem;
  color: rgba(255, 255, 255, 0.35);
  text-transform: uppercase;
  letter-spacing: 1.5px;
  margin-top: 2px;
  margin-bottom: 1rem;
}

/* Podium bars */
.podium-bar {
  width: 100%;
  border-radius: 0;
  position: relative;
  overflow: hidden;
}

.bar-1 {
  height: 100px;
  background: linear-gradient(180deg, rgba(255, 215, 0, 0.2), rgba(255, 215, 0, 0.05));
}

.bar-2 {
  height: 70px;
  background: linear-gradient(180deg, rgba(192, 192, 192, 0.15), rgba(192, 192, 192, 0.05));
}

.bar-3 {
  height: 50px;
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
  gap: 0.5rem;
}

.list-item {
  display: flex;
  align-items: center;
  gap: 1rem;
  padding: 0.85rem 1.25rem;
  background: rgba(255, 255, 255, 0.03);
  border: 1px solid rgba(255, 255, 255, 0.06);
  border-radius: 14px;
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
  font-size: 1.1rem;
  font-weight: 700;
  color: rgba(255, 255, 255, 0.35);
  min-width: 36px;
  text-align: center;
}

.list-avatar {
  width: 42px;
  height: 42px;
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
  font-size: 0.9rem;
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
  font-size: 1.3rem;
  font-weight: 800;
  color: #ff4444;
  line-height: 1.1;
}

.list-token-label {
  font-size: 0.65rem;
  color: rgba(255, 255, 255, 0.3);
  text-transform: uppercase;
  letter-spacing: 1px;
}

/* ==================== Footer ==================== */
.lb-footer {
  text-align: center;
  margin-top: 3rem;
  position: relative;
  z-index: 1;
}

.lb-footer p {
  font-size: 0.8rem;
  color: rgba(255, 255, 255, 0.2);
}

/* ==================== Responsive ==================== */
@media (max-width: 640px) {
  .leaderboard-page {
    padding: 1.5rem 0.75rem;
  }

  .lb-title {
    font-size: 1.8rem;
  }

  .podium-section {
    gap: 0.5rem;
    padding: 0 0.5rem;
  }

  .rank-1 {
    width: 140px;
  }

  .rank-2,
  .rank-3 {
    width: 110px;
  }

  .rank-1 .avatar-wrapper {
    width: 72px;
    height: 72px;
  }

  .avatar-wrapper {
    width: 56px;
    height: 56px;
  }

  .token-value {
    font-size: 1.3rem;
  }

  .token-value.champion {
    font-size: 1.5rem;
  }

  .user-email {
    font-size: 0.7rem;
    max-width: 100px;
  }

  .bar-1 { height: 70px; }
  .bar-2 { height: 50px; }
  .bar-3 { height: 35px; }

  .list-item {
    padding: 0.7rem 0.85rem;
    gap: 0.7rem;
  }

  .list-token-value {
    font-size: 1.1rem;
  }
}
</style>

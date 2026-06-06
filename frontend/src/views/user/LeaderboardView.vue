<template>
  <AppLayout>
    <div class="space-y-6">
      <div class="card p-5">
        <div class="flex flex-col gap-4 lg:flex-row lg:items-end lg:justify-between">
          <div class="min-w-0">
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('leaderboard.title') }}</h2>
            <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t('leaderboard.privacyHint') }}</p>
            <p v-if="participant?.public_name" class="mt-2 text-sm text-gray-700 dark:text-gray-200">
              {{ t('leaderboard.publicName') }}:
              <span class="font-semibold">{{ participant.public_name }}</span>
            </p>
            <div
              v-if="nextRefreshAt"
              class="mt-2 inline-flex items-center gap-2 rounded-full border border-gray-200 bg-gray-50 px-3 py-1.5 text-xs font-medium text-gray-600 dark:border-dark-600 dark:bg-dark-800 dark:text-gray-300"
            >
              <Icon name="refresh" size="sm" class="text-primary-500" />
              <span>{{ t('leaderboard.nextRefresh') }}: {{ formatDateTime(nextRefreshAt) }}</span>
            </div>
          </div>

          <form class="grid gap-3 sm:grid-cols-[minmax(0,220px)_auto_auto]" @submit.prevent="saveSettings">
            <label class="block">
              <span class="input-label">{{ t('leaderboard.nickname') }}</span>
              <input
                v-model="displayName"
                class="input w-full"
                maxlength="32"
                :placeholder="t('leaderboard.nicknamePlaceholder')"
              />
            </label>
            <label class="flex items-center gap-2 self-end rounded-md border border-gray-200 px-3 py-2 dark:border-dark-600">
              <input v-model="isOptedIn" type="checkbox" class="h-4 w-4 rounded border-gray-300 text-primary-600" />
              <span class="text-sm font-medium text-gray-700 dark:text-gray-200">{{ t('leaderboard.optIn') }}</span>
            </label>
            <button type="submit" class="btn btn-primary self-end" :disabled="saving">
              {{ saving ? t('common.saving') : t('common.save') }}
            </button>
          </form>
        </div>
        <p v-if="errorMessage" class="mt-3 text-sm text-red-600 dark:text-red-400">{{ errorMessage }}</p>
      </div>

      <div v-if="loading" class="flex items-center justify-center py-12">
        <LoadingSpinner />
      </div>

      <div v-else class="grid grid-cols-1 gap-5 xl:grid-cols-2">
        <LeaderboardPanel
          v-for="panel in panels"
          :key="panel.key"
          :title="panel.title"
          :subtitle="panel.subtitle"
          :window="panel.window"
        />
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, defineComponent, h, onMounted, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import Icon from '@/components/icons/Icon.vue'
import { leaderboardAPI } from '@/api/leaderboard'
import type { LeaderboardEntry, LeaderboardOverview, LeaderboardWindowOverview } from '@/types'
import { extractApiErrorMessage } from '@/utils/apiError'

const { t } = useI18n()
const LEADERBOARD_REFRESH_INTERVAL_MS = 15 * 60 * 1000

const overview = ref<LeaderboardOverview | null>(null)
const loading = ref(false)
const saving = ref(false)
const isOptedIn = ref(false)
const displayName = ref('')
const errorMessage = ref('')
const nextRefreshAt = ref<Date | null>(null)
let refreshTimer: number | null = null

const participant = computed(() => overview.value?.participant ?? null)

const panels = computed(() => {
  if (!overview.value) return []
  return [
    { key: 'daily', title: t('leaderboard.daily'), subtitle: formatWindowRange(overview.value.daily), window: overview.value.daily },
    { key: 'weekly', title: t('leaderboard.weekly'), subtitle: formatWindowRange(overview.value.weekly), window: overview.value.weekly },
    { key: 'monthly', title: t('leaderboard.monthly'), subtitle: formatWindowRange(overview.value.monthly), window: overview.value.monthly },
    { key: 'all_time', title: t('leaderboard.allTime'), subtitle: t('leaderboard.allTimeSubtitle'), window: overview.value.all_time },
  ]
})

function syncSettings() {
  const p = participant.value
  isOptedIn.value = p?.is_opted_in ?? false
  displayName.value = p?.display_name ?? ''
}

function clearRefreshTimer() {
  if (refreshTimer !== null) {
    window.clearTimeout(refreshTimer)
    refreshTimer = null
  }
}

function scheduleAutoRefresh() {
  clearRefreshTimer()
  nextRefreshAt.value = new Date(Date.now() + LEADERBOARD_REFRESH_INTERVAL_MS)
  refreshTimer = window.setTimeout(() => {
    refreshTimer = null
    void loadOverview(true)
  }, LEADERBOARD_REFRESH_INTERVAL_MS)
}

async function loadOverview(silent = false) {
  const shouldShowLoading = !silent || !overview.value
  if (shouldShowLoading) {
    loading.value = true
  }
  errorMessage.value = ''
  try {
    overview.value = await leaderboardAPI.getOverview()
    syncSettings()
  } catch (error) {
    errorMessage.value = extractApiErrorMessage(error, t('leaderboard.loadFailed'))
  } finally {
    loading.value = false
    scheduleAutoRefresh()
  }
}

async function saveSettings() {
  saving.value = true
  errorMessage.value = ''
  try {
    await leaderboardAPI.updateMe({
      is_opted_in: isOptedIn.value,
      display_name: displayName.value.trim() || null,
    })
    await loadOverview(true)
  } catch (error) {
    errorMessage.value = extractApiErrorMessage(error, t('leaderboard.saveFailed'))
  } finally {
    saving.value = false
  }
}

function formatWindowRange(window: LeaderboardWindowOverview): string {
  if (!window.starts_at) return ''
  const start = new Date(window.starts_at)
  const end = new Date(window.ends_at)
  return `${formatDateTime(start)} - ${formatDateTime(end)}`
}

function formatDateTime(value: Date): string {
  return new Intl.DateTimeFormat(undefined, {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  }).format(value)
}

function formatTokens(tokens: number): string {
  if (tokens >= 1_000_000_000) return `${(tokens / 1_000_000_000).toFixed(2)}B`
  if (tokens >= 1_000_000) return `${(tokens / 1_000_000).toFixed(2)}M`
  if (tokens >= 1_000) return `${(tokens / 1_000).toFixed(1)}K`
  return tokens.toLocaleString()
}

type MetricTone =
  | 'tokensLow'
  | 'tokensMid'
  | 'tokensHigh'
  | 'tokensUltra'
  | 'requestsLow'
  | 'requestsMid'
  | 'requestsHigh'
  | 'requestsUltra'
  | 'streakLow'
  | 'streakMid'
  | 'streakHigh'
  | 'streakUltra'
  | 'goldLow'
  | 'goldMid'
  | 'goldHigh'
  | 'goldUltra'
  | 'silverLow'
  | 'silverMid'
  | 'silverHigh'
  | 'silverUltra'
  | 'bronzeLow'
  | 'bronzeMid'
  | 'bronzeHigh'
  | 'bronzeUltra'

const METRIC_CHIP_STYLES: Record<MetricTone, { shellClass: string; dotClass: string }> = {
  tokensLow: {
    shellClass:
      'border-emerald-200/80 bg-gradient-to-br from-emerald-50 via-white to-teal-50 text-emerald-700 shadow-[0_8px_18px_-16px_rgba(16,185,129,0.7)] dark:border-emerald-500/25 dark:from-emerald-500/10 dark:via-dark-700/70 dark:to-teal-500/10 dark:text-emerald-100',
    dotClass: 'bg-emerald-500 dark:bg-emerald-300',
  },
  tokensMid: {
    shellClass:
      'border-sky-200/80 bg-gradient-to-br from-sky-50 via-white to-cyan-50 text-sky-700 shadow-[0_8px_18px_-16px_rgba(14,165,233,0.75)] dark:border-sky-500/25 dark:from-sky-500/10 dark:via-dark-700/70 dark:to-cyan-500/10 dark:text-sky-100',
    dotClass: 'bg-sky-500 dark:bg-sky-300',
  },
  tokensHigh: {
    shellClass:
      'border-fuchsia-200/80 bg-gradient-to-br from-fuchsia-50 via-white to-violet-50 text-fuchsia-700 shadow-[0_8px_18px_-16px_rgba(217,70,239,0.72)] dark:border-fuchsia-500/25 dark:from-fuchsia-500/10 dark:via-dark-700/70 dark:to-violet-500/10 dark:text-fuchsia-100',
    dotClass: 'bg-fuchsia-500 dark:bg-fuchsia-300',
  },
  tokensUltra: {
    shellClass:
      'border-amber-200/90 bg-gradient-to-br from-amber-50 via-rose-50 to-fuchsia-50 text-amber-800 shadow-[0_10px_22px_-15px_rgba(245,158,11,0.85)] dark:border-amber-400/30 dark:from-amber-500/15 dark:via-rose-500/10 dark:to-fuchsia-500/15 dark:text-amber-100',
    dotClass: 'bg-gradient-to-br from-amber-400 to-fuchsia-500 dark:from-amber-300 dark:to-fuchsia-300',
  },
  requestsLow: {
    shellClass:
      'border-indigo-200/80 bg-gradient-to-br from-indigo-50 via-white to-cyan-50 text-indigo-700 shadow-[0_8px_18px_-16px_rgba(79,70,229,0.72)] dark:border-indigo-500/25 dark:from-indigo-500/10 dark:via-dark-700/70 dark:to-cyan-500/10 dark:text-indigo-100',
    dotClass: 'bg-indigo-500 dark:bg-indigo-300',
  },
  requestsMid: {
    shellClass:
      'border-orange-200/80 bg-gradient-to-br from-orange-50 via-white to-amber-50 text-orange-700 shadow-[0_8px_18px_-16px_rgba(249,115,22,0.72)] dark:border-orange-500/25 dark:from-orange-500/10 dark:via-dark-700/70 dark:to-amber-500/10 dark:text-orange-100',
    dotClass: 'bg-orange-500 dark:bg-orange-300',
  },
  requestsHigh: {
    shellClass:
      'border-rose-200/80 bg-gradient-to-br from-rose-50 via-white to-pink-50 text-rose-700 shadow-[0_8px_18px_-16px_rgba(244,63,94,0.72)] dark:border-rose-500/25 dark:from-rose-500/10 dark:via-dark-700/70 dark:to-pink-500/10 dark:text-rose-100',
    dotClass: 'bg-rose-500 dark:bg-rose-300',
  },
  requestsUltra: {
    shellClass:
      'border-red-200/90 bg-gradient-to-br from-red-50 via-orange-50 to-amber-50 text-red-700 shadow-[0_10px_22px_-15px_rgba(220,38,38,0.82)] dark:border-red-400/30 dark:from-red-500/15 dark:via-orange-500/10 dark:to-amber-500/15 dark:text-red-100',
    dotClass: 'bg-gradient-to-br from-red-500 to-amber-400 dark:from-red-300 dark:to-amber-300',
  },
  streakLow: {
    shellClass:
      'border-lime-200/80 bg-gradient-to-br from-lime-50 via-white to-emerald-50 text-lime-700 shadow-[0_8px_18px_-16px_rgba(132,204,22,0.72)] dark:border-lime-500/25 dark:from-lime-500/10 dark:via-dark-700/70 dark:to-emerald-500/10 dark:text-lime-100',
    dotClass: 'bg-lime-500 dark:bg-lime-300',
  },
  streakMid: {
    shellClass:
      'border-amber-200/80 bg-gradient-to-br from-amber-50 via-white to-orange-50 text-amber-700 shadow-[0_8px_18px_-16px_rgba(245,158,11,0.75)] dark:border-amber-500/25 dark:from-amber-500/10 dark:via-dark-700/70 dark:to-orange-500/10 dark:text-amber-100',
    dotClass: 'bg-amber-500 dark:bg-amber-300',
  },
  streakHigh: {
    shellClass:
      'border-orange-200/80 bg-gradient-to-br from-orange-50 via-white to-rose-50 text-orange-700 shadow-[0_8px_18px_-16px_rgba(249,115,22,0.78)] dark:border-orange-500/25 dark:from-orange-500/10 dark:via-dark-700/70 dark:to-rose-500/10 dark:text-orange-100',
    dotClass: 'bg-orange-500 dark:bg-orange-300',
  },
  streakUltra: {
    shellClass:
      'border-violet-200/90 bg-gradient-to-br from-violet-50 via-fuchsia-50 to-amber-50 text-violet-700 shadow-[0_10px_22px_-15px_rgba(124,58,237,0.78)] dark:border-violet-400/30 dark:from-violet-500/15 dark:via-fuchsia-500/10 dark:to-amber-500/15 dark:text-violet-100',
    dotClass: 'bg-gradient-to-br from-violet-500 to-amber-400 dark:from-violet-300 dark:to-amber-300',
  },
  goldLow: {
    shellClass:
      'border-amber-300/90 bg-gradient-to-br from-amber-50 via-yellow-50 to-orange-50 text-amber-800 shadow-[0_8px_18px_-16px_rgba(245,158,11,0.85)] dark:border-amber-400/35 dark:from-amber-500/15 dark:via-yellow-500/10 dark:to-orange-500/10 dark:text-amber-100',
    dotClass: 'bg-amber-500 dark:bg-amber-300',
  },
  goldMid: {
    shellClass:
      'border-yellow-300/90 bg-gradient-to-br from-yellow-50 via-amber-50 to-orange-100 text-yellow-900 shadow-[0_8px_18px_-16px_rgba(234,179,8,0.85)] dark:border-yellow-400/35 dark:from-yellow-500/15 dark:via-amber-500/10 dark:to-orange-500/10 dark:text-yellow-100',
    dotClass: 'bg-gradient-to-br from-yellow-400 to-amber-600 dark:from-yellow-300 dark:to-amber-300',
  },
  goldHigh: {
    shellClass:
      'border-amber-400/90 bg-gradient-to-br from-amber-100 via-yellow-50 to-orange-100 text-amber-900 shadow-[0_10px_22px_-15px_rgba(217,119,6,0.9)] dark:border-amber-300/40 dark:from-amber-400/20 dark:via-yellow-500/10 dark:to-orange-500/15 dark:text-amber-100',
    dotClass: 'bg-gradient-to-br from-amber-400 to-orange-600 dark:from-amber-300 dark:to-orange-300',
  },
  goldUltra: {
    shellClass:
      'border-yellow-400 bg-gradient-to-br from-yellow-100 via-amber-100 to-orange-200 text-amber-950 shadow-[0_10px_24px_-15px_rgba(180,83,9,0.95)] dark:border-yellow-300/45 dark:from-yellow-400/20 dark:via-amber-500/15 dark:to-orange-500/20 dark:text-yellow-50',
    dotClass: 'bg-gradient-to-br from-yellow-300 to-orange-600 dark:from-yellow-200 dark:to-orange-300',
  },
  silverLow: {
    shellClass:
      'border-slate-200/90 bg-gradient-to-br from-slate-50 via-white to-sky-50 text-slate-700 shadow-[0_8px_18px_-16px_rgba(100,116,139,0.72)] dark:border-slate-400/25 dark:from-slate-500/10 dark:via-dark-700/70 dark:to-sky-500/10 dark:text-slate-100',
    dotClass: 'bg-slate-400 dark:bg-slate-300',
  },
  silverMid: {
    shellClass:
      'border-slate-300/90 bg-gradient-to-br from-slate-100 via-white to-indigo-50 text-slate-800 shadow-[0_8px_18px_-16px_rgba(71,85,105,0.76)] dark:border-slate-400/30 dark:from-slate-400/15 dark:via-dark-700/70 dark:to-indigo-500/10 dark:text-slate-100',
    dotClass: 'bg-gradient-to-br from-slate-300 to-indigo-400 dark:from-slate-200 dark:to-indigo-300',
  },
  silverHigh: {
    shellClass:
      'border-indigo-200/90 bg-gradient-to-br from-indigo-50 via-slate-50 to-cyan-50 text-indigo-700 shadow-[0_10px_22px_-15px_rgba(99,102,241,0.78)] dark:border-indigo-400/30 dark:from-indigo-500/15 dark:via-slate-500/10 dark:to-cyan-500/10 dark:text-indigo-100',
    dotClass: 'bg-gradient-to-br from-indigo-500 to-slate-400 dark:from-indigo-300 dark:to-slate-200',
  },
  silverUltra: {
    shellClass:
      'border-cyan-200/90 bg-gradient-to-br from-cyan-50 via-slate-50 to-violet-50 text-cyan-700 shadow-[0_10px_24px_-15px_rgba(6,182,212,0.82)] dark:border-cyan-400/30 dark:from-cyan-500/15 dark:via-slate-500/10 dark:to-violet-500/15 dark:text-cyan-100',
    dotClass: 'bg-gradient-to-br from-cyan-500 to-violet-500 dark:from-cyan-300 dark:to-violet-300',
  },
  bronzeLow: {
    shellClass:
      'border-stone-300/90 bg-gradient-to-br from-stone-100 via-orange-50 to-stone-50 text-stone-800 shadow-[0_8px_18px_-16px_rgba(120,113,108,0.78)] dark:border-stone-400/30 dark:from-stone-500/15 dark:via-orange-500/10 dark:to-dark-700 dark:text-stone-100',
    dotClass: 'bg-stone-500 dark:bg-stone-300',
  },
  bronzeMid: {
    shellClass:
      'border-orange-700/35 bg-gradient-to-br from-orange-100 via-stone-100 to-red-50 text-orange-950 shadow-[0_8px_18px_-16px_rgba(194,65,12,0.78)] dark:border-orange-500/35 dark:from-orange-600/18 dark:via-stone-500/12 dark:to-red-500/10 dark:text-orange-100',
    dotClass: 'bg-gradient-to-br from-orange-700 to-stone-500 dark:from-orange-300 dark:to-stone-300',
  },
  bronzeHigh: {
    shellClass:
      'border-orange-800/45 bg-gradient-to-br from-orange-100 via-red-50 to-stone-100 text-orange-950 shadow-[0_10px_22px_-15px_rgba(154,52,18,0.85)] dark:border-orange-400/40 dark:from-orange-700/20 dark:via-red-500/12 dark:to-stone-500/12 dark:text-orange-100',
    dotClass: 'bg-gradient-to-br from-orange-800 to-red-700 dark:from-orange-300 dark:to-red-300',
  },
  bronzeUltra: {
    shellClass:
      'border-red-900/45 bg-gradient-to-br from-red-100 via-orange-100 to-stone-200 text-red-950 shadow-[0_10px_24px_-15px_rgba(127,29,29,0.9)] dark:border-red-400/40 dark:from-red-700/22 dark:via-orange-600/16 dark:to-stone-500/14 dark:text-red-100',
    dotClass: 'bg-gradient-to-br from-red-800 to-orange-700 dark:from-red-300 dark:to-orange-300',
  },
}

function getTokenMetricTone(tokens: number): MetricTone {
  if (tokens >= 1_000_000_000) return 'tokensUltra'
  if (tokens >= 100_000_000) return 'tokensHigh'
  if (tokens >= 10_000_000) return 'tokensMid'
  return 'tokensLow'
}

function getRequestMetricTone(requests: number): MetricTone {
  if (requests >= 1_000_000) return 'requestsUltra'
  if (requests >= 100_000) return 'requestsHigh'
  if (requests >= 10_000) return 'requestsMid'
  return 'requestsLow'
}

function getStreakMetricTone(streak: number): MetricTone {
  if (streak >= 30) return 'streakUltra'
  if (streak >= 15) return 'streakHigh'
  if (streak >= 5) return 'streakMid'
  return 'streakLow'
}

function getGoldMetricTone(champions: number): MetricTone {
  if (champions >= 200) return 'goldUltra'
  if (champions >= 50) return 'goldHigh'
  if (champions >= 10) return 'goldMid'
  return 'goldLow'
}

function getSilverMetricTone(count: number): MetricTone {
  if (count >= 200) return 'silverUltra'
  if (count >= 50) return 'silverHigh'
  if (count >= 10) return 'silverMid'
  return 'silverLow'
}

function getBronzeMetricTone(count: number): MetricTone {
  if (count >= 200) return 'bronzeUltra'
  if (count >= 50) return 'bronzeHigh'
  if (count >= 10) return 'bronzeMid'
  return 'bronzeLow'
}

function renderMetricChip(content: string, tone: MetricTone) {
  const style = METRIC_CHIP_STYLES[tone]
  return h(
    'span',
    {
      class: [
        'inline-flex items-center gap-1.5 rounded-full border px-2.5 py-1 text-[11px] font-semibold leading-none tracking-normal shadow-sm transition-all duration-300 hover:-translate-y-0.5',
        style.shellClass,
      ],
    },
    [
      h('span', { class: ['h-1.5 w-1.5 shrink-0 rounded-full', style.dotClass] }),
      h('span', { class: 'tabular-nums tracking-normal' }, content),
    ]
  )
}

type MedalKind = 'gold' | 'silver' | 'bronze'

const MEDAL_STYLES: Record<MedalKind, { labelClass: string; crownClass: string; tone: (count: number) => MetricTone }> = {
  gold: {
    labelClass: 'text-amber-700 dark:text-amber-100',
    crownClass: 'text-amber-500 dark:text-amber-300',
    tone: getGoldMetricTone,
  },
  silver: {
    labelClass: 'text-slate-700 dark:text-slate-100',
    crownClass: 'text-slate-400 dark:text-slate-200',
    tone: getSilverMetricTone,
  },
  bronze: {
    labelClass: 'text-orange-950 dark:text-orange-100',
    crownClass: 'text-orange-700 dark:text-orange-300',
    tone: getBronzeMetricTone,
  },
}

function renderMedalCrowns(kind: MedalKind, count: number) {
  if (!count || count <= 0) return null
  const style = MEDAL_STYLES[kind]
  const tone = style.tone(count)
  const crownCount = Math.min(count, 10)
  const children = []
  if (count > 10) {
    children.push(h('span', { class: ['text-[11px] font-black leading-none tracking-normal tabular-nums', style.labelClass] }, '10x'))
  }
  for (let i = 0; i < crownCount; i += 1) {
    children.push(renderCrownIcon(style.crownClass))
  }
  return h(
    'div',
    {
      class: [
        'inline-flex min-h-[28px] items-center gap-1 rounded-full border px-2.5 py-1 shadow-sm',
        METRIC_CHIP_STYLES[tone].shellClass,
      ],
      title: t(`leaderboard.${kind}Count`, { n: count }),
    },
    children
  )
}

function renderHonorWall(entry: LeaderboardEntry) {
  const medals = [
    renderMedalCrowns('gold', entry.champion_count || 0),
    renderMedalCrowns('silver', entry.runner_up_count || 0),
    renderMedalCrowns('bronze', entry.third_place_count || 0),
  ].filter(Boolean)
  const streak = entry.current_streak && entry.current_streak > 1
    ? renderMetricChip(t('leaderboard.streak', { n: entry.current_streak }), getStreakMetricTone(entry.current_streak))
    : null
  const runnerUpStreakCount = entry.longest_runner_up_streak || 0
  const hasRunnerUpStreak = runnerUpStreakCount > 1
  const runnerUpTitle = entry.perennial_runner_up && (entry.runner_up_count || 0) > 0
    ? renderMetricChip(t('leaderboard.perennialRunnerUp'), getSilverMetricTone(entry.runner_up_count || 0))
    : null
  const runnerUpStreak =
    !entry.perennial_runner_up && hasRunnerUpStreak
      ? renderMetricChip(t('leaderboard.runnerUpStreak', { n: runnerUpStreakCount }), getSilverMetricTone(runnerUpStreakCount))
      : null
  const items = [...medals, streak, runnerUpTitle, runnerUpStreak].filter(Boolean)
  if (!items.length) return null
  return h('div', { class: 'mt-2 flex flex-wrap items-center gap-1.5' }, items)
}

type RankStyle = {
  shellClass: string
  accentClass: string
  badgeClass: string
  badgeTextClass: string
  avatarRingClass: string
  avatarFallbackClass: string
  crownClass: string
  rankChipClass: string
}

type EntryVariant = 'default' | 'current'

const DEFAULT_RANK_STYLE: RankStyle = {
  shellClass: 'border-gray-200 bg-white/95 shadow-[0_10px_30px_-26px_rgba(15,23,42,0.35)] dark:border-dark-700 dark:bg-dark-800/55',
  accentClass: 'bg-gray-300 dark:bg-gray-600',
  badgeClass: 'bg-gray-100 text-gray-700 dark:bg-dark-700 dark:text-gray-200',
  badgeTextClass: 'text-gray-700 dark:text-gray-200',
  avatarRingClass: 'ring-gray-200 dark:ring-dark-600',
  avatarFallbackClass: 'bg-gradient-to-br from-gray-100 to-gray-200 text-gray-700 dark:from-dark-700 dark:to-dark-600 dark:text-white',
  crownClass: 'text-gray-500 dark:text-gray-300',
  rankChipClass: 'border-gray-200 bg-white text-gray-900 dark:border-dark-600 dark:bg-dark-700 dark:text-white',
}

const PODIUM_RANK_STYLES: Record<number, RankStyle> = {
  1: {
    shellClass:
      'border-amber-300 bg-gradient-to-r from-amber-50 via-white to-amber-100/50 shadow-[0_18px_40px_-28px_rgba(245,158,11,0.75)] dark:border-amber-500/40 dark:from-amber-950/30 dark:via-dark-800 dark:to-dark-900',
    accentClass: 'bg-gradient-to-b from-amber-400 to-amber-500',
    badgeClass: 'bg-amber-500/10 text-amber-700 ring-1 ring-amber-200 dark:bg-amber-500/15 dark:text-amber-200 dark:ring-amber-500/25',
    badgeTextClass: 'text-amber-700 dark:text-amber-200',
    avatarRingClass: 'ring-amber-200/80 dark:ring-amber-500/30',
    avatarFallbackClass: 'bg-gradient-to-br from-amber-100 to-amber-200 text-amber-800 dark:from-amber-500/20 dark:to-amber-400/10 dark:text-amber-100',
    crownClass: 'text-amber-500',
    rankChipClass: 'border-amber-200 bg-gradient-to-br from-amber-100 to-amber-50 text-amber-700 dark:border-amber-500/30 dark:from-amber-500/10 dark:to-amber-500/5 dark:text-amber-100',
  },
  2: {
    shellClass:
      'border-slate-300 bg-gradient-to-r from-slate-50 via-white to-slate-100/50 shadow-[0_16px_34px_-28px_rgba(100,116,139,0.7)] dark:border-slate-500/40 dark:from-slate-950/25 dark:via-dark-800 dark:to-dark-900',
    accentClass: 'bg-gradient-to-b from-slate-300 to-slate-500',
    badgeClass: 'bg-slate-500/10 text-slate-700 ring-1 ring-slate-200 dark:bg-slate-500/15 dark:text-slate-200 dark:ring-slate-500/25',
    badgeTextClass: 'text-slate-700 dark:text-slate-200',
    avatarRingClass: 'ring-slate-200/80 dark:ring-slate-500/30',
    avatarFallbackClass: 'bg-gradient-to-br from-slate-100 to-slate-200 text-slate-700 dark:from-slate-500/20 dark:to-slate-400/10 dark:text-slate-100',
    crownClass: 'text-slate-400',
    rankChipClass: 'border-slate-200 bg-gradient-to-br from-slate-100 to-slate-50 text-slate-700 dark:border-slate-500/30 dark:from-slate-500/10 dark:to-slate-500/5 dark:text-slate-100',
  },
  3: {
    shellClass:
      'border-orange-300 bg-gradient-to-r from-orange-50 via-white to-orange-100/50 shadow-[0_16px_34px_-28px_rgba(249,115,22,0.7)] dark:border-orange-500/40 dark:from-orange-950/25 dark:via-dark-800 dark:to-dark-900',
    accentClass: 'bg-gradient-to-b from-orange-300 to-orange-500',
    badgeClass: 'bg-orange-500/10 text-orange-700 ring-1 ring-orange-200 dark:bg-orange-500/15 dark:text-orange-200 dark:ring-orange-500/25',
    badgeTextClass: 'text-orange-700 dark:text-orange-200',
    avatarRingClass: 'ring-orange-200/80 dark:ring-orange-500/30',
    avatarFallbackClass: 'bg-gradient-to-br from-orange-100 to-orange-200 text-orange-800 dark:from-orange-500/20 dark:to-orange-400/10 dark:text-orange-100',
    crownClass: 'text-orange-500',
    rankChipClass: 'border-orange-200 bg-gradient-to-br from-orange-100 to-orange-50 text-orange-700 dark:border-orange-500/30 dark:from-orange-500/10 dark:to-orange-500/5 dark:text-orange-100',
  },
}

function getCurrentUserRankStyle(): RankStyle {
  return {
    shellClass:
      'border-primary-200/70 bg-gradient-to-br from-primary-50/80 via-white to-cyan-50/40 shadow-[0_14px_34px_-28px_rgba(14,165,233,0.55)] dark:border-primary-500/20 dark:from-primary-500/10 dark:via-dark-800/85 dark:to-cyan-500/10',
    accentClass: 'bg-gradient-to-b from-primary-400 to-cyan-400',
    badgeClass: 'bg-primary-500/10 text-primary-700 ring-1 ring-primary-200 dark:bg-primary-500/15 dark:text-primary-100 dark:ring-primary-500/25',
    badgeTextClass: 'text-primary-700 dark:text-primary-100',
    avatarRingClass: 'ring-primary-200/80 dark:ring-primary-500/30',
    avatarFallbackClass: 'bg-gradient-to-br from-primary-100 to-cyan-100 text-primary-800 dark:from-primary-500/20 dark:to-cyan-500/10 dark:text-primary-100',
    crownClass: 'text-primary-500',
    rankChipClass: 'border-primary-200 bg-gradient-to-br from-primary-100 to-primary-50 text-primary-700 dark:border-primary-500/20 dark:from-primary-500/10 dark:to-primary-500/5 dark:text-primary-100',
  }
}

function getRankStyle(rank?: number, variant: EntryVariant = 'default'): RankStyle {
  if (variant === 'current' && !isPodium(rank)) {
    return getCurrentUserRankStyle()
  }
  if (!rank || rank < 1) return DEFAULT_RANK_STYLE
  return PODIUM_RANK_STYLES[rank] ?? DEFAULT_RANK_STYLE
}

function isPodium(rank?: number): boolean {
  return rank === 1 || rank === 2 || rank === 3
}

function avatarInitial(name: string): string {
  const trimmed = name.trim()
  if (!trimmed) return '?'
  const cleaned = trimmed.replace(/^用户\s*#\s*/i, '')
  return cleaned.charAt(0).toUpperCase() || trimmed.charAt(0).toUpperCase() || '?'
}

function renderCrownIcon(className: string) {
  return h(
    'svg',
    {
      viewBox: '0 0 24 24',
      fill: 'none',
      stroke: 'currentColor',
      'stroke-width': '1.9',
      'stroke-linecap': 'round',
      'stroke-linejoin': 'round',
      class: ['h-3.5 w-3.5 shrink-0', className],
    },
    [
      h('path', { d: 'M4 8l3.5 3.5L12 5l4.5 6.5L20 8l-1.5 9H5.5L4 8z' }),
      h('path', { d: 'M5.5 17h13' }),
    ]
  )
}

function renderAvatar(entry: LeaderboardEntry, style: RankStyle) {
  const source = entry.avatar_url?.trim() || ''
  const label = entry.display_name
  return h(
    'div',
    {
      class: [
        'flex h-12 w-12 shrink-0 items-center justify-center overflow-hidden rounded-2xl border shadow-sm',
        style.avatarRingClass,
        source ? 'bg-white dark:bg-dark-900' : style.avatarFallbackClass,
      ],
    },
    source
      ? [
          h('img', {
            src: source,
            alt: label,
            class: 'h-full w-full object-cover',
          }),
        ]
      : [h('span', { class: 'text-sm font-semibold' }, avatarInitial(label))]
  )
}

function renderRankBadge(entry: LeaderboardEntry, style: RankStyle) {
  const rank = entry.rank ?? 0
  if (!isPodium(rank)) {
    return h(
      'div',
      {
        class: [
          'flex h-12 w-12 shrink-0 items-center justify-center rounded-2xl border text-sm font-semibold',
          style.rankChipClass,
        ],
      },
      `#${rank || '-'}`
    )
  }

  return h(
    'div',
    {
      class: [
        'flex h-12 w-12 shrink-0 flex-col items-center justify-center rounded-2xl border text-[10px] font-black uppercase leading-none tracking-[0.18em] shadow-sm',
        style.rankChipClass,
      ],
    },
    [
      renderCrownIcon(style.crownClass),
      h('span', { class: ['mt-0.5', style.badgeTextClass] }, `#${rank}`),
    ]
  )
}

const LeaderboardPanel = defineComponent({
  name: 'LeaderboardPanel',
  props: {
    title: { type: String, required: true },
    subtitle: { type: String, default: '' },
    window: { type: Object as () => LeaderboardWindowOverview, required: true },
  },
  setup(props) {
    const renderEntry = (entry: LeaderboardEntry, variant: EntryVariant = 'default') => {
      const rankStyle = getRankStyle(entry.rank, variant)
      return h(
        'li',
        {
          class: [
            'group relative overflow-hidden rounded-[1.35rem] border px-3 py-3 transition-transform duration-200',
            rankStyle.shellClass,
            entry.is_me ? 'ring-2 ring-primary-300/70 dark:ring-primary-500/40' : '',
            isPodium(entry.rank) ? 'md:pl-4' : '',
          ],
        },
        [
          h('div', { class: ['absolute left-0 top-0 h-full w-1.5 rounded-r-full', rankStyle.accentClass] }),
          h('div', { class: 'relative flex items-center gap-3 pl-1' }, [
            renderRankBadge(entry, rankStyle),
            renderAvatar(entry, rankStyle),
            h('div', { class: 'min-w-0 flex-1' }, [
              h('div', { class: 'truncate text-sm font-semibold text-gray-900 dark:text-white' }, entry.display_name),
              h('div', { class: 'mt-1.5 flex flex-wrap gap-2 text-xs text-gray-500 dark:text-gray-400' }, [
                renderMetricChip(formatTokens(entry.tokens), getTokenMetricTone(entry.tokens)),
                renderMetricChip(t('leaderboard.requests', { n: entry.requests }), getRequestMetricTone(entry.requests)),
              ]),
              renderHonorWall(entry),
            ]),
            h('div', { class: 'ml-auto flex shrink-0 flex-col items-end gap-1 text-right text-xs text-gray-500 dark:text-gray-400' }, [
              entry.best_rank ? h('div', t('leaderboard.bestRank', { n: entry.best_rank })) : null,
              entry.top_appearances ? h('div', t('leaderboard.appearances', { n: entry.top_appearances })) : null,
            ]),
          ]),
        ]
      )
    }

    return () => h('section', { class: 'card p-5' }, [
      h('div', { class: 'mb-4 flex items-start justify-between gap-3' }, [
        h('div', [
          h('h3', { class: 'text-base font-semibold text-gray-900 dark:text-white' }, props.title),
          props.subtitle ? h('p', { class: 'mt-1 text-xs text-gray-500 dark:text-gray-400' }, props.subtitle) : null,
        ]),
        h('span', { class: 'rounded bg-gray-100 px-2 py-1 text-xs font-medium text-gray-600 dark:bg-dark-700 dark:text-gray-300' }, 'Top 10'),
      ]),
      props.window.top10.length
        ? h('ol', { class: 'space-y-2' }, props.window.top10.map((entry) => renderEntry(entry)))
        : h('div', { class: 'rounded-md border border-dashed border-gray-200 py-8 text-center text-sm text-gray-500 dark:border-dark-600 dark:text-gray-400' }, t('leaderboard.empty')),
      h('div', { class: 'mt-4 border-t border-gray-100 pt-4 dark:border-dark-700' }, [
        props.window.me
          ? h('div', { class: 'space-y-3' }, [
              h('div', { class: 'flex items-end justify-between gap-3' }, [
                h('div', [
                  h('div', { class: 'text-[11px] font-semibold uppercase tracking-[0.24em] text-gray-500 dark:text-gray-400' }, t('leaderboard.myRank')),
                  h('p', { class: 'mt-1 text-xs text-gray-500 dark:text-gray-400' }, t('leaderboard.myRankHint')),
                ]),
                h(
                  'span',
                  {
                    class: 'rounded-full border border-gray-200 bg-white px-2.5 py-1 text-[11px] font-semibold text-gray-600 dark:border-dark-600 dark:bg-dark-800 dark:text-gray-300',
                  },
                  `#${props.window.me.rank || '-'}`
                ),
              ]),
              renderEntry(props.window.me, 'current'),
            ])
          : h('p', { class: 'text-sm text-gray-500 dark:text-gray-400' }, t('leaderboard.notParticipating')),
      ]),
    ])
  },
})

onMounted(loadOverview)
onUnmounted(() => {
  clearRefreshTimer()
})
</script>

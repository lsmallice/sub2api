<template>
  <AppLayout>
    <div class="space-y-6">
      <!-- Loading State -->
      <div v-if="loading" class="flex justify-center py-12">
        <div
          class="h-8 w-8 animate-spin rounded-full border-2 border-primary-500 border-t-transparent"
        ></div>
      </div>

      <!-- Empty State -->
      <div v-else-if="subscriptions.length === 0" class="card p-12 text-center">
        <div
          class="mx-auto mb-4 flex h-16 w-16 items-center justify-center rounded-full bg-gray-100 dark:bg-dark-700"
        >
          <Icon name="creditCard" size="xl" class="text-gray-400" />
        </div>
        <h3 class="mb-2 text-lg font-semibold text-gray-900 dark:text-white">
          {{ t('userSubscriptions.noActiveSubscriptions') }}
        </h3>
        <p class="text-gray-500 dark:text-dark-400">
          {{ t('userSubscriptions.noActiveSubscriptionsDesc') }}
        </p>
      </div>

      <!-- Subscriptions Grid -->
      <div v-else class="grid gap-6 lg:grid-cols-2">
        <div
          v-for="subscription in subscriptions"
          :key="subscription.id"
          class="overflow-hidden rounded-2xl border bg-white dark:bg-dark-800"
          :class="platformBorderClass(subscription.group?.platform || '')"
        >
          <!-- Header -->
          <div
            class="flex items-center justify-between border-b border-gray-100 p-4 dark:border-dark-700"
          >
            <div class="flex items-center gap-3">
              <div :class="['h-1.5 w-1.5 shrink-0 rounded-full', platformAccentDotClass(subscription.group?.platform || '')]" />
              <div>
                <div class="flex items-center gap-2">
                  <h3 class="font-semibold text-gray-900 dark:text-white">
                    {{ subscription.group?.name || `Group #${subscription.group_id}` }}
                  </h3>
                  <span :class="['rounded-md border px-2 py-0.5 text-[11px] font-medium', platformBadgeClass(subscription.group?.platform || '')]">
                    {{ platformLabel(subscription.group?.platform || '') }}
                  </span>
                </div>
                <p v-if="subscription.group?.description" class="mt-0.5 text-xs text-gray-500 dark:text-dark-400">
                  {{ subscription.group.description }}
                </p>
                <div class="mt-1 flex flex-wrap gap-x-3 gap-y-1 text-[11px] text-gray-400 dark:text-gray-500">
                  <span>{{ t('payment.planCard.rate') }}: ×{{ subscription.group?.rate_multiplier ?? 1 }}</span>
                  <span v-if="subscriptionHasPeakRate(subscription)" class="text-amber-700 dark:text-amber-300">
                    {{ t('payment.planCard.peakRate') }}: {{ subscriptionPeakRateLabel(subscription) }}
                  </span>
                </div>
              </div>
            </div>
            <div class="flex items-center gap-2">
              <span
                :class="[
                  'rounded-full px-2 py-0.5 text-xs font-medium',
                  subscription.status === 'active'
                    ? 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-300'
                    : subscription.status === 'expired'
                      ? 'bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-gray-400'
                      : 'bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-300'
                ]"
              >
                {{ t(`userSubscriptions.status.${subscription.status}`) }}
              </span>
              <button
                v-if="subscription.status === 'active'"
                :class="['rounded-lg px-3 py-1.5 text-xs font-semibold text-white transition-colors', platformButtonClass(subscription.group?.platform || '')]"
                @click="router.push({ path: '/purchase', query: { tab: 'subscription', group: String(subscription.group_id) } })"
              >
                {{ t('payment.renewNow') }}
              </button>
            </div>
          </div>

          <!-- Usage Progress -->
          <div class="space-y-4 p-4">
            <!-- Expiration Info -->
            <div v-if="subscription.expires_at" class="flex items-center justify-between text-sm">
              <span class="text-gray-500 dark:text-dark-400">{{
                t('userSubscriptions.expires')
              }}</span>
              <span :class="getExpirationClass(subscription.expires_at)">
                {{ formatExpirationDate(subscription.expires_at) }}
              </span>
            </div>
            <div v-else class="flex items-center justify-between text-sm">
              <span class="text-gray-500 dark:text-dark-400">{{
                t('userSubscriptions.expires')
              }}</span>
              <span class="text-gray-700 dark:text-gray-300">{{
                t('userSubscriptions.noExpiration')
              }}</span>
            </div>

            <!-- Daily Usage -->
            <div v-if="subscription.group?.daily_limit_usd" class="space-y-2">
              <div class="flex items-center justify-between">
                <span class="text-sm font-medium text-gray-700 dark:text-gray-300">
                  {{ t('userSubscriptions.daily') }}
                </span>
                <div class="flex flex-wrap items-center justify-end gap-2">
                  <span class="text-sm text-gray-500 dark:text-dark-400">
                    ${{ (subscription.daily_usage_usd || 0).toFixed(2) }} / ${{
                      subscription.group.daily_limit_usd.toFixed(2)
                    }}
                  </span>
                  <button
                    v-if="canRefreshQuota(subscription, 'daily')"
                    type="button"
                    :disabled="refreshingQuota"
                    class="inline-flex items-center gap-1 rounded-md border border-emerald-200 px-2 py-1 text-xs font-medium text-emerald-700 transition-colors hover:bg-emerald-50 disabled:cursor-not-allowed disabled:opacity-60 dark:border-emerald-800 dark:text-emerald-300 dark:hover:bg-emerald-900/20"
                    @click="openRefreshQuotaDialog(subscription, 'daily')"
                  >
                    <Icon name="clock" size="xs" />
                    {{ t('userSubscriptions.refreshQuota') }}
                  </button>
                  <span v-else-if="quotaRefreshReason(subscription, 'daily')" class="text-xs text-gray-400 dark:text-dark-500">
                    {{ quotaRefreshReason(subscription, 'daily') }}
                  </span>
                </div>
              </div>
              <div class="relative h-2 overflow-hidden rounded-full bg-gray-200 dark:bg-dark-600">
                <div
                  class="absolute inset-y-0 left-0 rounded-full transition-all duration-300"
                  :class="
                    getProgressBarClass(
                      subscription.daily_usage_usd,
                      subscription.group.daily_limit_usd
                    )
                  "
                  :style="{
                    width: getProgressWidth(
                      subscription.daily_usage_usd,
                      subscription.group.daily_limit_usd
                    )
                  }"
                ></div>
              </div>
              <p
                v-if="subscription.daily_window_start"
                class="text-xs text-gray-500 dark:text-dark-400"
              >
                {{ formatDailyUsageWindow(subscription) }}
              </p>
            </div>

            <!-- Weekly Usage -->
            <div v-if="subscription.group?.weekly_limit_usd" class="space-y-2">
              <div class="flex items-center justify-between">
                <span class="text-sm font-medium text-gray-700 dark:text-gray-300">
                  {{ t('userSubscriptions.weekly') }}
                </span>
                <div class="flex flex-wrap items-center justify-end gap-2">
                  <span class="text-sm text-gray-500 dark:text-dark-400">
                    ${{ (subscription.weekly_usage_usd || 0).toFixed(2) }} / ${{
                      subscription.group.weekly_limit_usd.toFixed(2)
                    }}
                  </span>
                  <button
                    v-if="canRefreshQuota(subscription, 'weekly')"
                    type="button"
                    :disabled="refreshingQuota"
                    class="inline-flex items-center gap-1 rounded-md border border-emerald-200 px-2 py-1 text-xs font-medium text-emerald-700 transition-colors hover:bg-emerald-50 disabled:cursor-not-allowed disabled:opacity-60 dark:border-emerald-800 dark:text-emerald-300 dark:hover:bg-emerald-900/20"
                    @click="openRefreshQuotaDialog(subscription, 'weekly')"
                  >
                    <Icon name="clock" size="xs" />
                    {{ t('userSubscriptions.refreshQuota') }}
                  </button>
                  <span v-else-if="quotaRefreshReason(subscription, 'weekly')" class="text-xs text-gray-400 dark:text-dark-500">
                    {{ quotaRefreshReason(subscription, 'weekly') }}
                  </span>
                </div>
              </div>
              <div class="relative h-2 overflow-hidden rounded-full bg-gray-200 dark:bg-dark-600">
                <div
                  class="absolute inset-y-0 left-0 rounded-full transition-all duration-300"
                  :class="
                    getProgressBarClass(
                      subscription.weekly_usage_usd,
                      subscription.group.weekly_limit_usd
                    )
                  "
                  :style="{
                    width: getProgressWidth(
                      subscription.weekly_usage_usd,
                      subscription.group.weekly_limit_usd
                    )
                  }"
                ></div>
              </div>
              <p
                v-if="subscription.weekly_window_start"
                class="text-xs text-gray-500 dark:text-dark-400"
              >
                {{
                  t('userSubscriptions.resetIn', {
                    time: formatResetTime(subscription.weekly_window_start, 168)
                  })
                }}
              </p>
            </div>

            <!-- Monthly Usage -->
            <div v-if="subscription.group?.monthly_limit_usd" class="space-y-2">
              <div class="flex items-center justify-between">
                <span class="text-sm font-medium text-gray-700 dark:text-gray-300">
                  {{ t('userSubscriptions.monthly') }}
                </span>
                <div class="flex flex-wrap items-center justify-end gap-2">
                  <span class="text-sm text-gray-500 dark:text-dark-400">
                    ${{ (subscription.monthly_usage_usd || 0).toFixed(2) }} / ${{
                      subscription.group.monthly_limit_usd.toFixed(2)
                    }}
                  </span>
                  <button
                    v-if="canRefreshQuota(subscription, 'monthly')"
                    type="button"
                    :disabled="refreshingQuota"
                    class="inline-flex items-center gap-1 rounded-md border border-emerald-200 px-2 py-1 text-xs font-medium text-emerald-700 transition-colors hover:bg-emerald-50 disabled:cursor-not-allowed disabled:opacity-60 dark:border-emerald-800 dark:text-emerald-300 dark:hover:bg-emerald-900/20"
                    @click="openRefreshQuotaDialog(subscription, 'monthly')"
                  >
                    <Icon name="clock" size="xs" />
                    {{ t('userSubscriptions.refreshQuota') }}
                  </button>
                  <span v-else-if="quotaRefreshReason(subscription, 'monthly')" class="text-xs text-gray-400 dark:text-dark-500">
                    {{ quotaRefreshReason(subscription, 'monthly') }}
                  </span>
                </div>
              </div>
              <div class="relative h-2 overflow-hidden rounded-full bg-gray-200 dark:bg-dark-600">
                <div
                  class="absolute inset-y-0 left-0 rounded-full transition-all duration-300"
                  :class="
                    getProgressBarClass(
                      subscription.monthly_usage_usd,
                      subscription.group.monthly_limit_usd
                    )
                  "
                  :style="{
                    width: getProgressWidth(
                      subscription.monthly_usage_usd,
                      subscription.group.monthly_limit_usd
                    )
                  }"
                ></div>
              </div>
              <p
                v-if="subscription.monthly_window_start"
                class="text-xs text-gray-500 dark:text-dark-400"
              >
                {{
                  t('userSubscriptions.resetIn', {
                    time: formatResetTime(subscription.monthly_window_start, 720)
                  })
                }}
              </p>
            </div>

            <!-- No limits configured - Unlimited badge -->
            <div
              v-if="
                !subscription.group?.daily_limit_usd &&
                !subscription.group?.weekly_limit_usd &&
                !subscription.group?.monthly_limit_usd
              "
              class="flex items-center justify-center rounded-xl bg-gradient-to-r from-emerald-50 to-teal-50 py-6 dark:from-emerald-900/20 dark:to-teal-900/20"
            >
              <div class="flex items-center gap-3">
                <span class="text-4xl text-emerald-600 dark:text-emerald-400">∞</span>
                <div>
                  <p class="text-sm font-medium text-emerald-700 dark:text-emerald-300">
                    {{ t('userSubscriptions.unlimited') }}
                  </p>
                  <p class="text-xs text-emerald-600/70 dark:text-emerald-400/70">
                    {{ t('userSubscriptions.unlimitedDesc') }}
                  </p>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>

    <ConfirmDialog
      :show="!!quotaRefreshTarget"
      :title="t('userSubscriptions.refreshQuotaTitle')"
      :message="quotaRefreshConfirmMessage"
      :confirm-text="refreshingQuota ? t('userSubscriptions.refreshingQuota') : t('userSubscriptions.confirmRefreshQuota')"
      :cancel-text="t('common.cancel')"
      @confirm="confirmRefreshQuota"
      @cancel="closeRefreshQuotaDialog"
    >
      <div v-if="quotaRefreshTargetInfo" class="rounded-lg bg-gray-50 p-3 text-sm text-gray-600 dark:bg-dark-700 dark:text-gray-300">
        <div class="flex justify-between gap-3">
          <span>{{ t('userSubscriptions.deductValidity') }}</span>
          <span class="font-medium text-gray-900 dark:text-white">{{ formatDeductedSeconds(quotaRefreshTargetInfo.deducted_seconds) }}</span>
        </div>
        <div v-if="quotaRefreshTargetInfo.projected_expires_at" class="mt-2 flex justify-between gap-3">
          <span>{{ t('userSubscriptions.newExpiration') }}</span>
          <span class="font-medium text-gray-900 dark:text-white">{{ formatDateTime(quotaRefreshTargetInfo.projected_expires_at) }}</span>
        </div>
      </div>
    </ConfirmDialog>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { useAppStore } from '@/stores/app'
import { useSubscriptionStore } from '@/stores/subscriptions'
import subscriptionsAPI from '@/api/subscriptions'
import type {
  SubscriptionQuotaRefreshWindow,
  SubscriptionQuotaRefreshWindowInfo,
  UserSubscription
} from '@/types'
import AppLayout from '@/components/layout/AppLayout.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import { formatDateOnly, formatDateTime } from '@/utils/format'
import { hasPeakRate, formatPeakRateWindow, serverTimezoneLabel } from '@/utils/peak-rate'
import { platformBorderClass, platformBadgeClass, platformButtonClass, platformLabel } from '@/utils/platformColors'
import { getRemainingDurationParts, isOneTimeDailyQuota, type RemainingDurationParts } from '@/utils/subscriptionQuota'

function platformAccentDotClass(p: string): string {
  switch (p) {
    case 'anthropic': return 'bg-orange-500'
    case 'openai': return 'bg-emerald-500'
    case 'antigravity': return 'bg-purple-500'
    case 'gemini': return 'bg-blue-500'
    default: return 'bg-gray-400'
  }
}

const { t } = useI18n()
const router = useRouter()
const appStore = useAppStore()
const subscriptionStore = useSubscriptionStore()

const subscriptions = ref<UserSubscription[]>([])
const loading = ref(true)
const refreshingQuota = ref(false)
const quotaRefreshTarget = ref<{
  subscription: UserSubscription
  window: SubscriptionQuotaRefreshWindow
} | null>(null)

const quotaRefreshTargetInfo = computed(() => {
  if (!quotaRefreshTarget.value) return null
  return getQuotaRefreshInfo(
    quotaRefreshTarget.value.subscription,
    quotaRefreshTarget.value.window
  )
})

const quotaRefreshConfirmMessage = computed(() => {
  if (!quotaRefreshTarget.value) return ''
  return t('userSubscriptions.refreshQuotaConfirm', {
    window: quotaWindowLabel(quotaRefreshTarget.value.window)
  })
})

function subscriptionHasPeakRate(subscription: UserSubscription): boolean {
  return hasPeakRate(subscription.group)
}

function subscriptionPeakRateLabel(subscription: UserSubscription): string {
  return formatPeakRateWindow(subscription.group, serverTimezoneLabel(appStore.cachedPublicSettings?.server_utc_offset))
}

async function loadSubscriptions() {
  try {
    loading.value = true
    subscriptions.value = await subscriptionsAPI.getMySubscriptions()
  } catch (error) {
    console.error('Failed to load subscriptions:', error)
    appStore.showError(t('userSubscriptions.failedToLoad'))
  } finally {
    loading.value = false
  }
}

function getProgressWidth(used: number | undefined, limit: number | null | undefined): string {
  if (!limit || limit === 0) return '0%'
  const percentage = Math.min(((used || 0) / limit) * 100, 100)
  return `${percentage}%`
}

function getProgressBarClass(used: number | undefined, limit: number | null | undefined): string {
  if (!limit || limit === 0) return 'bg-gray-400'
  const percentage = ((used || 0) / limit) * 100
  if (percentage >= 90) return 'bg-red-500'
  if (percentage >= 70) return 'bg-orange-500'
  return 'bg-green-500'
}

function formatExpirationDate(expiresAt: string): string {
  const now = new Date()
  const expires = new Date(expiresAt)
  const diff = expires.getTime() - now.getTime()
  const days = Math.ceil(diff / (1000 * 60 * 60 * 24))

  if (days < 0) {
    return t('userSubscriptions.status.expired')
  }

  const dateStr = formatDateOnly(expires)

  if (days === 0) {
    return `${dateStr} (${t('common.today')})`
  }
  if (days === 1) {
    return `${dateStr} (${t('common.tomorrow')})`
  }

  return t('userSubscriptions.daysRemaining', { days }) + ` (${dateStr})`
}

function getExpirationClass(expiresAt: string): string {
  const now = new Date()
  const expires = new Date(expiresAt)
  const diff = expires.getTime() - now.getTime()
  const days = Math.ceil(diff / (1000 * 60 * 60 * 24))

  if (days <= 0) return 'text-red-600 dark:text-red-400 font-medium'
  if (days <= 3) return 'text-red-600 dark:text-red-400'
  if (days <= 7) return 'text-orange-600 dark:text-orange-400'
  return 'text-gray-700 dark:text-gray-300'
}

function formatDurationParts(parts: RemainingDurationParts): string {
  if (parts.days > 0) {
    return `${parts.days}d ${parts.hours}h`
  }

  if (parts.hours > 0) {
    return `${parts.hours}h ${parts.minutes}m`
  }

  return `${parts.minutes}m`
}

function formatDailyUsageWindow(subscription: UserSubscription): string {
  if (isOneTimeDailyQuota(subscription) && subscription.expires_at) {
    const parts = getRemainingDurationParts(subscription.expires_at)
    if (!parts) return t('userSubscriptions.windowNotActive')
    return t('userSubscriptions.quotaEndsIn', { time: formatDurationParts(parts) })
  }

  return t('userSubscriptions.resetIn', {
    time: formatResetTime(subscription.daily_window_start, 24)
  })
}

function getQuotaRefreshInfo(
  subscription: UserSubscription,
  window: SubscriptionQuotaRefreshWindow
): SubscriptionQuotaRefreshWindowInfo | null {
  return subscription.quota_refresh?.[window] ?? null
}

function canRefreshQuota(subscription: UserSubscription, window: SubscriptionQuotaRefreshWindow): boolean {
  return getQuotaRefreshInfo(subscription, window)?.eligible === true
}

function quotaRefreshReason(subscription: UserSubscription, window: SubscriptionQuotaRefreshWindow): string {
  const info = getQuotaRefreshInfo(subscription, window)
  if (!info || info.eligible || !info.reason) return ''
  return t(`userSubscriptions.refreshReasons.${info.reason}`)
}

function quotaWindowLabel(window: SubscriptionQuotaRefreshWindow): string {
  return t(`userSubscriptions.${window}`)
}

function formatDeductedSeconds(seconds: number): string {
  const minutes = Math.max(1, Math.ceil(seconds / 60))
  const hours = Math.floor(minutes / 60)
  const days = Math.floor(hours / 24)
  const remainingHours = hours % 24
  const remainingMinutes = minutes % 60
  if (days > 0 && remainingHours > 0) {
    return t('userSubscriptions.durationDaysHours', { days, hours: remainingHours })
  }
  if (days > 0) {
    return t('userSubscriptions.durationDays', { days })
  }
  if (hours > 0 && remainingMinutes > 0) {
    return t('userSubscriptions.durationHoursMinutes', { hours, minutes: remainingMinutes })
  }
  if (hours === 0) {
    return t('userSubscriptions.durationMinutes', { minutes })
  }
  return t('userSubscriptions.durationHours', { hours })
}

function openRefreshQuotaDialog(
  subscription: UserSubscription,
  window: SubscriptionQuotaRefreshWindow
) {
  if (!canRefreshQuota(subscription, window)) return
  quotaRefreshTarget.value = { subscription, window }
}

function closeRefreshQuotaDialog() {
  if (refreshingQuota.value) return
  quotaRefreshTarget.value = null
}

function apiErrorMessage(error: any, fallback: string): string {
  return error?.message || error?.response?.data?.message || error?.response?.data?.detail || fallback
}

async function confirmRefreshQuota() {
  if (!quotaRefreshTarget.value || refreshingQuota.value) return
  refreshingQuota.value = true
  const target = quotaRefreshTarget.value
  try {
    await subscriptionsAPI.refreshQuota(target.subscription.id, { window: target.window })
    appStore.showSuccess(t('userSubscriptions.refreshQuotaSuccess'))
    quotaRefreshTarget.value = null
    subscriptionStore.invalidateCache()
    await Promise.all([
      loadSubscriptions(),
      subscriptionStore.fetchActiveSubscriptions(true).catch(() => [])
    ])
  } catch (error: any) {
    appStore.showError(apiErrorMessage(error, t('userSubscriptions.refreshQuotaFailed')))
  } finally {
    refreshingQuota.value = false
  }
}

function formatResetTime(windowStart: string | null, windowHours: number): string {
  if (!windowStart) return t('userSubscriptions.windowNotActive')

  const start = new Date(windowStart)
  const end = new Date(start.getTime() + windowHours * 60 * 60 * 1000)
  const parts = getRemainingDurationParts(end)

  return parts ? formatDurationParts(parts) : t('userSubscriptions.windowNotActive')
}

onMounted(() => {
  loadSubscriptions()
})
</script>

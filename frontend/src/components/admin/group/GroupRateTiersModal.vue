<template>
  <BaseDialog :show="show" :title="t('admin.groups.rateTiers.title')" width="wide" @close="handleClose">
    <div v-if="group" class="space-y-4">
      <div class="flex flex-wrap items-center gap-3 rounded-lg bg-gray-50 px-4 py-2.5 text-sm dark:bg-dark-700">
        <span class="inline-flex items-center gap-1.5 text-emerald-700 dark:text-emerald-400">
          <PlatformIcon :platform="group.platform" size="sm" />
          {{ t('admin.groups.platforms.' + group.platform) }}
        </span>
        <span class="text-gray-400">|</span>
        <span class="font-medium text-gray-900 dark:text-white">{{ group.name }}</span>
        <span class="text-gray-400">|</span>
        <span class="text-gray-600 dark:text-gray-400">
          {{ t('admin.groups.columns.rateMultiplier') }}: {{ group.rate_multiplier }}x
        </span>
      </div>

      <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <p class="text-sm text-gray-500 dark:text-gray-400">
          {{ t('admin.groups.rateTiers.description') }}
        </p>
        <button
          type="button"
          class="btn btn-primary btn-sm"
          :disabled="!canAddTier"
          :title="canAddTier ? t('admin.groups.rateTiers.add') : t('admin.groups.rateTiers.noAvailableKey')"
          @click="addTier"
        >
          <Icon name="plus" size="sm" class="mr-1" />
          {{ t('admin.groups.rateTiers.add') }}
        </button>
      </div>

      <div v-if="loading" class="flex justify-center py-8">
        <Icon name="refresh" size="lg" class="animate-spin text-primary-500" />
      </div>

      <div v-else-if="tiers.length === 0" class="rounded-lg border border-dashed border-gray-300 py-10 text-center text-sm text-gray-400 dark:border-dark-600">
        <div class="space-y-3">
          <p>{{ t('admin.groups.rateTiers.empty') }}</p>
          <button
            type="button"
            class="btn btn-secondary btn-sm"
            :disabled="!canAddTier"
            :title="canAddTier ? t('admin.groups.rateTiers.add') : t('admin.groups.rateTiers.noAvailableKey')"
            @click="addTier"
          >
            <Icon name="plus" size="sm" class="mr-1" />
            {{ t('admin.groups.rateTiers.add') }}
          </button>
        </div>
      </div>

      <div v-else class="space-y-3">
        <div class="max-h-[430px] space-y-3 overflow-y-auto pr-1">
          <div
            v-for="(tier, index) in tiers"
            :key="tier.local_id"
            class="rounded-lg border border-gray-200 bg-white p-3 shadow-sm transition-colors dark:border-dark-600 dark:bg-dark-800"
          >
            <div class="grid gap-3 lg:grid-cols-[52px_1fr]">
              <div class="flex items-center gap-1 lg:flex-col">
                <button
                  type="button"
                  class="rounded-md p-1.5 text-gray-400 transition-colors hover:bg-gray-100 hover:text-gray-700 disabled:cursor-not-allowed disabled:opacity-30 dark:hover:bg-dark-700 dark:hover:text-gray-200"
                  :disabled="index === 0"
                  :title="t('admin.groups.rateTiers.moveUp')"
                  @click="moveTier(index, -1)"
                >
                  <Icon name="arrowUp" size="xs" />
                </button>
                <span class="rounded-full bg-gray-100 px-2 py-0.5 text-xs font-semibold text-gray-600 dark:bg-dark-700 dark:text-gray-300">
                  #{{ index + 1 }}
                </span>
                <button
                  type="button"
                  class="rounded-md p-1.5 text-gray-400 transition-colors hover:bg-gray-100 hover:text-gray-700 disabled:cursor-not-allowed disabled:opacity-30 dark:hover:bg-dark-700 dark:hover:text-gray-200"
                  :disabled="index === tiers.length - 1"
                  :title="t('admin.groups.rateTiers.moveDown')"
                  @click="moveTier(index, 1)"
                >
                  <Icon name="arrowDown" size="xs" />
                </button>
              </div>

              <div class="min-w-0">
                <div class="grid gap-3 lg:grid-cols-[1fr_1fr_120px_auto] lg:items-end">
                  <div>
                    <label class="input-label">{{ t('admin.groups.rateTiers.columns.key') }}</label>
                    <Select
                      v-model="tier.tier_key"
                      :options="getTierKeyOptions(index)"
                      :placeholder="t('admin.groups.rateTiers.selectKey')"
                      :searchable="false"
                      @change="value => handleTierKeyChange(index, String(value || ''))"
                    >
                      <template #selected="{ option }">
                        <span v-if="option" class="font-mono text-xs">{{ option.label }}</span>
                        <span v-else class="text-gray-400">{{ t('admin.groups.rateTiers.selectKey') }}</span>
                      </template>
                      <template #option="{ option }">
                        <div class="flex min-w-0 flex-1 items-center justify-between gap-3">
                          <div class="min-w-0">
                            <div class="truncate text-sm font-medium">{{ option.label }}</div>
                            <div v-if="option.description" class="text-xs text-gray-400">{{ option.description }}</div>
                          </div>
                          <span v-if="option.disabled" class="text-xs text-gray-400">
                            {{ t('admin.groups.rateTiers.used') }}
                          </span>
                        </div>
                      </template>
                    </Select>
                  </div>
                  <div>
                    <label class="input-label">{{ t('admin.groups.rateTiers.columns.name') }}</label>
                    <input v-model.trim="tier.display_name" class="input h-9" placeholder="PRO" />
                  </div>
                  <div>
                    <label class="input-label">{{ t('admin.groups.rateTiers.columns.multiplier') }}</label>
                    <input v-model.number="tier.rate_multiplier" type="number" min="0" step="0.001" class="hide-spinner input h-9" />
                  </div>
                  <div class="flex flex-wrap items-center gap-3 pb-1">
                    <label class="inline-flex items-center gap-1.5 text-xs font-medium text-gray-600 dark:text-gray-300">
                      <input v-model="tier.enabled" type="checkbox" class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500" />
                      {{ t('common.enabled') }}
                    </label>
                    <label class="inline-flex items-center gap-1.5 text-xs font-medium text-gray-600 dark:text-gray-300">
                      <input type="radio" name="default-tier" :checked="tier.is_default" class="h-4 w-4 border-gray-300 text-primary-600 focus:ring-primary-500" @change="setDefault(index)" />
                      {{ t('admin.groups.rateTiers.default') }}
                    </label>
                    <button
                      type="button"
                      class="rounded-md p-1.5 text-gray-400 transition-colors hover:bg-red-50 hover:text-red-600 dark:hover:bg-red-900/20 dark:hover:text-red-400"
                      :title="t('common.delete')"
                      @click="removeTier(index)"
                    >
                      <Icon name="trash" size="sm" />
                    </button>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>

        <div class="overflow-hidden rounded-lg border border-gray-200 bg-gray-50 dark:border-dark-600 dark:bg-dark-700/70">
          <button
            type="button"
            class="flex w-full items-center justify-between gap-3 px-3 py-2.5 text-left"
            @click="fallbackPanelOpen = !fallbackPanelOpen"
          >
            <span class="min-w-0">
              <span class="block text-sm font-semibold text-gray-900 dark:text-white">
                {{ t('admin.groups.rateTiers.fallbackPolicy') }}
              </span>
              <span class="mt-0.5 block truncate text-xs text-gray-500 dark:text-gray-400">
                {{ fallbackStrategySummary }}
              </span>
            </span>
            <Icon
              name="chevronDown"
              size="md"
              :class="['text-gray-400 transition-transform', fallbackPanelOpen && 'rotate-180']"
            />
          </button>

          <div v-if="fallbackPanelOpen" class="space-y-3 border-t border-gray-200 p-3 dark:border-dark-600">
            <div>
              <div class="mb-2 flex items-center justify-between gap-3">
                <label class="input-label mb-0">{{ t('admin.groups.rateTiers.columns.fallbackOrder') }}</label>
                <button
                  type="button"
                  class="text-xs font-medium text-primary-600 hover:text-primary-700 dark:text-primary-400"
                  @click="clearFallbackOrder"
                >
                  {{ t('admin.groups.rateTiers.useTierOrder') }}
                </button>
              </div>
              <div v-if="fallbackTierOptions.length > 1" class="flex flex-wrap gap-2">
                <button
                  v-for="option in fallbackTierOptions"
                  :key="option.tier_key"
                  type="button"
                  @click="toggleFallbackTier(option.tier_key)"
                  :class="[
                    'inline-flex items-center gap-1.5 rounded-full border px-2.5 py-1 text-xs font-medium transition-colors',
                    fallbackTierOrderIndex(option.tier_key) > 0
                      ? 'border-cyan-300 bg-cyan-50 text-cyan-700 dark:border-cyan-700 dark:bg-cyan-500/10 dark:text-cyan-300'
                      : 'border-gray-200 bg-white text-gray-600 hover:border-gray-300 dark:border-dark-600 dark:bg-dark-800 dark:text-gray-300'
                  ]"
                >
                  <span
                    v-if="fallbackTierOrderIndex(option.tier_key) > 0"
                    class="inline-flex h-4 min-w-4 items-center justify-center rounded-full bg-cyan-500 px-1 text-[10px] text-white"
                  >
                    {{ fallbackTierOrderIndex(option.tier_key) }}
                  </span>
                  <span>{{ option.display_name || option.tier_key }}</span>
                  <span class="font-mono text-[10px] opacity-70">{{ option.tier_key }}</span>
                </button>
              </div>
              <p
                v-else
                class="rounded-md bg-white px-3 py-2 text-sm text-gray-500 ring-1 ring-inset ring-gray-200 dark:bg-dark-800 dark:text-gray-400 dark:ring-dark-600"
              >
                {{ t('admin.groups.rateTiers.noFallbackTiers') }}
              </p>
              <p class="mt-2 text-xs text-gray-500 dark:text-gray-400">
                {{ t('admin.groups.rateTiers.fallbackOrderHint') }}
              </p>
            </div>

            <div class="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
              <div>
                <label class="input-label">{{ t('admin.groups.rateTiers.columns.ttft') }}</label>
                <input v-model.number="fallbackConfig.first_token_threshold_ms" type="number" min="0" step="100" class="hide-spinner input h-9" placeholder="3000" />
              </div>
              <div>
                <label class="input-label">{{ t('admin.groups.rateTiers.columns.errors') }}</label>
                <input v-model.number="fallbackConfig.degrade_after_errors" type="number" min="0" step="1" class="hide-spinner input h-9" placeholder="2" />
              </div>
              <div>
                <label class="input-label">{{ t('admin.groups.rateTiers.columns.cooldown') }}</label>
                <input v-model.number="fallbackConfig.cooldown_seconds" type="number" min="0" step="30" class="hide-spinner input h-9" placeholder="300" />
              </div>
              <div>
                <label class="input-label">{{ t('admin.groups.rateTiers.columns.recovery') }}</label>
                <input v-model.number="fallbackConfig.recovery_successes" type="number" min="0" step="1" class="hide-spinner input h-9" placeholder="2" />
              </div>
            </div>
          </div>
        </div>
      </div>

      <div class="flex items-center gap-3 border-t border-gray-200 pt-4 dark:border-dark-600">
        <span v-if="validationMessage" class="text-xs text-red-600 dark:text-red-400">{{ validationMessage }}</span>
        <span v-else-if="isDirty" class="text-xs text-amber-600 dark:text-amber-400">{{ t('admin.groups.unsavedChanges') }}</span>
        <div class="ml-auto flex items-center gap-3">
          <button type="button" class="btn btn-sm px-4 py-1.5" @click="handleClose">
            {{ t('common.close') }}
          </button>
          <button type="button" class="btn btn-primary btn-sm px-4 py-1.5" :disabled="saving || !!validationMessage || !isDirty" @click="handleSave">
            <Icon v-if="saving" name="refresh" size="sm" class="mr-1 animate-spin" />
            {{ t('common.save') }}
          </button>
        </div>
      </div>
    </div>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import type { GroupRateTier } from '@/api/admin/groups'
import type { AdminGroup } from '@/types'
import { useAppStore } from '@/stores/app'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import PlatformIcon from '@/components/common/PlatformIcon.vue'
import Select from '@/components/common/Select.vue'

interface LocalTier {
  local_id: string
  tier_key: string
  display_name: string
  rate_multiplier: number
  enabled: boolean
  is_default: boolean
}

interface FallbackConfig {
  fallback_order: string
  first_token_threshold_ms: number | null
  degrade_after_errors: number | null
  cooldown_seconds: number | null
  recovery_successes: number | null
}

const props = defineProps<{
  show: boolean
  group: AdminGroup | null
}>()

const emit = defineEmits<{
  close: []
  success: []
}>()

const { t } = useI18n()
const appStore = useAppStore()

const loading = ref(false)
const saving = ref(false)
const serverSnapshot = ref('')
const tiers = ref<LocalTier[]>([])
const fallbackPanelOpen = ref(false)

const createEmptyFallbackConfig = (): FallbackConfig => ({
  fallback_order: '',
  first_token_threshold_ms: null,
  degrade_after_errors: null,
  cooldown_seconds: null,
  recovery_successes: null
})

const fallbackConfig = ref<FallbackConfig>(createEmptyFallbackConfig())

const tierKeyPresets = [
  { key: 'pro', name: 'PRO', multiplier: 2 },
  { key: 'plus', name: 'Plus', multiplier: 1 },
  { key: 'pro2', name: 'Pro2', multiplier: 1.5 }
] as const

const normalizeTierKey = (value: string | null | undefined) => value?.trim().toLowerCase() || ''

const formatMultiplier = (value: number) =>
  Number.isInteger(value) ? value.toFixed(0) : value.toFixed(3).replace(/0+$/, '').replace(/\.$/, '')

const findTierKeyPreset = (key: string) => tierKeyPresets.find(preset => preset.key === key)

const availableTierKeyPresets = computed(() => {
  const usedKeys = new Set(tiers.value.map(tier => normalizeTierKey(tier.tier_key)).filter(Boolean))
  return tierKeyPresets.filter(preset => !usedKeys.has(preset.key))
})

const canAddTier = computed(() => availableTierKeyPresets.value.length > 0)

type TierKeyOption = {
  value: string
  label: string
  description: string
  disabled: boolean
}

const getTierKeyOptions = (currentIndex: number) => {
  const currentTier = tiers.value[currentIndex]
  const currentKey = normalizeTierKey(currentTier?.tier_key)
  const usedByOtherRows = new Set(
    tiers.value
      .filter((_, index) => index !== currentIndex)
      .map(tier => normalizeTierKey(tier.tier_key))
      .filter(Boolean)
  )
  const options: TierKeyOption[] = tierKeyPresets.map(preset => ({
    value: preset.key,
    label: `${preset.name} (${preset.key})`,
    description: `${formatMultiplier(preset.multiplier)}x`,
    disabled: usedByOtherRows.has(preset.key)
  }))

  if (currentKey && !findTierKeyPreset(currentKey)) {
    options.unshift({
      value: currentKey,
      label: `${currentTier?.display_name || currentKey} (${currentKey})`,
      description: t('admin.groups.rateTiers.customKey'),
      disabled: false
    })
  }

  return options
}

const handleTierKeyChange = (index: number, value: string) => {
  const tier = tiers.value[index]
  if (!tier) return
  const key = normalizeTierKey(value)
  tier.tier_key = key
  const preset = findTierKeyPreset(key)
  if (!preset) return
  tier.display_name = preset.name
  tier.rate_multiplier = preset.multiplier
  normalizeFallbackOrders()
}

const parseFallbackOrder = (value: string) =>
  value
    .split(',')
    .map(item => normalizeTierKey(item))
    .filter(Boolean)

const normalizeFallbackOrderKeys = (keys: string[]) => {
  const validKeys = new Set(tiers.value.map(tier => normalizeTierKey(tier.tier_key)).filter(Boolean))
  const defaultKey = normalizeTierKey(tiers.value.find(tier => tier.is_default)?.tier_key)
  const seen = new Set<string>()
  return keys
    .map(key => normalizeTierKey(key))
    .filter(key => {
      if (!key || !validKeys.has(key) || key === defaultKey || seen.has(key)) return false
      seen.add(key)
      return true
    })
}

const selectedFallbackTierKeys = computed(() =>
  normalizeFallbackOrderKeys(parseFallbackOrder(fallbackConfig.value.fallback_order))
)

const setFallbackOrder = (keys: string[]) => {
  fallbackConfig.value.fallback_order = normalizeFallbackOrderKeys(keys).join(',')
}

const fallbackTierOptions = computed(() =>
  tiers.value.filter(tier => {
    const key = normalizeTierKey(tier.tier_key)
    const defaultKey = normalizeTierKey(tiers.value.find(item => item.is_default)?.tier_key)
    return key && key !== defaultKey
  })
)

const fallbackTierOrderIndex = (key: string) =>
  selectedFallbackTierKeys.value.indexOf(normalizeTierKey(key)) + 1

const toggleFallbackTier = (key: string) => {
  const normalized = normalizeTierKey(key)
  if (!normalized) return
  const current = selectedFallbackTierKeys.value
  if (current.includes(normalized)) {
    setFallbackOrder(current.filter(item => item !== normalized))
    return
  }
  setFallbackOrder([...current, normalized])
}

const clearFallbackOrder = () => {
  fallbackConfig.value.fallback_order = ''
}

const hasFallbackTriggerConfig = () =>
  (fallbackConfig.value.first_token_threshold_ms ?? 0) > 0 ||
  (fallbackConfig.value.degrade_after_errors ?? 0) > 0 ||
  (fallbackConfig.value.cooldown_seconds ?? 0) > 0 ||
  (fallbackConfig.value.recovery_successes ?? 0) > 0

const getTierDisplayLabel = (key: string) => {
  const tier = tiers.value.find(item => normalizeTierKey(item.tier_key) === normalizeTierKey(key))
  return tier?.display_name || key
}

const fallbackStrategySummary = computed(() => {
  const order = selectedFallbackTierKeys.value
  const orderSummary = order.length > 0
    ? order.map((key, index) => `${index + 1}. ${getTierDisplayLabel(key)}`).join(' -> ')
    : fallbackTierOptions.value.length > 0
      ? `${t('admin.groups.rateTiers.useTierOrder')}: ${fallbackTierOptions.value.map(tier => tier.display_name || tier.tier_key).join(' -> ')}`
      : t('admin.groups.rateTiers.noFallbackTiers')
  return hasFallbackTriggerConfig()
    ? `${orderSummary} · ${t('admin.groups.rateTiers.triggerConfigured')}`
    : orderSummary
})

const normalizeFallbackOrders = () => {
  setFallbackOrder(selectedFallbackTierKeys.value)
}

const policyNumber = (policy: Record<string, unknown> | undefined, key: string): number | null => {
  const value = policy?.[key]
  if (typeof value === 'number' && Number.isFinite(value)) return value
  if (typeof value === 'string' && value.trim() !== '') {
    const parsed = Number(value)
    return Number.isFinite(parsed) ? parsed : null
  }
  return null
}

const policyOrder = (policy: Record<string, unknown> | undefined): string => {
  const value = policy?.fallback_order || policy?.fallback_tiers || policy?.order || policy?.tiers
  if (Array.isArray(value)) {
    return value.filter(item => typeof item === 'string').join(',')
  }
  if (typeof value === 'string') {
    return value
  }
  return ''
}

const fallbackConfigFromPolicy = (policy: Record<string, unknown> | undefined): FallbackConfig => ({
  fallback_order: policyOrder(policy),
  first_token_threshold_ms: policyNumber(policy, 'first_token_threshold_ms'),
  degrade_after_errors: policyNumber(policy, 'degrade_after_errors'),
  cooldown_seconds: policyNumber(policy, 'cooldown_seconds'),
  recovery_successes: policyNumber(policy, 'recovery_successes')
})

const hasPolicyValue = (policy: Record<string, unknown> | undefined) => {
  const config = fallbackConfigFromPolicy(policy)
  return config.fallback_order.trim().length > 0 ||
    (config.first_token_threshold_ms ?? 0) > 0 ||
    (config.degrade_after_errors ?? 0) > 0 ||
    (config.cooldown_seconds ?? 0) > 0 ||
    (config.recovery_successes ?? 0) > 0
}

const applyFallbackConfigFromTiers = (items: GroupRateTier[]) => {
  const source = items.find(tier => tier.is_default && hasPolicyValue(tier.fallback_policy)) ||
    items.find(tier => hasPolicyValue(tier.fallback_policy))
  fallbackConfig.value = source
    ? fallbackConfigFromPolicy(source.fallback_policy)
    : createEmptyFallbackConfig()
  normalizeFallbackOrders()
}

const cloneTier = (tier: GroupRateTier, index: number): LocalTier => {
  return {
    local_id: `${tier.id || 'new'}-${tier.tier_key}-${index}-${Date.now()}`,
    tier_key: tier.tier_key || '',
    display_name: tier.display_name || tier.tier_key || '',
    rate_multiplier: tier.rate_multiplier ?? 1,
    enabled: tier.enabled ?? true,
    is_default: tier.is_default ?? index === 0
  }
}

const buildFallbackPolicy = (): Record<string, unknown> => {
  const policy: Record<string, unknown> = {}
  const fallbackOrder = selectedFallbackTierKeys.value
  if (fallbackOrder.length > 0) policy.fallback_order = fallbackOrder
  if (fallbackConfig.value.first_token_threshold_ms && fallbackConfig.value.first_token_threshold_ms > 0) {
    policy.first_token_threshold_ms = fallbackConfig.value.first_token_threshold_ms
    policy.degrade_enabled = true
  }
  if (fallbackConfig.value.degrade_after_errors && fallbackConfig.value.degrade_after_errors > 0) {
    policy.degrade_after_errors = fallbackConfig.value.degrade_after_errors
    policy.degrade_enabled = true
  }
  if (fallbackConfig.value.cooldown_seconds && fallbackConfig.value.cooldown_seconds > 0) {
    policy.cooldown_seconds = fallbackConfig.value.cooldown_seconds
  }
  if (fallbackConfig.value.recovery_successes && fallbackConfig.value.recovery_successes > 0) {
    policy.recovery_successes = fallbackConfig.value.recovery_successes
  }
  return policy
}

const cloneFallbackPolicy = (policy: Record<string, unknown>) => {
  const next: Record<string, unknown> = { ...policy }
  if (Array.isArray(policy.fallback_order)) {
    next.fallback_order = [...policy.fallback_order]
  }
  return next
}

const toPayload = (items: LocalTier[]): GroupRateTier[] => {
  const fallbackPolicy = buildFallbackPolicy()
  return items.map((tier, index) => {
    return {
      tier_key: tier.tier_key.trim().toLowerCase(),
      display_name: tier.display_name.trim(),
      rate_multiplier: Number(tier.rate_multiplier) || 0,
      priority: (index + 1) * 10,
      enabled: tier.enabled,
      is_default: tier.is_default,
      fallback_policy: cloneFallbackPolicy(fallbackPolicy)
    }
  })
}

const normalizedSnapshot = computed(() => JSON.stringify(toPayload(tiers.value)))
const isDirty = computed(() => normalizedSnapshot.value !== serverSnapshot.value)

const validationMessage = computed(() => {
  const keys = new Set<string>()
  let defaultCount = 0
  for (const tier of tiers.value) {
    const key = tier.tier_key.trim().toLowerCase()
    if (!key) return t('admin.groups.rateTiers.errors.keyRequired')
    if (keys.has(key)) return t('admin.groups.rateTiers.errors.duplicateKey')
    keys.add(key)
    if (tier.rate_multiplier == null || Number(tier.rate_multiplier) < 0) {
      return t('admin.groups.rateTiers.errors.invalidMultiplier')
    }
    if (tier.is_default) defaultCount++
  }
  if (tiers.value.length > 0 && defaultCount !== 1) {
    return t('admin.groups.rateTiers.errors.oneDefault')
  }
  return ''
})

const loadTiers = async () => {
  if (!props.group) return
  loading.value = true
  try {
    const result = await adminAPI.groups.getGroupRateTiers(props.group.id)
    tiers.value = result.map(cloneTier)
    applyFallbackConfigFromTiers(result)
    fallbackPanelOpen.value = false
    serverSnapshot.value = normalizedSnapshot.value
  } catch (error) {
    appStore.showError(t('admin.groups.failedToLoad'))
    console.error('Error loading group rate tiers:', error)
  } finally {
    loading.value = false
  }
}

const addTier = () => {
  const preset = availableTierKeyPresets.value[0]
  if (!preset) {
    appStore.showError(t('admin.groups.rateTiers.noAvailableKey'))
    return
  }
  tiers.value.push({
    local_id: `new-${Date.now()}-${Math.random()}`,
    tier_key: preset.key,
    display_name: preset.name,
    rate_multiplier: preset.multiplier,
    enabled: true,
    is_default: tiers.value.length === 0
  })
  normalizeFallbackOrders()
}

const removeTier = (index: number) => {
  const wasDefault = tiers.value[index]?.is_default
  tiers.value.splice(index, 1)
  normalizeFallbackOrders()
  if (wasDefault && tiers.value.length > 0) {
    tiers.value[0].is_default = true
  }
}

const moveTier = (index: number, direction: -1 | 1) => {
  const target = index + direction
  if (target < 0 || target >= tiers.value.length) return
  const current = tiers.value[index]
  tiers.value.splice(index, 1)
  tiers.value.splice(target, 0, current)
}

const setDefault = (index: number) => {
  tiers.value.forEach((tier, i) => {
    tier.is_default = i === index
  })
  normalizeFallbackOrders()
}

const handleSave = async () => {
  if (!props.group || validationMessage.value) return
  saving.value = true
  try {
    await adminAPI.groups.batchSetGroupRateTiers(props.group.id, toPayload(tiers.value))
    appStore.showSuccess(t('admin.groups.rateTiers.saved'))
    emit('success')
    emit('close')
  } catch (error) {
    appStore.showError(t('admin.groups.failedToSave'))
    console.error('Error saving group rate tiers:', error)
  } finally {
    saving.value = false
  }
}

const handleClose = () => {
  emit('close')
}

watch(
  () => props.show,
  show => {
    if (show && props.group) {
      void loadTiers()
    }
  }
)
</script>

<style scoped>
.hide-spinner::-webkit-outer-spin-button,
.hide-spinner::-webkit-inner-spin-button {
  -webkit-appearance: none;
  margin: 0;
}
.hide-spinner {
  -moz-appearance: textfield;
}
</style>

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

      <div class="flex items-center justify-between gap-3">
        <p class="text-sm text-gray-500 dark:text-gray-400">
          {{ t('admin.groups.rateTiers.description') }}
        </p>
        <button type="button" class="btn btn-primary btn-sm" @click="addTier">
          <Icon name="plus" size="sm" class="mr-1" />
          {{ t('admin.groups.rateTiers.add') }}
        </button>
      </div>

      <div v-if="loading" class="flex justify-center py-8">
        <Icon name="refresh" size="lg" class="animate-spin text-primary-500" />
      </div>

      <div v-else-if="tiers.length === 0" class="rounded-lg border border-dashed border-gray-300 py-10 text-center text-sm text-gray-400 dark:border-dark-600">
        {{ t('admin.groups.rateTiers.empty') }}
      </div>

      <div v-else class="overflow-hidden rounded-lg border border-gray-200 dark:border-dark-600">
        <div class="max-h-[520px] overflow-auto">
          <table class="w-full min-w-[1120px] text-sm">
            <thead class="sticky top-0 z-[1] bg-gray-50 dark:bg-dark-700">
              <tr class="border-b border-gray-200 dark:border-dark-600">
                <th class="px-3 py-2 text-left text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.groups.rateTiers.columns.key') }}</th>
                <th class="px-3 py-2 text-left text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.groups.rateTiers.columns.name') }}</th>
                <th class="px-3 py-2 text-left text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.groups.rateTiers.columns.multiplier') }}</th>
                <th class="px-3 py-2 text-left text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.groups.rateTiers.columns.priority') }}</th>
                <th class="px-3 py-2 text-left text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.groups.rateTiers.columns.fallbackOrder') }}</th>
                <th class="px-3 py-2 text-left text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.groups.rateTiers.columns.ttft') }}</th>
                <th class="px-3 py-2 text-left text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.groups.rateTiers.columns.errors') }}</th>
                <th class="px-3 py-2 text-left text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.groups.rateTiers.columns.cooldown') }}</th>
                <th class="px-3 py-2 text-left text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.groups.rateTiers.columns.recovery') }}</th>
                <th class="px-3 py-2 text-left text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.groups.rateTiers.columns.flags') }}</th>
                <th class="w-10 px-2 py-2"></th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-100 dark:divide-dark-600">
              <tr v-for="(tier, index) in tiers" :key="tier.local_id" class="align-top hover:bg-gray-50 dark:hover:bg-dark-700/50">
                <td class="px-3 py-2">
                  <input v-model.trim="tier.tier_key" class="input h-9 w-28 font-mono text-xs" placeholder="pro" />
                </td>
                <td class="px-3 py-2">
                  <input v-model.trim="tier.display_name" class="input h-9 w-28" placeholder="PRO" />
                </td>
                <td class="px-3 py-2">
                  <input v-model.number="tier.rate_multiplier" type="number" min="0" step="0.001" class="hide-spinner input h-9 w-24" />
                </td>
                <td class="px-3 py-2">
                  <input v-model.number="tier.priority" type="number" step="1" class="hide-spinner input h-9 w-20" />
                </td>
                <td class="px-3 py-2">
                  <input v-model.trim="tier.fallback_order" class="input h-9 w-40 font-mono text-xs" placeholder="plus,pro2" />
                </td>
                <td class="px-3 py-2">
                  <input v-model.number="tier.first_token_threshold_ms" type="number" min="0" step="100" class="hide-spinner input h-9 w-24" placeholder="3000" />
                </td>
                <td class="px-3 py-2">
                  <input v-model.number="tier.degrade_after_errors" type="number" min="0" step="1" class="hide-spinner input h-9 w-20" placeholder="2" />
                </td>
                <td class="px-3 py-2">
                  <input v-model.number="tier.cooldown_seconds" type="number" min="0" step="30" class="hide-spinner input h-9 w-24" placeholder="300" />
                </td>
                <td class="px-3 py-2">
                  <input v-model.number="tier.recovery_successes" type="number" min="0" step="1" class="hide-spinner input h-9 w-20" placeholder="2" />
                </td>
                <td class="px-3 py-2">
                  <div class="flex flex-col gap-2">
                    <label class="inline-flex items-center gap-1.5 text-xs text-gray-600 dark:text-gray-300">
                      <input v-model="tier.enabled" type="checkbox" class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500" />
                      {{ t('common.enabled') }}
                    </label>
                    <label class="inline-flex items-center gap-1.5 text-xs text-gray-600 dark:text-gray-300">
                      <input type="radio" name="default-tier" :checked="tier.is_default" class="h-4 w-4 border-gray-300 text-primary-600 focus:ring-primary-500" @change="setDefault(index)" />
                      {{ t('admin.groups.rateTiers.default') }}
                    </label>
                  </div>
                </td>
                <td class="px-2 py-2">
                  <button type="button" class="rounded p-1.5 text-gray-400 transition-colors hover:bg-red-50 hover:text-red-600 dark:hover:bg-red-900/20 dark:hover:text-red-400" @click="removeTier(index)">
                    <Icon name="trash" size="sm" />
                  </button>
                </td>
              </tr>
            </tbody>
          </table>
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

interface LocalTier {
  local_id: string
  tier_key: string
  display_name: string
  rate_multiplier: number
  priority: number
  enabled: boolean
  is_default: boolean
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

const policyNumber = (policy: Record<string, unknown> | undefined, key: string): number | null => {
  const value = policy?.[key]
  if (typeof value === 'number' && Number.isFinite(value)) return value
  if (typeof value === 'string' && value.trim() !== '') {
    const parsed = Number(value)
    return Number.isFinite(parsed) ? parsed : null
  }
  return null
}

const cloneTier = (tier: GroupRateTier, index: number): LocalTier => {
  const policy = tier.fallback_policy || {}
  const order = Array.isArray(policy.fallback_order)
    ? policy.fallback_order.filter(item => typeof item === 'string').join(',')
    : Array.isArray(policy.fallback_tiers)
      ? policy.fallback_tiers.filter(item => typeof item === 'string').join(',')
      : ''
  return {
    local_id: `${tier.id || 'new'}-${tier.tier_key}-${index}-${Date.now()}`,
    tier_key: tier.tier_key || '',
    display_name: tier.display_name || tier.tier_key || '',
    rate_multiplier: tier.rate_multiplier ?? 1,
    priority: tier.priority ?? (index + 1) * 10,
    enabled: tier.enabled ?? true,
    is_default: tier.is_default ?? index === 0,
    fallback_order: order,
    first_token_threshold_ms: policyNumber(policy, 'first_token_threshold_ms'),
    degrade_after_errors: policyNumber(policy, 'degrade_after_errors'),
    cooldown_seconds: policyNumber(policy, 'cooldown_seconds'),
    recovery_successes: policyNumber(policy, 'recovery_successes')
  }
}

const toPayload = (items: LocalTier[]): GroupRateTier[] => {
  return items.map(tier => {
    const fallbackOrder = tier.fallback_order
      .split(',')
      .map(item => item.trim().toLowerCase())
      .filter(Boolean)
    const policy: Record<string, unknown> = {}
    if (fallbackOrder.length > 0) policy.fallback_order = fallbackOrder
    if (tier.first_token_threshold_ms && tier.first_token_threshold_ms > 0) {
      policy.first_token_threshold_ms = tier.first_token_threshold_ms
      policy.degrade_enabled = true
    }
    if (tier.degrade_after_errors && tier.degrade_after_errors > 0) {
      policy.degrade_after_errors = tier.degrade_after_errors
      policy.degrade_enabled = true
    }
    if (tier.cooldown_seconds && tier.cooldown_seconds > 0) policy.cooldown_seconds = tier.cooldown_seconds
    if (tier.recovery_successes && tier.recovery_successes > 0) policy.recovery_successes = tier.recovery_successes
    return {
      tier_key: tier.tier_key.trim().toLowerCase(),
      display_name: tier.display_name.trim(),
      rate_multiplier: Number(tier.rate_multiplier) || 0,
      priority: Number(tier.priority) || 0,
      enabled: tier.enabled,
      is_default: tier.is_default,
      fallback_policy: policy
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
    serverSnapshot.value = normalizedSnapshot.value
  } catch (error) {
    appStore.showError(t('admin.groups.failedToLoad'))
    console.error('Error loading group rate tiers:', error)
  } finally {
    loading.value = false
  }
}

const addTier = () => {
  const next = tiers.value.length + 1
  tiers.value.push({
    local_id: `new-${Date.now()}-${Math.random()}`,
    tier_key: next === 1 ? 'pro' : `tier${next}`,
    display_name: next === 1 ? 'PRO' : `Tier ${next}`,
    rate_multiplier: next === 1 ? 2 : 1,
    priority: next * 10,
    enabled: true,
    is_default: tiers.value.length === 0,
    fallback_order: '',
    first_token_threshold_ms: null,
    degrade_after_errors: null,
    cooldown_seconds: null,
    recovery_successes: null
  })
}

const removeTier = (index: number) => {
  const wasDefault = tiers.value[index]?.is_default
  tiers.value.splice(index, 1)
  if (wasDefault && tiers.value.length > 0) {
    tiers.value[0].is_default = true
  }
}

const setDefault = (index: number) => {
  tiers.value.forEach((tier, i) => {
    tier.is_default = i === index
  })
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

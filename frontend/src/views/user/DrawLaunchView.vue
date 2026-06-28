<template>
  <AppLayout>
    <div class="flex min-h-[320px] items-center justify-center">
      <div class="flex flex-col items-center gap-3 text-center">
        <LoadingSpinner />
        <p class="text-sm text-gray-500 dark:text-gray-400">正在打开 Smallice Draw...</p>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import { useAppStore, useAuthStore } from '@/stores'
import { buildDrawLaunchURL } from '@/utils/drawLaunch'

const { t } = useI18n()
const appStore = useAppStore()
const authStore = useAuthStore()

onMounted(() => {
  try {
    window.location.replace(buildDrawLaunchURL(authStore.token || ''))
  } catch (error) {
    console.error('Failed to open draw:', error)
    appStore.showError(error instanceof Error ? error.message : t('nav.openDrawFailed'))
  }
})
</script>

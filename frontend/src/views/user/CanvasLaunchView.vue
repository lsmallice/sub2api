<template>
  <AppLayout>
    <div class="flex min-h-[320px] items-center justify-center">
      <div class="flex flex-col items-center gap-3 text-center">
        <LoadingSpinner />
        <p class="text-sm text-gray-500 dark:text-gray-400">正在打开无限画布...</p>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import { useAppStore } from '@/stores'
import { canvasAPI } from '@/api/canvas'

const { t } = useI18n()
const appStore = useAppStore()

onMounted(async () => {
  try {
    const ticket = await canvasAPI.createSSOTicket()
    if (!ticket.redirect_url) throw new Error('Canvas URL is not configured')
    window.location.replace(withThemeParam(ticket.redirect_url))
  } catch (error) {
    console.error('Failed to open canvas:', error)
    appStore.showError(error instanceof Error ? error.message : t('nav.openCanvasFailed'))
  }
})

function currentThemeName(): 'light' | 'dark' {
  return document.documentElement.classList.contains('dark') ? 'dark' : 'light'
}

function withThemeParam(rawURL: string): string {
  try {
    const url = new URL(rawURL)
    url.searchParams.set('theme', currentThemeName())
    return url.toString()
  } catch {
    const separator = rawURL.includes('?') ? '&' : '?'
    return `${rawURL}${separator}theme=${currentThemeName()}`
  }
}
</script>

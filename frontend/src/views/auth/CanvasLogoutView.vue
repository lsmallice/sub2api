<template>
  <div class="flex min-h-screen items-center justify-center bg-gray-50 dark:bg-dark-950">
    <div class="flex flex-col items-center gap-3 text-center">
      <LoadingSpinner />
      <p class="text-sm text-gray-500 dark:text-gray-400">正在退出登录...</p>
    </div>
  </div>
</template>

<script setup lang="ts">
import { onMounted } from 'vue'
import { useRouter } from 'vue-router'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import { useAuthStore } from '@/stores/auth'

const authStore = useAuthStore()
const router = useRouter()

onMounted(async () => {
  try {
    if (authStore.isAuthenticated) {
      await authStore.logout()
    }
  } finally {
    await router.replace('/login')
  }
})
</script>

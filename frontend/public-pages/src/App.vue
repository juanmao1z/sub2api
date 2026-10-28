<script setup lang="ts">
import { watchEffect } from 'vue'
import { RouterView, useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import Toast from '@/components/common/Toast.vue'

const route = useRoute()
const appStore = useAppStore()
const { t } = useI18n()
watchEffect(() => {
  const key = route.path === '/login' ? 'auth.signIn' : 'home.navigation.home'
  document.title = `${t(key)} - ${appStore.siteName}`
})
</script>

<template>
  <RouterView />
  <Toast />
</template>

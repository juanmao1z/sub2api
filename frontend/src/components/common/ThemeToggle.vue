<template>
  <button
    type="button"
    class="theme-toggle btn-icon"
    :aria-label="isDark ? 'Switch to light theme' : 'Switch to dark theme'"
    :title="isDark ? 'Switch to light theme' : 'Switch to dark theme'"
    data-testid="theme-toggle"
    @click="handleToggle"
  >
    <Icon :name="isDark ? 'sun' : 'moon'" size="sm" />
  </button>
</template>

<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'
import Icon from '@/components/icons/Icon.vue'
import { getStoredTheme, setTheme, THEME_CHANGE_EVENT, type Theme } from '@/utils/theme'

const isDark = ref(false)

function syncTheme(theme: Theme = getStoredTheme()) {
  isDark.value = theme === 'dark'
}

function handleToggle() {
  setTheme(isDark.value ? 'light' : 'dark')
}

function handleThemeChange(event: Event) {
  const theme = (event as CustomEvent<Theme>).detail
  syncTheme(theme)
}

function handleStorageChange(event: StorageEvent) {
  if (event.key === 'theme') syncTheme()
}

onMounted(() => {
  syncTheme()
  window.addEventListener(THEME_CHANGE_EVENT, handleThemeChange)
  window.addEventListener('storage', handleStorageChange)
})

onBeforeUnmount(() => {
  window.removeEventListener(THEME_CHANGE_EVENT, handleThemeChange)
  window.removeEventListener('storage', handleStorageChange)
})
</script>

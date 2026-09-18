<template>
  <div class="reference-auth auth-shell relative flex min-h-screen items-center justify-center overflow-y-auto px-6 pb-8 pt-28 sm:px-8 sm:pt-32">
    <!-- Logo/Brand -->
    <div class="auth-brand absolute left-6 top-6 sm:left-12 sm:top-10">
      <template v-if="settingsLoaded">
        <div class="auth-brand-logo flex h-9 w-9 items-center justify-center overflow-hidden">
          <img :src="siteLogo || '/logo.png?v=20260715'" alt="Logo" class="h-full w-full object-contain" />
        </div>
        <div class="min-w-0">
          <h1 class="auth-brand-title truncate text-xl font-semibold tracking-tight">
            {{ siteName }}
          </h1>
          <p class="auth-brand-subtitle truncate text-xs text-gray-500 dark:text-dark-400">
            {{ siteSubtitle }}
          </p>
        </div>
      </template>
    </div>

    <!-- Content Container -->
    <div class="auth-content relative z-10 my-auto w-full max-w-[36.5rem]">
      <div class="auth-card">
        <slot />
      </div>

      <!-- Footer Links -->
      <div class="mt-6 text-center text-sm">
        <slot name="footer" />
      </div>

      <!-- Copyright -->
      <div class="mt-7 text-center text-xs text-gray-400 dark:text-dark-500">
        &copy; {{ currentYear }} {{ siteName }}. All rights reserved.
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted } from 'vue'
import '@/styles/public-pages.css'
import { useAppStore } from '@/stores'
import { sanitizeUrl } from '@/utils/url'

const appStore = useAppStore()

const siteName = computed(() => appStore.siteName || 'Sub2API')
const siteLogo = computed(() => sanitizeUrl(appStore.siteLogo || '', { allowRelative: true, allowDataUrl: true }))
const siteSubtitle = computed(() => appStore.cachedPublicSettings?.site_subtitle || 'Subscription to API Conversion Platform')
const settingsLoaded = computed(() => appStore.publicSettingsLoaded)

const currentYear = computed(() => new Date().getFullYear())

onMounted(() => {
  appStore.fetchPublicSettings()
})
</script>

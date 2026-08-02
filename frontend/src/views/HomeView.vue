<template>
  <!-- Custom Home Content: Full Page Mode -->
  <div v-if="hasHomeContent" class="min-h-screen">
    <!-- iframe mode -->
    <iframe
      v-if="isHomeContentUrl"
      :src="homeContent.trim()"
      class="h-screen w-full border-0"
      allowfullscreen
    ></iframe>
    <!-- HTML mode - SECURITY: homeContent is admin-only setting, XSS risk is acceptable -->
    <div v-else v-html="homeContent"></div>
  </div>

  <!-- Compact Home Page -->
  <div
    v-else-if="compactHomeEnabled"
    data-testid="compact-home"
    class="flex min-h-screen flex-col bg-gray-50 text-gray-900 dark:bg-dark-950 dark:text-white"
  >
    <header class="border-b border-gray-200 px-4 py-4 sm:px-6 dark:border-dark-800">
      <nav class="mx-auto flex max-w-5xl flex-wrap items-center justify-between gap-3 sm:gap-4">
        <div class="flex min-w-0 flex-1 items-center gap-3">
          <img
            :src="siteLogo || '/logo.svg'"
            alt="Logo"
            class="h-9 w-9 shrink-0 rounded-lg object-contain"
          />
          <span class="min-w-0 truncate text-base font-semibold">{{ siteName }}</span>
        </div>
        <div class="flex max-w-full shrink-0 flex-wrap items-center justify-end gap-2">
          <LocaleSwitcher />
          <a
            v-if="docUrl"
            :href="docUrl"
            target="_blank"
            rel="noopener noreferrer"
            class="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg text-gray-500 hover:bg-gray-100 dark:text-dark-400 dark:hover:bg-dark-800"
            :title="t('home.viewDocs')"
          >
            <Icon name="book" size="md" />
          </a>
          <button
            class="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg text-gray-500 hover:bg-gray-100 dark:text-dark-400 dark:hover:bg-dark-800"
            :title="isDark ? t('home.switchToLight') : t('home.switchToDark')"
            @click="toggleTheme"
          >
            <Icon v-if="isDark" name="sun" size="md" />
            <Icon v-else name="moon" size="md" />
          </button>
          <router-link
            :to="isAuthenticated ? dashboardPath : '/login'"
            class="inline-flex min-h-10 shrink-0 items-center justify-center rounded-lg bg-gray-900 px-4 py-2 text-sm font-medium text-white hover:bg-gray-800 dark:bg-white dark:text-gray-900 dark:hover:bg-gray-200"
          >
            {{ isAuthenticated ? t('home.dashboard') : t('home.login') }}
          </router-link>
        </div>
      </nav>
    </header>

    <main class="flex min-w-0 flex-1 items-center justify-center px-4 py-16 sm:px-6">
      <div class="min-w-0 max-w-2xl text-center">
        <img
          :src="siteLogo || '/logo.svg'"
          alt="Logo"
          class="mx-auto mb-6 h-20 w-20 rounded-2xl object-contain"
        />
        <h1 class="[overflow-wrap:anywhere] text-3xl font-bold md:text-4xl">{{ siteName }}</h1>
        <p class="mt-4 whitespace-pre-wrap [overflow-wrap:anywhere] text-base text-gray-600 dark:text-dark-300">{{ siteSubtitle }}</p>
        <router-link
          :to="isAuthenticated ? dashboardPath : '/login'"
          class="mt-8 inline-flex min-h-10 items-center justify-center rounded-lg bg-primary-600 px-5 py-2.5 text-sm font-medium text-white hover:bg-primary-700"
        >
          {{ isAuthenticated ? t('home.goToDashboard') : t('home.login') }}
        </router-link>
      </div>
    </main>

    <footer class="min-w-0 border-t border-gray-200 px-4 py-5 text-center text-sm text-gray-500 [overflow-wrap:anywhere] sm:px-6 dark:border-dark-800 dark:text-dark-400">
      &copy; {{ currentYear }} {{ siteName }}
    </footer>
  </div>

  <!-- Default Home Page -->
  <div
    v-else
    class="flex min-h-screen min-h-[100dvh] flex-col overflow-x-hidden bg-[#f7faf8] text-[#102523] transition-colors dark:bg-[#101514] dark:text-[#edf5f2]"
  >
    <header class="relative z-30 shrink-0 px-4 py-4 sm:px-6 lg:px-8">
      <nav class="mx-auto flex h-10 max-w-6xl items-center justify-between" aria-label="Primary">
        <router-link to="/home" class="flex min-w-0 items-center gap-2.5" @click="closeMobileMenu">
          <span class="flex h-9 w-9 shrink-0 items-center justify-center">
            <img
              :src="siteLogo || '/logo.png?v=20260715'"
              :alt="`${siteName} logo`"
              class="h-full w-full object-contain"
            />
          </span>
          <span class="truncate text-sm font-semibold text-[#102523] dark:text-white">
            {{ siteName }}
          </span>
        </router-link>

        <div class="hidden items-center gap-1 md:flex">
          <router-link to="/home" class="home-nav-link" aria-current="page">
            {{ t('home.navigation.home') }}
          </router-link>
          <router-link :to="isAuthenticated ? dashboardPath : '/login'" class="home-nav-link">
            {{ t('home.navigation.dashboard') }}
          </router-link>
          <span class="mx-2 h-4 w-px bg-[#dce5e1] dark:bg-[#33413d]"></span>
          <LocaleSwitcher />
          <button
            type="button"
            class="home-icon-button"
            :title="isDark ? t('home.switchToLight') : t('home.switchToDark')"
            @click="toggleTheme"
          >
            <Icon v-if="isDark" name="sun" size="md" />
            <Icon v-else name="moon" size="md" />
          </button>
          <router-link
            v-if="isAuthenticated"
            :to="dashboardPath"
            class="ml-2 inline-flex h-8 items-center rounded-full bg-[#102523] px-4 text-xs font-semibold text-white transition-colors hover:bg-[#24413d] dark:bg-white dark:text-[#102523] dark:hover:bg-[#dce7e3]"
          >
            {{ t('home.dashboard') }}
          </router-link>
          <router-link
            v-else
            to="/login"
            class="ml-2 inline-flex h-8 items-center rounded-full bg-[#102523] px-4 text-xs font-semibold text-white transition-colors hover:bg-[#24413d] dark:bg-white dark:text-[#102523] dark:hover:bg-[#dce7e3]"
          >
            {{ t('home.login') }}
          </router-link>
        </div>

        <div class="flex items-center gap-1 md:hidden">
          <LocaleSwitcher />
          <button
            type="button"
            class="home-icon-button"
            :title="isDark ? t('home.switchToLight') : t('home.switchToDark')"
            @click="toggleTheme"
          >
            <Icon v-if="isDark" name="sun" size="md" />
            <Icon v-else name="moon" size="md" />
          </button>
          <button
            type="button"
            class="home-icon-button"
            :aria-expanded="mobileMenuOpen"
            aria-controls="home-mobile-menu"
            :title="mobileMenuOpen ? t('home.navigation.closeMenu') : t('home.navigation.openMenu')"
            @click="mobileMenuOpen = !mobileMenuOpen"
          >
            <Icon :name="mobileMenuOpen ? 'x' : 'menu'" size="md" />
          </button>
        </div>
      </nav>

      <nav
        v-if="mobileMenuOpen"
        id="home-mobile-menu"
        class="absolute left-4 right-4 top-full overflow-hidden rounded-lg border border-[#dce5e1] bg-white p-2 shadow-lg dark:border-[#33413d] dark:bg-[#17201e] md:hidden"
        aria-label="Mobile"
      >
        <router-link to="/home" class="home-mobile-link" @click="closeMobileMenu">
          {{ t('home.navigation.home') }}
        </router-link>
        <router-link
          :to="isAuthenticated ? dashboardPath : '/login'"
          class="home-mobile-link"
          @click="closeMobileMenu"
        >
          {{ t('home.navigation.dashboard') }}
        </router-link>
        <router-link
          :to="isAuthenticated ? dashboardPath : '/login'"
          class="mt-1 flex h-9 items-center justify-center rounded-md bg-[#102523] px-4 text-sm font-semibold text-white dark:bg-white dark:text-[#102523]"
          @click="closeMobileMenu"
        >
          {{ isAuthenticated ? t('home.dashboard') : t('home.login') }}
        </router-link>
      </nav>
    </header>

    <main class="flex min-h-0 flex-1 flex-col items-center justify-center px-4 pb-8 pt-4 sm:px-6 sm:pb-12 lg:px-8">
      <div class="flex w-full max-w-6xl flex-col items-center text-center">
        <section class="min-h-[5.5rem]" aria-labelledby="uptime-heading">
          <p id="uptime-heading" class="text-xs font-semibold text-[#5f7a75] dark:text-[#8da59f]">
            {{ t('home.status.stable') }}
          </p>
          <div
            class="mt-1 flex flex-wrap items-baseline justify-center gap-x-2 gap-y-0.5 font-mono text-base font-semibold text-[#102523] dark:text-[#edf5f2] sm:text-xl"
            aria-live="polite"
          >
            <span><strong>{{ uptime?.years ?? '--' }}</strong> {{ t('home.status.years') }}</span>
            <span><strong>{{ uptime?.months ?? '--' }}</strong> {{ t('home.status.months') }}</span>
            <span><strong>{{ uptime?.days ?? '--' }}</strong> {{ t('home.status.days') }}</span>
            <span><strong>{{ uptime?.hours ?? '--' }}</strong> {{ t('home.status.hours') }}</span>
            <span><strong>{{ uptime?.minutes ?? '--' }}</strong> {{ t('home.status.minutes') }}</span>
            <span><strong>{{ uptime?.seconds ?? '--' }}</strong> {{ t('home.status.seconds') }}</span>
          </div>
          <p class="mt-1 text-[11px] font-medium text-[#78908b] dark:text-[#718781]">
            {{ homepageStatus?.started_at ? t('home.status.since', { date: startedAtDisplay }) : '--' }}
          </p>
        </section>

        <h1 class="brand-title mt-5 max-w-full text-4xl font-[720] leading-none text-[#092321] dark:text-white sm:text-7xl md:text-8xl lg:text-[8rem]">
          {{ siteName }}
        </h1>

        <nav class="mt-8 flex flex-wrap items-center justify-center gap-x-7 gap-y-3 text-sm font-semibold sm:mt-10" aria-label="Quick links">
          <router-link
            :to="isAuthenticated ? dashboardPath : '/login'"
            class="home-quick-link"
          >
            {{ t('home.navigation.dashboard') }}
          </router-link>
          <a :href="redeemUrl" class="home-quick-link">
            {{ t('home.navigation.redeem') }}
          </a>
          <a :href="leaderboardUrl" class="home-quick-link">
            {{ t('home.navigation.leaderboard') }}
          </a>
          <button
            type="button"
            class="home-quick-link"
            aria-haspopup="dialog"
            :aria-expanded="communityOpen"
            @click="communityOpen = true"
          >
            {{ t('home.navigation.community') }}
          </button>
        </nav>

        <section
          class="mt-12 grid w-full max-w-3xl grid-cols-3 divide-x divide-[#dce5e1] dark:divide-[#33413d] sm:mt-16"
          aria-label="Service statistics"
          aria-live="polite"
        >
          <div class="min-w-0 px-2 sm:px-6">
            <p class="min-h-8 text-[10px] font-semibold leading-4 text-[#67817c] dark:text-[#89a09a] sm:min-h-0 sm:text-xs">
              {{ t('home.status.activeUsers') }}
            </p>
            <p class="status-number mt-1 text-xl font-semibold text-[#102523] dark:text-white sm:text-2xl">
              {{ activeUsersDisplay }}
            </p>
          </div>
          <div class="min-w-0 px-2 sm:px-6">
            <p class="min-h-8 text-[10px] font-semibold leading-4 text-[#67817c] dark:text-[#89a09a] sm:min-h-0 sm:text-xs">
              {{ t('home.status.successRate') }}
            </p>
            <p class="status-number mt-1 text-xl font-semibold text-[#102523] dark:text-white sm:text-2xl">
              {{ successRateDisplay }}
            </p>
          </div>
          <div class="min-w-0 px-2 sm:px-6">
            <p class="min-h-8 text-[10px] font-semibold leading-4 text-[#67817c] dark:text-[#89a09a] sm:min-h-0 sm:text-xs">
              {{ t('home.status.totalTokens') }}
            </p>
            <p class="status-number mt-1 text-xl font-semibold text-[#102523] dark:text-white sm:text-2xl">
              {{ totalTokensDisplay }}
            </p>
          </div>
        </section>
      </div>
    </main>
  </div>

  <Teleport to="body">
    <Transition name="community-modal">
      <div
        v-if="communityOpen"
        class="fixed inset-0 z-[150] flex items-center justify-center bg-[#07110f]/70 p-4 backdrop-blur-sm"
        @click.self="closeCommunity"
      >
        <section
          role="dialog"
          aria-modal="true"
          aria-labelledby="community-dialog-title"
          class="flex max-h-[calc(100vh-2rem)] w-full max-w-md flex-col overflow-hidden rounded-lg border border-[#dce5e1] bg-white shadow-2xl dark:border-[#33413d] dark:bg-[#17201e]"
        >
          <header class="flex h-12 items-center justify-between border-b border-[#e4ebe8] px-4 dark:border-[#33413d]">
            <h2 id="community-dialog-title" class="text-sm font-semibold text-[#102523] dark:text-white">
              {{ t('home.community.title') }}
            </h2>
            <button
              type="button"
              class="home-icon-button"
              :title="t('home.community.close')"
              :aria-label="t('home.community.close')"
              @click="closeCommunity"
            >
              <Icon name="x" size="md" />
            </button>
          </header>
          <div class="min-h-0 flex-1 overflow-auto p-3">
            <img
              src="/community-placeholder.jpg?v=20260731-img3842"
              :alt="t('home.community.imageAlt')"
              class="mx-auto h-auto max-h-[calc(100vh-6.5rem)] w-full rounded-md object-contain"
            />
          </div>
        </section>
      </div>
    </Transition>
  </Teleport>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAuthStore, useAppStore } from '@/stores'
import LocaleSwitcher from '@/components/common/LocaleSwitcher.vue'
import Icon from '@/components/icons/Icon.vue'
import { useHomepageStatus } from '@/composables/useHomepageStatus'
import {
  calculateBeijingUptime,
  formatBeijingStartTime,
  formatCompactTokens,
  formatSuccessRate,
} from '@/utils/homepageStatus'
import { sanitizeUrl } from '@/utils/url'

const { t, locale } = useI18n()
const authStore = useAuthStore()
const appStore = useAppStore()

const siteName = computed(() => appStore.cachedPublicSettings?.site_name || appStore.siteName || 'Sub2API')
const siteLogo = computed(() => sanitizeUrl(appStore.cachedPublicSettings?.site_logo || appStore.siteLogo || '', { allowRelative: true, allowDataUrl: true }))
const siteSubtitle = computed(() => appStore.cachedPublicSettings?.site_subtitle || 'AI API Gateway Platform')
const docUrl = computed(() => sanitizeUrl(appStore.cachedPublicSettings?.doc_url || appStore.docUrl || ''))
const homeContent = computed(() => appStore.cachedPublicSettings?.home_content || '')
const hasHomeContent = computed(() => homeContent.value.trim().length > 0)
const compactHomeEnabled = computed(() => appStore.cachedPublicSettings?.compact_home_enabled === true)
const showDefaultHome = computed(
  () => appStore.publicSettingsLoaded
    && !hasHomeContent.value
    && !compactHomeEnabled.value,
)
const { status: homepageStatus } = useHomepageStatus(showDefaultHome, 60_000)

const isHomeContentUrl = computed(() => {
  const content = homeContent.value.trim()
  return content.startsWith('http://') || content.startsWith('https://')
})

const isAuthenticated = computed(() => authStore.isAuthenticated)
const dashboardPath = computed(() => authStore.isAdmin ? '/admin/dashboard' : '/dashboard')
const currentYear = computed(() => new Date().getFullYear())
const redeemUrl = 'https://pay.ldxp.cn/shop/1WGCPCG0'
const leaderboardUrl = 'https://api.zhouz.online/custom/usage-leaderboard'

const isDark = ref(document.documentElement.classList.contains('dark'))
const mobileMenuOpen = ref(false)
const communityOpen = ref(false)
const now = ref(new Date())
let clockTimer: ReturnType<typeof setInterval> | null = null

const uptime = computed(() => calculateBeijingUptime(homepageStatus.value?.started_at ?? null, now.value))
const startedAtDisplay = computed(() => (
  formatBeijingStartTime(homepageStatus.value?.started_at ?? null, locale.value)
))
const activeUsersDisplay = computed(() => (
  homepageStatus.value
    ? new Intl.NumberFormat(locale.value, { maximumFractionDigits: 0 }).format(homepageStatus.value.active_users_1h)
    : '--'
))
const successRateDisplay = computed(() => (
  formatSuccessRate(homepageStatus.value?.success_rate_today ?? null, locale.value)
))
const totalTokensDisplay = computed(() => (
  formatCompactTokens(homepageStatus.value?.total_tokens ?? null, locale.value)
))

function closeMobileMenu(): void {
  mobileMenuOpen.value = false
}

function closeCommunity(): void {
  communityOpen.value = false
}

function handleKeydown(event: KeyboardEvent): void {
  if (event.key === 'Escape' && communityOpen.value) closeCommunity()
}

function toggleTheme(): void {
  isDark.value = !isDark.value
  document.documentElement.classList.toggle('dark', isDark.value)
  localStorage.setItem('theme', isDark.value ? 'dark' : 'light')
}

function initTheme(): void {
  const savedTheme = localStorage.getItem('theme')
  if (
    savedTheme === 'dark'
    || (!savedTheme && window.matchMedia('(prefers-color-scheme: dark)').matches)
  ) {
    isDark.value = true
    document.documentElement.classList.add('dark')
  }
}

function stopClock(): void {
  if (clockTimer !== null) clearInterval(clockTimer)
  clockTimer = null
}

function syncClock(enabled: boolean): void {
  stopClock()
  if (!enabled) return
  now.value = new Date()
  clockTimer = setInterval(() => {
    now.value = new Date()
  }, 1000)
}

watch(showDefaultHome, syncClock)

onMounted(() => {
  initTheme()
  authStore.checkAuth()
  syncClock(showDefaultHome.value)
  document.addEventListener('keydown', handleKeydown)
  if (!appStore.publicSettingsLoaded) {
    void appStore.fetchPublicSettings()
  }
})

onUnmounted(() => {
  stopClock()
  document.removeEventListener('keydown', handleKeydown)
})
</script>

<style scoped>
.brand-title {
  overflow-wrap: anywhere;
  letter-spacing: 0;
}

.status-number {
  font-variant-numeric: tabular-nums;
  letter-spacing: 0;
}

.home-nav-link {
  @apply rounded-md px-3 py-2 text-xs font-semibold text-[#425c57] transition-colors hover:bg-white hover:text-[#102523] dark:text-[#a9bbb6] dark:hover:bg-[#1d2926] dark:hover:text-white;
}

.home-icon-button {
  @apply flex h-9 w-9 items-center justify-center rounded-md text-[#526b66] transition-colors hover:bg-white hover:text-[#102523] dark:text-[#a9bbb6] dark:hover:bg-[#1d2926] dark:hover:text-white;
}

.home-mobile-link {
  @apply flex min-h-10 items-center rounded-md px-3 text-sm font-semibold text-[#425c57] hover:bg-[#f1f5f3] dark:text-[#b7c8c3] dark:hover:bg-[#22302d];
}

.home-quick-link {
  @apply rounded-sm text-[#183c37] underline-offset-4 transition-colors hover:text-[#0f766e] hover:underline focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[#0f766e] focus-visible:ring-offset-4 dark:text-[#c9d8d4] dark:hover:text-[#5eead4] dark:focus-visible:ring-[#5eead4] dark:focus-visible:ring-offset-[#101514];
}

.community-modal-enter-active,
.community-modal-leave-active {
  transition: opacity 160ms ease;
}

.community-modal-enter-from,
.community-modal-leave-to {
  opacity: 0;
}
</style>

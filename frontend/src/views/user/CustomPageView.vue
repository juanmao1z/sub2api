<template>
  <AppLayout>
    <div class="custom-page-layout">
      <div
        class="flex-1 min-h-0 overflow-hidden"
        :class="[
          menuItemId === 'usage-leaderboard' ? 'leaderboard-embed-host' : '',
          !isMarkdownMode && menuItemId !== 'usage-leaderboard' ? 'card' : '',
          isMarkdownMode ? 'guide-page-host' : '',
        ]"
      >
        <div v-if="loading" class="flex h-full items-center justify-center py-12">
          <div
            class="h-8 w-8 animate-spin rounded-full border-2 border-primary-500 border-t-transparent"
          ></div>
        </div>

        <div
          v-else-if="!menuItem"
          class="flex h-full items-center justify-center p-10 text-center"
        >
          <div class="max-w-md">
            <div
              class="mx-auto mb-4 flex h-12 w-12 items-center justify-center rounded-full bg-gray-100 dark:bg-dark-700"
            >
              <Icon name="link" size="lg" class="text-gray-400" />
            </div>
            <h3 class="text-lg font-semibold text-gray-900 dark:text-white">
              {{ t('customPage.notFoundTitle') }}
            </h3>
            <p class="mt-2 text-sm text-gray-500 dark:text-dark-400">
              {{ t('customPage.notFoundDesc') }}
            </p>
          </div>
        </div>

        <!-- Markdown guide mode -->
        <div v-else-if="isMarkdownMode" class="guide-shell">
          <nav v-if="showGuideTabs" class="guide-tabs" role="tablist" aria-label="使用说明栏目">
            <button
              v-for="tab in guideTabs"
              :id="`guide-tab-${tab.slug}`"
              :key="tab.slug"
              type="button"
              role="tab"
              class="guide-tab"
              :class="{ 'guide-tab-active': activeGuideSlug === tab.slug }"
              :aria-selected="activeGuideSlug === tab.slug"
              aria-controls="guide-article"
              @click="selectGuideTab(tab.slug)"
            >
              {{ tab.label }}
            </button>
          </nav>

          <div class="guide-reader">
            <div
              ref="markdownScrollContainer"
              class="guide-content-scroll"
              @scroll="onContentScroll"
            >
              <details v-if="tocItems.length > 0" class="guide-mobile-toc">
                <summary>{{ t('customPage.tableOfContents') }}</summary>
                <nav class="guide-mobile-toc-nav">
                  <a
                    v-for="item in tocItems"
                    :key="`mobile-${item.id}`"
                    :href="'#' + item.id"
                    :class="[`toc-level-${item.level}`, { 'toc-active': activeHeadingId === item.id }]"
                    @click.prevent="scrollToHeading(item.id)"
                  >
                    {{ item.text }}
                  </a>
                </nav>
              </details>

              <article
                id="guide-article"
                ref="markdownContainer"
                class="markdown-page-content"
                role="tabpanel"
                :aria-labelledby="showGuideTabs ? `guide-tab-${activeGuideSlug}` : undefined"
                v-html="renderedHtml"
              ></article>
            </div>

            <aside v-if="tocItems.length > 0" class="toc-sidebar">
              <div class="toc-header">
                <span class="toc-title">{{ t('customPage.tableOfContents') }}</span>
              </div>
              <nav class="toc-nav">
                <a
                  v-for="item in tocItems"
                  :key="item.id"
                  :href="'#' + item.id"
                  class="toc-item"
                  :class="[
                    `toc-level-${item.level}`,
                    { 'toc-active': activeHeadingId === item.id }
                  ]"
                  @click.prevent="scrollToHeading(item.id)"
                >
                  {{ item.text }}
                </a>
              </nav>
            </aside>
          </div>
        </div>

        <!-- URL not configured -->
        <div v-else-if="!isValidUrl" class="flex h-full items-center justify-center p-10 text-center">
          <div class="max-w-md">
            <div
              class="mx-auto mb-4 flex h-12 w-12 items-center justify-center rounded-full bg-gray-100 dark:bg-dark-700"
            >
              <Icon name="link" size="lg" class="text-gray-400" />
            </div>
            <h3 class="text-lg font-semibold text-gray-900 dark:text-white">
              {{ t('customPage.notConfiguredTitle') }}
            </h3>
            <p class="mt-2 text-sm text-gray-500 dark:text-dark-400">
              {{ t('customPage.notConfiguredDesc') }}
            </p>
          </div>
        </div>

        <!-- Iframe embed mode -->
        <div
          v-else
          class="custom-embed-shell"
          :class="{ 'custom-embed-shell-leaderboard': menuItemId === 'usage-leaderboard' }"
        >
          <a
            v-if="showOpenInNewTab"
            :href="embeddedUrl"
            target="_blank"
            rel="noopener noreferrer"
            class="btn btn-secondary btn-sm custom-open-fab"
          >
            <Icon name="externalLink" size="sm" class="mr-1.5" :stroke-width="2" />
            {{ t('customPage.openInNewTab') }}
          </a>
          <iframe
            :src="embeddedUrl"
            class="custom-embed-frame"
            allowfullscreen
          ></iframe>
        </div>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores'
import { useAuthStore } from '@/stores/auth'
import { useAdminSettingsStore } from '@/stores/adminSettings'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import { buildApiUrl } from '@/api/client'
import { buildEmbeddedUrl, detectTheme } from '@/utils/embedded-url'
import { marked } from 'marked'
import DOMPurify from 'dompurify'
import ccSwitchCodexGuide from '@/content/cc-switch-codex.md?raw'
import siteUsageGuide from '@/content/site-usage.md?raw'

interface TocItem {
  id: string
  text: string
  level: number
}

interface GuideTab {
  slug: string
  label: string
}

const { t, locale } = useI18n()
const route = useRoute()
const appStore = useAppStore()
const authStore = useAuthStore()
const adminSettingsStore = useAdminSettingsStore()

const loading = ref(false)
const pageTheme = ref<'light' | 'dark'>('light')
const renderedHtml = ref('')
const markdownContainer = ref<HTMLElement | null>(null)
const markdownScrollContainer = ref<HTMLElement | null>(null)
const tocItems = ref<TocItem[]>([])
const activeHeadingId = ref('')
const activeGuideSlug = ref('')
let themeObserver: MutationObserver | null = null

const guideTabs: GuideTab[] = [
  { slug: 'cc-switch-codex', label: 'CC Switch 配置 Codex' },
  { slug: 'site-usage', label: '网站使用说明' },
]
const guideTabSlugs = new Set(guideTabs.map((tab) => tab.slug))

const builtinMarkdownPages: Record<string, string> = {
  'cc-switch-codex': ccSwitchCodexGuide,
  'site-usage': siteUsageGuide,
}

const menuItemId = computed(() => route.params.id as string)

const menuItem = computed(() => {
  const id = menuItemId.value
  const publicItems = appStore.cachedPublicSettings?.custom_menu_items ?? []
  const found = publicItems.find((item) => item.id === id) ?? null
  if (found) return found
  if (authStore.isAdmin) {
    return adminSettingsStore.customMenuItems.find((item) => item.id === id) ?? null
  }
  return null
})

const markdownSlug = computed(() => {
  const item = menuItem.value
  if (!item) return ''
  if (item.page_slug) return item.page_slug
  if (item.url?.startsWith('md:')) return item.url.slice(3)
  return ''
})

const isMarkdownMode = computed(() => !!markdownSlug.value)
const showGuideTabs = computed(() => markdownSlug.value === 'cc-switch-codex')
const displayedMarkdownSlug = computed(() => activeGuideSlug.value || markdownSlug.value)

const embeddedUrl = computed(() => {
  if (!menuItem.value || isMarkdownMode.value) return ''
  return buildEmbeddedUrl(
    menuItem.value.url,
    authStore.user?.id,
    authStore.token,
    pageTheme.value,
    locale.value,
  )
})

const isValidUrl = computed(() => {
  if (isMarkdownMode.value) return false
  const url = embeddedUrl.value
  return url.startsWith('http://') || url.startsWith('https://')
})

const showOpenInNewTab = computed(() => menuItemId.value !== 'usage-leaderboard')

function generateHeadingId(text: string, index: number): string {
  const base = text
    .toLowerCase()
    .replace(/[^\w一-鿿]+/g, '-')
    .replace(/^-+|-+$/g, '')
  return base ? `${base}-${index}` : `heading-${index}`
}

function isRelativeMarkdownAsset(src: string): boolean {
  const trimmed = src.trim()
  if (!trimmed || /^[a-z][a-z0-9+.-]*:/i.test(trimmed) || trimmed.startsWith('//') || trimmed.startsWith('/')) {
    return false
  }
  const [pathPart] = trimmed.split(/([?#].*)/, 2)
  return pathPart
    .split('/')
    .filter((part) => part && part !== '.')
    .every((part) => part !== '..' && !part.includes('\\'))
}

function buildPageImageUrl(slug: string, src: string): string {
  const trimmed = src.trim()
  const [pathPart, suffix = ''] = trimmed.split(/([?#].*)/, 2)
  const encodedPath = pathPart
    .split('/')
    .filter((part) => part && part !== '.')
    .map((part) => encodeURIComponent(part))
    .join('/')
  return buildApiUrl(`/pages/${encodeURIComponent(slug)}/images/${encodedPath}${suffix}`)
}

async function fetchAndRenderMarkdown(slug: string) {
  loading.value = true
  tocItems.value = []
  activeHeadingId.value = ''
  try {
    let raw = builtinMarkdownPages[slug]
    if (!raw) {
      const resp = await fetch(buildApiUrl(`/pages/${encodeURIComponent(slug)}`), {
        headers: authStore.token ? { Authorization: `Bearer ${authStore.token}` } : {},
      })
      if (!resp.ok) {
        renderedHtml.value = `<p class="text-red-500">${t('common.pageNotFound')}</p>`
        return
      }
      raw = await resp.text()
    }

    raw = raw.replace(
      /!\[([^\]]*)\]\(([^)]+)\)/g,
      (match, alt, src) => isRelativeMarkdownAsset(src) ? `![${alt}](${buildPageImageUrl(slug, src)})` : match
    )

    const html = marked.parse(raw) as string
    const sanitized = DOMPurify.sanitize(html, {
      ADD_TAGS: ['iframe'],
      ADD_ATTR: ['allowfullscreen', 'frameborder', 'src'],
    })

    // Inject IDs into headings and build TOC
    const toc: TocItem[] = []
    let headingIndex = 0
    const withIds = sanitized.replace(
      /<(h[1-4])[^>]*>(.*?)<\/h[1-4]>/gi,
      (_, tag: string, content: string) => {
        const level = parseInt(tag[1])
        const text = content.replace(/<[^>]+>/g, '').trim()
        const id = generateHeadingId(text, headingIndex++)
        if (level >= 2) {
          toc.push({ id, text, level })
        }
        return `<${tag} id="${id}">${content}</${tag}>`
      }
    )

    renderedHtml.value = withIds
    tocItems.value = toc
  } catch {
    renderedHtml.value = '<p class="text-red-500">Failed to load page</p>'
  } finally {
    loading.value = false
    await nextTick()
    await nextTick()
    injectCopyButtons()
  }
}

function scrollToHeading(id: string) {
  const container = markdownContainer.value
  if (!container) return
    const el = container.querySelector(`#${CSS.escape(id)}`)
    if (el) {
      el.scrollIntoView({ behavior: 'smooth', block: 'start' })
      activeHeadingId.value = id
  }
}

function selectGuideTab(slug: string) {
  if (!guideTabSlugs.has(slug) || activeGuideSlug.value === slug) return
  activeGuideSlug.value = slug
  if (markdownScrollContainer.value) {
    markdownScrollContainer.value.scrollTop = 0
  }
}

let scrollRafId = 0
function onContentScroll() {
  if (scrollRafId) return
  scrollRafId = requestAnimationFrame(() => {
    scrollRafId = 0
    const scrollContainer = markdownScrollContainer.value
    const content = markdownContainer.value
    if (!scrollContainer || !content || tocItems.value.length === 0) return

    const containerRect = scrollContainer.getBoundingClientRect()
    let current = ''

    for (const item of tocItems.value) {
      const el = content.querySelector(`#${CSS.escape(item.id)}`) as HTMLElement | null
      if (el) {
        const elRect = el.getBoundingClientRect()
        if (elRect.top - containerRect.top <= 100) {
          current = item.id
        }
      }
    }
    activeHeadingId.value = current
  })
}

function injectCopyButtons() {
  const container = markdownContainer.value
  if (!container) return

  container.querySelectorAll('pre').forEach((pre) => {
    if (pre.querySelector('.copy-btn')) return
    const btn = document.createElement('button')
    btn.className = 'copy-btn'
    btn.textContent = t('customPage.copyCode')
    btn.addEventListener('click', async () => {
      const code = pre.querySelector('code')?.textContent ?? pre.textContent ?? ''
      try {
        await navigator.clipboard.writeText(code)
        btn.textContent = t('customPage.copiedCode')
        setTimeout(() => { btn.textContent = t('customPage.copyCode') }, 2000)
      } catch {
        btn.textContent = t('customPage.copyCodeFailed')
        setTimeout(() => { btn.textContent = t('customPage.copyCode') }, 2000)
      }
    })
    pre.style.position = 'relative'
    pre.appendChild(btn)
  })
}

watch(markdownSlug, (slug) => {
  activeGuideSlug.value = guideTabSlugs.has(slug) ? slug : ''
}, { immediate: true })

watch(displayedMarkdownSlug, (slug) => {
  if (slug) {
    fetchAndRenderMarkdown(slug)
  } else {
    renderedHtml.value = ''
    tocItems.value = []
  }
}, { immediate: true })

onMounted(async () => {
  pageTheme.value = detectTheme()

  if (typeof document !== 'undefined') {
    themeObserver = new MutationObserver(() => {
      pageTheme.value = detectTheme()
    })
    themeObserver.observe(document.documentElement, {
      attributes: true,
      attributeFilter: ['class'],
    })
  }

  if (appStore.publicSettingsLoaded) return
  loading.value = true
  try {
    await appStore.fetchPublicSettings()
  } finally {
    loading.value = false
  }
})

onUnmounted(() => {
  if (themeObserver) {
    themeObserver.disconnect()
    themeObserver = null
  }
})
</script>

<style scoped>
.custom-page-layout {
  @apply flex flex-col;
  height: calc(100vh - 64px - 4rem);
}

.guide-page-host {
  min-height: 0;
}

.guide-shell {
  display: flex;
  height: 100%;
  min-height: 0;
  flex-direction: column;
  overflow: hidden;
  border: 1px solid rgb(229 231 235);
  border-radius: 8px;
  background: rgb(255 255 255);
}

:global(.dark) .guide-shell {
  border-color: rgb(51 65 85);
  background: rgb(15 23 42);
}

.guide-tabs {
  display: flex;
  flex: 0 0 auto;
  gap: 4px;
  overflow-x: auto;
  border-bottom: 1px solid rgb(229 231 235);
  padding: 0 16px;
  background: rgb(249 250 251);
}

.guide-tab {
  min-height: 48px;
  flex: 0 0 auto;
  border-bottom: 3px solid transparent;
  padding: 0 16px;
  font-size: 15px;
  font-weight: 600;
  color: rgb(75 85 99);
  transition: border-color 150ms ease, color 150ms ease, background-color 150ms ease;
}

.guide-tab:hover {
  background: rgb(243 244 246);
  color: rgb(17 24 39);
}

.guide-tab:focus-visible {
  outline: 2px solid rgb(20 184 166);
  outline-offset: -2px;
}

.guide-tab-active {
  border-bottom-color: rgb(13 148 136);
  color: rgb(15 118 110);
}

:global(.dark) .guide-tabs {
  border-color: rgb(51 65 85);
  background: rgb(30 41 59);
}

:global(.dark) .guide-tab {
  color: rgb(203 213 225);
}

:global(.dark) .guide-tab:hover {
  background: rgb(51 65 85);
  color: rgb(248 250 252);
}

:global(.dark) .guide-tab-active {
  border-bottom-color: rgb(45 212 191);
  color: rgb(94 234 212);
}

.guide-reader {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 260px;
  flex: 1;
  min-height: 0;
  overflow: hidden;
}

.guide-content-scroll {
  min-width: 0;
  overflow-y: auto;
  scroll-behavior: smooth;
}

.toc-sidebar {
  @apply flex h-full flex-col border-l border-gray-200 bg-gray-50 dark:border-dark-600 dark:bg-dark-800;
  overflow: hidden;
}

@media (max-width: 1024px) {
  .guide-reader {
    display: block;
  }

  .toc-sidebar {
    display: none;
  }
}

@media (max-width: 640px) {
  .guide-tabs {
    padding: 0 8px;
  }

  .guide-tab {
    min-height: 44px;
    padding: 0 12px;
    font-size: 14px;
  }
}

.toc-header {
  @apply flex items-center justify-between px-4 py-3 border-b border-gray-200 dark:border-dark-600;
}

.toc-title {
  @apply text-sm font-semibold text-gray-700 dark:text-dark-200;
}

.toc-nav {
  @apply flex-1 overflow-y-auto px-3 py-3;
}

.toc-item {
  @apply relative block border-l border-gray-300 px-3 py-2 text-sm leading-5 text-gray-600 transition-colors dark:border-dark-500 dark:text-dark-300;
  @apply hover:bg-gray-100 hover:text-gray-900 dark:hover:bg-dark-700 dark:hover:text-white;
}

.toc-item.toc-active {
  @apply border-l-2 border-primary-500 bg-primary-50 font-semibold text-primary-700 dark:bg-primary-900/20 dark:text-primary-300;
}

.toc-level-2 { padding-left: 12px; }
.toc-level-3 { padding-left: 24px; }
.toc-level-4 { padding-left: 36px; }

.guide-mobile-toc {
  display: none;
  margin: 20px 20px 0;
  border: 1px solid rgb(209 213 219);
  border-radius: 8px;
  background: rgb(249 250 251);
}

.guide-mobile-toc summary {
  cursor: pointer;
  padding: 12px 14px;
  font-size: 14px;
  font-weight: 700;
  color: rgb(31 41 55);
}

.guide-mobile-toc-nav {
  display: grid;
  gap: 2px;
  padding: 0 8px 10px;
}

.guide-mobile-toc-nav a {
  display: block;
  border-radius: 6px;
  padding-top: 7px;
  padding-bottom: 7px;
  font-size: 13px;
  line-height: 1.4;
  color: rgb(75 85 99);
}

.guide-mobile-toc-nav a.toc-active {
  background: rgb(204 251 241);
  color: rgb(15 118 110);
  font-weight: 700;
}

:global(.dark) .guide-mobile-toc {
  border-color: rgb(71 85 105);
  background: rgb(30 41 59);
}

:global(.dark) .guide-mobile-toc summary {
  color: rgb(241 245 249);
}

:global(.dark) .guide-mobile-toc-nav a {
  color: rgb(203 213 225);
}

:global(.dark) .guide-mobile-toc-nav a.toc-active {
  background: rgba(20, 184, 166, 0.16);
  color: rgb(94 234 212);
}

@media (max-width: 1024px) {
  .guide-mobile-toc {
    display: block;
  }
}

.custom-embed-shell {
  @apply relative;
  @apply h-full w-full overflow-hidden rounded-2xl;
  @apply bg-gradient-to-b from-gray-50 to-white dark:from-dark-900 dark:to-dark-950;
  @apply p-0;
}

.leaderboard-embed-host,
.custom-embed-shell-leaderboard {
  background: transparent;
}

.custom-embed-shell-leaderboard {
  border-radius: 0;
}

.custom-open-fab {
  @apply absolute right-3 top-3 z-10;
  @apply shadow-sm backdrop-blur supports-[backdrop-filter]:bg-white/80 dark:supports-[backdrop-filter]:bg-dark-800/80;
}

.custom-embed-frame {
  display: block;
  margin: 0;
  width: 100%;
  height: 100%;
  border: 0;
  border-radius: 0;
  box-shadow: none;
  background: transparent;
}
</style>

<style>
.markdown-page-content {
  width: min(100%, 920px);
  margin: 0 auto;
  padding: 52px 64px 96px;
  line-height: 1.75;
  color: inherit;
}
.markdown-page-content h1 {
  margin: 0 0 18px;
  font-size: 42px;
  line-height: 1.12;
  font-weight: 800;
  letter-spacing: 0;
  color: rgb(17 24 39);
}
.dark .markdown-page-content h1 { color: rgb(248 250 252); }
.markdown-page-content h2 {
  @apply mb-4 mt-12 text-2xl font-bold text-gray-900 dark:text-white;
  scroll-margin-top: 24px;
}
.markdown-page-content h3 {
  @apply mb-3 mt-8 text-xl font-semibold text-gray-900 dark:text-white;
  scroll-margin-top: 24px;
}
.markdown-page-content h4 { @apply mb-2 mt-6 text-lg font-semibold text-gray-900 dark:text-white; }
.markdown-page-content > p:first-of-type {
  margin-bottom: 28px;
  font-size: 17px;
  line-height: 1.75;
  color: rgb(75 85 99);
}
.dark .markdown-page-content > p:first-of-type { color: rgb(203 213 225); }
.markdown-page-content p { @apply mb-4 text-gray-700 dark:text-dark-200; }
.markdown-page-content ul { @apply mb-5 list-disc space-y-1.5 pl-6 text-gray-700 dark:text-dark-200; }
.markdown-page-content ol { @apply mb-5 list-decimal space-y-1.5 pl-6 text-gray-700 dark:text-dark-200; }
.markdown-page-content li { padding-left: 2px; }
.markdown-page-content a { @apply font-semibold text-primary-600 underline decoration-2 underline-offset-2 hover:text-primary-700 dark:text-primary-300 dark:hover:text-primary-200; }
.markdown-page-content blockquote {
  margin: 24px 0;
  border: 2px solid rgb(17 24 39);
  border-radius: 6px;
  background: rgb(254 240 138);
  padding: 16px 18px;
  box-shadow: 4px 4px 0 rgb(17 24 39);
  color: rgb(31 41 55);
}
.markdown-page-content blockquote p { margin: 0; color: inherit; }
.dark .markdown-page-content blockquote {
  border-color: rgb(226 232 240);
  background: rgb(113 63 18);
  box-shadow: 4px 4px 0 rgb(226 232 240);
  color: rgb(254 249 195);
}
.markdown-page-content img {
  @apply my-6 h-auto max-w-full;
  border: 2px solid rgb(17 24 39);
  border-radius: 6px;
  box-shadow: 4px 4px 0 rgb(17 24 39);
}
.dark .markdown-page-content img {
  border-color: rgb(226 232 240);
  box-shadow: 4px 4px 0 rgb(226 232 240);
}
.markdown-page-content table { @apply my-6 w-full border-collapse overflow-hidden text-sm; }
.markdown-page-content th { @apply border border-gray-300 bg-gray-50 px-4 py-3 text-left font-semibold text-gray-900 dark:border-dark-500 dark:bg-dark-700 dark:text-white; }
.markdown-page-content td { @apply border border-gray-300 px-4 py-3 text-gray-700 dark:border-dark-500 dark:text-dark-200; }
.markdown-page-content code { @apply rounded bg-gray-100 px-1.5 py-0.5 font-mono text-sm text-gray-900 dark:bg-dark-700 dark:text-dark-100; }
.markdown-page-content pre { @apply relative my-6 overflow-x-auto rounded-md bg-gray-900 p-4 text-gray-100 dark:bg-dark-950; }
.markdown-page-content pre code { @apply bg-transparent p-0 text-inherit; }
.markdown-page-content hr { @apply my-6 border-gray-200 dark:border-dark-600; }

@media (max-width: 768px) {
  .markdown-page-content {
    padding: 32px 20px 72px;
  }

  .markdown-page-content h1 {
    font-size: 32px;
    line-height: 1.2;
  }

  .markdown-page-content h2 {
    margin-top: 38px;
    font-size: 22px;
  }

  .markdown-page-content table {
    display: block;
    overflow-x: auto;
  }
}

.copy-btn {
  position: absolute;
  top: 8px;
  right: 8px;
  padding: 4px 10px;
  font-size: 12px;
  border-radius: 4px;
  background: rgba(255, 255, 255, 0.15);
  color: #e2e8f0;
  border: 1px solid rgba(255, 255, 255, 0.2);
  cursor: pointer;
  opacity: 0;
  transition: opacity 0.2s, background 0.2s;
  font-family: inherit;
}
.copy-btn:hover { background: rgba(255, 255, 255, 0.25); }
pre:hover .copy-btn { opacity: 1; }

@media (hover: none) {
  .copy-btn { opacity: 1; }
}
</style>

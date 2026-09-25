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
          ref="embedShell"
          class="custom-embed-shell"
          :class="{ 'custom-embed-shell-leaderboard': menuItemId === 'usage-leaderboard' }"
        >
          <a
            ref="openButton"
            v-if="showOpenInNewTab"
            :href="embeddedUrl"
            target="_blank"
            rel="noopener noreferrer"
            class="btn btn-secondary btn-sm custom-open-fab"
            :style="openButtonPosition ? { left: `${openButtonPosition.x}px`, top: `${openButtonPosition.y}px`, right: 'auto' } : undefined"
            @pointerdown="startButtonDrag"
            @pointermove="moveButtonDrag"
            @pointerup="endButtonDrag"
            @pointercancel="endButtonDrag"
            @lostpointercapture="endButtonDrag"
            @dragstart.prevent
            @click="handleOpenButtonClick"
          >
            <Icon name="externalLink" size="sm" class="mr-1.5" :stroke-width="2" />
            {{ t('customPage.openInNewTab') }}
          </a>
          <iframe
            :src="embeddedUrl"
            class="custom-embed-frame"
            allowfullscreen
            referrerpolicy="no-referrer"
          ></iframe>
        </div>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { useResizeObserver } from '@vueuse/core'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores'
import { useAuthStore } from '@/stores/auth'
import { useAdminSettingsStore } from '@/stores/adminSettings'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import { apiClient, buildApiUrl } from '@/api/client'
import { buildEmbeddedUrl, detectTheme } from '@/utils/embedded-url'
import { marked } from 'marked'
import DOMPurify from 'dompurify'
import ccSwitchCodexGuide from '@/content/cc-switch-codex.md?raw'
import siteUsageGuide from '@/content/site-usage.md?raw'
import sessionRecoveryGuide from '@/content/session-recovery.md?raw'
import apiErrorsGuide from '@/content/api-errors.md?raw'

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
  { slug: 'session-recovery', label: '会话与配置恢复' },
  { slug: 'api-errors', label: 'API 错误与网络排查' },
]
const guideTabSlugs = new Set(guideTabs.map((tab) => tab.slug))

const builtinMarkdownPages: Record<string, string> = {
  'cc-switch-codex': ccSwitchCodexGuide,
  'site-usage': siteUsageGuide,
  'session-recovery': sessionRecoveryGuide,
  'api-errors': apiErrorsGuide,
}

const embedShell = ref<HTMLElement | null>(null)
const openButton = ref<HTMLAnchorElement | null>(null)
const openButtonPosition = ref<{ x: number; y: number } | null>(null)
let buttonDrag: { pointerId: number; startX: number; startY: number; x: number; y: number } | null = null
let suppressButtonClick = false

function setOpenButtonPosition(x: number, y: number) {
  const shell = embedShell.value
  const button = openButton.value
  if (!shell || !button) return
  openButtonPosition.value = {
    x: Math.max(0, Math.min(x, shell.clientWidth - button.offsetWidth)),
    y: Math.max(0, Math.min(y, shell.clientHeight - button.offsetHeight)),
  }
}

function startButtonDrag(event: PointerEvent) {
  if (event.button !== 0 || !event.isPrimary || !openButton.value) return
  const button = openButton.value
  suppressButtonClick = false
  buttonDrag = {
    pointerId: event.pointerId, startX: event.clientX, startY: event.clientY,
    x: button.offsetLeft, y: button.offsetTop,
  }
  // Capture keeps receiving moves when the pointer crosses the embedded iframe.
  button.setPointerCapture(event.pointerId)
}

function moveButtonDrag(event: PointerEvent) {
  if (!buttonDrag || event.pointerId !== buttonDrag.pointerId) return
  const dx = event.clientX - buttonDrag.startX
  const dy = event.clientY - buttonDrag.startY
  if (!suppressButtonClick && Math.hypot(dx, dy) < 4) return
  suppressButtonClick = true
  setOpenButtonPosition(buttonDrag.x + dx, buttonDrag.y + dy)
  event.preventDefault()
}

function endButtonDrag(event: PointerEvent) {
  if (!buttonDrag || event.pointerId !== buttonDrag.pointerId) return
  buttonDrag = null
  if (openButton.value?.hasPointerCapture(event.pointerId)) {
    openButton.value.releasePointerCapture(event.pointerId)
  }
}

function handleOpenButtonClick(event: MouseEvent) {
  if (suppressButtonClick && event.detail !== 0) event.preventDefault()
  suppressButtonClick = false
}

useResizeObserver([embedShell, openButton], () => {
  if (openButtonPosition.value) {
    setOpenButtonPosition(openButtonPosition.value.x, openButtonPosition.value.y)
  }
})

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
  const url = buildEmbeddedUrl(
    menuItem.value.url,
    authStore.user?.id,
    authStore.token,
    pageTheme.value,
    locale.value,
  )

  // The standalone leaderboard reads its short-lived access credential from
  // the token query parameter; other custom embeds must not receive it.
  if (menuItemId.value !== 'usage-leaderboard' || !authStore.token) return url
  try {
    const embedded = new URL(url)
    embedded.searchParams.set('token', authStore.token)
    return embedded.toString()
  } catch {
    return url
  }
})

const isValidUrl = computed(() => {
  if (isMarkdownMode.value) return false
  const url = embeddedUrl.value
  return url.startsWith('http://') || url.startsWith('https://')
})

/** @brief Show the external-link control only when the selected menu permits it. */
const showOpenInNewTab = computed(
  () => menuItemId.value !== 'usage-leaderboard' && menuItem.value?.hide_open_button !== true,
)

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
      const resp = await apiClient.get<string>(`/pages/${encodeURIComponent(slug)}`)
      raw = resp.data
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

/* @brief Guide surfaces follow the shared console palette in both themes. */
.guide-page-host {
  min-height: 0;
}

.guide-shell {
  --guide-surface: var(--signal-surface, #fff);
  --guide-raised: var(--signal-raised, #f5f5f6);
  --guide-line: var(--signal-line, #e3e3e6);
  --guide-text: var(--signal-text, #232326);
  --guide-muted: var(--signal-muted, #737379);
  --guide-hover: var(--signal-hover, #ececee);
  display: flex;
  height: 100%;
  min-height: 0;
  flex-direction: column;
  overflow: hidden;
  container-type: inline-size;
  border: 1px solid var(--guide-line);
  border-radius: 12px;
  background: var(--guide-surface);
  color: var(--guide-text);
  box-shadow: 0 2px 8px #00000003;
}

.guide-tabs {
  display: flex;
  flex: 0 0 auto;
  gap: 6px;
  overflow-x: auto;
  border-bottom: 1px solid var(--guide-line);
  padding: 12px 20px;
  scrollbar-width: thin;
}

.guide-tab {
  min-height: 38px;
  flex: 0 0 auto;
  border: 1px solid transparent;
  border-radius: 7px;
  padding: 8px 14px;
  font-size: 13px;
  font-weight: 500;
  line-height: 20px;
  color: var(--guide-muted);
  transition: color 160ms ease, background-color 160ms ease, border-color 160ms ease;
}

.guide-tab:hover {
  background: var(--guide-raised);
  color: var(--guide-text);
}

.guide-tab-active {
  border-color: var(--guide-line);
  background: var(--guide-raised);
  color: var(--guide-text);
  font-weight: 600;
}

.guide-tab:focus-visible,
.toc-item:focus-visible,
.guide-mobile-toc summary:focus-visible,
.guide-mobile-toc-nav a:focus-visible {
  outline: 2px solid var(--guide-muted);
  outline-offset: -2px;
}

.guide-reader {
  display: grid;
  grid-template-columns: minmax(0, 1fr);
  flex: 1;
  min-height: 0;
  overflow: hidden;
}

.guide-reader:has(.toc-sidebar) {
  grid-template-columns: minmax(0, 1fr) 232px;
}

.guide-content-scroll {
  min-width: 0;
  overflow-y: auto;
  overscroll-behavior: contain;
  scrollbar-width: thin;
  scrollbar-color: var(--guide-line) transparent;
  scrollbar-gutter: stable;
}

.toc-sidebar {
  display: flex;
  min-height: 0;
  flex-direction: column;
  overflow: hidden;
  border-left: 1px solid var(--guide-line);
  background: var(--signal-bg, #fafafa);
}

.toc-header {
  padding: 28px 24px 14px;
}

.toc-title {
  font-size: 12px;
  font-weight: 600;
  color: var(--guide-text);
}

.toc-nav {
  flex: 1;
  overflow-y: auto;
  padding: 0 14px 28px;
  scrollbar-width: thin;
  scrollbar-color: var(--guide-line) transparent;
}

.toc-item {
  display: block;
  margin: 2px 0;
  border-left: 2px solid transparent;
  border-radius: 0 6px 6px 0;
  padding: 8px 10px;
  font-size: 12px;
  line-height: 1.6;
  color: var(--guide-muted);
  overflow-wrap: anywhere;
  transition: color 160ms ease, background-color 160ms ease, border-color 160ms ease;
}

.toc-item:hover,
.guide-mobile-toc-nav a:hover {
  background: var(--guide-raised);
  color: var(--guide-text);
}

.toc-item.toc-active {
  border-left-color: var(--guide-text);
  background: var(--guide-hover);
  color: var(--guide-text);
  font-weight: 600;
}

.toc-level-2 { padding-left: 10px; }
.toc-level-3 { padding-left: 22px; }
.toc-level-4 { padding-left: 34px; }

.guide-mobile-toc {
  display: none;
  margin: 20px 24px 0;
  border: 1px solid var(--guide-line);
  border-radius: 8px;
  background: var(--guide-raised);
}

.guide-mobile-toc summary {
  cursor: pointer;
  padding: 12px 16px;
  font-size: 13px;
  font-weight: 600;
  color: var(--guide-text);
}

.guide-mobile-toc-nav {
  display: grid;
  gap: 2px;
  max-height: 260px;
  overflow-y: auto;
  padding: 0 8px 10px;
}

.guide-mobile-toc-nav a {
  display: block;
  border-radius: 5px;
  padding-top: 8px;
  padding-bottom: 8px;
  font-size: 12px;
  line-height: 1.5;
  color: var(--guide-muted);
}

.guide-mobile-toc-nav a.toc-active {
  background: var(--guide-hover);
  color: var(--guide-text);
  font-weight: 600;
}

@container (max-width: 760px) {
  .guide-reader:has(.toc-sidebar) { grid-template-columns: minmax(0, 1fr); }
  .toc-sidebar { display: none; }
  .guide-mobile-toc { display: block; }
  .guide-tabs { padding: 10px 12px; }
  .guide-tab { padding-inline: 12px; }
}

@media (prefers-reduced-motion: reduce) {
  .guide-tab,
  .toc-item { transition: none; }
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
  @apply absolute right-3 top-3 z-10 w-max max-w-full touch-none select-none transition-colors;
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
/* @brief Markdown typography and controls inherit the guide's neutral theme. */
.markdown-page-content {
  width: min(100%, 960px);
  margin: 0 auto;
  padding: 40px clamp(24px, 4vw, 56px) 80px;
  font-size: 14px;
  line-height: 1.9;
  color: var(--guide-text);
  overflow-wrap: anywhere;
}

.markdown-page-content h1 {
  margin: 0 0 16px;
  font-size: clamp(24px, 2.2vw, 30px);
  line-height: 1.4;
  font-weight: 600;
  letter-spacing: -0.025em;
}

.markdown-page-content h2 {
  margin: 40px 0 16px;
  border-top: 1px solid var(--guide-line);
  padding-top: 28px;
  font-size: 20px;
  line-height: 1.5;
  font-weight: 600;
  letter-spacing: -0.015em;
}

.markdown-page-content h3 {
  margin: 28px 0 12px;
  font-size: 16px;
  line-height: 1.6;
  font-weight: 600;
}

.markdown-page-content h4 {
  margin: 24px 0 10px;
  font-size: 14px;
  font-weight: 600;
}

.markdown-page-content :is(h1, h2, h3, h4) {
  color: var(--guide-text);
  scroll-margin-top: 24px;
}

.markdown-page-content > p:first-of-type {
  margin-bottom: 28px;
  font-size: 14px;
  color: var(--guide-muted);
}

.markdown-page-content p { margin-bottom: 16px; }
.markdown-page-content :is(ul, ol) { margin-bottom: 20px; padding-left: 24px; }
.markdown-page-content ul { list-style-type: disc; }
.markdown-page-content ol { list-style-type: decimal; }
.markdown-page-content li { margin-block: 6px; padding-left: 4px; }
.markdown-page-content li::marker { color: var(--guide-muted); }
.markdown-page-content li > :is(ul, ol) { margin-block: 8px; }
.markdown-page-content strong { font-weight: 600; }

.markdown-page-content a {
  border-radius: 2px;
  color: var(--guide-text);
  font-weight: 500;
  text-decoration: underline;
  text-decoration-color: var(--guide-muted);
  text-underline-offset: 4px;
  transition: text-decoration-color 160ms ease, background-color 160ms ease;
}

.markdown-page-content a:hover {
  background: var(--guide-raised);
  text-decoration-color: var(--guide-text);
}

.markdown-page-content a:focus-visible {
  outline: 2px solid var(--guide-muted);
  outline-offset: 3px;
}

.markdown-page-content blockquote {
  margin: 24px 0;
  border: 1px solid var(--guide-line);
  border-left: 3px solid var(--guide-muted);
  border-radius: 0 8px 8px 0;
  background: var(--guide-raised);
  padding: 16px 20px;
  font-size: 13px;
  color: var(--guide-text);
}

.markdown-page-content blockquote > :last-child { margin-bottom: 0; }

.markdown-page-content img {
  display: block;
  height: auto;
  max-width: 100%;
  margin: 24px auto;
  border: 1px solid var(--guide-line);
  border-radius: 8px;
}

.markdown-page-content table {
  display: block;
  width: 100%;
  margin: 24px 0;
  overflow-x: auto;
  border: 1px solid var(--guide-line);
  border-radius: 8px;
  border-spacing: 0;
  font-size: 13px;
  scrollbar-width: thin;
}

.markdown-page-content :is(th, td) {
  width: 1%;
  border-bottom: 1px solid var(--guide-line);
  padding: 12px 16px;
  text-align: left;
  vertical-align: top;
}

.markdown-page-content th {
  background: var(--guide-raised);
  font-weight: 600;
  white-space: nowrap;
}

.markdown-page-content tr:last-child td { border-bottom: 0; }
.markdown-page-content tbody tr:hover { background: var(--guide-raised); }

.markdown-page-content code {
  border: 1px solid var(--guide-line);
  border-radius: 4px;
  background: var(--guide-raised);
  padding: 2px 5px;
  font-family: ui-monospace, SFMono-Regular, Consolas, monospace;
  font-size: 0.88em;
}

.markdown-page-content pre {
  position: relative;
  margin: 24px 0;
  overflow-x: auto;
  border: 1px solid #303034;
  border-radius: 8px;
  background: #202023;
  padding: 48px 20px 20px;
  color: #ededee;
  font-size: 13px;
  line-height: 1.8;
  scrollbar-width: thin;
  scrollbar-color: #525258 transparent;
}

.markdown-page-content pre code {
  border: 0;
  background: transparent;
  padding: 0;
  color: inherit;
  font-size: inherit;
}

.markdown-page-content hr { margin: 32px 0; border-color: var(--guide-line); }

.markdown-page-content .copy-btn {
  position: absolute;
  right: 12px;

  top: 10px;

  padding: 4px 10px;
  border: 1px solid #525258;
  border-radius: 5px;
  background: #303034;
  color: #ededee;
  font-family: inherit;
  font-size: 12px;
  line-height: 20px;
  cursor: pointer;
  transition: background-color 160ms ease, border-color 160ms ease;
}

.markdown-page-content .copy-btn:hover { background: #45454b; border-color: #737379; }
.markdown-page-content .copy-btn:focus-visible { outline: 2px solid #ededee; outline-offset: 2px; }

@container (max-width: 760px) {
  .markdown-page-content { padding: 28px 24px 56px; }
  .markdown-page-content h2 { margin-top: 32px; padding-top: 24px; font-size: 18px; }
}

@media (prefers-reduced-motion: reduce) {
  .markdown-page-content a,
  .markdown-page-content .copy-btn { transition: none; }
}
</style>

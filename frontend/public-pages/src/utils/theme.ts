export type Theme = 'light' | 'dark'

const THEME_STORAGE_KEY = 'theme'
const THEME_CHANGE_EVENT = 'sub2api-theme-change'

function canUseDOM() {
  return typeof window !== 'undefined' && typeof document !== 'undefined'
}

export function getStoredTheme(): Theme {
  if (!canUseDOM()) return 'light'

  try {
    return window.localStorage.getItem(THEME_STORAGE_KEY) === 'dark' ? 'dark' : 'light'
  } catch {
    return 'light'
  }
}

export function applyTheme(theme: Theme) {
  if (!canUseDOM()) return

  const isDark = theme === 'dark'
  document.documentElement.classList.toggle('dark', isDark)
  document.documentElement.dataset.theme = theme
  document.documentElement.style.colorScheme = theme
}

export function initTheme(): Theme {
  const theme = getStoredTheme()
  applyTheme(theme)
  return theme
}

export function setTheme(theme: Theme) {
  if (!canUseDOM()) return

  try {
    window.localStorage.setItem(THEME_STORAGE_KEY, theme)
  } catch {
    // Keep the in-memory theme working when storage is unavailable.
  }
  applyTheme(theme)
  window.dispatchEvent(new CustomEvent<Theme>(THEME_CHANGE_EVENT, { detail: theme }))
}

export function toggleTheme(): Theme {
  const nextTheme = getStoredTheme() === 'dark' ? 'light' : 'dark'
  setTheme(nextTheme)
  return nextTheme
}

export { THEME_CHANGE_EVENT }

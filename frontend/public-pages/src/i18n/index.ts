import { createI18n } from 'vue-i18n'
import enCommon from './locales/en/common'
import enLanding from './locales/en/landing'
import enDashboard from './locales/en/dashboard'
import zhCommon from './locales/zh/common'
import zhLanding from './locales/zh/landing'
import zhDashboard from './locales/zh/dashboard'

type LocaleCode = 'en' | 'zh'
const LOCALE_KEY = 'sub2api_locale'
const DEFAULT_LOCALE: LocaleCode = 'en'
function isLocaleCode(value: string | null): value is LocaleCode {
  return value === 'en' || value === 'zh'
}
function getDefaultLocale(): LocaleCode {
  const saved = localStorage.getItem(LOCALE_KEY)
  return isLocaleCode(saved) ? saved : navigator.language.toLowerCase().startsWith('zh') ? 'zh' : DEFAULT_LOCALE
}

export const i18n = createI18n({
  legacy: false,
  locale: getDefaultLocale(),
  fallbackLocale: DEFAULT_LOCALE,
  messages: {
    en: { ...enCommon, ...enLanding, ...enDashboard },
    zh: { ...zhCommon, ...zhLanding, ...zhDashboard },
  },
  warnHtmlMessage: false,
})
export async function initI18n(): Promise<void> {
  document.documentElement.lang = getLocale()
}
export async function setLocale(locale: string): Promise<void> {
  if (!isLocaleCode(locale)) return
  i18n.global.locale.value = locale
  localStorage.setItem(LOCALE_KEY, locale)
  document.documentElement.lang = locale
}
export function getLocale(): LocaleCode {
  const current = i18n.global.locale.value
  return isLocaleCode(current) ? current : DEFAULT_LOCALE
}
export const availableLocales = [
  { code: 'en', name: 'English', flag: '🇺🇸' },
  { code: 'zh', name: '中文', flag: '🇨🇳' },
] as const
export default i18n

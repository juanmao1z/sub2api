/**
 * Shared URL builder for iframe-embedded pages.
 * Used by PurchaseSubscriptionView and CustomPageView to build consistent URLs
 * with user_id, token, theme, lang, ui_mode, src_host, and src parameters.
 */

const EMBEDDED_USER_ID_QUERY_KEY = 'user_id'
const EMBEDDED_AUTH_TOKEN_QUERY_KEY = 'token'
const EMBEDDED_THEME_QUERY_KEY = 'theme'
const EMBEDDED_LANG_QUERY_KEY = 'lang'
const EMBEDDED_UI_MODE_QUERY_KEY = 'ui_mode'
const EMBEDDED_UI_MODE_VALUE = 'embedded'
const EMBEDDED_SRC_HOST_QUERY_KEY = 'src_host'
const EMBEDDED_SRC_QUERY_KEY = 'src_url'

/**
 * @brief Build an embedded page URL using the supported light color scheme.
 * @param baseUrl Base URL for the embedded page.
 * @param userId Optional authenticated user identifier.
 * @param authToken Optional bearer token passed to the embedded page.
 * @param _theme Legacy theme argument retained for caller compatibility.
 * @param lang Optional locale identifier.
 * @return The augmented URL, or the original value when it is invalid.
 */
export function buildEmbeddedUrl(
  baseUrl: string,
  userId?: number,
  authToken?: string | null,
  _theme: 'light' | 'dark' = 'light',
  lang?: string,
): string {
  if (!baseUrl) return baseUrl
  try {
    const url = new URL(baseUrl)
    if (userId) {
      url.searchParams.set(EMBEDDED_USER_ID_QUERY_KEY, String(userId))
    }
    if (authToken) {
      url.searchParams.set(EMBEDDED_AUTH_TOKEN_QUERY_KEY, authToken)
    }
    url.searchParams.set(EMBEDDED_THEME_QUERY_KEY, 'light')
    if (lang) {
      url.searchParams.set(EMBEDDED_LANG_QUERY_KEY, lang)
    }
    url.searchParams.set(EMBEDDED_UI_MODE_QUERY_KEY, EMBEDDED_UI_MODE_VALUE)
    // Source tracking: let the embedded page know where it's being loaded from
    if (typeof window !== 'undefined') {
      url.searchParams.set(EMBEDDED_SRC_HOST_QUERY_KEY, window.location.origin)
      url.searchParams.set(EMBEDDED_SRC_QUERY_KEY, window.location.href)
    }
    return url.toString()
  } catch {
    return baseUrl
  }
}

/** @brief Return the application's only supported color scheme. @return Always `light`. */
export function detectTheme(): 'light' {
  return 'light'
}

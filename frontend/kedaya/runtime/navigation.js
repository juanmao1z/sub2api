/** @file @brief Keep the homepage separate while letting the native console own internal routes. */

/** @brief Identify the original homepage entry, including root aliases.
 * @param pathname Absolute URL pathname. @return Whether the homepage runtime owns it.
 */
export function isHomePath(pathname) {
  return ['/', '/home', '/home/', '/index.html'].includes(pathname);
}

/** @brief Select the application that implements a route.
 * @param pathname Absolute URL pathname. @return Homepage or the native console entry.
 */
export function runtimeForPath(pathname) {
  if (isHomePath(pathname)) return 'home';
  // The native bundle contains both the console router and custom-page views.
  return 'native';
}

/** @brief Install full navigation when crossing application entry boundaries.
 * @param win Browser window or equivalent test double.
 * @param originalOrigin Production origin whose internal links also work in preview.
 */
export function installNavigation(win, originalOrigin = 'https://api.zhouz.online') {
  const runtime = runtimeForPath(win.location.pathname);
  const crosses = url => url.origin === win.location.origin && runtimeForPath(url.pathname) !== runtime;
  for (const method of ['pushState', 'replaceState']) {
    const original = win.history[method].bind(win.history);
    win.history[method] = (state, unused, target) => {
      if (target !== undefined && target !== null) {
        const url = new URL(target, win.location.href);
        if (crosses(url)) {
          win.location[method === 'replaceState' ? 'replace' : 'assign'](url.href);
          return;
        }
      }
      return original(state, unused, target);
    };
  }
  win.addEventListener('popstate', () => {
    if (runtimeForPath(win.location.pathname) !== runtime) win.location.reload();
  });
  win.document.addEventListener('click', event => {
    if (event.defaultPrevented || event.button !== 0 || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return;
    const anchor = event.target.closest?.('a[href]');
    if (!anchor || anchor.hasAttribute('download') || (anchor.target && anchor.target !== '_self')) return;
    const url = new URL(anchor.href, win.location.href);
    const previewLink = win.location.origin !== originalOrigin && url.origin === originalOrigin;
    if (previewLink) url.host = win.location.host, url.protocol = win.location.protocol;
    if (crosses(url) || previewLink) {
      event.preventDefault();
      event.stopImmediatePropagation();
      win.location.assign(url.href);
    }
  }, true);
}

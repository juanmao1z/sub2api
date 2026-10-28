/** @file @brief Keep restored public pages and the current console in separate documents. */

/** @brief Only the original landing and login routes use the native public entry. */
export function runtimeForPath(pathname = '/') {
  const path = pathname.replace(/\/+$/, '') || '/';
  return ['/', '/home', '/login'].includes(path) ? 'native' : 'review';
}

/** @brief Reload at runtime boundaries so global CSS and application state cannot leak. */
export function installNavigation(win, originalOrigin = 'https://api.zhouz.online') {
  const activeRuntime = runtimeForPath(win.location.pathname);
  const localURL = target => {
    const url = new URL(target, win.location.href);
    if (url.origin === originalOrigin && win.location.origin !== originalOrigin) {
      url.protocol = win.location.protocol;
      url.host = win.location.host;
    }
    return url;
  };
  const crossesRuntime = url => url.origin === win.location.origin && runtimeForPath(url.pathname) !== activeRuntime;

  for (const method of ['pushState', 'replaceState']) {
    const original = win.history[method].bind(win.history);
    win.history[method] = (state, unused, target) => {
      if (target !== undefined && target !== null) {
        const url = localURL(target);
        if (crossesRuntime(url)) {
          if (method === 'replaceState') win.location.replace(url.href);
          else win.location.assign(url.href);
          return;
        }
        if (url.origin === win.location.origin) target = url.href;
      }
      return original(state, unused, target);
    };
  }

  win.addEventListener('popstate', event => {
    if (runtimeForPath(win.location.pathname) === activeRuntime) return;
    event.stopImmediatePropagation();
    win.location.reload();
  }, true);

  win.document.addEventListener('click', event => {
    if (event.defaultPrevented || event.button !== 0 || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return;
    const anchor = event.target.closest?.('a[href]');
    if (!anchor || anchor.hasAttribute('download') || (anchor.target && anchor.target !== '_self')) return;
    const originalURL = new URL(anchor.href, win.location.href);
    const url = localURL(anchor.href);
    const remapped = url.origin !== originalURL.origin;
    if (!crossesRuntime(url) && !remapped) return;
    event.preventDefault();
    event.stopImmediatePropagation();
    win.location.assign(url.href);
  }, true);
}

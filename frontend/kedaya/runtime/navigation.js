/** @file @brief Keep all application routes on the production review entry. */

/** @brief The review build contains the public, user, admin, and payment routers. */
export function runtimeForPath() {
  return 'review';
}

/** @brief Rewrite captured absolute application links to the current origin in previews. */
export function installNavigation(win, originalOrigin = 'https://api.zhouz.online') {
  for (const method of ['pushState', 'replaceState']) {
    const original = win.history[method].bind(win.history);
    win.history[method] = (state, unused, target) => {
      if (target !== undefined && target !== null) {
        const url = new URL(target, win.location.href);
        if (url.origin === originalOrigin && win.location.origin !== originalOrigin) {
          url.protocol = win.location.protocol;
          url.host = win.location.host;
          target = url.href;
        }
      }
      return original(state, unused, target);
    };
  }
  win.document.addEventListener('click', event => {
    if (event.defaultPrevented || event.button !== 0 || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return;
    const anchor = event.target.closest?.('a[href]');
    if (!anchor || anchor.hasAttribute('download') || (anchor.target && anchor.target !== '_self')) return;
    const url = new URL(anchor.href, win.location.href);
    if (url.origin !== originalOrigin || win.location.origin === originalOrigin) return;
    url.protocol = win.location.protocol;
    url.host = win.location.host;
    event.preventDefault();
    event.stopImmediatePropagation();
    win.location.assign(url.href);
  }, true);
}

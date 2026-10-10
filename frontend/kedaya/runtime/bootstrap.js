/** @file @brief Load the site's original homepage or the Kedaya console, with isolated styles. */
import { runtimeForPath, installNavigation } from './navigation.js';
import { entries } from './entries.js';

installNavigation(window);
const kind = runtimeForPath(location.pathname);
document.documentElement.dataset.frontend = kind;

/** @brief Load one stylesheet before application mount to avoid a flash of unstyled content.
 * @param href Same-origin CSS path. @return Promise resolved once the sheet is ready.
 */
function stylesheet(href, legacy = false) {
  return new Promise((resolve, reject) => {
    const link = document.createElement('link');
    link.rel = 'stylesheet';
    link.href = href;
    if (legacy) link.dataset.legacyUi = 'true';
    link.onload = () => {
      if (legacy) link.disabled = document.documentElement.dataset.uiLegacyStyles === 'false';
      resolve();
    };
    link.onerror = () => reject(new Error(`Unable to load stylesheet: ${href}`));
    document.head.append(link);
  });
}

// Keep protected pages' original cascade, including after client-side navigation.
if (kind === 'native') {
  const observer = new MutationObserver(() => {
    for (const link of document.querySelectorAll('link[data-legacy-ui]')) {
      link.disabled = document.documentElement.dataset.uiLegacyStyles === 'false';
    }
  });
  observer.observe(document.documentElement, { attributes: true, attributeFilter: ['data-ui-legacy-styles'] });
}

let stopWatchingApp;

try {
  const appRoot = document.getElementById('app');
  const appMounted = new Promise(resolve => {
    if (!appRoot || appRoot.hasChildNodes()) {
      resolve();
      return;
    }
    const observer = new MutationObserver(() => {
      if (appRoot.hasChildNodes()) {
        observer.disconnect();
        resolve();
      }
    });
    observer.observe(appRoot, { childList: true });
    stopWatchingApp = () => observer.disconnect();
  });

  await Promise.all([
    Promise.all([
      ...entries[kind].styles.map(href => stylesheet(href)),
      ...(entries[kind].legacyStyles || []).map(href => stylesheet(href, true)),
    ]),
    import(entries[kind].script),
    appMounted,
  ]);
  document.getElementById('startup-loading')?.remove();
} catch (error) {
  stopWatchingApp?.();
  document.getElementById('startup-loading')?.remove();
  console.error(error);
  const message = document.createElement('p');
  message.textContent = '页面资源加载失败，请刷新重试。';
  document.getElementById('app').replaceChildren(message);
}

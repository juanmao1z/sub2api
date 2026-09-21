/** @file @brief Load the site's original homepage or the Kedaya console, with isolated styles. */
import { isHomePath, installNavigation } from './navigation.js';
import { entries } from './entries.js';

installNavigation(window);
const kind = isHomePath(location.pathname) ? 'home' : 'console';
document.documentElement.dataset.frontend = kind;

/** @brief Load one stylesheet before application mount to avoid a flash of unstyled content.
 * @param href Same-origin CSS path. @return Promise resolved once the sheet is ready.
 */
function stylesheet(href) {
  return new Promise((resolve, reject) => {
    const link = document.createElement('link');
    link.rel = 'stylesheet';
    link.href = href;
    link.onload = resolve;
    link.onerror = () => reject(new Error(`Unable to load stylesheet: ${href}`));
    document.head.append(link);
  });
}

try {
  await Promise.all(entries[kind].styles.map(stylesheet));
  await import(entries[kind].script);
} catch (error) {
  console.error(error);
  const message = document.createElement('p');
  message.textContent = '页面资源加载失败，请刷新重试。';
  document.getElementById('app').replaceChildren(message);
}

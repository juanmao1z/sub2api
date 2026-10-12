/** @file @brief Load the production Sub2API review UI entry. */
import { runtimeForPath, installNavigation } from './navigation.js';
import { installTicketAdminTools } from './ticket-admin.js';
import { entries } from './entries.js';

installNavigation(window);
const kind = runtimeForPath(location.pathname);
document.documentElement.dataset.frontend = kind;

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
    Promise.all(entries[kind].styles.map(stylesheet)),
    import(entries[kind].script),
    appMounted,
  ]);
  document.getElementById('startup-loading')?.remove();
  installTicketAdminTools();
} catch (error) {
  stopWatchingApp?.();
  document.getElementById('startup-loading')?.remove();
  console.error(error);
  const message = document.createElement('p');
  message.textContent = '页面资源加载失败，请刷新重试。';
  document.getElementById('app')?.replaceChildren(message);
}

import { createApp } from 'vue'
import { createPinia } from 'pinia'
import '@fontsource-variable/public-sans'
import './style.css'
import App from './App.vue'
import router from './router'
import i18n, { initI18n } from './i18n'
import { useAppStore } from './stores/app'
import { initTheme } from './utils/theme'

async function bootstrap(): Promise<void> {
  initTheme()
  const app = createApp(App)
  app.use(createPinia())
  useAppStore().initFromInjectedConfig()
  await initI18n()
  app.use(i18n)
  app.use(router)
  await router.isReady()
  app.mount('#app')
  document.getElementById('startup-loading')?.remove()
}
void bootstrap()

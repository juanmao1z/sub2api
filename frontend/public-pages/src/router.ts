import { defineComponent } from 'vue'
import { createRouter, createWebHistory } from 'vue-router'
import { useAppStore, useAuthStore } from '@/stores'
import HomeView from '@/views/HomeView.vue'
import LoginView from '@/views/auth/LoginView.vue'

const Bridge = defineComponent({ render: () => null })
const router = createRouter({
  // Asset base is relative; public application URLs are always rooted at '/'.
  history: createWebHistory('/'),
  routes: [
    { path: '/', component: HomeView },
    { path: '/home', name: 'Home', component: HomeView },
    { path: '/login', name: 'Login', component: LoginView },
    { path: '/legal/:documentId', name: 'LegalDocument', component: Bridge, meta: { bridge: true } },
    { path: '/register', name: 'Register', component: Bridge, meta: { bridge: true } },
    { path: '/forgot-password', name: 'ForgotPassword', component: Bridge, meta: { bridge: true } },
    { path: '/dashboard', name: 'Dashboard', component: Bridge, meta: { bridge: true } },
    { path: '/admin/dashboard', name: 'AdminDashboard', component: Bridge, meta: { bridge: true } },
    { path: '/:pathMatch(.*)*', component: Bridge, meta: { bridge: true } },
  ],
  scrollBehavior: () => ({ top: 0 }),
})

let authInitialized = false
router.beforeEach(async (to) => {
  if (to.meta.bridge) {
    window.location.assign(to.fullPath)
    return false
  }
  const authStore = useAuthStore()
  const appStore = useAppStore()
  if (!authInitialized) {
    authStore.checkAuth()
    authInitialized = true
  }
  if (!appStore.publicSettingsLoaded) await appStore.fetchPublicSettings()
  if (to.path === '/login' && authStore.isAuthenticated) {
    // Preserve the old backend-mode exception to prevent a non-admin redirect loop.
    if (!(appStore.backendModeEnabled && !authStore.isAdmin)) {
      return authStore.isAdmin ? '/admin/dashboard' : '/dashboard'
    }
  }
  if (appStore.backendModeEnabled && !authStore.isAuthenticated && to.path !== '/login') {
    return '/login'
  }
  return true
})

export default router

import { createRouter, createWebHistory } from 'vue-router'
import AppShell from './components/AppShell.vue'
import AdminView from './views/AdminView.vue'
import DashboardView from './views/DashboardView.vue'
import RegistryExplorerView from './views/RegistryExplorerView.vue'

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes: [
    {
      path: '/',
      component: AppShell,
      children: [
        { path: '', redirect: '/explorer' },
        {
          path: 'dashboard',
          name: 'dashboard',
          component: DashboardView,
          meta: {
            title: 'Dashboard',
            subtitle: 'Platform status and operational highlights',
          },
        },
        {
          path: 'explorer',
          name: 'explorer',
          component: RegistryExplorerView,
          meta: {
            title: 'Registry Explorer',
            subtitle: 'Search, filter, and inspect packages across registries',
          },
        },
        {
          path: 'admin',
          name: 'admin',
          component: AdminView,
          meta: {
            title: 'Admin',
            subtitle: 'Registry governance, ownership, and access controls',
          },
        },
      ],
    },
  ],
})

export default router

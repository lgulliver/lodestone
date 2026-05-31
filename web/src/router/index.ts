import { createRouter, createWebHistory, type RouteRecordRaw } from 'vue-router'

const routes: RouteRecordRaw[] = [
  {
    path: '/',
    component: () => import('@/layouts/AppLayout.vue'),
    children: [
      { path: '', redirect: '/explorer' },
      {
        path: 'dashboard',
        name: 'dashboard',
        component: () => import('@/views/RegistryExplorer.vue'),
      },
      {
        path: 'explorer',
        name: 'explorer',
        component: () => import('@/views/RegistryExplorer.vue'),
      },
      {
        path: 'admin',
        name: 'admin',
        component: () => import('@/views/RegistryExplorer.vue'),
      },
    ],
  },
]

export const router = createRouter({
  history: createWebHistory(),
  routes,
})

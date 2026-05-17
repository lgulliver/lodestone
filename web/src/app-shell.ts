import type { InjectionKey, Ref } from 'vue'

export type NavigationItem = {
  label: string
  to: string
  icon: 'grid' | 'database' | 'admin'
}

export const globalSearchKey: InjectionKey<Ref<string>> = Symbol('global-search')

export const navigationItems: NavigationItem[] = [
  { label: 'Dashboard', to: '/dashboard', icon: 'grid' },
  { label: 'Artifact Explorer', to: '/explorer', icon: 'database' },
  { label: 'Admin', to: '/admin', icon: 'admin' },
]

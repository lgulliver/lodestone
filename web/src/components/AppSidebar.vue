<script setup lang="ts">
import { RouterLink } from 'vue-router'
import { LayoutGrid, Database, ShieldCheck } from '@lucide/vue'

const nav = [
  { label: 'Dashboard', icon: LayoutGrid, to: '/dashboard' },
  { label: 'Artifact Explorer', icon: Database, to: '/explorer' },
  { label: 'Admin', icon: ShieldCheck, to: '/admin' },
]
</script>

<template>
  <aside
    class="flex w-[260px] shrink-0 flex-col border-r border-border bg-sidebar"
  >
    <div class="px-6 py-5">
      <h1 class="text-2xl font-bold tracking-tight text-primary">Lodestone</h1>
      <p class="mt-0.5 text-xs text-muted-foreground">Artifact Registry</p>
    </div>

    <nav class="flex flex-col gap-1 px-3 py-2">
      <RouterLink
        v-for="item in nav"
        :key="item.to"
        :to="item.to"
        class="flex items-center gap-3 rounded-md px-3 py-2.5 text-sm font-medium text-foreground/80 transition-colors hover:bg-accent/50"
        active-class="!bg-accent !text-accent-foreground font-semibold"
      >
        <component :is="item.icon" class="size-5" />
        {{ item.label }}
      </RouterLink>
    </nav>

    <div class="mx-3 mt-2 border-t border-border" />

    <div class="mt-auto flex items-center gap-3 px-5 py-5">
      <div
        class="flex size-9 items-center justify-center rounded-full bg-secondary text-xs font-semibold text-secondary-foreground"
      >
        JD
      </div>
      <div class="leading-tight">
        <p class="text-sm font-semibold text-foreground">John Doe</p>
        <p class="text-xs text-muted-foreground">Engineer</p>
      </div>
    </div>
  </aside>
</template>

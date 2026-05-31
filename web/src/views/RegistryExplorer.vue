<script setup lang="ts">
import { ref } from 'vue'
import {
  ListFilter,
  ChevronDown,
  RefreshCw,
  ChevronLeft,
  ChevronRight,
  ExternalLink,
} from '@lucide/vue'
import Card from '@/components/ui/Card.vue'
import Button from '@/components/ui/Button.vue'
import Switch from '@/components/ui/Switch.vue'
import StatCard from '@/components/StatCard.vue'
import ArtifactTable from '@/components/ArtifactTable.vue'
import ActivityChart from '@/components/ActivityChart.vue'
import { artifacts, stats } from '@/data/mock'
import { useHealth } from '@/composables/useHealth'

const publicOnly = ref(false)
const pages = [1, 2, 3, '…', 257]
const currentPage = ref(1)

const { data: health, isError: healthError } = useHealth()
</script>

<template>
  <div class="mx-auto max-w-[1440px] px-8 py-7">
    <!-- Breadcrumb + title -->
    <nav class="flex items-center gap-2 text-sm text-muted-foreground">
      <span>Registry</span>
      <ChevronRight class="size-3.5" />
      <span class="font-medium text-primary">Artifact Explorer</span>
    </nav>
    <h1 class="mt-1 text-3xl font-bold tracking-tight text-foreground">
      Registry Explorer
    </h1>

    <!-- Stat cards -->
    <div class="mt-6 grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
      <StatCard v-for="s in stats" :key="s.label" v-bind="s" />
    </div>

    <!-- Filter bar -->
    <div class="mt-5 flex flex-wrap items-center gap-3">
      <Button variant="secondary" size="sm">
        <ListFilter class="size-4" /> All Types <ChevronDown class="size-4" />
      </Button>
      <Button variant="secondary" size="sm">
        Last 30 Days <ChevronDown class="size-4" />
      </Button>
      <div
        class="flex h-8 items-center gap-2 rounded border border-border px-3 text-sm"
      >
        <span class="text-foreground">Public Only</span>
        <Switch v-model="publicOnly" />
      </div>
      <button
        class="ml-auto inline-flex items-center gap-2 text-sm font-medium text-muted-foreground transition-colors hover:text-foreground"
      >
        <RefreshCw class="size-4" /> Refresh Data
      </button>
    </div>

    <!-- Table -->
    <Card class="mt-4 overflow-hidden">
      <ArtifactTable :rows="artifacts" />
      <div
        class="flex items-center justify-between border-t border-border px-6 py-3 text-sm"
      >
        <p class="text-muted-foreground">Showing 1 to 5 of 1,284 artifacts</p>
        <div class="flex items-center gap-1">
          <button
            class="flex size-8 items-center justify-center rounded text-muted-foreground hover:bg-muted"
          >
            <ChevronLeft class="size-4" />
          </button>
          <button
            v-for="p in pages"
            :key="p"
            class="flex size-8 items-center justify-center rounded text-sm"
            :class="
              p === currentPage
                ? 'bg-primary font-semibold text-primary-foreground'
                : 'text-foreground hover:bg-muted'
            "
            @click="typeof p === 'number' && (currentPage = p)"
          >
            {{ p }}
          </button>
          <button
            class="flex size-8 items-center justify-center rounded text-muted-foreground hover:bg-muted"
          >
            <ChevronRight class="size-4" />
          </button>
        </div>
      </div>
    </Card>

    <!-- Chart + help -->
    <div class="mt-5 grid grid-cols-1 gap-5 lg:grid-cols-3">
      <div class="lg:col-span-2">
        <ActivityChart />
      </div>
      <div
        class="flex flex-col justify-start rounded-lg bg-linear-to-br from-primary to-[#1e40af] p-6 text-primary-foreground"
      >
        <h2 class="text-xl font-semibold">Need help?</h2>
        <p class="mt-2 text-sm text-primary-foreground/80">
          Explore our technical documentation for registry integration, CI/CD
          pipelines, and security best practices.
        </p>
        <button
          class="mt-5 inline-flex w-fit items-center gap-2 rounded bg-white/15 px-4 py-2 text-sm font-semibold backdrop-blur transition-colors hover:bg-white/25"
        >
          Read Docs <ExternalLink class="size-4" />
        </button>
      </div>
    </div>

    <!-- Footer -->
    <footer
      class="mt-8 flex flex-wrap items-center gap-x-6 gap-y-2 border-t border-border pt-5 text-xs text-muted-foreground"
    >
      <a href="#" class="hover:text-foreground">Documentation</a>
      <a href="#" class="hover:text-foreground">Security</a>
      <span class="ml-auto inline-flex items-center gap-2">
        <span
          class="size-2 rounded-full"
          :class="healthError ? 'bg-destructive' : 'bg-emerald-600'"
        />
        System Status: {{ healthError ? 'Unavailable' : 'All Operational' }}
      </span>
      <span class="text-border">|</span>
      <span class="font-mono">Registry API: {{ health?.version ?? '—' }}</span>
    </footer>
  </div>
</template>

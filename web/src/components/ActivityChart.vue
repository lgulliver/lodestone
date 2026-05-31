<script setup lang="ts">
import { ref } from 'vue'
import Card from '@/components/ui/Card.vue'
import { activity } from '@/data/mock'

const max = Math.max(...activity.flatMap((a) => [a.pulls, a.publishes]))
const h = (v: number) => `${Math.round((v / max) * 100)}%`
const fmt = (v: number) => (v >= 1000 ? `${(v / 1000).toFixed(1)}k` : `${v}`)

const hovered = ref<number | null>(null)
</script>

<template>
  <Card class="flex flex-col p-6">
    <div class="mb-6 flex items-center justify-between">
      <h2 class="text-lg font-semibold tracking-tight text-foreground">
        Registry Activity Trend
      </h2>
      <div class="flex items-center gap-4 text-xs text-muted-foreground">
        <span class="inline-flex items-center gap-1.5">
          <span class="size-2.5 rounded-full bg-primary" /> Pulls
        </span>
        <span class="inline-flex items-center gap-1.5">
          <span class="size-2.5 rounded-full bg-slate-400" /> Publishes
        </span>
      </div>
    </div>

    <div class="flex flex-1 items-end gap-2">
      <div
        v-for="(bar, i) in activity"
        :key="bar.day"
        class="group relative flex flex-1 cursor-default flex-col items-center gap-2 rounded-md pt-2 transition-colors"
        :class="hovered === i ? 'bg-muted/50' : 'hover:bg-muted/30'"
        @mouseenter="hovered = i"
        @mouseleave="hovered = null"
      >
        <!-- Tooltip -->
        <div
          v-if="hovered === i"
          class="pointer-events-none absolute -top-2 z-10 -translate-y-full rounded-md border border-border bg-popover px-3 py-2 text-xs shadow-[0px_4px_12px_rgba(0,0,0,0.08)]"
        >
          <p class="mb-1 font-semibold text-foreground">{{ bar.day }}</p>
          <p class="flex items-center gap-1.5 whitespace-nowrap text-muted-foreground">
            <span class="size-2 rounded-full bg-primary" /> Pulls
            <span class="ml-auto font-mono text-foreground">{{ fmt(bar.pulls) }}</span>
          </p>
          <p class="flex items-center gap-1.5 whitespace-nowrap text-muted-foreground">
            <span class="size-2 rounded-full bg-slate-400" /> Publishes
            <span class="ml-auto font-mono text-foreground">{{
              fmt(bar.publishes)
            }}</span>
          </p>
        </div>

        <div class="flex h-44 w-full items-end justify-center gap-1">
          <div
            class="w-1/2 max-w-6 rounded-t-sm bg-primary transition-all duration-150"
            :class="hovered === null || hovered === i ? 'opacity-100' : 'opacity-40'"
            :style="{ height: h(bar.pulls) }"
          />
          <div
            class="w-1/2 max-w-6 rounded-t-sm bg-slate-400 transition-all duration-150"
            :class="hovered === null || hovered === i ? 'opacity-100' : 'opacity-40'"
            :style="{ height: h(bar.publishes) }"
          />
        </div>
        <span
          class="text-xs transition-colors"
          :class="hovered === i ? 'font-medium text-foreground' : 'text-muted-foreground'"
          >{{ bar.day }}</span
        >
      </div>
    </div>
  </Card>
</template>

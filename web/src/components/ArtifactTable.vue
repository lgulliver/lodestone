<script setup lang="ts">
import { ChevronUp } from '@lucide/vue'
import Badge from '@/components/ui/Badge.vue'
import type { Artifact, ArtifactStatus, ArtifactType } from '@/data/mock'

defineProps<{ rows: Artifact[] }>()

const statusMeta: Record<ArtifactStatus, { label: string; dot: string; text: string }> = {
  stable: { label: 'Stable', dot: 'bg-emerald-600', text: 'text-foreground' },
  beta: { label: 'Beta', dot: 'bg-amber-500', text: 'text-amber-700' },
  critical: { label: 'Critical Vuln', dot: 'bg-destructive', text: 'text-destructive' },
}

const tileTint: Record<ArtifactType, string> = {
  docker: 'border-blue-200 bg-blue-50 text-blue-700',
  npm: 'border-orange-200 bg-orange-50 text-orange-700',
  python: 'border-emerald-200 bg-emerald-50 text-emerald-700',
  binary: 'border-border bg-muted text-muted-foreground',
}
</script>

<template>
  <table class="w-full border-collapse text-sm">
    <thead>
      <tr class="border-b border-border bg-muted/40 text-left">
        <th class="px-6 py-3 font-semibold text-muted-foreground">
          <span class="inline-flex items-center gap-1">
            Name <ChevronUp class="size-3.5" />
          </span>
        </th>
        <th class="px-3 py-3 font-semibold text-muted-foreground">Type</th>
        <th class="px-3 py-3 font-semibold text-muted-foreground">Version</th>
        <th class="px-3 py-3 font-semibold text-muted-foreground">Pulls</th>
        <th class="px-3 py-3 font-semibold text-muted-foreground">Last Updated</th>
        <th class="px-3 py-3 font-semibold text-muted-foreground">Status</th>
      </tr>
    </thead>
    <tbody>
      <tr
        v-for="row in rows"
        :key="row.name"
        class="border-b border-border last:border-0 transition-colors hover:bg-muted/30"
      >
        <td class="px-6 py-4">
          <div class="flex items-center gap-3">
            <div
              class="flex size-9 items-center justify-center rounded-md border"
              :class="tileTint[row.type]"
            >
              <component :is="row.icon" class="size-4" />
            </div>
            <div class="leading-tight">
              <a class="font-semibold text-primary hover:underline" href="#">{{
                row.name
              }}</a>
              <p class="font-mono text-xs text-muted-foreground">{{ row.path }}</p>
            </div>
          </div>
        </td>
        <td class="px-3 py-4">
          <Badge :tone="row.type">{{ row.type }}</Badge>
        </td>
        <td class="px-3 py-4 font-mono text-xs text-foreground/80">
          {{ row.version }}
        </td>
        <td class="px-3 py-4 font-semibold text-foreground">{{ row.pulls }}</td>
        <td class="px-3 py-4 text-muted-foreground">{{ row.updated }}</td>
        <td class="px-3 py-4">
          <span class="inline-flex items-center gap-2" :class="statusMeta[row.status].text">
            <span class="size-2 rounded-full" :class="statusMeta[row.status].dot" />
            {{ statusMeta[row.status].label }}
          </span>
        </td>
      </tr>
    </tbody>
  </table>
</template>

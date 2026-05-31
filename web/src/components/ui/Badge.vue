<script setup lang="ts">
import { computed } from 'vue'
import { cva, type VariantProps } from 'class-variance-authority'
import { cn } from '@/lib/utils'

const badge = cva(
  'inline-flex items-center rounded-sm px-2 py-0.5 text-[11px] font-semibold uppercase tracking-wider leading-none',
  {
    variants: {
      tone: {
        docker: 'bg-blue-100 text-blue-700',
        npm: 'bg-orange-100 text-orange-700',
        python: 'bg-emerald-100 text-emerald-700',
        binary: 'bg-muted text-muted-foreground',
        neutral: 'bg-muted text-muted-foreground',
      },
    },
    defaultVariants: { tone: 'neutral' },
  },
)

const props = defineProps<{
  tone?: VariantProps<typeof badge>['tone']
  class?: string
}>()
const classes = computed(() => cn(badge({ tone: props.tone }), props.class))
</script>

<template>
  <span :class="classes"><slot /></span>
</template>

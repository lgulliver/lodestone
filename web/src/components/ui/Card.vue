<script setup lang="ts">
import { computed } from 'vue'
import { cn } from '@/lib/utils'

const props = defineProps<{ class?: string; interactive?: boolean }>()
const classes = computed(() =>
  cn(
    'rounded-lg border border-border bg-card text-card-foreground',
    props.interactive &&
      'transition-shadow hover:shadow-[0px_4px_12px_rgba(0,0,0,0.05)]',
    props.class,
  ),
)
</script>

<template>
  <div :class="classes">
    <slot />
  </div>
</template>

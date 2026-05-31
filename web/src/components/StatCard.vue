<script setup lang="ts">
import Card from '@/components/ui/Card.vue'

defineProps<{
  label: string
  value: string
  delta: string
  tone: 'positive' | 'muted' | 'info'
}>()

const deltaClass: Record<string, string> = {
  positive: 'text-[#15803d]',
  muted: 'text-muted-foreground',
  info: 'text-primary',
}
</script>

<template>
  <Card class="px-5 py-4">
    <p class="text-sm text-muted-foreground">{{ label }}</p>
    <div class="mt-2 flex items-baseline gap-2">
      <span class="text-2xl font-bold tracking-tight text-foreground">{{
        value
      }}</span>
      <span class="text-xs font-semibold" :class="deltaClass[tone]">{{
        delta
      }}</span>
    </div>
  </Card>
</template>

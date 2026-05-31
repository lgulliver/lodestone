<script setup lang="ts">
import { computed } from 'vue'
import { cn } from '@/lib/utils'

const props = defineProps<{ class?: string; modelValue?: string }>()
defineEmits<{ 'update:modelValue': [value: string] }>()
const classes = computed(() =>
  cn(
    'flex h-9 w-full rounded border border-input bg-card px-3 text-sm text-foreground placeholder:text-muted-foreground transition-shadow focus-visible:outline-none focus-visible:border-primary focus-visible:ring-2 focus-visible:ring-primary/10',
    props.class,
  ),
)
</script>

<template>
  <input
    :class="classes"
    :value="modelValue"
    @input="$emit('update:modelValue', ($event.target as HTMLInputElement).value)"
  />
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { cn } from '@/lib/utils'

const props = defineProps<{ modelValue?: boolean; class?: string }>()
const emit = defineEmits<{ 'update:modelValue': [value: boolean] }>()
const trackClass = computed(() =>
  cn(
    'relative inline-flex h-5 w-9 shrink-0 cursor-pointer items-center rounded-full transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-1 focus-visible:ring-offset-background',
    props.modelValue ? 'bg-primary' : 'bg-muted-foreground/40',
    props.class,
  ),
)
</script>

<template>
  <button
    type="button"
    role="switch"
    :aria-checked="modelValue"
    :class="trackClass"
    @click="emit('update:modelValue', !modelValue)"
  >
    <span
      class="inline-block size-4 transform rounded-full bg-white shadow-sm transition-transform"
      :class="modelValue ? 'translate-x-4' : 'translate-x-0.5'"
    />
  </button>
</template>

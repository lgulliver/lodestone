<script setup lang="ts">
import { computed } from 'vue'
import { cva, type VariantProps } from 'class-variance-authority'
import { cn } from '@/lib/utils'

const button = cva(
  'inline-flex items-center justify-center gap-2 whitespace-nowrap rounded font-semibold transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-1 focus-visible:ring-offset-background disabled:pointer-events-none disabled:opacity-50 [&_svg]:size-4 [&_svg]:shrink-0',
  {
    variants: {
      variant: {
        primary: 'bg-primary text-primary-foreground hover:bg-primary/90',
        secondary:
          'border border-border bg-transparent text-foreground hover:bg-muted',
        ghost: 'bg-transparent text-primary hover:bg-accent/60',
        destructive:
          'bg-destructive text-destructive-foreground hover:bg-destructive/90',
      },
      size: {
        sm: 'h-8 px-3 text-xs',
        md: 'h-9 px-4 text-sm',
        lg: 'h-10 px-5 text-sm',
        icon: 'h-9 w-9',
      },
    },
    defaultVariants: { variant: 'primary', size: 'md' },
  },
)

type Props = {
  variant?: VariantProps<typeof button>['variant']
  size?: VariantProps<typeof button>['size']
  class?: string
}
const props = defineProps<Props>()
const classes = computed(() =>
  cn(button({ variant: props.variant, size: props.size }), props.class),
)
</script>

<template>
  <button :class="classes">
    <slot />
  </button>
</template>

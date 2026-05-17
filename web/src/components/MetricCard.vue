<script setup lang="ts">
import type { Metric } from '../types'

defineProps<{
  metric: Metric
}>()
</script>

<template>
  <article class="metric-card">
    <p class="metric-card__label">{{ metric.label }}</p>
    <div class="metric-card__value-row">
      <span class="metric-card__value">{{ metric.value }}</span>
      <span v-if="metric.delta" :class="['metric-card__delta', `metric-card__delta--${metric.tone ?? 'neutral'}`]">
        {{ metric.delta }}
      </span>
      <span v-if="metric.suffix" class="metric-card__suffix">{{ metric.suffix }}</span>
    </div>
    <p v-if="metric.caption" class="metric-card__caption">{{ metric.caption }}</p>
  </article>
</template>

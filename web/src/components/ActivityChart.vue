<script setup lang="ts">
import { computed } from 'vue'
import type { ActivityPoint } from '../types'

const props = defineProps<{
  points: ActivityPoint[]
}>()

const maxValue = computed(() =>
  Math.max(...props.points.flatMap((point) => [point.pulls, point.publishes]), 1),
)

const percent = (value: number) => `${Math.round((value / maxValue.value) * 100)}%`
</script>

<template>
  <section class="activity-card">
    <div class="activity-card__header">
      <h2>Registry Activity Trend</h2>
      <div class="activity-card__legend">
        <span><i class="legend-dot legend-dot--pulls" /> Pulls</span>
        <span><i class="legend-dot legend-dot--publishes" /> Publishes</span>
      </div>
    </div>

    <div class="activity-chart">
      <div v-for="point in points" :key="point.day" class="activity-chart__day">
        <div class="activity-chart__bars">
          <span class="activity-chart__bar activity-chart__bar--publishes" :style="{ height: percent(point.publishes) }" />
          <span class="activity-chart__bar activity-chart__bar--pulls" :style="{ height: percent(point.pulls) }" />
        </div>
        <span class="activity-chart__label">{{ point.day }}</span>
      </div>
    </div>
  </section>
</template>

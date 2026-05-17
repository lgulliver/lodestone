<script setup lang="ts">
import { dashboardHighlights, registryMetrics } from '../data/registry'
import MetricCard from '../components/MetricCard.vue'
</script>

<template>
  <section class="placeholder-view">
    <div class="metrics-grid">
      <MetricCard v-for="metric in registryMetrics.slice(0, 3)" :key="metric.label" :metric="metric" />
    </div>

    <section class="placeholder-card">
      <h2>Operational overview</h2>
      <p>
        The shell is ready for a richer dashboard experience with live pull analytics, storage
        distribution, and contributor insights pulled from Lodestone APIs.
      </p>
      <ul class="placeholder-list">
        <li v-for="highlight in dashboardHighlights" :key="highlight">{{ highlight }}</li>
      </ul>
    </section>
  </section>
</template>

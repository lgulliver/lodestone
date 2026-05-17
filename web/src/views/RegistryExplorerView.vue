<script setup lang="ts">
import { computed, inject, ref, watch } from 'vue'
import { globalSearchKey } from '../app-shell'
import ArtifactTable from '../components/ArtifactTable.vue'
import AppIcon from '../components/AppIcon.vue'
import ActivityChart from '../components/ActivityChart.vue'
import MetricCard from '../components/MetricCard.vue'
import { featuredArtifacts, registryActivity, registryMetrics } from '../data/registry'

const searchQuery = inject(globalSearchKey, ref(''))
const typeFilter = ref<'all' | 'docker' | 'npm' | 'python' | 'binary' | 'helm'>('all')
const recencyFilter = ref<'30d' | '7d' | '24h'>('30d')
const publicOnly = ref(false)
const currentPage = ref(1)
const pageSize = 5
const refreshedAt = ref(new Date())

const maxAge = computed(() => {
  switch (recencyFilter.value) {
    case '24h':
      return 1
    case '7d':
      return 7
    default:
      return 30
  }
})

const filteredArtifacts = computed(() =>
  featuredArtifacts.filter((artifact) => {
    const matchesSearch =
      artifact.name.toLowerCase().includes(searchQuery.value.toLowerCase()) ||
      artifact.namespace.toLowerCase().includes(searchQuery.value.toLowerCase())
    const matchesType =
      typeFilter.value === 'all' || artifact.type.toLowerCase() === typeFilter.value
    const matchesVisibility = !publicOnly.value || artifact.isPublic
    const matchesRecency = artifact.recencyDays <= maxAge.value

    return matchesSearch && matchesType && matchesVisibility && matchesRecency
  }),
)

const pageCount = computed(() => Math.max(Math.ceil(filteredArtifacts.value.length / pageSize), 1))

const paginatedArtifacts = computed(() => {
  const start = (currentPage.value - 1) * pageSize
  return filteredArtifacts.value.slice(start, start + pageSize)
})

const tableSummary = computed(() => {
  if (!filteredArtifacts.value.length) {
    return 'No artifacts match the current filters.'
  }

  const start = (currentPage.value - 1) * pageSize + 1
  const end = Math.min(currentPage.value * pageSize, filteredArtifacts.value.length)
  return `Showing ${start} to ${end} of ${filteredArtifacts.value.length} featured artifacts`
})

const lastRefreshedLabel = computed(() =>
  new Intl.DateTimeFormat('en', {
    hour: 'numeric',
    minute: '2-digit',
  }).format(refreshedAt.value),
)

watch([filteredArtifacts, maxAge], () => {
  if (currentPage.value > pageCount.value) {
    currentPage.value = 1
  }
})

const refreshData = () => {
  refreshedAt.value = new Date()
}
</script>

<template>
  <section class="explorer-grid">
    <div class="metrics-grid">
      <MetricCard v-for="metric in registryMetrics" :key="metric.label" :metric="metric" />
    </div>

    <section class="filters-card">
      <div class="filters-card__controls">
        <label class="select-control">
          <span>All Types</span>
          <select v-model="typeFilter" aria-label="Artifact type filter">
            <option value="all">All Types</option>
            <option value="docker">Docker</option>
            <option value="npm">NPM</option>
            <option value="python">Python</option>
            <option value="binary">Binary</option>
            <option value="helm">Helm</option>
          </select>
          <AppIcon name="chevron" :size="16" />
        </label>

        <label class="select-control">
          <span>{{ recencyFilter === '30d' ? 'Last 30 Days' : recencyFilter === '7d' ? 'Last 7 Days' : 'Last 24 Hours' }}</span>
          <select v-model="recencyFilter" aria-label="Recency filter">
            <option value="30d">Last 30 Days</option>
            <option value="7d">Last 7 Days</option>
            <option value="24h">Last 24 Hours</option>
          </select>
          <AppIcon name="chevron" :size="16" />
        </label>

        <label class="toggle-control">
          <span>Public Only</span>
          <input v-model="publicOnly" type="checkbox" aria-label="Public artifacts only" />
          <span class="toggle-control__indicator" />
        </label>
      </div>

      <button class="refresh-button" type="button" @click="refreshData">
        <AppIcon name="refresh" :size="16" />
        <span>Refresh Data</span>
      </button>
    </section>

    <ArtifactTable
      :artifacts="paginatedArtifacts"
      :page="currentPage"
      :page-count="pageCount"
      :total-label="tableSummary"
      @update:page="currentPage = $event"
    />

    <ActivityChart :points="registryActivity" />

    <aside class="help-card">
      <div>
        <p class="help-card__eyebrow">Updated {{ lastRefreshedLabel }}</p>
        <h2>Need help?</h2>
      </div>
      <p>
        Explore technical documentation for registry integration, CI/CD pipelines, artifact
        promotion, and security best practices.
      </p>
      <a class="help-card__link" href="https://github.com/lgulliver/lodestone/tree/main/docs" target="_blank" rel="noreferrer">
        <span>Read Docs</span>
        <AppIcon name="external" :size="15" />
      </a>
    </aside>
  </section>
</template>

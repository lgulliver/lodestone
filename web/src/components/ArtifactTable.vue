<script setup lang="ts">
import { computed } from 'vue'
import type { ArtifactRecord, PageLink } from '../types'
import AppIcon from './AppIcon.vue'

const props = defineProps<{
  artifacts: ArtifactRecord[]
  page: number
  pageCount: number
  totalLabel: string
}>()

const emit = defineEmits<{
  'update:page': [value: number]
}>()

const visiblePages = computed<PageLink[]>(() => {
  if (props.pageCount <= 4) {
    return Array.from({ length: props.pageCount }, (_, index) => index + 1)
  }

  const pages: PageLink[] = [1]
  const windowStart = Math.max(2, props.page - 1)
  const windowEnd = Math.min(props.pageCount - 1, props.page + 1)

  if (windowStart > 2) {
    pages.push('ellipsis')
  }

  for (let page = windowStart; page <= windowEnd; page += 1) {
    pages.push(page)
  }

  if (windowEnd < props.pageCount - 1) {
    pages.push('ellipsis')
  }

  pages.push(props.pageCount)
  return pages
})

const changePage = (page: number) => {
  if (page >= 1 && page <= props.pageCount && page !== props.page) {
    emit('update:page', page)
  }
}

const registryClassMap: Record<ArtifactRecord['type'], string> = {
  Docker: 'registry-badge--docker',
  NPM: 'registry-badge--npm',
  Python: 'registry-badge--python',
  Binary: 'registry-badge--binary',
  Helm: 'registry-badge--helm',
}
</script>

<template>
  <section class="table-card">
    <table class="artifact-table">
      <thead>
        <tr>
          <th scope="col">Name ↑</th>
          <th scope="col">Type</th>
          <th scope="col">Version</th>
          <th scope="col">Pulls</th>
          <th scope="col">Last Updated</th>
          <th scope="col">Status</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="artifact in artifacts" :key="artifact.id">
          <td>
            <div class="artifact-name">
              <span class="artifact-name__icon">
                <AppIcon name="box" :size="18" />
              </span>
              <div>
                <span class="artifact-name__title">{{ artifact.name }}</span>
                <span class="artifact-name__subtitle">{{ artifact.namespace }}</span>
              </div>
            </div>
          </td>
          <td>
            <span :class="['registry-badge', registryClassMap[artifact.type]]">{{ artifact.type }}</span>
          </td>
          <td class="artifact-table__mono">{{ artifact.version }}</td>
          <td class="artifact-table__metric">{{ artifact.pulls }}</td>
          <td>{{ artifact.lastUpdated }}</td>
          <td>
            <span :class="['status-pill', `status-pill--${artifact.statusTone}`]">
              <span class="status-pill__dot" />
              {{ artifact.status }}
            </span>
          </td>
        </tr>
      </tbody>
    </table>

    <div class="table-card__footer">
      <span>{{ totalLabel }}</span>
      <div class="pagination" aria-label="Pagination">
        <button type="button" class="pagination__arrow" :disabled="page === 1" @click="changePage(page - 1)">
          ‹
        </button>
        <button
          v-for="pageLink in visiblePages"
          :key="`${pageLink}`"
          type="button"
          :class="['pagination__page', { 'pagination__page--active': pageLink === page }]"
          :disabled="pageLink === 'ellipsis'"
          @click="typeof pageLink === 'number' ? changePage(pageLink) : undefined"
        >
          {{ pageLink === 'ellipsis' ? '…' : pageLink }}
        </button>
        <button
          type="button"
          class="pagination__arrow"
          :disabled="page === pageCount"
          @click="changePage(page + 1)"
        >
          ›
        </button>
      </div>
    </div>
  </section>
</template>

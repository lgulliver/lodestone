<script setup lang="ts">
import { adminHighlights } from '../data/registry'
</script>

<template>
  <section class="placeholder-view">
    <section class="placeholder-card">
      <h2>Registry administration</h2>
      <p>
        This area is positioned for registry settings, ownership workflows, and governance tools
        without changing the shared application shell.
      </p>
      <ul class="placeholder-list">
        <li v-for="highlight in adminHighlights" :key="highlight">{{ highlight }}</li>
      </ul>
    </section>
  </section>
</template>

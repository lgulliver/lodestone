<script setup lang="ts">
defineProps<{
  name:
    | 'grid'
    | 'database'
    | 'admin'
    | 'bell'
    | 'help'
    | 'plus'
    | 'refresh'
    | 'chevron'
    | 'box'
    | 'external'
    | 'search'
  size?: number
}>()
</script>

<template>
  <svg
    v-if="name === 'grid'"
    :width="size ?? 20"
    :height="size ?? 20"
    viewBox="0 0 20 20"
    fill="none"
    stroke="currentColor"
    stroke-width="1.8"
    stroke-linecap="round"
    stroke-linejoin="round"
    aria-hidden="true"
  >
    <rect x="3" y="3" width="5" height="5" />
    <rect x="12" y="3" width="5" height="5" />
    <rect x="3" y="12" width="5" height="5" />
    <rect x="12" y="12" width="5" height="5" />
  </svg>
  <svg
    v-else-if="name === 'database'"
    :width="size ?? 20"
    :height="size ?? 20"
    viewBox="0 0 20 20"
    fill="none"
    stroke="currentColor"
    stroke-width="1.8"
    stroke-linecap="round"
    stroke-linejoin="round"
    aria-hidden="true"
  >
    <ellipse cx="10" cy="5" rx="6.5" ry="2.5" />
    <path d="M3.5 5v9c0 1.4 2.9 2.5 6.5 2.5s6.5-1.1 6.5-2.5V5" />
    <path d="M3.5 9.5C3.5 10.9 6.4 12 10 12s6.5-1.1 6.5-2.5" />
  </svg>
  <svg
    v-else-if="name === 'admin'"
    :width="size ?? 20"
    :height="size ?? 20"
    viewBox="0 0 20 20"
    fill="none"
    stroke="currentColor"
    stroke-width="1.8"
    stroke-linecap="round"
    stroke-linejoin="round"
    aria-hidden="true"
  >
    <path d="M8.5 3.8a2.7 2.7 0 1 1 3 4.4" />
    <path d="M9.2 8.2a4.9 4.9 0 0 0-4.9 4.9v2.4h5.4" />
    <circle cx="14.5" cy="14.5" r="2.7" />
    <path d="M14.5 10.1v1.1M14.5 17.8v1.1M18.9 14.5h-1.1M11.2 14.5h-1.1" />
  </svg>
  <svg
    v-else-if="name === 'bell'"
    :width="size ?? 20"
    :height="size ?? 20"
    viewBox="0 0 20 20"
    fill="none"
    stroke="currentColor"
    stroke-width="1.8"
    stroke-linecap="round"
    stroke-linejoin="round"
    aria-hidden="true"
  >
    <path d="M10 3.5a3 3 0 0 0-3 3v1.1c0 .8-.2 1.5-.7 2.1L5 11.4v1.1h10v-1.1l-1.3-1.7c-.4-.6-.7-1.4-.7-2.1V6.5a3 3 0 0 0-3-3Z" />
    <path d="M8.2 14.7a2 2 0 0 0 3.6 0" />
  </svg>
  <svg
    v-else-if="name === 'help'"
    :width="size ?? 20"
    :height="size ?? 20"
    viewBox="0 0 20 20"
    fill="none"
    stroke="currentColor"
    stroke-width="1.8"
    stroke-linecap="round"
    stroke-linejoin="round"
    aria-hidden="true"
  >
    <circle cx="10" cy="10" r="8" />
    <path d="M7.8 7.2a2.6 2.6 0 1 1 4.1 2.1c-.9.7-1.4 1.1-1.4 2.2" />
    <circle cx="10" cy="14.4" r=".6" fill="currentColor" stroke="none" />
  </svg>
  <svg
    v-else-if="name === 'plus'"
    :width="size ?? 20"
    :height="size ?? 20"
    viewBox="0 0 20 20"
    fill="none"
    stroke="currentColor"
    stroke-width="1.8"
    stroke-linecap="round"
    aria-hidden="true"
  >
    <path d="M10 4.5v11M4.5 10h11" />
  </svg>
  <svg
    v-else-if="name === 'refresh'"
    :width="size ?? 20"
    :height="size ?? 20"
    viewBox="0 0 20 20"
    fill="none"
    stroke="currentColor"
    stroke-width="1.8"
    stroke-linecap="round"
    stroke-linejoin="round"
    aria-hidden="true"
  >
    <path d="M16 6.5V3.8h-2.7" />
    <path d="M4 13.5v2.7h2.7" />
    <path d="M5.2 8a5.5 5.5 0 0 1 9-1.5L16 8" />
    <path d="M14.8 12a5.5 5.5 0 0 1-9 1.5L4 12" />
  </svg>
  <svg
    v-else-if="name === 'chevron'"
    :width="size ?? 16"
    :height="size ?? 16"
    viewBox="0 0 16 16"
    fill="none"
    stroke="currentColor"
    stroke-width="1.8"
    stroke-linecap="round"
    stroke-linejoin="round"
    aria-hidden="true"
  >
    <path d="m5 6 3 3 3-3" />
  </svg>
  <svg
    v-else-if="name === 'box'"
    :width="size ?? 22"
    :height="size ?? 22"
    viewBox="0 0 22 22"
    fill="none"
    stroke="currentColor"
    stroke-width="1.8"
    stroke-linecap="round"
    stroke-linejoin="round"
    aria-hidden="true"
  >
    <path d="m11 2.8 7 4.1v8.2l-7 4.1-7-4.1V6.9l7-4.1Z" />
    <path d="m4 7 7 4 7-4M11 11v8" />
  </svg>
  <svg
    v-else-if="name === 'external'"
    :width="size ?? 16"
    :height="size ?? 16"
    viewBox="0 0 16 16"
    fill="none"
    stroke="currentColor"
    stroke-width="1.8"
    stroke-linecap="round"
    stroke-linejoin="round"
    aria-hidden="true"
  >
    <path d="M9.5 3H13v3.5" />
    <path d="M7 9 13 3" />
    <path d="M13 9.5V13H3V3h3.5" />
  </svg>
  <svg
    v-else
    :width="size ?? 20"
    :height="size ?? 20"
    viewBox="0 0 20 20"
    fill="none"
    stroke="currentColor"
    stroke-width="1.8"
    stroke-linecap="round"
    stroke-linejoin="round"
    aria-hidden="true"
  >
    <circle cx="9" cy="9" r="5.5" />
    <path d="m13 13 4 4" />
  </svg>
</template>

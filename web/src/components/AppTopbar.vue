<script setup lang="ts">
import { ref } from 'vue'
import { Search, Bell, CircleHelp, Plus } from '@lucide/vue'
import Button from '@/components/ui/Button.vue'

const query = ref('')
</script>

<template>
  <header
    class="flex h-[72px] shrink-0 items-center gap-4 border-b border-border bg-background px-6"
  >
    <div class="relative max-w-xl flex-1">
      <Search
        class="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground"
      />
      <input
        v-model="query"
        type="search"
        placeholder="Search registry..."
        class="h-10 w-full rounded-md border border-input bg-card pl-9 pr-3 text-sm text-foreground placeholder:text-muted-foreground transition-shadow focus-visible:border-primary focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary/10"
      />
    </div>

    <div class="ml-auto flex items-center gap-1">
      <button
        class="flex size-9 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
        aria-label="Notifications"
      >
        <Bell class="size-5" />
      </button>
      <button
        class="flex size-9 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
        aria-label="Help"
      >
        <CircleHelp class="size-5" />
      </button>
      <div class="mx-2 h-6 w-px bg-border" />
      <Button>
        <Plus class="size-4" />
        Publish Artifact
      </Button>
    </div>
  </header>
</template>

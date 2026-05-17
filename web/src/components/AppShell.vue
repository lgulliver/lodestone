<script setup lang="ts">
import { computed, provide, ref } from 'vue'
import { RouterLink, RouterView, useRoute } from 'vue-router'
import { globalSearchKey, navigationItems } from '../app-shell'
import AppIcon from './AppIcon.vue'

const route = useRoute()
const searchQuery = ref('')

provide(globalSearchKey, searchQuery)

const currentTitle = computed(() => String(route.meta.title ?? 'Lodestone'))
const currentSubtitle = computed(() => String(route.meta.subtitle ?? 'Artifact registry'))
</script>

<template>
  <div class="app-shell">
    <aside class="sidebar">
      <div class="brand">
        <span class="brand__name">Lodestone</span>
        <span class="brand__tagline">Artifact Registry</span>
      </div>

      <nav class="sidebar__nav" aria-label="Primary">
        <RouterLink
          v-for="item in navigationItems"
          :key="item.to"
          :to="item.to"
          class="nav-link"
          active-class="nav-link--active"
        >
          <AppIcon :name="item.icon" :size="18" />
          <span>{{ item.label }}</span>
        </RouterLink>
      </nav>

      <div class="sidebar__profile">
        <div class="profile__avatar">JD</div>
        <div>
          <p class="profile__name">John Doe</p>
          <p class="profile__role">Engineer</p>
        </div>
      </div>
    </aside>

    <div class="shell-main">
      <header class="topbar">
        <label class="searchbar" aria-label="Search registry">
          <AppIcon name="search" :size="18" />
          <input
            v-model="searchQuery"
            type="search"
            placeholder="Search registry..."
            autocomplete="off"
          />
        </label>

        <div class="topbar__actions">
          <button class="icon-button" type="button" aria-label="Notifications">
            <AppIcon name="bell" />
          </button>
          <button class="icon-button" type="button" aria-label="Help">
            <AppIcon name="help" />
          </button>
          <button class="publish-button" type="button">
            <AppIcon name="plus" :size="18" />
            <span>Publish Artifact</span>
          </button>
        </div>
      </header>

      <main class="workspace">
        <section class="workspace__heading">
          <p class="breadcrumb">Registry <span>›</span> {{ currentTitle }}</p>
          <div>
            <h1>{{ currentTitle }}</h1>
            <p class="workspace__subtitle">{{ currentSubtitle }}</p>
          </div>
        </section>

        <RouterView />
      </main>

      <footer class="footer">
        <div class="footer__links">
          <span>© 2024 Lodestone Systems Inc.</span>
          <a href="/">Privacy Policy</a>
          <a href="/">Security</a>
        </div>
        <div class="footer__status">
          <span class="status-dot" />
          <span>System Status: All Operational</span>
          <span class="footer__divider" />
          <span>Registry API: v4.2.1-stable</span>
        </div>
      </footer>
    </div>
  </div>
</template>

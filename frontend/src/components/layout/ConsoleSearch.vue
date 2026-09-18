<!-- @file Search the currently available sidebar navigation without bypassing feature or role restrictions. -->
<template>
  <button type="button" class="btn btn-secondary btn-icon" :aria-label="t('common.search')" @click="openSearch">
    <Icon name="search" size="sm" />
  </button>
  <BaseDialog :show="open" :title="t('common.search')" close-on-click-outside @close="open = false">
    <input v-model="query" class="input" :aria-label="t('common.search')" :placeholder="t('common.search')" />
    <div class="console-search-results">
      <router-link v-for="item in results" :key="item.path" :to="item.path" class="console-search-result" @click="open = false">
        <Icon name="grid" size="sm" /><span>{{ item.label }}</span><Icon name="chevronRight" size="sm" />
      </router-link>
      <p v-if="results.length === 0" class="empty-state-description py-4">{{ t('common.noData') }}</p>
    </div>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'

/** @brief A destination already exposed by the permission-filtered sidebar. */
interface SearchDestination { path: string; label: string }
const { t } = useI18n()
const open = ref(false)
const query = ref('')
const destinations = ref<SearchDestination[]>([])
const results = computed(() => destinations.value.filter(item => item.label.toLocaleLowerCase().includes(query.value.trim().toLocaleLowerCase())))

/** @brief Capture visible navigation destinations when opening the search dialog. */
function openSearch(): void {
  const items = new Map<string, SearchDestination>()
  document.querySelectorAll<HTMLAnchorElement>('.sidebar-nav a[href]').forEach(link => {
    const path = link.getAttribute('href') || ''
    const label = link.textContent?.trim() || ''
    if (path.startsWith('/') && !path.startsWith('//') && label) items.set(path, { path, label })
  })
  destinations.value = [...items.values()]
  query.value = ''
  open.value = true
}
</script>

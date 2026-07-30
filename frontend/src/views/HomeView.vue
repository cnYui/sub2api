<template>
  <!-- Custom Home Content: Full Page Mode -->
  <div v-if="homeContent" class="min-h-screen">
    <!-- iframe mode -->
    <iframe
      v-if="isHomeContentUrl"
      :src="homeContent.trim()"
      class="h-screen w-full border-0"
      allowfullscreen
    ></iframe>
    <!-- HTML mode - SECURITY: homeContent is admin-only setting, XSS risk is acceptable -->
    <div v-else v-html="homeContent"></div>
  </div>

  <!-- Default Home Page -->
  <div
    v-else
    class="relative flex min-h-screen flex-col overflow-hidden bg-white text-gray-900 dark:bg-dark-950 dark:text-gray-100"
  >
    <!-- Header -->
    <header class="relative z-20 px-6 py-4">
      <nav class="mx-auto flex max-w-6xl items-center justify-between">
        <!-- Logo -->
        <div class="flex items-center">
          <div class="h-10 w-10 overflow-hidden rounded-md border border-gray-200 bg-white shadow-card dark:border-dark-700 dark:bg-dark-900 dark:shadow-card">
            <img :src="siteLogo || '/logo.png'" alt="Logo" class="h-full w-full object-contain" />
          </div>
        </div>

        <!-- Nav Actions -->
        <div class="flex items-center gap-3">
          <!-- Language Switcher -->
          <LocaleSwitcher />

          <!-- Doc Link -->
          <a
            v-if="docUrl"
            :href="docUrl"
            target="_blank"
            rel="noopener noreferrer"
            class="rounded-lg p-2 text-gray-500 transition-colors hover:bg-gray-100 hover:text-gray-700 dark:text-dark-400 dark:hover:bg-dark-800 dark:hover:text-white"
            :title="t('home.viewDocs')"
          >
            <Icon name="book" size="md" />
          </a>

          <!-- Theme Toggle -->
          <button
            @click="toggleTheme"
            class="rounded-lg p-2 text-gray-500 transition-colors hover:bg-gray-100 hover:text-gray-700 dark:text-dark-400 dark:hover:bg-dark-800 dark:hover:text-white"
            :title="isDark ? t('home.switchToLight') : t('home.switchToDark')"
          >
            <Icon v-if="isDark" name="sun" size="md" />
            <Icon v-else name="moon" size="md" />
          </button>

          <!-- Dashboard Button -->
          <router-link
            v-if="isAuthenticated"
            :to="dashboardPath"
            class="inline-flex items-center gap-1.5 rounded-md bg-gray-900 py-1 pl-1 pr-2.5 transition-colors hover:bg-gray-800 dark:bg-gray-100 dark:text-dark-950 dark:hover:bg-white"
          >
            <span
              class="flex h-5 w-5 items-center justify-center rounded-sm bg-gray-700 text-[10px] font-semibold text-white dark:bg-dark-700"
            >
              {{ userInitial }}
            </span>
            <span class="text-xs font-medium text-white dark:text-dark-950">{{ t('home.dashboard') }}</span>
            <svg
              class="h-3 w-3 text-gray-400"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
              stroke-width="2"
            >
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                d="M4.5 19.5l15-15m0 0H8.25m11.25 0v11.25"
              />
            </svg>
          </router-link>
        </div>
      </nav>
    </header>

    <!-- Main Content -->
    <main class="relative z-10 flex-1 px-6 py-16">
      <div class="mx-auto max-w-6xl">
        <!-- Hero Section - Left/Right Layout -->
        <div class="mb-12 flex flex-col items-center justify-between gap-12 lg:flex-row lg:gap-16">
          <!-- Left: Text Content -->
          <div class="flex-1 text-center lg:text-left">
            <h1
              class="mb-4 font-display text-5xl font-semibold leading-tight text-gray-950 dark:text-gray-100 md:text-6xl lg:text-7xl"
            >
              {{ siteName }}
            </h1>
            <p class="mb-8 text-lg text-gray-600 dark:text-dark-300 md:text-xl">
              {{ siteSubtitle }}
            </p>

            <!-- CTA Button -->
            <div>
              <router-link
                :to="isAuthenticated ? dashboardPath : '/login'"
                class="btn btn-primary px-8 py-3 text-base"
              >
                {{ isAuthenticated ? t('home.goToDashboard') : t('home.getStarted') }}
                <Icon name="arrowRight" size="md" class="ml-2" :stroke-width="2" />
              </router-link>
            </div>
          </div>

          <!-- Right: Terminal Animation -->
          <div class="flex flex-1 justify-center lg:justify-end">
            <div class="terminal-container">
              <div class="terminal-window">
                <!-- Window header -->
                <div class="terminal-header">
                  <div class="terminal-buttons">
                    <span class="btn-close"></span>
                    <span class="btn-minimize"></span>
                    <span class="btn-maximize"></span>
                  </div>
                  <span class="terminal-title">terminal</span>
                </div>
                <!-- Terminal content -->
                <div class="terminal-body">
                  <div class="code-line line-1">
                    <span class="code-prompt">$</span>
                    <span class="code-cmd">curl</span>
                    <span class="code-flag">-X POST</span>
                    <span class="code-url">/v1/messages</span>
                  </div>
                  <div class="code-line line-2">
                    <span class="code-comment"># Routing to upstream...</span>
                  </div>
                  <div class="code-line line-3">
                    <span class="code-success">200 OK</span>
                    <span class="code-response">{ "content": "Hello!" }</span>
                  </div>
                  <div class="code-line line-4">
                    <span class="code-prompt">$</span>
                    <span class="cursor"></span>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>

        <!-- Supported Providers -->
        <div class="mb-8 text-center">
          <h2 class="mb-3 text-2xl font-bold text-gray-900 dark:text-white">
            {{ t('home.providers.title') }}
          </h2>
          <p class="text-sm text-gray-600 dark:text-dark-400">
            {{ t('home.providers.description') }}
          </p>
        </div>

        <div class="mb-12 flex flex-wrap items-center justify-center gap-4">
          <div
            v-for="model in supportedModels"
            :key="model"
            class="flex items-center gap-2 rounded-md border border-gray-200 bg-white px-5 py-3 dark:border-dark-700 dark:bg-dark-900"
          >
            <div
              class="flex h-8 w-8 items-center justify-center rounded-md bg-gray-900 dark:bg-gray-100"
            >
              <span class="text-xs font-bold text-white dark:text-dark-950">G</span>
            </div>
            <span class="text-sm font-medium text-gray-700 dark:text-dark-200">{{ model }}</span>
            <span
              class="rounded bg-gray-100 px-1.5 py-0.5 text-[10px] font-medium text-gray-700 dark:bg-dark-700 dark:text-gray-200"
              >{{ t('home.providers.supported') }}</span
            >
          </div>
        </div>

        <section data-testid="home-price-list" class="mb-16">
          <div class="mb-6 text-center">
            <h2 class="text-2xl font-bold text-gray-900 dark:text-white">套餐与流量卡</h2>
            <p class="mt-2 text-sm text-gray-600 dark:text-dark-400">订阅套餐 28 天有效，流量卡 365 天有效</p>
          </div>
          <div class="grid gap-4 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
            <router-link
              v-for="product in homeProducts"
              :key="product.title"
              :to="purchasePath"
              data-testid="home-product-link"
              class="group block rounded-lg border border-gray-200 bg-white p-5 transition-colors hover:border-gray-400 dark:border-dark-700 dark:bg-dark-900 dark:hover:border-dark-500"
            >
              <article data-testid="home-product-card">
                <p class="text-xs font-medium text-gray-500 dark:text-dark-400">{{ product.category }}</p>
                <div class="mt-2 flex items-baseline justify-between gap-3">
                  <h3 class="text-base font-semibold text-gray-900 dark:text-white">{{ product.title }}</h3>
                  <span class="text-lg font-bold text-gray-950 dark:text-gray-100">{{ product.price }}</span>
                </div>
                <p class="mt-3 text-sm text-gray-600 dark:text-dark-300">{{ product.detail }}</p>
              </article>
            </router-link>
          </div>
        </section>
        </div>
    </main>

    <!-- Footer -->
    <footer class="relative z-10 border-t border-gray-200/50 px-6 py-8 dark:border-dark-800/50">
      <div
        class="mx-auto flex max-w-6xl flex-col items-center justify-center gap-4 text-center sm:flex-row sm:text-left"
      >
        <p class="text-sm text-gray-500 dark:text-dark-400">
          &copy; {{ currentYear }} {{ siteName }}. {{ t('home.footer.allRightsReserved') }}
        </p>
        <div class="flex items-center gap-4">
          <a
            v-if="docUrl"
            :href="docUrl"
            target="_blank"
            rel="noopener noreferrer"
            class="text-sm text-gray-500 transition-colors hover:text-gray-700 dark:text-dark-400 dark:hover:text-white"
          >
            {{ t('home.docs') }}
          </a>
          <a
            :href="githubUrl"
            target="_blank"
            rel="noopener noreferrer"
            class="text-sm text-gray-500 transition-colors hover:text-gray-700 dark:text-dark-400 dark:hover:text-white"
          >
            GitHub
          </a>
        </div>
      </div>
    </footer>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAuthStore, useAppStore } from '@/stores'
import { DEFAULT_SITE_NAME, DEFAULT_SITE_SUBTITLE } from '@/constants/branding'
import LocaleSwitcher from '@/components/common/LocaleSwitcher.vue'
import Icon from '@/components/icons/Icon.vue'

const { t } = useI18n()

const authStore = useAuthStore()
const appStore = useAppStore()

// 站点设置优先使用后台配置，未配置时回退到当前品牌默认值。
const siteName = computed(() => appStore.cachedPublicSettings?.site_name || appStore.siteName || DEFAULT_SITE_NAME)
const siteLogo = computed(() => appStore.cachedPublicSettings?.site_logo || appStore.siteLogo || '')
const siteSubtitle = computed(() => {
  const subtitle = appStore.cachedPublicSettings?.site_subtitle
  return subtitle === undefined ? DEFAULT_SITE_SUBTITLE : subtitle
})
const docUrl = computed(() => appStore.cachedPublicSettings?.doc_url || appStore.docUrl || '')
const homeContent = computed(() => appStore.cachedPublicSettings?.home_content || '')

// Check if homeContent is a URL (for iframe display)
const isHomeContentUrl = computed(() => {
  const content = homeContent.value.trim()
  return content.startsWith('http://') || content.startsWith('https://')
})

// Theme
const isDark = ref(document.documentElement.classList.contains('dark'))

// GitHub URL
const githubUrl = 'https://github.com/Wei-Shaw/sub2api'

const supportedModels = ['gpt-5.6-luna', 'gpt-5.6-sol', 'gpt-5.6-terra'] as const

const homeProducts = [
  { category: '订阅套餐', title: '29 元套餐', price: '¥29', detail: '28 天有效' },
  { category: '订阅套餐', title: '39 元套餐', price: '¥39', detail: '28 天有效' },
  { category: '订阅套餐', title: '49 元套餐', price: '¥49', detail: '28 天有效' },
  { category: '订阅套餐', title: '59 元套餐', price: '¥59', detail: '28 天有效' },
  { category: '订阅套餐', title: '79 元套餐', price: '¥79', detail: '28 天有效' },
  { category: '订阅套餐', title: '99 元套餐', price: '¥99', detail: '28 天有效' },
  { category: '订阅套餐', title: '149 元套餐', price: '¥149', detail: '28 天有效' },
  { category: '订阅套餐', title: '199 元套餐', price: '¥199', detail: '28 天有效' },
  { category: '订阅套餐', title: '249 元套餐', price: '¥249', detail: '28 天有效' },
  { category: '订阅套餐', title: '299 元套餐', price: '¥299', detail: '28 天有效' },
  { category: 'GPT 流量卡', title: '5 刀额度', price: '¥2', detail: '365 天有效' },
  { category: 'GPT 流量卡', title: '10 刀额度', price: '¥3', detail: '365 天有效' },
  { category: 'GPT 流量卡', title: '20 刀额度', price: '¥5', detail: '365 天有效' },
] as const

// Auth state
const isAuthenticated = computed(() => authStore.isAuthenticated)
const isAdmin = computed(() => authStore.isAdmin)
const dashboardPath = computed(() => isAdmin.value ? '/admin/dashboard' : '/dashboard')
const purchasePath = computed(() => isAuthenticated.value ? '/purchase' : '/login')
const userInitial = computed(() => {
  const user = authStore.user
  if (!user || !user.email) return ''
  return user.email.charAt(0).toUpperCase()
})

// Current year for footer
const currentYear = computed(() => new Date().getFullYear())

// Toggle theme
function toggleTheme() {
  isDark.value = !isDark.value
  document.documentElement.classList.toggle('dark', isDark.value)
  localStorage.setItem('theme', isDark.value ? 'dark' : 'light')
}

// Initialize theme
function initTheme() {
  const savedTheme = localStorage.getItem('theme')
  if (
    savedTheme === 'dark' ||
    (!savedTheme && window.matchMedia('(prefers-color-scheme: dark)').matches)
  ) {
    isDark.value = true
    document.documentElement.classList.add('dark')
  }
}

onMounted(() => {
  initTheme()

  // Check auth state
  authStore.checkAuth()

  // Ensure public settings are loaded (will use cache if already loaded from injected config)
  if (!appStore.publicSettingsLoaded) {
    appStore.fetchPublicSettings()
  }
})
</script>

<style scoped>
/* Terminal Container */
.terminal-container {
  position: relative;
  display: inline-block;
}

/* Terminal Window */
.terminal-window {
  width: 420px;
  background: #111827;
  border-radius: 8px;
  border: 1px solid rgba(255, 255, 255, 0.08);
  box-shadow:
    0 18px 36px -18px rgba(15, 23, 42, 0.65),
    0 0 0 1px rgba(255, 255, 255, 0.04);
  overflow: hidden;
  transform: none;
  transition: transform 180ms var(--ease-out);
}

@media (hover: hover) and (pointer: fine) {
  .terminal-window {
    transform: translateY(0);
  }

  .terminal-window:hover {
    transform: translateY(-2px);
  }
}

/* Terminal Header */
.terminal-header {
  display: flex;
  align-items: center;
  padding: 12px 16px;
  background: rgba(17, 24, 39, 0.96);
  border-bottom: 1px solid rgba(255, 255, 255, 0.06);
}

.terminal-buttons {
  display: flex;
  gap: 8px;
}

.terminal-buttons span {
  width: 12px;
  height: 12px;
  border-radius: 50%;
}

.btn-close {
  background: #6b7280;
}
.btn-minimize {
  background: #9ca3af;
}
.btn-maximize {
  background: #d1d5db;
}

.terminal-title {
  flex: 1;
  text-align: center;
  font-size: 12px;
  font-family: ui-monospace, monospace;
  color: #9ca3af;
  margin-right: 52px;
}

/* Terminal Body */
.terminal-body {
  padding: 20px 24px;
  font-family: ui-monospace, 'Fira Code', monospace;
  font-size: 14px;
  line-height: 2;
}

.code-line {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
  opacity: 0;
  animation: line-appear 180ms var(--ease-out) forwards;
}

.line-1 {
  animation-delay: 0ms;
}
.line-2 {
  animation-delay: 40ms;
}
.line-3 {
  animation-delay: 80ms;
}
.line-4 {
  animation-delay: 120ms;
}

@keyframes line-appear {
  from {
    opacity: 0;
    transform: translateY(5px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

.code-prompt {
  color: #d1d5db;
  font-weight: bold;
}
.code-cmd {
  color: #f3f4f6;
}
.code-flag {
  color: #d1d5db;
}
.code-url {
  color: #e5e7eb;
}
.code-comment {
  color: #9ca3af;
  font-style: italic;
}
.code-success {
  color: #f9fafb;
  background: rgba(255, 255, 255, 0.12);
  padding: 2px 8px;
  border-radius: 4px;
  font-weight: 600;
}
.code-response {
  color: #d1d5db;
}

/* Blinking Cursor */
.cursor {
  display: inline-block;
  width: 8px;
  height: 16px;
  background: #f3f4f6;
  animation: blink 1s step-end infinite;
}

@keyframes blink {
  0%,
  50% {
    opacity: 1;
  }
  51%,
  100% {
    opacity: 0;
  }
}

@media (prefers-reduced-motion: reduce) {
  .code-line {
    animation: none;
    opacity: 1;
  }

  .terminal-window {
    transform: none;
    transition: none;
  }

  .terminal-window:hover {
    transform: none;
  }

  .cursor {
    animation: none;
    opacity: 1;
  }
}

/* Dark mode adjustments */
:deep(.dark) .terminal-window {
  box-shadow:
    0 25px 50px -12px rgba(0, 0, 0, 0.6),
    0 0 0 1px rgba(255, 255, 255, 0.12),
    0 0 40px rgba(255, 255, 255, 0.04),
    inset 0 1px 0 rgba(255, 255, 255, 0.1);
}
</style>

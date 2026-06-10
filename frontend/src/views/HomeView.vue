<template>
  <div v-if="homeContent" class="min-h-screen">
    <iframe
      v-if="isHomeContentUrl"
      :src="homeContent.trim()"
      class="h-screen w-full border-0"
      allowfullscreen
    ></iframe>
    <div v-else v-html="homeContent"></div>
  </div>

  <div v-else class="home-page min-h-screen overflow-hidden bg-[#171622] text-white">
    <header class="sticky top-0 z-30 border-b border-white/[0.08] bg-[#171622]/90 backdrop-blur-xl">
      <nav class="mx-auto flex h-16 max-w-7xl items-center justify-between px-4 sm:px-6 lg:px-8">
        <router-link to="/home" class="flex min-w-0 items-center gap-3">
          <span class="flex h-10 w-10 shrink-0 items-center justify-center overflow-hidden rounded-xl bg-[#222033] ring-1 ring-white/10">
            <img :src="siteLogo || '/logo.png'" alt="Logo" class="h-full w-full object-contain" />
          </span>
          <span class="truncate text-lg font-semibold tracking-normal">{{ siteName }}</span>
        </router-link>

        <div class="hidden items-center gap-2 lg:flex">
          <router-link to="/home" class="nav-pill nav-pill-active">首页</router-link>
          <router-link :to="dashboardPath" class="nav-pill">控制台</router-link>
          <a
            v-if="docUrl"
            :href="docUrl"
            target="_blank"
            rel="noopener noreferrer"
            class="nav-pill"
          >
            文档
          </a>
          <a href="#about" class="nav-pill">关于</a>
        </div>

        <div class="flex items-center gap-2">
          <router-link
            to="/login"
            class="hidden rounded-xl border border-blue-400/25 bg-blue-500/10 px-4 py-2 text-sm font-medium text-blue-100 transition hover:border-blue-300/[0.45] hover:bg-blue-500/[0.18] sm:inline-flex"
          >
            提供算力
          </router-link>
          <button
            @click="toggleTheme"
            class="hidden h-10 w-10 items-center justify-center rounded-full bg-white/[0.08] text-gray-200 ring-1 ring-white/10 transition hover:bg-white/[0.12] md:inline-flex"
            :title="isDark ? t('home.switchToLight') : t('home.switchToDark')"
          >
            <Icon v-if="isDark" name="sun" size="sm" />
            <Icon v-else name="moon" size="sm" />
          </button>
          <span class="hidden items-center gap-1 rounded-full bg-white/[0.08] px-3 py-2 text-sm text-gray-200 ring-1 ring-white/10 md:inline-flex">
            <Icon name="globe" size="sm" />
            简体中文
          </span>
          <router-link
            v-if="isAuthenticated"
            :to="dashboardPath"
            class="rounded-full bg-white/10 px-4 py-2 text-sm font-medium text-white transition hover:bg-white/[0.16]"
          >
            {{ t('home.dashboard') }}
          </router-link>
          <template v-else>
            <router-link to="/login" class="rounded-full bg-white/10 px-4 py-2 text-sm font-medium text-white transition hover:bg-white/[0.16]">
              登录
            </router-link>
            <router-link to="/register" class="rounded-full bg-[#5aa7ff] px-4 py-2 text-sm font-semibold text-white shadow-lg shadow-blue-500/25 transition hover:bg-[#78b8ff]">
              注册
            </router-link>
          </template>
        </div>
      </nav>
    </header>

    <main class="relative">
      <section class="relative mx-auto max-w-7xl px-4 pb-10 pt-16 sm:px-6 sm:pt-20 lg:px-8">
        <div class="pointer-events-none absolute inset-x-0 top-0 h-[520px] bg-[radial-gradient(circle_at_50%_0%,rgba(90,117,255,0.22),transparent_58%)]"></div>

        <div class="relative text-center">
          <p class="mx-auto mb-5 inline-flex items-center gap-2 rounded-full border border-white/10 bg-white/[0.07] px-4 py-2 text-sm text-blue-100">
            <Icon name="sparkles" size="sm" />
            {{ siteSubtitle || 'TokenQS AI Gateway' }}
          </p>
          <h1 class="mx-auto max-w-5xl text-balance text-4xl font-semibold leading-tight text-white sm:text-5xl lg:text-6xl">
            一站式大模型服务统一入口
          </h1>
          <p class="mx-auto mt-5 max-w-3xl text-base leading-8 text-gray-300 sm:text-lg">
            源头安全可控，价格更具优势，稳定可靠。统一接入 DeepSeek、GLM、Qwen、Gemini 等模型，按量计费，开箱即用。
          </p>
        </div>

        <div class="relative mt-10 overflow-hidden rounded-2xl border border-white/[0.12] bg-white/[0.06] shadow-2xl shadow-black/25">
          <div class="absolute inset-0 bg-[linear-gradient(100deg,rgba(86,72,255,0.32),rgba(70,201,255,0.10),rgba(255,255,255,0.06))]"></div>
          <div class="absolute right-0 top-0 h-full w-1/2 bg-[radial-gradient(circle_at_70%_35%,rgba(49,185,255,0.35),transparent_45%)]"></div>
          <div class="relative flex flex-col items-start justify-between gap-6 px-6 py-7 md:flex-row md:items-center md:px-10">
            <div class="flex items-start gap-4">
              <span class="rounded-full bg-gradient-to-r from-orange-400 to-pink-500 px-3 py-1 text-xs font-bold text-white shadow-lg shadow-pink-500/25">
                HOT
              </span>
              <div>
                <h2 class="text-2xl font-semibold text-white">GLM-5.1</h2>
                <p class="mt-2 text-sm text-gray-300">
                  复杂创作的旗舰大模型，官方定价低至 <span class="font-semibold text-red-300">7.4折</span> 起
                </p>
              </div>
            </div>
            <router-link
              :to="isAuthenticated ? dashboardPath : '/register'"
              class="inline-flex items-center gap-2 rounded-xl bg-gradient-to-r from-[#6d5dfc] to-[#7d61ff] px-6 py-3 text-sm font-semibold text-white shadow-lg shadow-indigo-500/30 transition hover:translate-y-[-1px] hover:shadow-indigo-500/[0.45]"
            >
              立即体验
              <Icon name="arrowRight" size="sm" />
            </router-link>
          </div>
        </div>
      </section>

      <section id="models" class="mx-auto grid max-w-7xl gap-8 px-4 pb-16 sm:px-6 lg:grid-cols-[280px_minmax(0,1fr)] lg:px-8">
        <aside class="space-y-8 lg:sticky lg:top-24 lg:self-start">
          <div class="flex items-center justify-between">
            <h2 class="text-xl font-semibold">筛选</h2>
            <button
              @click="resetFilters"
              class="rounded-xl border border-white/10 bg-white/[0.07] px-4 py-2 text-sm text-gray-200 transition hover:bg-white/[0.12]"
            >
              重置
            </button>
          </div>

          <section class="space-y-3">
            <h3 class="text-sm font-medium text-gray-300">模型类型</h3>
            <div class="space-y-2">
              <button
                v-for="item in categoryFilters"
                :key="item.id"
                type="button"
                :class="filterButtonClass(selectedCategory === item.id)"
                @click="selectedCategory = item.id"
              >
                <span class="truncate">{{ item.label }}</span>
                <span class="ml-3 rounded-full bg-white/10 px-2 py-0.5 text-xs text-gray-300">{{ item.count }}</span>
              </button>
            </div>
          </section>

          <section class="space-y-3">
            <h3 class="text-sm font-medium text-gray-300">标签</h3>
            <div class="space-y-2">
              <button
                v-for="item in tagFilters"
                :key="item.id"
                type="button"
                :class="filterButtonClass(selectedTag === item.id)"
                @click="selectedTag = item.id"
              >
                <span class="truncate">{{ item.label }}</span>
                <span class="ml-3 rounded-full bg-white/10 px-2 py-0.5 text-xs text-gray-300">{{ item.count }}</span>
              </button>
            </div>
          </section>

          <section class="space-y-3">
            <h3 class="text-sm font-medium text-gray-300">供应商类型</h3>
            <div class="space-y-2">
              <button
                v-for="item in supplierFilters"
                :key="item.id"
                type="button"
                :class="filterButtonClass(selectedSupplier === item.id)"
                @click="selectedSupplier = item.id"
              >
                <span class="truncate">{{ item.label }}</span>
                <span class="ml-3 rounded-full bg-white/10 px-2 py-0.5 text-xs text-gray-300">{{ item.count }}</span>
              </button>
            </div>
          </section>
        </aside>

        <div class="min-w-0">
          <div class="mb-6 flex flex-col gap-3 md:flex-row md:items-center">
            <label class="relative flex-1">
              <Icon name="search" size="sm" class="pointer-events-none absolute left-4 top-1/2 -translate-y-1/2 text-gray-500" />
              <input
                v-model="searchQuery"
                type="search"
                class="h-12 w-full rounded-xl border border-white/[0.08] bg-white/[0.07] pl-11 pr-4 text-sm text-white outline-none transition placeholder:text-gray-500 focus:border-blue-400/60 focus:bg-white/10"
                placeholder="模糊搜索模型名称"
              />
            </label>
            <label class="flex h-12 items-center gap-3 rounded-xl border border-white/[0.08] bg-white/[0.07] px-4 text-sm text-gray-300">
              排序
              <select v-model="sortMode" class="bg-transparent text-white outline-none">
                <option class="bg-[#1d1c2a]" value="popular">热门</option>
                <option class="bg-[#1d1c2a]" value="discount">折扣优先</option>
                <option class="bg-[#1d1c2a]" value="name">名称</option>
              </select>
            </label>
          </div>

          <div class="grid gap-4 md:grid-cols-2">
            <article
              v-for="model in visibleModels"
              :key="model.name"
              class="model-card group relative overflow-hidden rounded-2xl border border-white/10 bg-[#111018] p-5 transition hover:-translate-y-px hover:border-blue-400/[0.45] hover:shadow-2xl hover:shadow-blue-950/30"
            >
              <div class="flex items-start justify-between gap-4">
                <div class="flex min-w-0 items-center gap-3">
                  <span :class="['model-logo', model.accent]">{{ model.providerInitial }}</span>
                  <div class="min-w-0">
                    <p class="truncate text-xs font-medium text-gray-500">{{ model.provider }}</p>
                    <h3 class="truncate text-xl font-semibold text-gray-100">{{ model.name }}</h3>
                  </div>
                </div>
                <button
                  @click="copyModelName(model.name)"
                  class="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg border border-white/10 bg-white/5 text-gray-400 transition hover:bg-white/10 hover:text-white"
                  :title="copiedModelName === model.name ? '已复制' : '复制模型名'"
                >
                  <Icon :name="copiedModelName === model.name ? 'check' : 'copy'" size="sm" />
                </button>
              </div>

              <dl class="mt-5 space-y-4">
                <div class="grid grid-cols-[76px_minmax(0,1fr)] items-center gap-4 text-sm">
                  <dt class="text-gray-400">输入价格</dt>
                  <dd class="min-w-0">
                    <span class="mr-2 text-xs text-gray-500 line-through">官方 {{ model.officialInput }}</span>
                    <span class="rounded-md bg-red-500/15 px-2 py-0.5 text-xs font-semibold text-red-200">{{ model.inputDiscount }}</span>
                    <p class="mt-1 font-semibold text-amber-100">我们 {{ model.ourInput }} / 1M Tokens</p>
                  </dd>
                </div>
                <div class="grid grid-cols-[76px_minmax(0,1fr)] items-center gap-4 text-sm">
                  <dt class="text-gray-400">输出价格</dt>
                  <dd class="min-w-0">
                    <span class="mr-2 text-xs text-gray-500 line-through">官方 {{ model.officialOutput }}</span>
                    <span class="rounded-md bg-red-500/15 px-2 py-0.5 text-xs font-semibold text-red-200">{{ model.outputDiscount }}</span>
                    <p class="mt-1 font-semibold text-amber-100">我们 {{ model.ourOutput }} / 1M Tokens</p>
                  </dd>
                </div>
                <div class="grid grid-cols-[76px_minmax(0,1fr)] items-center gap-4 text-sm">
                  <dt class="text-gray-400">供应商</dt>
                  <dd>
                    <span class="rounded-lg bg-emerald-400/[0.12] px-2.5 py-1 text-xs font-medium text-emerald-100">{{ model.supplier }}</span>
                  </dd>
                </div>
              </dl>

              <div class="mt-5 flex flex-wrap gap-2">
                <span class="rounded-full bg-indigo-500/15 px-3 py-1 text-xs text-indigo-100">{{ model.billing }}</span>
                <span v-for="tag in model.tags" :key="tag" class="rounded-full bg-white/[0.08] px-3 py-1 text-xs text-gray-200">
                  {{ tag }}
                </span>
              </div>
            </article>
          </div>

          <div v-if="visibleModels.length === 0" class="rounded-2xl border border-white/10 bg-white/[0.06] p-10 text-center text-gray-300">
            没有匹配的模型，换个关键词或重置筛选。
          </div>
        </div>
      </section>
    </main>

    <footer id="about" class="border-t border-white/[0.08] bg-[#14131d] px-4 py-8 text-sm text-gray-400 sm:px-6 lg:px-8">
      <div class="mx-auto flex max-w-7xl flex-col items-center justify-between gap-4 text-center md:flex-row">
        <p>&copy; {{ currentYear }} {{ siteName }}. {{ t('home.footer.allRightsReserved') }}</p>
        <div class="flex flex-wrap items-center justify-center gap-3">
          <router-link to="/legal/user-agreement" class="transition hover:text-white">用户协议</router-link>
          <span class="text-gray-600">·</span>
          <router-link to="/legal/privacy-policy" class="transition hover:text-white">隐私政策</router-link>
          <template v-if="docUrl">
            <span class="text-gray-600">·</span>
            <a :href="docUrl" target="_blank" rel="noopener noreferrer" class="transition hover:text-white">文档</a>
          </template>
        </div>
      </div>
    </footer>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAuthStore, useAppStore } from '@/stores'
import Icon from '@/components/icons/Icon.vue'

type FilterItem = {
  id: string
  label: string
  count: number
}

type ModelCard = {
  name: string
  provider: string
  providerInitial: string
  category: string
  tags: string[]
  supplierType: string
  supplier: string
  officialInput: string
  ourInput: string
  inputDiscount: string
  officialOutput: string
  ourOutput: string
  outputDiscount: string
  billing: string
  hot?: boolean
  accent: string
}

const { t } = useI18n()
const authStore = useAuthStore()
const appStore = useAppStore()

const modelCatalog: ModelCard[] = [
  {
    name: 'deepseek-v3',
    provider: 'DeepSeek',
    providerInitial: 'D',
    category: 'DeepSeek',
    tags: ['文本'],
    supplierType: '公有云',
    supplier: '公有云',
    officialInput: '¥2.04',
    ourInput: '¥0.91',
    inputDiscount: '-56%',
    officialOutput: '¥8.16',
    ourOutput: '¥3.63',
    outputDiscount: '-56%',
    billing: '按量计费',
    hot: true,
    accent: 'from-blue-500 to-indigo-500'
  },
  {
    name: 'deepseek-v4-flash',
    provider: 'DeepSeek',
    providerInitial: 'D',
    category: 'DeepSeek',
    tags: ['文本', '热门'],
    supplierType: '公有云',
    supplier: '公有云',
    officialInput: '¥0.97',
    ourInput: '¥0.29',
    inputDiscount: '-70%',
    officialOutput: '¥1.90',
    ourOutput: '¥0.57',
    outputDiscount: '-70%',
    billing: '按量计费',
    hot: true,
    accent: 'from-sky-500 to-blue-600'
  },
  {
    name: 'qwen3.6-flash',
    provider: 'Qwen',
    providerInitial: 'Q',
    category: 'Qwen',
    tags: ['文本'],
    supplierType: '企业中转站',
    supplier: '企业中转站',
    officialInput: '¥1.20',
    ourInput: '¥1.02',
    inputDiscount: '-15%',
    officialOutput: '¥7.21',
    ourOutput: '¥6.13',
    outputDiscount: '-15%',
    billing: '按量计费',
    accent: 'from-violet-500 to-indigo-500'
  },
  {
    name: 'GLM-5.1',
    provider: 'Zhipu',
    providerInitial: 'G',
    category: 'Zhipu',
    tags: ['文本', '热门'],
    supplierType: '公有云',
    supplier: '公有云',
    officialInput: '¥5.98',
    ourInput: '¥4.49',
    inputDiscount: '-25%',
    officialOutput: '¥23.94',
    ourOutput: '¥17.95',
    outputDiscount: '-25%',
    billing: '按量计费',
    hot: true,
    accent: 'from-cyan-500 to-blue-500'
  },
  {
    name: 'minimax-m2',
    provider: 'Minimax',
    providerInitial: 'M',
    category: 'Minimax',
    tags: ['文本', '多模态'],
    supplierType: 'AIDC',
    supplier: 'AIDC',
    officialInput: '¥4.00',
    ourInput: '¥2.40',
    inputDiscount: '-40%',
    officialOutput: '¥16.00',
    ourOutput: '¥9.60',
    outputDiscount: '-40%',
    billing: '按量计费',
    accent: 'from-pink-500 to-rose-500'
  },
  {
    name: 'kling-v2.5-turbo',
    provider: 'Kling',
    providerInitial: 'K',
    category: 'Kling',
    tags: ['视频', '多模态'],
    supplierType: '企业中转站',
    supplier: '企业中转站',
    officialInput: '¥12.00',
    ourInput: '¥8.88',
    inputDiscount: '-26%',
    officialOutput: '¥28.00',
    ourOutput: '¥20.72',
    outputDiscount: '-26%',
    billing: '按量计费',
    accent: 'from-emerald-500 to-teal-500'
  }
]

const siteName = computed(() => appStore.cachedPublicSettings?.site_name || appStore.siteName || 'TokenQS')
const siteLogo = computed(() => appStore.cachedPublicSettings?.site_logo || appStore.siteLogo || '')
const siteSubtitle = computed(() => appStore.cachedPublicSettings?.site_subtitle || 'AI API Gateway Platform')
const docUrl = computed(() => appStore.cachedPublicSettings?.doc_url || appStore.docUrl || '')
const homeContent = computed(() => appStore.cachedPublicSettings?.home_content || '')

const isHomeContentUrl = computed(() => {
  const content = homeContent.value.trim()
  return content.startsWith('http://') || content.startsWith('https://')
})

const isDark = ref(document.documentElement.classList.contains('dark'))
const searchQuery = ref('')
const selectedCategory = ref('all')
const selectedTag = ref('all')
const selectedSupplier = ref('all')
const sortMode = ref<'popular' | 'discount' | 'name'>('popular')
const copiedModelName = ref('')

const isAuthenticated = computed(() => authStore.isAuthenticated)
const isAdmin = computed(() => authStore.isAdmin)
const dashboardPath = computed(() => isAdmin.value ? '/admin/dashboard' : '/dashboard')
const currentYear = computed(() => new Date().getFullYear())

const categoryFilters = computed<FilterItem[]>(() => [
  { id: 'all', label: '全部模型', count: modelCatalog.length },
  ...buildFilterItems(modelCatalog.map(model => model.category))
])

const tagFilters = computed<FilterItem[]>(() => [
  { id: 'all', label: '全部标签', count: modelCatalog.length },
  ...buildFilterItems(modelCatalog.flatMap(model => model.tags))
])

const supplierFilters = computed<FilterItem[]>(() => [
  { id: 'all', label: '全部类型', count: modelCatalog.length },
  ...buildFilterItems(modelCatalog.map(model => model.supplierType))
])

const visibleModels = computed(() => {
  const keyword = searchQuery.value.trim().toLowerCase()
  const models = modelCatalog.filter(model => {
    const matchesKeyword = !keyword || [
      model.name,
      model.provider,
      model.category,
      ...model.tags
    ].some(value => value.toLowerCase().includes(keyword))

    return matchesKeyword &&
      (selectedCategory.value === 'all' || model.category === selectedCategory.value) &&
      (selectedTag.value === 'all' || model.tags.includes(selectedTag.value)) &&
      (selectedSupplier.value === 'all' || model.supplierType === selectedSupplier.value)
  })

  return [...models].sort((a, b) => {
    if (sortMode.value === 'name') return a.name.localeCompare(b.name)
    if (sortMode.value === 'discount') return discountValue(a.inputDiscount) - discountValue(b.inputDiscount)
    return Number(Boolean(b.hot)) - Number(Boolean(a.hot)) || a.name.localeCompare(b.name)
  })
})

function buildFilterItems(values: string[]): FilterItem[] {
  const counts = values.reduce<Record<string, number>>((acc, value) => {
    acc[value] = (acc[value] || 0) + 1
    return acc
  }, {})

  return Object.entries(counts).map(([label, count]) => ({ id: label, label, count }))
}

function discountValue(discount: string) {
  return Number(discount.replace('%', '')) || 0
}

function resetFilters() {
  searchQuery.value = ''
  selectedCategory.value = 'all'
  selectedTag.value = 'all'
  selectedSupplier.value = 'all'
  sortMode.value = 'popular'
}

function filterButtonClass(active: boolean) {
  return [
    'flex w-full items-center justify-between rounded-xl border px-4 py-2.5 text-left text-sm transition',
    active
      ? 'border-white/[0.12] bg-white/[0.16] text-white'
      : 'border-white/[0.08] bg-transparent text-gray-300 hover:bg-white/[0.08]'
  ]
}

async function copyModelName(name: string) {
  try {
    await navigator.clipboard.writeText(name)
    copiedModelName.value = name
    window.setTimeout(() => {
      if (copiedModelName.value === name) copiedModelName.value = ''
    }, 1600)
  } catch {
    copiedModelName.value = ''
  }
}

function toggleTheme() {
  isDark.value = !isDark.value
  document.documentElement.classList.toggle('dark', isDark.value)
  localStorage.setItem('theme', isDark.value ? 'dark' : 'light')
}

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
  authStore.checkAuth()

  if (!appStore.publicSettingsLoaded) {
    appStore.fetchPublicSettings()
  }
})
</script>

<style scoped>
.home-page {
  color-scheme: dark;
}

.nav-pill {
  border-radius: 0.75rem;
  color: rgb(209 213 219);
  font-size: 0.875rem;
  line-height: 1.25rem;
  padding: 0.55rem 0.9rem;
  transition: background-color 0.16s ease, color 0.16s ease;
}

.nav-pill:hover,
.nav-pill-active {
  background: rgba(255, 255, 255, 0.1);
  color: white;
}

.model-card {
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.04);
}

.model-card::before {
  background: radial-gradient(circle at top left, rgba(91, 141, 255, 0.2), transparent 38%);
  content: "";
  inset: 0;
  opacity: 0;
  pointer-events: none;
  position: absolute;
  transition: opacity 0.16s ease;
}

.model-card:hover::before {
  opacity: 1;
}

.model-logo {
  align-items: center;
  background-image: linear-gradient(135deg, var(--tw-gradient-stops));
  border-radius: 0.95rem;
  box-shadow: 0 16px 32px rgba(37, 99, 235, 0.18);
  color: white;
  display: inline-flex;
  flex: 0 0 auto;
  font-size: 0.95rem;
  font-weight: 700;
  height: 2.9rem;
  justify-content: center;
  width: 2.9rem;
}
</style>

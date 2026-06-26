<template>
  <AppLayout>
    <div class="space-y-6">
      <section
        v-if="featuredPriceItems.length > 0"
        data-test="available-channel-price-summary"
        class="rounded-lg border border-gray-200 bg-white px-4 py-3 dark:border-dark-700 dark:bg-dark-800"
      >
        <div class="mb-3 flex flex-col gap-1 sm:flex-row sm:items-end sm:justify-between">
          <div>
            <h2 class="text-sm font-semibold text-gray-900 dark:text-gray-100">
              {{ t('availableChannels.priceSummary.title') }}
            </h2>
            <p class="text-xs text-gray-500 dark:text-gray-400">
              {{ t('availableChannels.priceSummary.subtitle') }}
            </p>
          </div>
        </div>

        <div class="grid gap-3 md:grid-cols-3">
          <div
            v-for="item in featuredPriceItems"
            :key="item.key"
            class="rounded-md border border-gray-100 bg-gray-50 px-3 py-2 dark:border-dark-700 dark:bg-dark-900/40"
          >
            <div class="mb-2 text-sm font-semibold text-gray-900 dark:text-gray-100">
              {{ item.title }}
            </div>
            <dl class="space-y-1">
              <div
                v-for="row in item.rows"
                :key="`${item.key}-${row.label}`"
                class="flex items-center justify-between gap-3 text-xs"
              >
                <dt class="text-gray-500 dark:text-gray-400">{{ row.label }}</dt>
                <dd class="font-mono font-medium text-gray-900 dark:text-gray-100">
                  {{ row.value }}
                </dd>
              </div>
            </dl>
          </div>
        </div>
      </section>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import userChannelsAPI, {
  type UserAvailableChannel,
  type UserModelPrice,
  type UserSupportedModel,
} from '@/api/channels'
import userGroupsAPI from '@/api/groups'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import { formatScaled } from '@/utils/pricing'
import { BILLING_MODE_IMAGE } from '@/constants/channel'
import type { Group } from '@/types'

const { t } = useI18n()
const appStore = useAppStore()

const channels = ref<UserAvailableChannel[]>([])
const modelPrices = ref<UserModelPrice[]>([])
const groups = ref<Group[]>([])

const FEATURED_TEXT_MODELS = ['gpt-5.4', 'gpt-5.5'] as const
const TOKEN_PRICE_SCALE = 1_000_000

interface PriceSummaryRow {
  label: string
  value: string
}

interface PriceSummaryItem {
  key: string
  title: string
  rows: PriceSummaryRow[]
}

const featuredPriceItems = computed<PriceSummaryItem[]>(() => {
  const models = flattenSupportedModels(channels.value)
  const modelByName = new Map<string, UserSupportedModel>()
  for (const model of models) {
    const key = model.name.toLowerCase()
    if (!modelByName.has(key)) {
      modelByName.set(key, model)
    }
  }

  const priceByName = new Map<string, UserModelPrice>()
  for (const price of modelPrices.value) {
    priceByName.set(price.name.toLowerCase(), price)
  }

  const items = FEATURED_TEXT_MODELS
    .map((name) =>
      buildTextModelPriceItemFromPrice(priceByName.get(name)) ??
      buildTextModelPriceItemFromModel(modelByName.get(name)),
    )
    .filter((item): item is PriceSummaryItem => item !== null)

  const imageItem = buildImagePriceItem(groups.value, models)
  if (imageItem) {
    items.push(imageItem)
  }

  return items
})

async function loadChannels() {
  try {
    const [list, prices, availableGroups] = await Promise.all([
      userChannelsAPI.getAvailable(),
      userChannelsAPI.getPrices().catch((err: unknown) => {
        console.error('Failed to load channel prices:', err)
        return [] as UserModelPrice[]
      }),
      userGroupsAPI.getAvailable().catch((err: unknown) => {
        console.error('Failed to load available groups:', err)
        return [] as Group[]
      }),
    ])
    channels.value = list
    modelPrices.value = prices
    groups.value = availableGroups
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('common.error')))
  }
}

function flattenSupportedModels(list: UserAvailableChannel[]): UserSupportedModel[] {
  return list.flatMap((channel) =>
    channel.platforms.flatMap((section) => section.supported_models),
  )
}

function buildTextModelPriceItemFromModel(model: UserSupportedModel | undefined): PriceSummaryItem | null {
  if (!model?.pricing) return null
  return {
    key: model.name,
    title: model.name,
    rows: [
      {
        label: t('availableChannels.priceSummary.input'),
        value: formatTokenPrice(model.pricing.input_price),
      },
      {
        label: t('availableChannels.priceSummary.output'),
        value: formatTokenPrice(model.pricing.output_price),
      },
      {
        label: t('availableChannels.priceSummary.cacheWrite'),
        value: formatTokenPrice(model.pricing.cache_write_price),
      },
      {
        label: t('availableChannels.priceSummary.cacheRead'),
        value: formatTokenPrice(model.pricing.cache_read_price),
      },
    ],
  }
}

function buildTextModelPriceItemFromPrice(price: UserModelPrice | undefined): PriceSummaryItem | null {
  if (!price) return null
  const rows: PriceSummaryRow[] = [
    {
      label: t('availableChannels.priceSummary.input'),
      value: formatTokenPrice(price.input_price),
    },
    {
      label: t('availableChannels.priceSummary.output'),
      value: formatTokenPrice(price.output_price),
    },
    {
      label: t('availableChannels.priceSummary.cacheWrite'),
      value: formatTokenPrice(price.cache_write_price),
    },
    {
      label: t('availableChannels.priceSummary.cacheRead'),
      value: formatTokenPrice(price.cache_read_price),
    },
  ]

  addPriorityPriceRow(rows, {
    label: t('availableChannels.priceSummary.priorityInput'),
    value: price.priority_input_price,
    baseValue: price.input_price,
  })
  addPriorityPriceRow(rows, {
    label: t('availableChannels.priceSummary.priorityOutput'),
    value: price.priority_output_price,
    baseValue: price.output_price,
  })
  addPriorityPriceRow(rows, {
    label: t('availableChannels.priceSummary.priorityCacheRead'),
    value: price.priority_cache_read_price,
    baseValue: price.cache_read_price,
  })

  return {
    key: price.name,
    title: price.name,
    rows,
  }
}

function addPriorityPriceRow(
  rows: PriceSummaryRow[],
  input: { label: string; value: number | null; baseValue: number | null },
) {
  if (input.value == null || input.value === input.baseValue) return
  rows.push({
    label: input.label,
    value: formatTokenPrice(input.value),
  })
}

function buildImagePriceItem(groups: Group[], models: UserSupportedModel[]): PriceSummaryItem | null {
  const group = groups.find((g) =>
    g.allow_image_generation &&
    (g.image_price_1k != null || g.image_price_2k != null || g.image_price_4k != null),
  )
  if (group) {
    return {
      key: 'image-generation',
      title: t('availableChannels.priceSummary.imageTitle'),
      rows: [
        {
          label: t('availableChannels.priceSummary.image1k'),
          value: formatImagePrice(group.image_price_1k),
        },
        {
          label: t('availableChannels.priceSummary.image2k'),
          value: formatImagePrice(group.image_price_2k),
        },
        {
          label: t('availableChannels.priceSummary.image4k'),
          value: formatImagePrice(group.image_price_4k),
        },
      ],
    }
  }

  const imageModel = models.find((m) =>
    m.pricing?.billing_mode === BILLING_MODE_IMAGE ||
    m.name.toLowerCase().includes('image'),
  )
  if (!imageModel?.pricing) return null
  const price = imageModel.pricing.image_output_price ?? imageModel.pricing.per_request_price
  return {
    key: imageModel.name,
    title: imageModel.name,
    rows: [
      {
        label: t('availableChannels.priceSummary.imageOutput'),
        value: formatImagePrice(price),
      },
    ],
  }
}

function formatTokenPrice(value: number | null): string {
  return `${formatScaled(value, TOKEN_PRICE_SCALE)} / 1M token`
}

function formatImagePrice(value: number | null): string {
  if (value == null) return '-'
  return `$${value.toFixed(2)} / 张`
}

onMounted(loadChannels)
</script>

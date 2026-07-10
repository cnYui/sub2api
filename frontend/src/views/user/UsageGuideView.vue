<template>
  <AppLayout>
    <section class="usage-guide-shell">
      <nav
        data-test="usage-guide-topic-nav-desktop"
        class="usage-guide-side-nav"
        aria-label="使用方法分类"
      >
        <button
          v-for="topic in guideTopics"
          :key="topic.id"
          type="button"
          class="usage-guide-nav-item"
          :class="{ 'usage-guide-nav-item-active': topic.id === activeTopicId }"
          @click="activeTopicId = topic.id"
        >
          <span class="usage-guide-nav-title">{{ topic.title }}</span>
          <span class="usage-guide-nav-desc">{{ topic.description }}</span>
        </button>
      </nav>

      <div class="usage-guide-main">
        <div
          data-test="usage-guide-topic-tabs-mobile"
          class="usage-guide-mobile-tabs"
          role="tablist"
          aria-label="使用方法分类"
        >
          <button
            v-for="topic in guideTopics"
            :key="topic.id"
            type="button"
            class="usage-guide-mobile-tab"
            :class="{ 'usage-guide-mobile-tab-active': topic.id === activeTopicId }"
            role="tab"
            :aria-selected="topic.id === activeTopicId"
            @click="activeTopicId = topic.id"
          >
            {{ topic.title }}
          </button>
        </div>

        <header class="usage-guide-header">
          <span class="usage-guide-kicker">使用方法</span>
          <h2 class="usage-guide-heading">{{ activeTopic.title }}</h2>
          <p class="usage-guide-description">{{ activeTopic.description }}</p>
        </header>

        <div v-if="activeTopic.kind === 'steps'" class="usage-guide-steps">
          <article
            v-for="step in activeTopic.steps"
            :key="step.step"
            data-test="usage-guide-step"
            class="guide-step"
          >
            <div
              v-if="step.imagePosition === 'beforeTitle'"
              class="guide-images guide-images-before"
            >
              <img
                v-for="image in step.images"
                :key="image.alt"
                :src="image.src"
                :alt="image.alt"
                class="guide-image"
                loading="lazy"
              >
            </div>

            <div class="guide-heading">
              <span class="guide-kicker">步骤 {{ step.step }}</span>
              <h3 class="guide-title">{{ step.title }}</h3>
            </div>

            <div v-if="step.imagePosition !== 'beforeTitle'" class="guide-images">
              <img
                v-for="image in step.images"
                :key="image.alt"
                :src="image.src"
                :alt="image.alt"
                class="guide-image"
                loading="lazy"
              >
            </div>
          </article>
        </div>

        <div v-else class="usage-guide-sections">
          <section
            v-for="section in activeTopic.sections"
            :key="section.title"
            class="usage-guide-section-card"
          >
            <h3 class="usage-guide-section-title">{{ section.title }}</h3>
            <p
              v-for="paragraph in section.paragraphs"
              :key="paragraph"
              class="usage-guide-section-text"
            >
              {{ paragraph }}
            </p>

            <div v-if="section.priceRows" class="usage-guide-price-grid">
              <div
                v-for="row in section.priceRows"
                :key="row.label"
                class="usage-guide-price-row"
              >
                <span>{{ row.label }}</span>
                <strong>{{ row.price }}</strong>
              </div>
            </div>

            <div v-if="section.endpointRows" class="usage-guide-table-wrap">
              <table class="usage-guide-endpoint-table">
                <thead>
                  <tr>
                    <th>用途</th>
                    <th>方法</th>
                    <th>规范 URL</th>
                    <th>大白话说明</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="row in section.endpointRows" :key="row.url">
                    <td>{{ row.label }}</td>
                    <td>{{ row.method }}</td>
                    <td><code>{{ row.url }}</code></td>
                    <td>{{ row.meaning }}</td>
                  </tr>
                </tbody>
              </table>
            </div>

            <div v-if="section.legacyRows" class="usage-guide-table-wrap">
              <table class="usage-guide-endpoint-table">
                <thead>
                  <tr>
                    <th>不要再这样写</th>
                    <th>现在会怎样</th>
                    <th>改成这样</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="row in section.legacyRows" :key="row.oldUrl">
                    <td><code>{{ row.oldUrl }}</code></td>
                    <td>{{ row.result }}</td>
                    <td><code>{{ row.useInstead }}</code></td>
                  </tr>
                </tbody>
              </table>
            </div>

            <pre v-if="section.code" class="usage-guide-code"><code>{{ section.code }}</code></pre>
          </section>
        </div>
      </div>
    </section>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import step01Image from '@/assets/usage-guide/step-01-shop-entry.png'
import step02Image from '@/assets/usage-guide/step-02-login-register.png'
import step03Image from '@/assets/usage-guide/step-03-subscription-plans.png'
import step04ConfirmPaymentImage from '@/assets/usage-guide/step-04-confirm-payment.png'
import step05Image from '@/assets/usage-guide/step-05-create-api-key.png'
import step06Image from '@/assets/usage-guide/step-06-key-group-advanced.png'
import step07ProviderImage from '@/assets/usage-guide/step-07-cc-switch-provider-list.png'
import step07EditImage from '@/assets/usage-guide/step-07-cc-switch-edit-provider.png'
import step08Image from '@/assets/usage-guide/step-08-cc-switch-active.png'
import traeStep01Image from '@/assets/usage-guide/trae-step-01-add-model.png'
import traeStep02Image from '@/assets/usage-guide/trae-step-02-custom-config.png'
import traeStep03Image from '@/assets/usage-guide/trae-step-03-fill-url-key.png'
import traeStep04Image from '@/assets/usage-guide/trae-step-04-select-model.png'

type GuideStep = {
  step: number
  title: string
  images: Array<{
    src: string
    alt: string
  }>
  imagePosition?: 'beforeTitle'
}

type GuideEndpointRow = {
  label: string
  method: string
  url: string
  meaning: string
}

type GuideLegacyRow = {
  oldUrl: string
  result: string
  useInstead: string
}

type GuidePriceRow = {
  label: string
  price: string
}

type GuideSection = {
  title: string
  paragraphs: string[]
  priceRows?: GuidePriceRow[]
  endpointRows?: GuideEndpointRow[]
  legacyRows?: GuideLegacyRow[]
  code?: string
}

type GuideTopic =
  | {
    id: string
    title: string
    description: string
    kind: 'steps'
    steps: GuideStep[]
  }
  | {
    id: string
    title: string
    description: string
    kind: 'sections'
    sections: GuideSection[]
  }

const codexSetupSteps: GuideStep[] = [
  {
    step: 1,
    title: '访问 aaccx.pw/shop 页面，点击图中的进入按钮',
    images: [{ src: step01Image, alt: '步骤 1 截图 1' }],
  },
  {
    step: 2,
    title: '新用户注册，老用户登录',
    images: [{ src: step02Image, alt: '步骤 2 截图 1' }],
  },
  {
    step: 3,
    title: '选择订阅的页面，选择合适的套餐',
    images: [{ src: step03Image, alt: '步骤 3 截图 1' }],
  },
  {
    step: 4,
    title: '完成支付',
    images: [{ src: step04ConfirmPaymentImage, alt: '步骤 4 截图 1' }],
  },
  {
    step: 5,
    title: '支付完成后，去 API Key 页面生成密钥',
    images: [{ src: step05Image, alt: '步骤 5 截图 1' }],
  },
  {
    step: 6,
    title: '选择分组，并且可以设置高级功能',
    images: [{ src: step06Image, alt: '步骤 6 截图 1' }],
  },
  {
    step: 7,
    title: '启动 cc-switch，粘贴 API Key 和请求端口',
    images: [
      { src: step07ProviderImage, alt: '步骤 7 截图 1' },
      { src: step07EditImage, alt: '步骤 7 截图 2' },
    ],
  },
  {
    step: 8,
    imagePosition: 'beforeTitle',
    images: [{ src: step08Image, alt: '步骤 8 截图 1' }],
    title: '保存配置后，重启 Codex，即可使用！',
  },
]

const imageEditRequestExample = `# JSON 方式：适合已经有公网图片链接
curl https://api.aaccx.pw/v1/images/edits \\
  -H "Authorization: Bearer sk-xxxx" \\
  -H "Content-Type: application/json" \\
  -d '{
    "model": "gpt-image-2",
    "prompt": "把这张图改成黑白极简海报风格，保留主体轮廓",
    "images": [
      {
        "image_url": "https://example.com/input.png"
      }
    ],
    "size": "1024x1024",
    "response_format": "b64_json"
  }'

# multipart 方式：适合直接上传本地图片
curl https://api.aaccx.pw/v1/images/edits \\
  -H "Authorization: Bearer sk-xxxx" \\
  -F "model=gpt-image-2" \\
  -F "prompt=把这张图改成黑白极简海报风格，保留主体轮廓" \\
  -F "image=@/absolute/path/input.png" \\
  -F "size=1024x1024" \\
  -F "response_format=b64_json"`

const formalAPIEndpointRows: GuideEndpointRow[] = [
  {
    label: 'Responses API',
    method: 'POST',
    url: 'https://api.aaccx.pw/v1/responses',
    meaning: 'Codex 推荐走这个接口，正常对话、工具调用、流式输出都优先用它。',
  },
  {
    label: 'Responses 子路径',
    method: 'POST',
    url: 'https://api.aaccx.pw/v1/responses/*',
    meaning: '只有工具明确需要 responses 的子路径时才用，比如 compact 这类路径。',
  },
  {
    label: 'Chat Completions',
    method: 'POST',
    url: 'https://api.aaccx.pw/v1/chat/completions',
    meaning: '老版 OpenAI 兼容客户端常用这个，对话消息按 chat completions 格式传。',
  },
  {
    label: 'Embeddings',
    method: 'POST',
    url: 'https://api.aaccx.pw/v1/embeddings',
    meaning: '把文本转成向量，用在搜索、相似度匹配、知识库召回这类场景。',
  },
  {
    label: 'Image Generation',
    method: 'POST',
    url: 'https://api.aaccx.pw/v1/images/generations',
    meaning: '文字生成图片，提示词描述你想要的画面。',
  },
  {
    label: 'Image Edit',
    method: 'POST',
    url: 'https://api.aaccx.pw/v1/images/edits',
    meaning: '上传或传入图片后改图，比如局部修改、换风格、重绘。',
  },
  {
    label: 'Models',
    method: 'GET',
    url: 'https://api.aaccx.pw/v1/models',
    meaning: '查看当前 Key 能看到的模型列表。',
  },
  {
    label: 'Usage',
    method: 'GET',
    url: 'https://api.aaccx.pw/v1/usage',
    meaning: '查看当前 Key 的用量和额度信息。',
  },
  {
    label: 'Claude Messages',
    method: 'POST',
    url: 'https://api.aaccx.pw/v1/messages',
    meaning: 'Claude/Anthropic 格式客户端使用；OpenAI 分组会自动按平台处理。',
  },
  {
    label: 'Claude Count Tokens',
    method: 'POST',
    url: 'https://api.aaccx.pw/v1/messages/count_tokens',
    meaning: '给 Claude 格式请求估算 token 数，OpenAI 平台一般不需要。',
  },
]

const legacyAPIPathRows: GuideLegacyRow[] = [
  {
    oldUrl: 'https://api.aaccx.pw/responses',
    result: '400 INVALID_BASE_URL',
    useInstead: 'https://api.aaccx.pw/v1/responses',
  },
  {
    oldUrl: 'https://api.aaccx.pw/chat/completions',
    result: '400 INVALID_BASE_URL',
    useInstead: 'https://api.aaccx.pw/v1/chat/completions',
  },
  {
    oldUrl: 'https://api.aaccx.pw/embeddings',
    result: '400 INVALID_BASE_URL',
    useInstead: 'https://api.aaccx.pw/v1/embeddings',
  },
  {
    oldUrl: 'https://api.aaccx.pw/images/generations',
    result: '400 INVALID_BASE_URL',
    useInstead: 'https://api.aaccx.pw/v1/images/generations',
  },
  {
    oldUrl: 'https://api.aaccx.pw/images/edits',
    result: '400 INVALID_BASE_URL',
    useInstead: 'https://api.aaccx.pw/v1/images/edits',
  },
  {
    oldUrl: 'https://api.aaccx.pw/models',
    result: '400 INVALID_BASE_URL',
    useInstead: 'https://api.aaccx.pw/v1/models',
  },
  {
    oldUrl: 'https://api.aaccx.pw/backend-api/codex/responses',
    result: '400 INVALID_BASE_URL',
    useInstead: 'https://api.aaccx.pw/v1/responses',
  },
]

const codexFormalConfigExample = `# Codex config.toml 推荐写法
base_url = "https://api.aaccx.pw/v1"
wire_api = "responses"

# 不要把完整接口填进 base_url。比如 /responses 应该由 Codex 自己拼出来。`

const copilotLanguageModelConfigExample = `{
  "providers": [
    {
      "id": "aaccx",
      "name": "AACCX",
      "vendor": "customendpoint",
      "apiType": "responses",
      "url": "https://api.aaccx.pw/v1/responses",
      "models": [
        {
          "id": "gpt-5.5",
          "name": "GPT-5.5",
          "model": "gpt-5.5",
          "toolCalling": true,
          "supportsReasoningEffort": true,
          "reasoningEffortFormat": "openai",
          "supportedReasoningEfforts": ["minimal", "low", "medium", "high", "xhigh"],
          "zeroDataRetentionEnabled": true,
          "requestHeaders": {
            "Authorization": "Bearer sk-xxxx"
          }
        }
      ]
    }
  ]
}`

const traeSetupSteps: GuideStep[] = [
  {
    step: 1,
    title: '点击添加模型',
    images: [{ src: traeStep01Image, alt: 'Trae 接入步骤 1 截图' }],
  },
  {
    step: 2,
    title: '选择自定义配置',
    images: [{ src: traeStep02Image, alt: 'Trae 接入步骤 2 截图' }],
  },
  {
    step: 3,
    title: '填入 https://api.aaccx.pw/v1、自己的 API Key 和 gpt-5.5',
    images: [{ src: traeStep03Image, alt: 'Trae 接入步骤 3 截图' }],
  },
  {
    step: 4,
    title: '点击自定义模型中的 gpt-5.5 即可使用',
    images: [{ src: traeStep04Image, alt: 'Trae 接入步骤 4 截图' }],
  },
]

const guideTopics: GuideTopic[] = [
  {
    id: 'codex',
    title: 'Codex 接入',
    description: '从购买订阅、兑换、创建 API Key 到配置 cc-switch 的完整步骤。',
    kind: 'steps',
    steps: codexSetupSteps,
  },
  {
    id: 'formal-api',
    title: '规范使用',
    description: '只使用 /v1 开头的正式模型 API，避免裸路径被拒绝。',
    kind: 'sections',
    sections: [
      {
        title: '先记住一句话',
        paragraphs: [
          '正式 Base URL 只填 https://api.aaccx.pw/v1。工具里如果分开填写 Base URL 和接口路径，Base URL 就停在 /v1，接口路径再填 /responses、/chat/completions 这类相对路径。',
          '不要填 https://api.aaccx.pw/responses，也不要把 /v1 写两次。裸 /responses、/models、/chat/completions 这些旧写法现在会直接返回 400 INVALID_BASE_URL。',
        ],
      },
      {
        title: '正式请求路径',
        paragraphs: [
          '下面这些是现在对外推荐的规范 URL。客户端能分开填时优先填 Base URL；只有工具要求完整地址时，才照抄完整 URL。',
        ],
        endpointRows: formalAPIEndpointRows,
      },
      {
        title: '旧写法怎么改',
        paragraphs: [
          '如果你之前按裸路径调用，请按这一列迁移。服务不会再把裸路径偷偷转发到 /v1，报错就是提醒你改配置。',
        ],
        legacyRows: legacyAPIPathRows,
      },
      {
        title: 'Codex 推荐配置',
        paragraphs: [
          'Codex 里只配置 base_url 到 /v1，wire_api 使用 responses。这样 Codex 会自己请求 /v1/responses。',
          'API Key 只放在本机配置里，不要发到公开聊天、截图或文档里。',
        ],
        code: codexFormalConfigExample,
      },
    ],
  },
  {
    id: 'copilot-vscode',
    title: 'VS Code Copilot 接入',
    description: '把 VS Code Copilot 的 Custom Endpoint Provider 指向 AACCX 的 Responses API。',
    kind: 'sections',
    sections: [
      {
        title: '改哪两个文件',
        paragraphs: [
          '把 VS Code Copilot 的 Custom Endpoint Provider 指向 AACCX 的 Responses API。需要改两个 VS Code 用户配置文件，普通 Chat 和 Agent profile 都要写，避免只在一个入口生效。',
          '普通用户配置：~/Library/Application Support/Code/User/chatLanguageModels.json。',
          'Agent profile 配置：~/Library/Application Support/Code/User/profiles/builtin/agents/chatLanguageModels.json。',
          'API Key 只写在本机 VS Code 用户配置里，不要写进项目源码、公开文档、截图或聊天记录；页面示例统一使用 sk-xxxx 占位。',
        ],
      },
      {
        title: '配置要点',
        paragraphs: [
          '使用 vendor=customendpoint，apiType 必须是 responses，url 必须是 https://api.aaccx.pw/v1/responses。这样 Copilot 会走 /v1/responses，而不是 /v1/chat/completions。',
          '每个模型里显式写 requestHeaders.Authorization: Bearer sk-xxxx。这样可以避开 Copilot 运行时没有把顶层 apiKey 合并进 Authorization 请求头的问题。',
          'Agent 模式需要工具调用，所以 gpt-5.5 要保留 toolCalling: true。',
        ],
        code: copilotLanguageModelConfigExample,
      },
      {
        title: '设置思考程度',
        paragraphs: [
          '给 gpt-5.5 声明 supportsReasoningEffort: true、reasoningEffortFormat: openai，并把 supportedReasoningEfforts 设置为 minimal、low、medium、high、xhigh。',
          'xhigh 对应 Copilot 模型选择器里的 Extra High。选择 Extra High 后，请求会携带 reasoning.effort=xhigh。',
          '如果 medium 能用、xhigh 失败，并且报 previous_response_id is only supported on Responses WebSocket v2，重点检查模型配置里是否有 zeroDataRetentionEnabled: true。这个字段会让 Copilot 不再把 previous_response_id 带给普通 /v1/responses。',
        ],
      },
      {
        title: '刷新和排查',
        paragraphs: [
          '保存两个 chatLanguageModels.json 后，在 VS Code 命令面板执行 Developer: Reload Window，然后新开一个 Copilot Chat 会话。',
          '如果模型没有出现，先执行 Chat: Manage Language Models，确认 AACCX provider 和 GPT-5.5 可见。',
          '如果仍然走 /v1/chat/completions，说明 apiType 或 url 仍是旧配置；如果报 API key is required，说明 Authorization header 没有写到对应模型的 requestHeaders 里。',
        ],
      },
    ],
  },
  {
    id: 'image-generation',
    title: '生图方法',
    description: '使用现有 API Key 调用 OpenAI 兼容图生图接口，并了解图片额度扣费方式。',
    kind: 'sections',
    sections: [
      {
        title: '可用范围',
        paragraphs: [
          '29/39/59/79/99 元套餐已支持生图和图生图，使用你已经生成的 API Key 即可直接请求图片接口。',
          '客户端 Base URL 填 https://api.aaccx.pw/v1，不需要更换新的服务端点；分开填写接口路径时不要再追加一次 /v1。',
        ],
      },
      {
        title: '接口与扣费',
        paragraphs: [
          '图生图编辑接口：如果客户端把 Base URL 和路径分开填写，接口路径填 /images/edits；如果工具要求完整 URL，使用 https://api.aaccx.pw/v1/images/edits。模型填写 gpt-image-2。',
          'JSON 请求可传 images[].image_url，上传本地文件时改用 multipart 的 image=@...；需要局部修改时可以再加 mask。',
          '图片按实际分辨率消耗订阅日额度，余额和用量记录会按图片价格统计。',
        ],
        priceRows: [
          { label: '1K 图片', price: '$0.10 / 张' },
          { label: '2K 图片', price: '$0.20 / 张' },
          { label: '4K 图片', price: '$0.40 / 张' },
        ],
      },
      {
        title: '请求示例',
        paragraphs: [
          '把示例中的 sk-xxxx 换成你自己的 API Key。不要在公开文档、聊天或截图里展示完整密钥。',
          '如果你只是想从文字直接生成图片，文本生图完整 URL 是 https://api.aaccx.pw/v1/images/generations；在分离填写的工具里接口路径填 /images/generations。',
        ],
        code: imageEditRequestExample,
      },
    ],
  },
  {
    id: 'trae',
    title: 'Trae 接入',
    description: '把这里生成的 API Key 配置到 Trae 自定义模型中使用。',
    kind: 'steps',
    steps: traeSetupSteps,
  },
]

const activeTopicId = ref(guideTopics[0].id)

const activeTopic = computed(() => (
  guideTopics.find((topic) => topic.id === activeTopicId.value) ?? guideTopics[0]
))
</script>

<style scoped>
.usage-guide-shell {
  display: grid;
  gap: 1rem;
  max-width: 80rem;
  margin: 0 auto;
}

.usage-guide-side-nav {
  display: none;
}

.usage-guide-main {
  min-width: 0;
}

.usage-guide-mobile-tabs {
  display: flex;
  gap: 0.5rem;
  overflow-x: auto;
  padding-bottom: 0.75rem;
}

.usage-guide-mobile-tab,
.usage-guide-nav-item {
  border: 1px solid rgb(229 231 235);
  background: rgb(255 255 255);
  color: rgb(55 65 81);
  text-align: left;
  transition:
    background 0.18s ease,
    border-color 0.18s ease,
    color 0.18s ease;
}

.dark .usage-guide-mobile-tab,
.dark .usage-guide-nav-item {
  border-color: rgb(55 65 81);
  background: rgb(17 24 39);
  color: rgb(209 213 219);
}

.usage-guide-mobile-tab {
  flex: 0 0 auto;
  border-radius: 0.5rem;
  padding: 0.55rem 0.8rem;
  font-size: 0.875rem;
  font-weight: 700;
}

.usage-guide-mobile-tab-active,
.usage-guide-nav-item-active {
  border-color: rgb(17 24 39);
  background: rgb(17 24 39);
  color: rgb(255 255 255);
}

.dark .usage-guide-mobile-tab-active,
.dark .usage-guide-nav-item-active {
  border-color: rgb(243 244 246);
  background: rgb(243 244 246);
  color: rgb(17 24 39);
}

.usage-guide-header {
  margin-bottom: 1rem;
  border: 1px solid rgb(229 231 235);
  border-radius: 0.5rem;
  background: rgb(255 255 255);
  padding: 1rem;
}

.dark .usage-guide-header {
  border-color: rgb(55 65 81);
  background: rgb(17 24 39);
}

.usage-guide-kicker {
  color: rgb(107 114 128);
  font-size: 0.75rem;
  font-weight: 800;
}

.dark .usage-guide-kicker {
  color: rgb(156 163 175);
}

.usage-guide-heading {
  margin-top: 0.25rem;
  color: rgb(17 24 39);
  font-size: 1.35rem;
  font-weight: 800;
  line-height: 1.25;
}

.dark .usage-guide-heading {
  color: rgb(243 244 246);
}

.usage-guide-description {
  margin-top: 0.5rem;
  color: rgb(75 85 99);
  font-size: 0.9rem;
  line-height: 1.65;
}

.dark .usage-guide-description {
  color: rgb(209 213 219);
}

.usage-guide-steps,
.usage-guide-sections {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.guide-step,
.usage-guide-section-card {
  display: flex;
  flex-direction: column;
  gap: 1rem;
  border: 1px solid rgb(229 231 235);
  border-radius: 0.5rem;
  background: rgb(255 255 255);
  padding: 1rem;
  box-shadow: 0 1px 2px rgb(15 23 42 / 0.05);
}

.dark .guide-step,
.dark .usage-guide-section-card {
  border-color: rgb(55 65 81);
  background: rgb(17 24 39);
}

.guide-heading {
  display: flex;
  flex-direction: column;
  gap: 0.35rem;
}

.guide-kicker {
  color: rgb(107 114 128);
  font-size: 0.75rem;
  font-weight: 700;
  line-height: 1rem;
}

.dark .guide-kicker {
  color: rgb(156 163 175);
}

.guide-title,
.usage-guide-section-title {
  color: rgb(17 24 39);
  font-size: 1rem;
  font-weight: 700;
  line-height: 1.6;
}

.dark .guide-title,
.dark .usage-guide-section-title {
  color: rgb(243 244 246);
}

.usage-guide-section-text {
  color: rgb(75 85 99);
  font-size: 0.92rem;
  line-height: 1.75;
}

.dark .usage-guide-section-text {
  color: rgb(209 213 219);
}

.usage-guide-price-grid {
  display: grid;
  gap: 0.5rem;
}

.usage-guide-price-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  border: 1px solid rgb(229 231 235);
  border-radius: 0.5rem;
  padding: 0.75rem;
  color: rgb(55 65 81);
  font-size: 0.9rem;
}

.dark .usage-guide-price-row {
  border-color: rgb(55 65 81);
  color: rgb(209 213 219);
}

.usage-guide-price-row strong {
  color: rgb(17 24 39);
  white-space: nowrap;
}

.dark .usage-guide-price-row strong {
  color: rgb(243 244 246);
}

.usage-guide-table-wrap {
  overflow-x: auto;
  border: 1px solid rgb(229 231 235);
  border-radius: 0.5rem;
}

.dark .usage-guide-table-wrap {
  border-color: rgb(55 65 81);
}

.usage-guide-endpoint-table {
  width: 100%;
  min-width: 44rem;
  border-collapse: collapse;
  color: rgb(55 65 81);
  font-size: 0.86rem;
  line-height: 1.55;
}

.dark .usage-guide-endpoint-table {
  color: rgb(209 213 219);
}

.usage-guide-endpoint-table th,
.usage-guide-endpoint-table td {
  border-bottom: 1px solid rgb(229 231 235);
  padding: 0.75rem;
  text-align: left;
  vertical-align: top;
}

.dark .usage-guide-endpoint-table th,
.dark .usage-guide-endpoint-table td {
  border-bottom-color: rgb(55 65 81);
}

.usage-guide-endpoint-table tr:last-child td {
  border-bottom: 0;
}

.usage-guide-endpoint-table th {
  background: rgb(249 250 251);
  color: rgb(17 24 39);
  font-weight: 800;
  white-space: nowrap;
}

.dark .usage-guide-endpoint-table th {
  background: rgb(3 7 18);
  color: rgb(243 244 246);
}

.usage-guide-endpoint-table code {
  color: rgb(17 24 39);
  font-family:
    ui-monospace,
    SFMono-Regular,
    Menlo,
    Monaco,
    Consolas,
    "Liberation Mono",
    "Courier New",
    monospace;
  font-size: 0.82rem;
  overflow-wrap: anywhere;
}

.dark .usage-guide-endpoint-table code {
  color: rgb(243 244 246);
}

.usage-guide-code {
  overflow-x: auto;
  border: 1px solid rgb(229 231 235);
  border-radius: 0.5rem;
  background: rgb(17 24 39);
  color: rgb(243 244 246);
  padding: 1rem;
  font-size: 0.82rem;
  line-height: 1.7;
}

.guide-images {
  display: grid;
  gap: 1rem;
}

.guide-images-before {
  margin-bottom: 0.25rem;
}

.guide-image {
  display: block;
  width: 100%;
  max-width: 100%;
  height: auto;
  border: 1px solid rgb(229 231 235);
  border-radius: 0.5rem;
  background: rgb(249 250 251);
}

.dark .guide-image {
  border-color: rgb(55 65 81);
  background: rgb(3 7 18);
}

@media (min-width: 768px) {
  .guide-step,
  .usage-guide-section-card,
  .usage-guide-header {
    padding: 1.25rem;
  }

  .guide-title,
  .usage-guide-section-title {
    font-size: 1.125rem;
  }
}

@media (min-width: 1024px) {
  .usage-guide-shell {
    grid-template-columns: 16rem minmax(0, 1fr);
    align-items: start;
  }

  .usage-guide-side-nav {
    position: sticky;
    top: 5.5rem;
    display: flex;
    flex-direction: column;
    gap: 0.75rem;
  }

  .usage-guide-mobile-tabs {
    display: none;
  }

  .usage-guide-nav-item {
    display: flex;
    flex-direction: column;
    gap: 0.35rem;
    border-radius: 0.5rem;
    padding: 0.9rem;
  }

  .usage-guide-nav-title {
    font-size: 0.95rem;
    font-weight: 800;
  }

  .usage-guide-nav-desc {
    font-size: 0.78rem;
    line-height: 1.5;
    opacity: 0.78;
  }
}
</style>

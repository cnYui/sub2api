<template>
  <AppLayout>
    <section class="usage-guide-shell" aria-labelledby="usage-guide-heading">
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
          :aria-pressed="topic.id === activeTopicId"
          @click="activeTopicId = topic.id"
        >
          <span class="usage-guide-nav-title">{{ topic.title }}</span>
          <time class="usage-guide-nav-date" :datetime="topic.updatedAt">更新于 {{ topic.updatedAt }}</time>
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
            :tabindex="topic.id === activeTopicId ? 0 : -1"
            @click="activeTopicId = topic.id"
          >
            <span>{{ topic.title }}</span>
            <time :datetime="topic.updatedAt">{{ topic.updatedAt }}</time>
          </button>
        </div>

        <header class="usage-guide-header">
          <span class="usage-guide-kicker">使用方法</span>
          <h2 id="usage-guide-heading" class="usage-guide-heading">{{ activeTopic.title }}</h2>
          <time class="usage-guide-date" :datetime="activeTopic.updatedAt">更新于 {{ activeTopic.updatedAt }}</time>
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

        <section v-else-if="activeTopic.kind === 'video'" class="usage-guide-video-section">
          <h3 class="usage-guide-video-title">{{ activeTopic.video.title }}</h3>
          <video
            data-test="usage-guide-video"
            class="usage-guide-video"
            :src="activeTopic.video.src"
            :poster="activeTopic.video.poster"
            controls
            playsinline
            preload="metadata"
          >
            当前浏览器无法播放视频，请
            <a :href="activeTopic.video.src">打开视频文件</a>。
          </video>
        </section>

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
                    <th>兼容入口</th>
                    <th>当前行为</th>
                    <th>建议配置</th>
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

            <div v-if="section.errorRows" class="usage-guide-table-wrap">
              <table class="usage-guide-endpoint-table usage-guide-error-table">
                <thead>
                  <tr>
                    <th>协议 / 场景</th>
                    <th>英文代码</th>
                    <th>HTTP / 事件</th>
                    <th>当前含义</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="row in section.errorRows" :key="row.id">
                    <td><code>{{ row.id }}</code></td>
                    <td><code>{{ row.code }}</code></td>
                    <td>{{ row.http }}</td>
                    <td>{{ row.message }}</td>
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
import traeStep01Image from '@/assets/usage-guide/trae-step-01-add-model.png'
import traeStep02Image from '@/assets/usage-guide/trae-step-02-custom-config.png'
import traeStep03Image from '@/assets/usage-guide/trae-step-03-fill-url-key.png'
import traeStep04Image from '@/assets/usage-guide/trae-step-04-select-model.png'
import claudeCodeStep01Image from '@/assets/usage-guide/claude-code-step-01-select-group.png'
import claudeCodeStep02Image from '@/assets/usage-guide/claude-code-step-02-ccswitch-route.png'
import claudeCodeStep03Image from '@/assets/usage-guide/claude-code-step-03-provider-config.png'
import claudeCodeStep04Image from '@/assets/usage-guide/claude-code-step-04-model-select.png'
import claudeCodeStep05Image from '@/assets/usage-guide/claude-code-step-05-enable-route.png'
import claudeCodeStep06Image from '@/assets/usage-guide/claude-code-step-06-restart-desktop.png'
import codexCCSwitchStep01Image from '@/assets/usage-guide/codex-ccswitch-step-01.png'
import codexCCSwitchStep02Image from '@/assets/usage-guide/codex-ccswitch-step-02.png'
import codexCCSwitchStep03Image from '@/assets/usage-guide/codex-ccswitch-step-03.png'
import codexCCSwitchStep04Image from '@/assets/usage-guide/codex-ccswitch-step-04.png'
import codexCCSwitchStep05Image from '@/assets/usage-guide/codex-ccswitch-step-05.png'
import codexCCSwitchStep06Image from '@/assets/usage-guide/codex-ccswitch-step-06.png'
import codexCCSwitchStep07Image from '@/assets/usage-guide/codex-ccswitch-step-07.png'
import codexCCSwitchStep08Image from '@/assets/usage-guide/codex-ccswitch-step-08.png'
import codexCCSwitchStep09Image from '@/assets/usage-guide/codex-ccswitch-step-09.png'
import codexCCSwitchStep10Image from '@/assets/usage-guide/codex-ccswitch-step-10.png'

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

type GuideErrorRow = {
  id: string
  code: string
  http: string
  message: string
}

type GuideSection = {
  title: string
  paragraphs: string[]
  endpointRows?: GuideEndpointRow[]
  legacyRows?: GuideLegacyRow[]
  errorRows?: GuideErrorRow[]
  code?: string
}

type GuideTopic =
  | {
    id: string
    title: string
    updatedAt: string
    description: string
    kind: 'steps'
    steps: GuideStep[]
  }
  | {
    id: string
    title: string
    updatedAt: string
    description: string
    kind: 'sections'
    sections: GuideSection[]
  }
  | {
    id: string
    title: string
    updatedAt: string
    description: string
    kind: 'video'
    video: {
      title: string
      src: string
      poster: string
    }
  }

const codexSetupSteps: GuideStep[] = [
  {
    step: 1,
    title: '在网站创建包含 KIMI 分组的 API Key',
    images: [{ src: codexCCSwitchStep01Image, alt: 'Codex 接入步骤 1：创建包含 KIMI 分组的密钥' }],
  },
  {
    step: 2,
    title: '打开 CC Switch，点击 Codex 图标和 ChatGPT 图标，再点击右上角加号新建凭证',
    images: [{ src: codexCCSwitchStep02Image, alt: 'Codex 接入步骤 2：在 CC Switch 打开 Codex 和 ChatGPT 并新建凭证' }],
  },
  {
    step: 3,
    title: '填写供应商名称、API Key 和 API 请求地址',
    images: [{ src: codexCCSwitchStep03Image, alt: 'Codex 接入步骤 3：填写 API Key 和 API 请求地址' }],
  },
  {
    step: 4,
    title: '确认 API Key 和 API 请求地址已填写完成',
    images: [{ src: codexCCSwitchStep04Image, alt: 'Codex 接入步骤 4：确认 API Key 和 API 请求地址已填写' }],
  },
  {
    step: 5,
    title: '翻到页面下部，打开“高级选项”',
    images: [{ src: codexCCSwitchStep05Image, alt: 'Codex 接入步骤 5：打开高级选项' }],
  },
  {
    step: 6,
    title: '点击“获取模型列表”，查看获取到的对应模型',
    images: [{ src: codexCCSwitchStep06Image, alt: 'Codex 接入步骤 6：获取模型列表' }],
  },
  {
    step: 7,
    title: '点击“添加”，在下拉列表中选择获取到的模型进行模型映射',
    images: [{ src: codexCCSwitchStep07Image, alt: 'Codex 接入步骤 7：添加模型并选择模型映射' }],
  },
  {
    step: 8,
    title: '确认模型映射成功，菜单显示名与实际请求模型均已填好',
    images: [{ src: codexCCSwitchStep08Image, alt: 'Codex 接入步骤 8：模型映射成功' }],
  },
  {
    step: 9,
    title: '点击“保存”完成供应商凭证配置',
    images: [{ src: codexCCSwitchStep09Image, alt: 'Codex 接入步骤 9：保存供应商凭证配置' }],
  },
  {
    step: 10,
    title: '重启 Codex，底部模型选择变成自定义模型即表示接入成功',
    images: [{ src: codexCCSwitchStep10Image, alt: 'Codex 接入步骤 10：重启 Codex 后选择自定义模型' }],
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
    meaning: 'OpenAI/Codex 的首选接口，支持普通对话、工具调用和流式输出。',
  },
  {
    label: 'Responses compact',
    method: 'POST',
    url: 'https://api.aaccx.pw/v1/responses/compact',
    meaning: '仅供明确要求 compact 的 Codex 客户端使用，普通请求不要自行拼接子路径。',
  },
  {
    label: 'Chat Completions',
    method: 'POST',
    url: 'https://api.aaccx.pw/v1/chat/completions',
    meaning: '兼容仍使用 chat.completions 格式的 OpenAI 客户端。',
  },
  {
    label: 'Claude Messages',
    method: 'POST',
    url: 'https://api.aaccx.pw/v1/messages',
    meaning: 'Claude Code、Anthropic SDK 等消息格式客户端使用；服务端按分组平台调度。',
  },
  {
    label: 'Count Tokens',
    method: 'POST',
    url: 'https://api.aaccx.pw/v1/messages/count_tokens',
    meaning: '估算 Claude 消息请求的输入 token；OpenAI 客户端通常不需要调用。',
  },
  {
    label: 'Embeddings',
    method: 'POST',
    url: 'https://api.aaccx.pw/v1/embeddings',
    meaning: 'OpenAI 分组的文本向量接口，用于搜索、相似度匹配和知识库召回。',
  },
  {
    label: 'Image Generation',
    method: 'POST',
    url: 'https://api.aaccx.pw/v1/images/generations',
    meaning: '文字生成图片；是否可用以当前 API Key 的模型列表和权限为准。',
  },
  {
    label: 'Image Edit',
    method: 'POST',
    url: 'https://api.aaccx.pw/v1/images/edits',
    meaning: '上传或传入图片后改图；不要把图片接口配置到文本折扣分组。',
  },
  {
    label: 'Models',
    method: 'GET',
    url: 'https://api.aaccx.pw/v1/models',
    meaning: '查看当前 API Key 实际可见的模型；以它作为客户端模型选择的准确信息。',
  },
  {
    label: 'Usage',
    method: 'GET',
    url: 'https://api.aaccx.pw/v1/usage',
    meaning: '查看当前 API Key 的用量和额度信息。',
  },
  {
    label: 'Alpha Search',
    method: 'POST',
    url: 'https://api.aaccx.pw/v1/alpha/search',
    meaning: '仅供支持该能力的 OpenAI 分组使用，不是普通聊天接口的替代品。',
  },
  {
    label: 'Grok Videos',
    method: 'POST/GET',
    url: 'https://api.aaccx.pw/v1/videos/*',
    meaning: 'Grok 视频生成、编辑和查询；只有视频分组与模型支持时才可用。',
  },
]

const legacyAPIPathRows: GuideLegacyRow[] = [
  {
    oldUrl: 'https://api.aaccx.pw/responses',
    result: '兼容别名，当前仍可转发到 Responses。',
    useInstead: 'https://api.aaccx.pw/v1/responses',
  },
  {
    oldUrl: 'https://api.aaccx.pw/chat/completions',
    result: '兼容别名，当前仍可转发到 Chat Completions。',
    useInstead: 'https://api.aaccx.pw/v1/chat/completions',
  },
  {
    oldUrl: 'https://api.aaccx.pw/embeddings',
    result: '兼容别名，当前仍可用，但只接受支持 Embeddings 的 OpenAI 分组。',
    useInstead: 'https://api.aaccx.pw/v1/embeddings',
  },
  {
    oldUrl: 'https://api.aaccx.pw/images/generations',
    result: '兼容别名，当前仍可用；新客户端不要依赖无版本路径。',
    useInstead: 'https://api.aaccx.pw/v1/images/generations',
  },
  {
    oldUrl: 'https://api.aaccx.pw/images/edits',
    result: '兼容别名，当前仍可用；新客户端不要依赖无版本路径。',
    useInstead: 'https://api.aaccx.pw/v1/images/edits',
  },
  {
    oldUrl: 'https://api.aaccx.pw/models',
    result: '兼容别名，当前仍可返回模型列表。',
    useInstead: 'https://api.aaccx.pw/v1/models',
  },
  {
    oldUrl: 'https://api.aaccx.pw/backend-api/codex/responses',
    result: 'Codex 直连兼容入口，只有客户端明确要求时才使用。',
    useInstead: 'https://api.aaccx.pw/v1/responses',
  },
  {
    oldUrl: 'https://api.aaccx.pw/messages',
    result: '未注册这个裸路径，不能把所有接口都去掉 /v1。',
    useInstead: 'https://api.aaccx.pw/v1/messages',
  },
  {
    oldUrl: 'https://api.aaccx.pw/usage',
    result: '未注册这个裸路径，不能把所有接口都去掉 /v1。',
    useInstead: 'https://api.aaccx.pw/v1/usage',
  },
]

const errorCatalogRows: GuideErrorRow[] = [
  { id: 'OpenAI / Claude', code: 'invalid_request_error', http: '400', message: '请求体为空、解析失败、缺少 model 或参数不符合当前接口要求。' },
  { id: 'OpenAI / Claude', code: 'rate_limit_error', http: '429', message: '请求频率、并发、图片并发或订阅窗口达到限制；存在 Retry-After 时应按它退避。' },
  { id: 'OpenAI', code: 'insufficient_quota', http: '429', message: 'API Key 额度已用完；这是 Responses 兼容入口的额度错误格式。' },
  { id: 'OpenAI / Claude', code: 'upstream_error', http: '502/503/504', message: '上游鉴权失败、拒绝、暂不可用、超时或转发失败；具体原因看 HTTP 状态和服务日志。' },
  { id: 'OpenAI', code: 'content_policy_violation', http: '400 / SSE', message: '内容审核拦截请求；流已经开始时会作为错误事件写入 SSE。' },
  { id: 'API Key', code: 'API_KEY_REQUIRED', http: '401', message: '缺少 Authorization Bearer、x-api-key 或 Gemini 使用的 x-goog-api-key。' },
  { id: 'API Key', code: 'INVALID_API_KEY', http: '401', message: 'API Key 不存在或无法通过鉴权。不要把 Key 放在 query 参数中。' },
  { id: 'API Key', code: 'api_key_in_query_deprecated', http: '400', message: 'query 中的 key/api_key 已弃用，改用请求头认证。' },
  { id: 'API Key', code: 'API_KEY_DISABLED', http: '401', message: 'API Key 已被停用。' },
  { id: 'API Key', code: 'API_KEY_EXPIRED', http: '403', message: 'API Key 已过期。' },
  { id: '权限', code: 'ACCESS_DENIED', http: '403', message: 'IP 限制、分组权限或其它访问控制拒绝了请求。' },
  { id: '权限', code: 'GROUP_NOT_ALLOWED', http: '403', message: 'API Key 所属专属分组不再允许当前用户使用。' },
  { id: '订阅', code: 'SUBSCRIPTION_NOT_FOUND', http: '403', message: '当前 Key 分组没有有效订阅。' },
  { id: '订阅', code: 'USAGE_LIMIT_EXCEEDED', http: '429', message: '订阅的 5 小时、1 天或 7 天用量窗口已达到上限。' },
  { id: '余额', code: 'INSUFFICIENT_BALANCE', http: '403', message: '普通余额不足且当前请求没有可用的 OpenAI 流量卡额度。' },
  { id: '额度', code: 'API_KEY_QUOTA_EXHAUSTED', http: '429', message: 'API Key 自身配额已用完；OpenAI Responses 可能改用 insufficient_quota 格式返回。' },
  { id: '系统', code: 'API_KEY_AUTH_OVERLOADED', http: '503', message: 'API Key 鉴权服务暂时过载，请稍后重试。' },
  { id: '系统', code: 'SUBSCRIPTION_MAINTENANCE_FAILED', http: '500', message: '订阅用量窗口维护失败，请稍后重试或联系管理员。' },
  { id: '系统', code: 'INTERNAL_ERROR', http: '500', message: '服务内部错误；不要把响应中的内部细节当作稳定契约。' },
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

const claudeCodeSetupSteps: GuideStep[] = [
  {
    step: 1,
    title: '在网站的 API Key 页面，把密钥分组切换为 Cloud 分组',
    images: [{ src: claudeCodeStep01Image, alt: 'Claude Code 桌面端接入步骤 1：切换 Cloud 分组' }],
  },
  {
    step: 2,
    title: '打开 CC Switch，点击 Cloud Desktop 所属栏目，再点击右上角的加号',
    images: [{ src: claudeCodeStep02Image, alt: 'Claude Code 桌面端接入步骤 2：在 CC Switch 添加供应商' }],
  },
  {
    step: 3,
    title: '填写 API Key 和请求地址，点击“获取模型列表”并选择模型；保存退出后打开顶部路由，重启 Claude Code 桌面端即可使用',
    images: [
      { src: claudeCodeStep03Image, alt: 'Claude Code 桌面端接入步骤 3：填写 API Key 和请求地址并获取模型列表' },
      { src: claudeCodeStep04Image, alt: 'Claude Code 桌面端接入步骤 3：选择模型映射' },
      { src: claudeCodeStep05Image, alt: 'Claude Code 桌面端接入步骤 3：打开顶部路由' },
      { src: claudeCodeStep06Image, alt: 'Claude Code 桌面端接入步骤 3：重启 Claude Code 桌面端' },
    ],
  },
]

const allGuideTopics: GuideTopic[] = [
  {
    id: 'codex',
    title: 'Codex 接入',
    updatedAt: '2026-08-09',
    description: '使用 KIMI 分组 API Key，通过 CC Switch 配置模型映射并在 Codex 中启用自定义模型。',
    kind: 'steps',
    steps: codexSetupSteps,
  },
  {
    id: 'ccswitch-video',
    title: 'CCSwitch 视频教程',
    updatedAt: '2026-07-14',
    description: '完整演示使用 CCSwitch 接入中转站，解决 99% 常见的连接不上、断连问题。',
    kind: 'video',
    video: {
      title: '使用 CCSwitch 接入中转站',
      src: '/usage-guide/ccswitch-relay-connection-guide.mp4',
      poster: '/usage-guide/ccswitch-relay-connection-guide-poster.webp',
    },
  },
  {
    id: 'formal-api',
    title: '规范使用',
    updatedAt: '2026-08-05',
    description: '按当前网关真实路由配置 Base URL、认证头、模型和额度，避免客户端接入时猜路径。',
    kind: 'sections',
    sections: [
      {
        title: '先统一三项配置',
        paragraphs: [
          'OpenAI、Claude 和大多数兼容客户端的 Base URL 填 https://api.aaccx.pw/v1；如果工具另有“接口路径”输入框，只填 /responses、/chat/completions 或 /messages 这一段，不要把完整 URL 再拼一次。',
          'Gemini 原生客户端使用 https://api.aaccx.pw/v1beta；只使用 Gemini 兼容的模型与接口，不要拿 /v1 的 OpenAI 路径替代它。',
          '请求认证首选 Authorization: Bearer sk-xxxx。服务也兼容 x-api-key；Gemini 客户端可使用 x-goog-api-key。API Key 放在本机配置，不要写入项目源码、截图或公开聊天。',
        ],
      },
      {
        title: '按客户端选择接口',
        paragraphs: [
          '下面是当前网关注册的规范路径。能配置 Base URL 的客户端优先填 /v1，再让客户端自行拼接；只能填写完整地址时，使用表格中的 URL。模型名称先从当前 API Key 的 /v1/models 读取，不要照抄别的 Key 的模型名。',
        ],
        endpointRows: formalAPIEndpointRows,
      },
      {
        title: '无 /v1 路径的真实行为',
        paragraphs: [
          '网关为 Responses、Chat Completions、Embeddings、图片、Models 和 Codex 直连保留了部分无 /v1 兼容别名，所以旧客户端不一定马上报错；这不代表所有接口都支持省略版本前缀。新配置统一使用 /v1，兼容别名只用于迁移和特殊客户端。',
        ],
        legacyRows: legacyAPIPathRows,
      },
      {
        title: 'Codex、Claude Code 和额度排查',
        paragraphs: [
          'Codex 的 base_url 只配置到 /v1，wire_api 使用 responses；不要把 /responses 或 /backend-api/codex/responses 填进 base_url。Claude Code 使用 /v1/messages，并把 API Key 放到客户端要求的认证字段。',
          '先用同一个 API Key 请求 /v1/models，确认目标模型确实属于该 Key 的分组；模型列表为空、模型不支持或没有可用上游时，换路径不会解决问题，应回到 API Key 分组和服务状态排查。',
          '扣费、订单状态和退款以服务端为准。普通余额与流量卡额度分开显示；OpenAI 请求在普通余额不足时才会按服务端规则尝试扣有效流量卡，额度不足仍会拒绝请求。',
        ],
        code: codexFormalConfigExample,
      },
    ],
  },
  {
    id: 'copilot-vscode',
    title: 'VS Code Copilot 接入',
    updatedAt: '2026-07-10',
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
    id: 'error-codes',
    title: '错误编号参考',
    updatedAt: '2026-08-05',
    description: '按当前 main 的实际响应格式排查认证、额度、协议和上游错误。',
    kind: 'sections',
    sections: [
      {
        title: '先看响应格式',
        paragraphs: [
          '当前 main 的通用 REST 错误响应通常是 {"code": HTTP 状态, "message": "...", "reason": "...", "metadata": {...}}；reason 可能为空，不能把它误当成全局统一的 S2A 编号。',
          'OpenAI Responses/Chat Completions 使用 error.type、error.message、error.code 等兼容字段；Anthropic 使用 type=error 和 error.type/error.message；Gemini 使用 Google 风格的 error.code、error.status 和 error.message。',
          '当前 main 尚未把所有端点统一迁移到 X-Sub2API-Error-ID / S2A-* 契约。排查时先保留 HTTP 状态、响应 body、Retry-After 和请求时间；不要依据旧目录自行生成 S2A 编号。',
        ],
      },
      {
        title: '当前常见代码',
        paragraphs: [
          '下面只列当前代码中会直接返回、或由网关兼容层稳定使用的常见代码。上游原始 code/message 可能透传到诊断日志或显式配置的透传规则，不应被客户端当作平台稳定编号。',
        ],
        errorRows: errorCatalogRows,
      },
    ],
  },
  {
    id: 'image-generation',
    title: '生图方法',
    updatedAt: '2026-07-07',
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
          '图片生成按上游实际返回的 Token 用量和套餐有效倍率计费；图片数量和文件大小不作为单独收费单位。',
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
    updatedAt: '2026-06-24',
    description: '把这里生成的 API Key 配置到 Trae 自定义模型中使用。',
    kind: 'steps',
    steps: traeSetupSteps,
  },
  {
    id: 'claude-code-desktop',
    title: 'Claude Code 桌面端接入',
    updatedAt: '2026-08-05',
    description: '使用 Cloud 分组和 CC Switch，把 Claude Code 桌面端连接到当前 API 服务。',
    kind: 'steps',
    steps: claudeCodeSetupSteps,
  },
]

const hiddenGuideTopicIds = new Set<GuideTopic['id']>(['image-generation'])
const guideTopics = allGuideTopics
  .filter((topic) => !hiddenGuideTopicIds.has(topic.id))
  .sort((left, right) => right.updatedAt.localeCompare(left.updatedAt))

const activeTopicId = ref(guideTopics[0].id)

const activeTopic = computed(() => (
  guideTopics.find((topic) => topic.id === activeTopicId.value) ?? guideTopics[0]
))
</script>

<style scoped>
.usage-guide-shell {
  display: grid;
  gap: 1.5rem;
  max-width: 96rem;
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
  padding: 0.125rem 0.125rem 0.875rem;
  scrollbar-width: none;
}

.usage-guide-mobile-tabs::-webkit-scrollbar {
  display: none;
}

.usage-guide-mobile-tab,
.usage-guide-nav-item {
  border: 1px solid rgb(229 231 235);
  background: rgb(255 255 255);
  color: rgb(55 65 81);
  text-align: left;
  cursor: pointer;
  transition:
    background-color 180ms ease-out,
    border-color 180ms ease-out,
    color 180ms ease-out,
    transform 160ms ease-out;
}

.dark .usage-guide-mobile-tab,
.dark .usage-guide-nav-item {
  border-color: rgb(55 65 81);
  background: rgb(17 24 39);
  color: rgb(209 213 219);
}

.usage-guide-mobile-tab {
  flex: 0 0 auto;
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 0.12rem;
  border-radius: 0.5rem;
  padding: 0.55rem 0.8rem;
  font-size: 0.875rem;
  font-weight: 700;
}

.usage-guide-mobile-tab time,
.usage-guide-nav-date,
.usage-guide-date {
  color: rgb(107 114 128);
  font-size: 0.72rem;
  font-weight: 600;
  line-height: 1.35;
}

.usage-guide-mobile-tab-active time,
.usage-guide-nav-item-active .usage-guide-nav-date {
  color: rgb(209 213 219);
}

.dark .usage-guide-mobile-tab time,
.dark .usage-guide-nav-date,
.dark .usage-guide-date {
  color: rgb(156 163 175);
}

.dark .usage-guide-mobile-tab-active time,
.dark .usage-guide-nav-item-active .usage-guide-nav-date {
  color: rgb(75 85 99);
}

.usage-guide-mobile-tab-active,
.usage-guide-nav-item-active {
  border-color: rgb(17 24 39);
  background: rgb(17 24 39);
  color: rgb(255 255 255);
  box-shadow: 0 4px 12px rgb(17 24 39 / 0.16);
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
  border-radius: 1rem;
  background: rgb(255 255 255);
  padding: 1.25rem;
  box-shadow: 0 10px 30px rgb(15 23 42 / 0.06);
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
  font-size: clamp(1.35rem, 1.6vw, 1.75rem);
  font-weight: 800;
  line-height: 1.25;
}

.usage-guide-date {
  display: block;
  margin-top: 0.35rem;
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

.usage-guide-video-section {
  width: 100%;
  max-width: 60rem;
  border: 1px solid rgb(229 231 235);
  border-radius: 1rem;
  background: rgb(255 255 255);
  padding: 1.25rem;
  box-shadow: 0 10px 30px rgb(15 23 42 / 0.06);
}

.dark .usage-guide-video-section {
  border-color: rgb(55 65 81);
  background: rgb(17 24 39);
  box-shadow: 0 10px 30px rgb(0 0 0 / 0.18);
}

.usage-guide-video-title {
  margin: 0 0 0.875rem;
  color: rgb(17 24 39);
  font-size: 1.125rem;
  font-weight: 700;
}

.dark .usage-guide-video-title {
  color: rgb(248 250 252);
}

.usage-guide-video {
  display: block;
  width: 100%;
  aspect-ratio: 16 / 9;
  border-radius: 0.75rem;
  background: rgb(0 0 0);
  object-fit: contain;
}

.guide-step,
.usage-guide-section-card {
  display: flex;
  flex-direction: column;
  gap: 1rem;
  border: 1px solid rgb(229 231 235);
  border-radius: 1rem;
  background: rgb(255 255 255);
  padding: 1.25rem;
  box-shadow: 0 10px 30px rgb(15 23 42 / 0.05);
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

.usage-guide-table-wrap {
  overflow-x: auto;
  border: 1px solid rgb(229 231 235);
  border-radius: 0.75rem;
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

.usage-guide-error-table {
  min-width: 58rem;
}

.usage-guide-endpoint-table th,
.usage-guide-endpoint-table td {
  border-bottom: 1px solid rgb(229 231 235);
  padding: 0.75rem;
  text-align: left;
  vertical-align: top;
}

.usage-guide-endpoint-table tbody tr {
  transition: background-color 150ms ease-out;
}

.usage-guide-mobile-tab:focus-visible,
.usage-guide-nav-item:focus-visible {
  outline: 2px solid rgb(59 130 246);
  outline-offset: 2px;
}

@media (hover: hover) and (pointer: fine) {
  .usage-guide-mobile-tab:hover,
  .usage-guide-nav-item:hover {
    border-color: rgb(156 163 175);
    transform: translateY(-1px);
  }

  .usage-guide-mobile-tab:active,
  .usage-guide-nav-item:active {
    transform: scale(0.98);
  }

  .usage-guide-endpoint-table tbody tr:hover {
    background: rgb(249 250 251);
  }

  .dark .usage-guide-endpoint-table tbody tr:hover {
    background: rgb(31 41 55 / 0.55);
  }
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
  border-radius: 0.75rem;
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
  border-radius: 0.75rem;
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
    top: 5rem;
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
    border-radius: 0.75rem;
    padding: 1rem;
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

@media (prefers-reduced-motion: reduce) {
  .usage-guide-mobile-tab,
  .usage-guide-nav-item,
  .usage-guide-endpoint-table tbody tr {
    transition: none;
  }
}
</style>

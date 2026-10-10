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
import workbuddyStep01Image from '@/assets/usage-guide/workbuddy-step-01-add-custom-model.png'
import workbuddyStep02Image from '@/assets/usage-guide/workbuddy-step-02-select-custom-provider.png'
import workbuddyStep03Image from '@/assets/usage-guide/workbuddy-step-03-fill-custom-model.png'
import workbuddyStep04Image from '@/assets/usage-guide/workbuddy-step-04-start-chat.png'
import claudeDesktopStep01Image from '@/assets/usage-guide/claude-desktop-step-01-select-and-add.png'
import claudeDesktopStep02Image from '@/assets/usage-guide/claude-desktop-step-02-create-key.png'
import claudeDesktopStep03Image from '@/assets/usage-guide/claude-desktop-step-03-provider-fields.png'
import claudeDesktopStep04Image from '@/assets/usage-guide/claude-desktop-step-04-provider-result.png'
import claudeDesktopStep05Image from '@/assets/usage-guide/claude-desktop-step-05-enable-and-restart.png'
import claudeDesktopStep06Image from '@/assets/usage-guide/claude-desktop-step-06-quit-menu.png'
import claudeDesktopStep07Image from '@/assets/usage-guide/claude-desktop-step-07-select-model.png'
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
import codexGptStep01Image from '@/assets/usage-guide/codex-gpt-step-01-create-key.png'
import codexGptStep02Image from '@/assets/usage-guide/codex-gpt-step-02-select-gpt.png'
import codexGptStep03Image from '@/assets/usage-guide/codex-gpt-step-03-provider-config.png'
import codexGptStep04Image from '@/assets/usage-guide/codex-gpt-step-04-enable-restart.png'
import deepseekHarnessStep01Image from '@/assets/usage-guide/deepseek-harness-step-01-clone-repo.png'
import deepseekHarnessStep02Image from '@/assets/usage-guide/deepseek-harness-step-02-local-start.png'
import deepseekHarnessStep03Image from '@/assets/usage-guide/deepseek-harness-step-03-open-settings.png'
import deepseekHarnessStep04Image from '@/assets/usage-guide/deepseek-harness-step-04-add-custom-provider.png'
import deepseekHarnessStep05Image from '@/assets/usage-guide/deepseek-harness-step-05-fill-url-key.png'

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

const codexGptSetupSteps: GuideStep[] = [
  {
    step: 1,
    title: '选择 GPT 分组，然后点击右上角橙色的加号',
    images: [
      { src: codexGptStep01Image, alt: 'Codex GPT 接入步骤 1：创建 GPT 分组密钥' },
      { src: codexGptStep02Image, alt: 'Codex GPT 接入步骤 1：选择 GPT 栏目并点击加号' },
    ],
  },
  {
    step: 2,
    title: '按图中箭头正确填写供应商名称、API Key 和请求地址，然后保存；请求地址填写 https://api.aaccx.pw 即可',
    images: [{ src: codexGptStep03Image, alt: 'Codex GPT 接入步骤 2：填写配置并保存' }],
  },
  {
    step: 3,
    title: '点击启用后，重启 Codex 即可使用',
    images: [{ src: codexGptStep04Image, alt: 'Codex GPT 接入步骤 3：启用供应商并重启 Codex' }],
  },
]

const imageEndpointRows: GuideEndpointRow[] = [
  {
    label: '文字生图',
    method: 'POST',
    url: 'https://api.aaccx.pw/v1/images/generations',
    meaning: '只用文字描述生成新图片。',
  },
  {
    label: '图生图',
    method: 'POST',
    url: 'https://api.aaccx.pw/v1/images/edits',
    meaning: '传入参考图，按描述修改它；可以再传 mask 只改局部。',
  },
  {
    label: '可用模型',
    method: 'GET',
    url: 'https://api.aaccx.pw/v1/models',
    meaning: '查看当前 Key 实际能用的模型，用来确认选对了分组。',
  },
]

const imageGenerateCurlExample = `curl https://api.aaccx.pw/v1/images/generations \\
  -H "Authorization: Bearer sk-xxxx" \\
  -H "Content-Type: application/json" \\
  -d '{
    "model": "gpt-image-2",
    "prompt": "一只戴着宇航员头盔的橘猫，扁平插画风格",
    "size": "1024x1024"
  }' \\
  -o result.json

# 需要已安装 jq：把返回里的 base64 解码成图片文件
jq -r '.data[0].b64_json' result.json | base64 -d > cat.png`

const imageGeneratePythonExample = `import base64
import requests

API_KEY = "sk-xxxx"  # 换成「GPT生图1倍率」分组的 Key，不要写进公开仓库

resp = requests.post(
    "https://api.aaccx.pw/v1/images/generations",
    headers={"Authorization": f"Bearer {API_KEY}"},
    json={
        "model": "gpt-image-2",
        "prompt": "一只戴着宇航员头盔的橘猫，扁平插画风格",
        "size": "1024x1024",
    },
    timeout=300,
)
if resp.status_code != 200:
    raise SystemExit(f"请求失败 {resp.status_code}: {resp.text}")

item = resp.json()["data"][0]
if item.get("b64_json"):
    with open("cat.png", "wb") as f:
        f.write(base64.b64decode(item["b64_json"]))
    print("已保存 cat.png")
else:
    print("图片链接：", item["url"])`

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
    "size": "1024x1024"
  }'

# multipart 方式：适合直接上传本地图片
curl https://api.aaccx.pw/v1/images/edits \\
  -H "Authorization: Bearer sk-xxxx" \\
  -F "model=gpt-image-2" \\
  -F "prompt=把这张图改成黑白极简海报风格，保留主体轮廓" \\
  -F "image=@/absolute/path/input.png" \\
  -F "size=1024x1024"`

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
  { id: '余额', code: 'INSUFFICIENT_BALANCE', http: '403', message: '普通余额不足且当前请求没有可用的流量卡额度。' },
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

const workbuddySetupSteps: GuideStep[] = [
  {
    step: 1,
    title: '下载并打开 WorkBuddy，在右下角模型选择器中点击“添加自定义模型”',
    images: [{ src: workbuddyStep01Image, alt: 'WorkBuddy 接入步骤 1：点击添加自定义模型' }],
  },
  {
    step: 2,
    title: '在添加模型窗口打开服务商下拉框，滚动到列表最底部，在“Other”下选择“Custom”',
    images: [{ src: workbuddyStep02Image, alt: 'WorkBuddy 接入步骤 2：选择 Custom 服务商' }],
  },
  {
    step: 3,
    title: '填写 Endpoint https://api.aaccx.pw/v1、自己的 API Key 和准确的模型名称；模型名称必须与该 API Key 的 /v1/models 返回值一致（示例：gpt-5.5）',
    images: [{ src: workbuddyStep03Image, alt: 'WorkBuddy 接入步骤 3：填写 Endpoint、API Key 和模型名称' }],
  },
  {
    step: 4,
    title: '保存后选择自定义模型开始对话；能正常收到回复即表示 WorkBuddy 已连通',
    images: [{ src: workbuddyStep04Image, alt: 'WorkBuddy 接入步骤 4：选择模型并开始对话' }],
  },
]

const claudeDesktopSetupSteps: GuideStep[] = [
  {
    step: 1,
    title: '点击 Cloud Desktop，然后点击右上角的加号',
    images: [{ src: claudeDesktopStep01Image, alt: 'Claude Desktop 接入步骤 1：点击 Cloud Desktop 和加号' }],
  },
  {
    step: 2,
    title: '创建密钥并按图中箭头填写 API Key；请求地址填写 https://api.aaccx.pw，不要在末尾添加斜杠',
    images: [
      { src: claudeDesktopStep02Image, alt: 'Claude Desktop 接入步骤 2：创建 Claude 渠道密钥' },
      { src: claudeDesktopStep03Image, alt: 'Claude Desktop 接入步骤 2：填写 API Key 和请求地址' },
      { src: claudeDesktopStep04Image, alt: 'Claude Desktop 接入步骤 2：确认 Claude 渠道配置结果' },
    ],
  },
  {
    step: 3,
    title: '点击“添加”并启用该供应商，然后重新启动 Claude Desktop',
    images: [
      { src: claudeDesktopStep05Image, alt: 'Claude Desktop 接入步骤 3：添加并启用供应商' },
    ],
  },
  {
    step: 4,
    title: '选择模型，然后愉快地开始聊天吧！',
    images: [
      { src: claudeDesktopStep06Image, alt: 'Claude Desktop 接入步骤 4：退出并重新启动桌面端' },
      { src: claudeDesktopStep07Image, alt: 'Claude Desktop 接入步骤 4：选择 Claude 模型并开始聊天' },
    ],
  },
]

const deepseekHarnessSetupSteps: GuideStep[] = [
  {
    step: 1,
    title: '打开 DeepSeek 官方 Harness 仓库（github.com/deepseek-ai/deepseek-harness），克隆它的源代码到本地',
    images: [{ src: deepseekHarnessStep01Image, alt: 'DeepSeek Harness 接入步骤 1：克隆官方 Harness 源代码' }],
  },
  {
    step: 2,
    title: '在本地找一个 AI 编程工具，按仓库说明本地启动 DeepSeek Harness',
    images: [{ src: deepseekHarnessStep02Image, alt: 'DeepSeek Harness 接入步骤 2：让本地 AI 启动 DeepSeek Harness' }],
  },
  {
    step: 3,
    title: '启动后进入 DeepSeek Harness 主页，点击左下角的“设置”',
    images: [{ src: deepseekHarnessStep03Image, alt: 'DeepSeek Harness 接入步骤 3：进入主页并点击设置' }],
  },
  {
    step: 4,
    title: '在设置的“模型”页选择自定义模型，点击“添加自定义提供方”',
    images: [{ src: deepseekHarnessStep04Image, alt: 'DeepSeek Harness 接入步骤 4：添加自定义提供方' }],
  },
  {
    step: 5,
    title: '从本站复制并填入 API 地址与 API 密钥：API 地址填写 https://api.aaccx.pw/v1，API 协议保持 openai-completions，然后点“创建提供方”',
    images: [{ src: deepseekHarnessStep05Image, alt: 'DeepSeek Harness 接入步骤 5：填写 API 地址、API 协议和 API 密钥' }],
  },
]

const guideTopics: GuideTopic[] = [
  {
    id: 'deepseek-harness',
    title: 'DeepSeek Harness 接入中转站 DeepSeek 模型',
    updatedAt: '2026-09-10',
    description: '克隆 DeepSeek 官方 Harness 源码并本地启动，在设置里添加自定义提供方，接入中转站的 DeepSeek 模型。',
    kind: 'steps',
    steps: deepseekHarnessSetupSteps,
  },
  {
    id: 'codex',
    title: 'Codex 接入中转站除GPT模型以外的外部模型',
    updatedAt: '2026-08-09',
    description: '使用 KIMI 分组 API Key，通过 CC Switch 配置模型映射并在 Codex 中启用自定义模型。',
    kind: 'steps',
    steps: codexSetupSteps,
  },
  {
    id: 'codex-gpt',
    title: 'Codex 接入 GPT 模型',
    updatedAt: '2026-08-11',
    description: '通过 CC Switch 配置 GPT 分组 API Key，在 Codex 中启用中转站 GPT 模型。',
    kind: 'steps',
    steps: codexGptSetupSteps,
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
          '扣费、订单状态和退款以服务端为准。普通余额与流量卡额度分开显示；所有渠道在普通余额不足后都会按服务端规则尝试扣有效流量卡，额度不足仍会拒绝请求。',
        ],
        code: codexFormalConfigExample,
      },
    ],
  },
  {
    id: 'copilot-vscode',
    title: 'VS Code Copilot 接入中转站所有模型',
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
    updatedAt: '2026-10-09',
    description: '用「GPT生图1倍率」分组的 API Key 调用文字生图和图生图接口，了解可用模型、按张计费规则和常见报错。',
    kind: 'sections',
    sections: [
      {
        title: '先创建生图专用的 API Key',
        paragraphs: [
          '生图只对「GPT生图1倍率」分组开放。其它分组（GPT、Claude、Grok、国产模型等）的 Key 没有生图权限，请求图片接口会直接返回 403：Image generation is not enabled for this group。',
          '创建方法：登录后点击左侧「API 密钥」，再点「创建密钥」，在「分组」下拉框里选「GPT生图1倍率」，起个好认的名称后点「创建」，复制生成的 sk- 开头的密钥。建议专门新建一个生图 Key；如果把已有的聊天或写代码 Key 直接改选成生图分组，它原来的用途就用不了了。',
          '客户端里的 Base URL 填 https://api.aaccx.pw/v1；工具把 Base URL 和接口路径分开填写时，路径只填 /images/generations 或 /images/edits，不要再多写一次 /v1。',
          '配好之后先请求一次 GET https://api.aaccx.pw/v1/models 自检：返回里应当有下面列出的 4 个生图模型。看到的是别的模型，说明这个 Key 选错了分组。',
        ],
      },
      {
        title: '可用模型与计费',
        paragraphs: [
          '目前可用 4 个生图模型，model 要一字不差：gpt-image-2、gpt-image-1.5、gpt-image-2.5-flare、gpt-image-2.5-sunburst。不写 model 时默认用 gpt-image-2；没有开放的名字（比如 gpt-image-1）会返回 404。',
          '按张计费：成功生成一张图，就按一张图的单价扣费，和提示词长短、参考图大小、Token 数都无关。目前四个模型的单价相同。',
          '单价按图片尺寸分三档：最长边不超过 1024 像素算 1K，不超过 2048 像素算 2K，再大算 4K。例如 1024x1024 是 1K，1536x1024 是 2K，3840x2160 是 4K；系统判断不出尺寸时按 2K 计。档位越高单价越高，具体数字看「模型广场」里「GPT生图1倍率」分组。',
          '一次要多张图（n 大于 1）时，每张各算一次钱。没有产出图片的失败请求（上游出错、超时、被内容审核拦截等）不扣费。生图和文字模型共用同一份余额与流量卡额度，额度用完后请求会被拒绝。',
        ],
      },
      {
        title: '接口地址',
        paragraphs: [
          '认证方式和其它接口一样：请求头带 Authorization: Bearer sk-xxxx。把 sk-xxxx 换成你自己的 Key，不要在公开文档、聊天或截图里展示完整密钥。',
        ],
        endpointRows: imageEndpointRows,
      },
      {
        title: '文字生图',
        paragraphs: [
          '请求体至少要有 prompt。常用字段：model、prompt、size（如 1024x1024 方图、1536x1024 横图、1024x1536 竖图，不写则由模型自行决定）、n（张数，默认 1）。各模型支持的具体尺寸以上游为准，不支持的尺寸会被拒绝。',
          '成功时返回 JSON，图片在 data[0].b64_json 里，是 base64 文本，需要解码后存成 png 文件；某次返回的是 data[0].url 的话，直接下载这个链接。不需要传 response_format。',
          '生成一张图通常要几十秒，客户端超时请设长一些（建议 300 秒）。超过约两分钟还没返回，可能收到 524 或 502，这次不会扣费，换小一点的尺寸或稍后重试。',
          '下面的 curl 示例用 bash 的续行符 \\，macOS、Linux 和 Git Bash 可以直接用；Windows PowerShell 请用下一节的 Python 示例。',
        ],
        code: imageGenerateCurlExample,
      },
      {
        title: 'Python 示例（保存图片）',
        paragraphs: [
          '用 requests 发请求，并把返回的 base64 存成图片，Windows、macOS、Linux 通用。先 pip install requests，再把 API_KEY 换成你的生图 Key。请求失败时会直接打印返回内容，可以对照最后一节「常见报错」排查。',
        ],
        code: imageGeneratePythonExample,
      },
      {
        title: '图生图',
        paragraphs: [
          '图生图用 /v1/images/edits，model 同样选上面 4 个之一。有两种传图方式：JSON 方式用 images[].image_url 传一个能公开访问的图片链接；multipart 方式直接上传本地文件，文件字段名叫 image。',
          '只想改图的一部分时，再多传一个 mask：JSON 方式写 mask.image_url，multipart 方式上传名为 mask 的文件。',
          '图生图同样按张计费，档位看输出图的尺寸，参考图的大小和数量不另收费。返回格式与文字生图相同。',
        ],
        code: imageEditRequestExample,
      },
      {
        title: '常见报错',
        paragraphs: [
          '出错时先看返回 JSON 里的 code 和 message，对照下面几条；认证、额度等通用错误见「错误编号参考」。',
          '403 permission_error（Image generation is not enabled for this group）：这个 Key 所属分组没有生图权限，改用「GPT生图1倍率」分组的 Key。',
          '404 model_not_found：模型没有开放，例如 gpt-image-1。model 只能填 gpt-image-2、gpt-image-1.5、gpt-image-2.5-flare、gpt-image-2.5-sunburst。',
          '400 invalid_request_error：请求参数不符合要求，例如 model 填成了文字模型、图生图缺少 images[].image_url 或 image 文件、用了不支持的 file_id，或尺寸不被支持；按返回的 message 逐项检查。',
          '400 content_policy_violation：提示词或参考图被内容审核拦截，改写描述后重试。',
          '429 rate_limit_error（Image generation concurrency limit exceeded）：同一时间生成的图片太多，等几秒再重试，不要一次并发提交很多张。',
          '403 INSUFFICIENT_BALANCE：普通余额和流量卡额度都不足，充值或购买流量卡后再试。',
          '502、503、504 upstream_error，或 524：上游暂时不可用，或这次生成耗时过长（Cloudflare 约两分钟没收到响应会返回 524）。没出图就不扣费，稍后重试，或先用 1024x1024 小尺寸。',
        ],
      },
    ],
  },
  {
    id: 'trae',
    title: 'Trae 接入中转站所有模型',
    updatedAt: '2026-06-24',
    description: '把这里生成的 API Key 配置到 Trae 自定义模型中使用。',
    kind: 'steps',
    steps: traeSetupSteps,
  },
  {
    id: 'workbuddy',
    title: 'WorkBuddy 接入中转站所有模型',
    updatedAt: '2026-08-10',
    description: '在 WorkBuddy 中添加 OpenAI 兼容的 Custom 模型，使用本站 API Key 连接外部模型。',
    kind: 'steps',
    steps: workbuddySetupSteps,
  },
  {
    id: 'claude-desktop',
    title: 'Claude Desktop 接入中转站 Claude 渠道模型方法',
    updatedAt: '2026-08-11',
    description: '使用 CC Switch 配置 Claude 渠道模型，并在 Claude Desktop 中启用中转站服务。',
    kind: 'steps',
    steps: claudeDesktopSetupSteps,
  },
]

guideTopics.sort((left, right) => right.updatedAt.localeCompare(left.updatedAt))

const activeTopicId = ref(guideTopics[0].id)

const activeTopic = computed(() => (
  guideTopics.find((topic) => topic.id === activeTopicId.value) ?? guideTopics[0]
))
</script>

<style scoped>
/* 与 /dashboard、/ops、/users 共用一套卡片语言：.card 的圆角/边框/阴影 + dark-* 灰阶，
   不再用 gray-900(#111827) 这种偏蓝的硬编码底色。 */
.usage-guide-shell {
  @apply mx-auto grid max-w-screen-2xl gap-6;
}

.usage-guide-side-nav {
  display: none;
}

.usage-guide-main {
  @apply min-w-0 space-y-6;
}

.usage-guide-mobile-tabs {
  @apply flex gap-2 overflow-x-auto pb-1;
  scrollbar-width: none;
}

.usage-guide-mobile-tabs::-webkit-scrollbar {
  display: none;
}

.usage-guide-mobile-tab,
.usage-guide-nav-item {
  @apply cursor-pointer text-left;
  @apply rounded-xl border border-gray-100 bg-white text-gray-600 shadow-card;
  @apply dark:border-dark-700/50 dark:bg-dark-900 dark:text-dark-300;
  @apply transition-all duration-200;
}

.usage-guide-mobile-tab {
  @apply flex flex-none flex-col items-start gap-0.5 px-3 py-2 text-sm font-semibold;
}

.usage-guide-mobile-tab time,
.usage-guide-nav-date,
.usage-guide-date {
  @apply text-xs font-medium leading-5 text-gray-400 dark:text-dark-500;
}

.usage-guide-mobile-tab-active,
.usage-guide-nav-item-active {
  @apply border-primary-200 bg-primary-50 text-primary-700 shadow-none;
  @apply dark:border-primary-800/50 dark:bg-primary-900/20 dark:text-primary-300;
}

.usage-guide-mobile-tab-active time,
.usage-guide-nav-item-active .usage-guide-nav-date {
  @apply text-primary-500 dark:text-primary-400/80;
}

.usage-guide-header {
  @apply card p-6;
}

.usage-guide-kicker {
  @apply text-xs font-semibold uppercase tracking-wider text-gray-400 dark:text-dark-500;
}

.usage-guide-heading {
  @apply mt-1 text-2xl font-bold leading-tight text-gray-900 dark:text-white;
}

.usage-guide-date {
  @apply mt-2 block;
}

.usage-guide-description {
  @apply mt-3 text-sm leading-6 text-gray-500 dark:text-dark-400;
}

.usage-guide-steps,
.usage-guide-sections {
  @apply flex flex-col gap-6;
}

.usage-guide-video-section {
  @apply card w-full max-w-5xl p-6;
}

.usage-guide-video-title {
  @apply mb-4 text-lg font-semibold text-gray-900 dark:text-white;
}

.usage-guide-video {
  @apply block w-full rounded-xl bg-black object-contain;
  aspect-ratio: 16 / 9;
}

.guide-step,
.usage-guide-section-card {
  @apply card flex flex-col gap-4 p-6;
}

.guide-heading {
  @apply flex flex-col gap-1;
}

.guide-kicker {
  @apply text-xs font-semibold uppercase tracking-wider text-gray-400 dark:text-dark-500;
}

.guide-title,
.usage-guide-section-title {
  @apply text-base font-semibold leading-7 text-gray-900 dark:text-white;
}

.usage-guide-section-text {
  @apply text-sm leading-7 text-gray-600 dark:text-dark-300;
}

.usage-guide-table-wrap {
  @apply overflow-x-auto rounded-xl border border-gray-200 dark:border-dark-700;
}

.usage-guide-endpoint-table {
  @apply w-full border-collapse text-sm leading-6 text-gray-700 dark:text-gray-300;
  min-width: 44rem;
}

.usage-guide-error-table {
  min-width: 58rem;
}

.usage-guide-endpoint-table th,
.usage-guide-endpoint-table td {
  @apply border-b border-gray-100 px-4 py-3 text-left align-top dark:border-dark-800;
}

.usage-guide-endpoint-table th {
  @apply whitespace-nowrap border-gray-200 bg-gray-50 font-medium text-gray-600;
  @apply dark:border-dark-700 dark:bg-dark-800/50 dark:text-dark-300;
}

.usage-guide-endpoint-table tr:last-child td {
  border-bottom: 0;
}

.usage-guide-endpoint-table tbody tr {
  @apply transition-colors duration-150;
}

.usage-guide-endpoint-table code {
  @apply font-mono text-xs text-gray-900 dark:text-gray-100;
  overflow-wrap: anywhere;
}

.usage-guide-code {
  @apply overflow-x-auto rounded-xl border border-gray-200 bg-gray-900 p-4 text-xs leading-relaxed text-gray-100;
  @apply dark:border-dark-700 dark:bg-dark-950;
}

.guide-images {
  @apply grid gap-4;
}

.guide-images-before {
  @apply mb-1;
}

.guide-image {
  @apply block h-auto w-full max-w-full rounded-xl border border-gray-200 bg-gray-50;
  @apply dark:border-dark-700 dark:bg-dark-950;
}

.usage-guide-mobile-tab:focus-visible,
.usage-guide-nav-item:focus-visible {
  @apply outline-none ring-2 ring-primary-500/30 ring-offset-2 dark:ring-offset-dark-900;
}

@media (hover: hover) and (pointer: fine) {
  .usage-guide-mobile-tab:hover,
  .usage-guide-nav-item:hover {
    @apply border-gray-200 bg-gray-50 dark:border-dark-600 dark:bg-dark-800;
  }

  .usage-guide-mobile-tab-active:hover,
  .usage-guide-nav-item-active:hover {
    @apply border-primary-200 bg-primary-100 dark:border-primary-800/50 dark:bg-primary-900/30;
  }

  .usage-guide-endpoint-table tbody tr:hover {
    @apply bg-gray-50 dark:bg-dark-800/50;
  }
}

@media (min-width: 768px) {
  .guide-title,
  .usage-guide-section-title {
    @apply text-lg;
  }
}

@media (min-width: 1024px) {
  .usage-guide-shell {
    grid-template-columns: 16rem minmax(0, 1fr);
    align-items: start;
  }

  .usage-guide-side-nav {
    @apply sticky top-20 flex flex-col gap-3;
  }

  .usage-guide-mobile-tabs {
    display: none;
  }

  .usage-guide-nav-item {
    @apply flex flex-col gap-1 p-4;
  }

  .usage-guide-nav-title {
    @apply text-sm font-semibold;
  }

  .usage-guide-nav-desc {
    @apply text-xs leading-5 opacity-80;
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

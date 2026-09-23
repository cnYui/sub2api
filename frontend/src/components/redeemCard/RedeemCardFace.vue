<template>
  <div class="rc-face" :class="[`rc-${data.theme}`, `rc-${side}`]">
    <div class="rc-inner">
      <template v-if="side === 'front'">
        <img class="rc-avatar" :src="avatarUrl" alt="" draggable="false" />
        <img class="rc-wordmark" :src="wordmarkUrl" alt="天才程序员小站" draggable="false" />
        <div class="rc-owner">
          <div class="rc-label"><span class="rc-glitch-dot"></span><span>{{ data.profile.owner_label }}</span></div>
          <div class="rc-owner-name">{{ data.profile.owner_name }}</div>
          <div class="rc-owner-lines">
            <div v-for="(line, i) in data.profile.owner_lines" :key="i">{{ line }}</div>
          </div>
        </div>
        <div class="rc-heatmap">
          <div v-for="(color, i) in heat" :key="i" class="rc-heat-cell" :style="{ background: color }"></div>
        </div>
        <div class="rc-serial">{{ serial }}</div>
      </template>

      <template v-else>
        <div class="rc-code-panel" :class="{ 'rc-stamped': stamp }">
          <div class="rc-label"><span class="rc-glitch-dot"></span><span>兑换码</span></div>
          <div class="rc-code" :style="{ fontSize: `${codeSize}px` }">
            <div v-for="(line, li) in lines" :key="li" class="rc-code-line">
              <span v-for="(group, gi) in line" :key="gi" class="rc-code-group">{{ group }}</span>
            </div>
          </div>
          <div v-if="stamp" class="rc-stamp">{{ stamp }}</div>
        </div>

        <div class="rc-stats">
          <div class="rc-stat"><div class="rc-stat-label">面值</div><div class="rc-stat-value">{{ data.content.amount }}</div></div>
          <div class="rc-stat"><div class="rc-stat-label">套餐</div><div class="rc-stat-value">{{ data.content.plan }}</div></div>
          <div class="rc-stat"><div class="rc-stat-label">用量</div><div class="rc-stat-value">{{ data.content.tokens }}</div></div>
          <div class="rc-stat"><div class="rc-stat-label">有效期至</div><div class="rc-stat-value">{{ data.content.valid_until }}</div></div>
        </div>

        <div class="rc-bottom">
          <div class="rc-qr">
            <div class="rc-qr-frame"><div class="rc-qr-plate"><img :src="leftQR" alt="" draggable="false" /></div></div>
            <div class="rc-qr-caption">{{ data.profile.left_qr_caption }}</div>
          </div>
          <div class="rc-steps">
            <div class="rc-label"><span class="rc-glitch-dot"></span><span>兑换步骤</span></div>
            <div class="rc-step-list">
              <div v-for="(step, i) in data.profile.steps" :key="i" class="rc-step">
                <span class="rc-step-no">{{ String(i + 1).padStart(2, '0') }}</span><span>{{ step }}</span>
              </div>
            </div>
          </div>
          <div class="rc-qr">
            <div class="rc-qr-frame"><div class="rc-qr-plate"><img :src="rightQR" alt="" draggable="false" /></div></div>
            <div class="rc-qr-caption">{{ data.profile.right_qr_caption }}</div>
          </div>
        </div>
      </template>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import avatarUrl from '@/assets/redeem-card/avatar.png'
import wordmarkDarkUrl from '@/assets/redeem-card/wordmark-dark.png'
import wordmarkLightUrl from '@/assets/redeem-card/wordmark-light.png'
import {
  DEFAULT_QR_IMAGES,
  codeFontSize,
  codeLines,
  formatSerial,
  heatmapColors,
  type RedeemCardData,
  type RedeemCardSide
} from './redeemCardModel'

const props = defineProps<{
  data: RedeemCardData
  side: RedeemCardSide
  // 兑换码已兑换 / 过期 / 停用时盖在兑换码上的章，未使用时不传
  stamp?: string
}>()

const wordmarkUrl = computed(() => (props.data.theme === 'dark' ? wordmarkDarkUrl : wordmarkLightUrl))
const heat = computed(() => heatmapColors(props.data.content.heatmap_seed, props.data.theme))
const serial = computed(() => formatSerial(props.data.content.serial))
const lines = computed(() => codeLines(props.data.code))
const codeSize = computed(() => codeFontSize(lines.value.length))
const leftQR = computed(() => props.data.profile.left_qr_image || DEFAULT_QR_IMAGES.left)
const rightQR = computed(() => props.data.profile.right_qr_image || DEFAULT_QR_IMAGES.right)
</script>

<style>
@font-face {
  font-family: 'RC JetBrains Mono';
  src: url('@/assets/redeem-card/jetbrains-mono-latin.woff2') format('woff2');
  font-weight: 100 800;
  font-display: swap;
}
</style>

<style scoped>
/* 数值全部取自站长的设计稿（1712 × 1080，20px/mm）。 */
.rc-face {
  --rc-sans: 'Noto Sans SC', -apple-system, BlinkMacSystemFont, 'PingFang SC', 'Hiragino Sans GB', 'Microsoft YaHei', system-ui, sans-serif;
  --rc-mono: 'RC JetBrains Mono', 'JetBrains Mono', 'SF Mono', Menlo, Consolas, monospace;
  position: relative;
  width: 1712px;
  height: 1080px;
  box-sizing: border-box;
  padding: 3px;
  border-radius: 64px;
  background: linear-gradient(135deg, #2b8a99 0%, #3a3f6e 50%, #8e2f72 100%);
  font-family: var(--rc-sans);
  user-select: none;
}
.rc-dark {
  --rc-bg: #0d1117;
  --rc-fg: #f0f6fc;
  --rc-label: #9da7b3;
  --rc-soft: #b1bac4;
  --rc-step: #d5dce3;
  --rc-step-no: #39d353;
  --rc-dot: #f0f6fc;
  --rc-grid: rgba(255, 255, 255, 0.06);
  --rc-stamp-bg: rgba(13, 17, 23, 0.7);
}
.rc-light {
  --rc-bg: #ffffff;
  --rc-fg: #0d1117;
  --rc-label: #57606a;
  --rc-soft: #424a53;
  --rc-step: #24292f;
  --rc-step-no: #1a7f37;
  --rc-dot: #0d1117;
  --rc-grid: rgba(0, 0, 0, 0.055);
  --rc-stamp-bg: rgba(255, 255, 255, 0.7);
}
.rc-inner {
  position: relative;
  width: 100%;
  height: 100%;
  box-sizing: border-box;
  border-radius: 61px;
  overflow: hidden;
  background: var(--rc-bg);
  color: var(--rc-fg);
}
.rc-back .rc-inner {
  display: flex;
  flex-direction: column;
  justify-content: space-between;
  align-items: center;
  padding: 80px 88px;
  background-image: linear-gradient(var(--rc-grid) 1px, transparent 1px), linear-gradient(90deg, var(--rc-grid) 1px, transparent 1px);
  background-size: 40px 40px;
}

.rc-label {
  display: flex;
  align-items: center;
  gap: 16px;
  white-space: nowrap;
  font-family: var(--rc-mono);
  font-size: 30px;
  font-weight: 500;
  letter-spacing: 0.16em;
  color: var(--rc-label);
}
.rc-glitch-dot {
  display: block;
  flex: none;
  width: 14px;
  height: 14px;
  background: var(--rc-dot);
  box-shadow: -4px -3px 0 #17e0f8, 4px 3px 0 #ff1fd3;
}

/* 正面 */
.rc-avatar {
  position: absolute;
  left: 96px;
  top: 96px;
  width: 300px;
  height: 300px;
  display: block;
}
.rc-wordmark {
  position: absolute;
  left: 436px;
  top: 174px;
  width: 936px;
  height: 145px;
  display: block;
}
.rc-owner {
  position: absolute;
  left: 96px;
  top: 480px;
  width: 900px;
  display: flex;
  flex-direction: column;
  gap: 14px;
  white-space: nowrap;
}
.rc-owner-name {
  font-size: 88px;
  font-weight: 700;
  line-height: 1.1;
  letter-spacing: 0.06em;
}
.rc-owner-lines {
  display: flex;
  flex-direction: column;
  gap: 6px;
  font-size: 38px;
  line-height: 1.4;
  font-weight: 500;
  color: var(--rc-soft);
}
.rc-heatmap {
  position: absolute;
  left: 96px;
  top: 852px;
  display: grid;
  grid-template-rows: repeat(7, 16px);
  grid-auto-flow: column;
  grid-auto-columns: 16px;
  gap: 4px;
}
.rc-heat-cell {
  width: 16px;
  height: 16px;
  border-radius: 3px;
}
.rc-serial {
  position: absolute;
  right: 96px;
  bottom: 92px;
  width: 560px;
  text-align: right;
  white-space: nowrap;
  font-family: var(--rc-mono);
  font-size: 48px;
  font-weight: 700;
  letter-spacing: 0.08em;
}

/* 背面 */
.rc-code-panel {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 20px;
  padding: 32px 56px;
  border-radius: 28px;
}
/* 设计稿用 backdrop-filter 把面板后的网格模糊掉；网格只有 6% 透明度，模糊后等于纯底色，
   这里直接垫一层底色，效果一样，还避开了 iOS 上 backdrop-filter 与 3D 变换叠用的渲染问题。 */
.rc-code-panel {
  position: relative;
  background-color: var(--rc-bg);
}
.rc-dark .rc-code-panel {
  border: 1.5px solid rgba(255, 255, 255, 0.16);
  background-image: linear-gradient(160deg, rgba(255, 255, 255, 0.085) 0%, rgba(255, 255, 255, 0.035) 45%, rgba(255, 255, 255, 0.015) 100%);
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.3), inset 0 -1px 0 rgba(255, 255, 255, 0.05), 0 16px 40px rgba(0, 0, 0, 0.3);
}
.rc-light .rc-code-panel {
  border: 1.5px solid rgba(0, 0, 0, 0.09);
  background-image: linear-gradient(160deg, rgba(255, 255, 255, 0.92) 0%, rgba(246, 248, 250, 0.9) 45%, rgba(240, 243, 246, 0.88) 100%);
  box-shadow: inset 0 1px 0 #ffffff, inset 0 -1px 0 rgba(0, 0, 0, 0.04), 0 16px 40px rgba(13, 17, 23, 0.08);
}
.rc-code {
  display: flex;
  flex-direction: column;
  align-items: center;
  font-family: var(--rc-mono);
  font-weight: 700;
  line-height: 1.1;
  letter-spacing: 0.04em;
  white-space: nowrap;
  text-shadow: -3px -2px 0 #17e0f8, 3px 2px 0 #ff1fd3;
  user-select: text;
}
.rc-code-group + .rc-code-group {
  margin-left: 0.42em;
}
.rc-stamped .rc-code {
  opacity: 0.32;
}
.rc-stamp {
  position: absolute;
  left: 50%;
  top: 50%;
  transform: translate(-50%, -50%) rotate(-9deg);
  padding: 10px 44px;
  border: 7px solid #ff1fd3;
  border-radius: 22px;
  white-space: nowrap;
  font-size: 76px;
  font-weight: 800;
  letter-spacing: 0.24em;
  color: #ff1fd3;
  background: var(--rc-stamp-bg);
  box-shadow: -4px -3px 0 rgba(23, 224, 248, 0.55);
}
.rc-stats {
  width: 100%;
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 40px;
}
.rc-stat {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
}
.rc-stat-label {
  font-size: 30px;
  font-weight: 500;
  color: var(--rc-label);
}
.rc-stat-value {
  font-size: 44px;
  font-weight: 700;
  line-height: 1.2;
  white-space: nowrap;
}
.rc-bottom {
  width: 100%;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 48px;
}
.rc-qr {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 16px;
}
.rc-qr-frame {
  padding: 10px;
  border-radius: 32px;
}
.rc-dark .rc-qr-frame {
  border: 1.5px solid rgba(255, 255, 255, 0.18);
  background: linear-gradient(160deg, rgba(255, 255, 255, 0.12), rgba(255, 255, 255, 0.04));
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.35), 0 16px 40px rgba(0, 0, 0, 0.35);
}
.rc-light .rc-qr-frame {
  border: 1.5px solid rgba(0, 0, 0, 0.09);
  background: linear-gradient(160deg, rgba(255, 255, 255, 0.95), rgba(240, 243, 246, 0.9));
  box-shadow: inset 0 1px 0 #ffffff, 0 16px 40px rgba(13, 17, 23, 0.1);
}
.rc-qr-plate {
  width: 340px;
  height: 340px;
  box-sizing: border-box;
  padding: 18px;
  background: #ffffff;
  border-radius: 24px;
}
.rc-light .rc-qr-plate {
  border: 1px solid rgba(0, 0, 0, 0.06);
}
.rc-qr-plate img {
  width: 300px;
  height: 300px;
  display: block;
  object-fit: contain;
}
.rc-qr-caption {
  font-size: 28px;
  font-weight: 500;
  white-space: nowrap;
  color: var(--rc-soft);
}
.rc-steps {
  display: flex;
  flex-direction: column;
  gap: 20px;
  min-width: 0;
}
.rc-step-list {
  display: flex;
  flex-direction: column;
  gap: 14px;
  white-space: nowrap;
  font-size: 34px;
  line-height: 1.3;
  font-weight: 500;
  color: var(--rc-step);
}
.rc-step {
  display: flex;
  gap: 20px;
  align-items: baseline;
}
.rc-step-no {
  font-family: var(--rc-mono);
  font-weight: 700;
  color: var(--rc-step-no);
}
</style>

<template>
  <main class="rcv-page" :class="`rcv-${theme}`" @pointerdown="onBackgroundPointerDown">
    <div v-if="card" class="rcv-card">
      <RedeemCard3D ref="card3d" :data="card" :stamp="stamp" copyable />
    </div>
    <p v-else-if="errorKey" class="rcv-error">{{ t(errorKey) }}</p>
  </main>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import RedeemCard3D from '@/components/redeemCard/RedeemCard3D.vue'
import { codeStatusStamp } from '@/components/redeemCard/redeemCardModel'
import { getPublicRedeemCard, type PublicRedeemCard, type PublicRedeemCardError } from '@/api/redeemCards'

// 管理员发给用户的兑换卡：页面上只有这张 3D 卡片，没有其它文字。
const { t } = useI18n()
const route = useRoute()

const card = ref<PublicRedeemCard | null>(null)
const errorKey = ref('')
const card3d = ref<InstanceType<typeof RedeemCard3D> | null>(null)

const theme = computed(() => card.value?.theme ?? 'dark')
const stamp = computed(() => codeStatusStamp(card.value?.code_status))

// 在卡片外面拖也能转，手机上卡片小，更好上手。
function onBackgroundPointerDown(event: PointerEvent) {
  if (event.target !== event.currentTarget) return
  card3d.value?.startDrag(event)
}

onMounted(async () => {
  const token = String(route.params.token || '')
  try {
    card.value = await getPublicRedeemCard(token)
  } catch (error) {
    const status = (error as PublicRedeemCardError)?.status
    errorKey.value = status === 404 ? 'redeemCardPage.notFound' : 'redeemCardPage.loadFailed'
  }
})
</script>

<style scoped>
.rcv-page {
  position: fixed;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  overflow: hidden;
  touch-action: none;
  user-select: none;
  -webkit-user-select: none;
}
.rcv-dark {
  background: radial-gradient(ellipse at 50% 42%, #161c27 0%, #0b0f15 55%, #05070a 100%);
}
.rcv-light {
  background: radial-gradient(ellipse at 50% 42%, #ffffff 0%, #eef1f5 55%, #dfe4ea 100%);
}
/* 卡片在视口里尽量大，同时给旋转和底部阴影留出余量。 */
.rcv-card {
  width: min(86vw, calc(76vh * 1712 / 1080), 1040px);
  width: min(86vw, calc(76dvh * 1712 / 1080), 1040px);
}
.rcv-error {
  padding: 0 24px;
  font-size: 15px;
  line-height: 1.6;
  text-align: center;
  color: #8b949e;
}
</style>

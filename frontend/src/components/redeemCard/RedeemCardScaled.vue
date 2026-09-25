<template>
  <div ref="host" class="relative w-full" :style="{ aspectRatio: `${CARD_WIDTH} / ${CARD_HEIGHT}` }">
    <div class="absolute left-0 top-0 origin-top-left" :style="{ transform: `scale(${scale})` }">
      <RedeemCardFace :data="data" :side="side" :stamp="stamp" />
    </div>
  </div>
</template>

<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'
import RedeemCardFace from './RedeemCardFace.vue'
import { CARD_HEIGHT, CARD_WIDTH, type RedeemCardData, type RedeemCardSide } from './redeemCardModel'

// 卡面按设计尺寸排版，这里按容器宽度整体缩放，保证预览和用户看到的卡片完全一致。
defineProps<{
  data: RedeemCardData
  side: RedeemCardSide
  stamp?: string
}>()

const host = ref<HTMLElement | null>(null)
const scale = ref(0.3)
let observer: ResizeObserver | null = null

onMounted(() => {
  if (!host.value) return
  const update = () => {
    if (host.value) scale.value = host.value.clientWidth / CARD_WIDTH
  }
  update()
  observer = new ResizeObserver(update)
  observer.observe(host.value)
})

onBeforeUnmount(() => observer?.disconnect())
</script>

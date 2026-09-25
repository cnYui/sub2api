<template>
  <main class="rcv-page">
    <div v-if="card" class="rcv-card">
      <RedeemCard3D :data="card" :stamp="stamp" copyable />
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

// 管理员发给用户的兑换卡：整页就是 3D 卡片的舞台，没有其它文字。
const { t } = useI18n()
const route = useRoute()

const card = ref<PublicRedeemCard | null>(null)
const errorKey = ref('')

const stamp = computed(() => codeStatusStamp(card.value?.code_status))

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
/* 背景用 3D 模板舞台的暖纸色，黑白两版都一样，地面的软阴影要落在浅底上才看得见。 */
.rcv-page {
  position: fixed;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  overflow: hidden;
  background: #e9e8e4;
  touch-action: none;
  user-select: none;
  -webkit-user-select: none;
}
.rcv-card {
  position: absolute;
  inset: 0;
}
.rcv-error {
  padding: 0 24px;
  font-size: 15px;
  line-height: 1.6;
  text-align: center;
  color: #57606a;
}
</style>

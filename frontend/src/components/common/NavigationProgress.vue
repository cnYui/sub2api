<script setup lang="ts">
/**
 * 导航进度条组件
 * 在页面顶部显示加载进度，提供导航反馈
 */
import { computed } from 'vue'
import { useNavigationLoadingState } from '@/composables/useNavigationLoading'

const { isLoading } = useNavigationLoadingState()

// 进度条可见性
const isVisible = computed(() => isLoading.value)
</script>

<template>
  <Transition name="progress-fade">
    <div
      v-show="isVisible"
      class="navigation-progress"
      role="progressbar"
      aria-label="Loading"
      aria-valuenow="0"
      aria-valuemin="0"
      aria-valuemax="100"
    >
      <div class="navigation-progress-bar" />
    </div>
  </Transition>
</template>

<style scoped>
.navigation-progress {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  height: 3px;
  z-index: 9999;
  overflow: hidden;
  background: transparent;
}

.navigation-progress-bar {
  height: 100%;
  width: 100%;
  background: linear-gradient(
    90deg,
    transparent 0%,
    theme('colors.gray.400') 20%,
    theme('colors.gray.900') 50%,
    theme('colors.gray.400') 80%,
    transparent 100%
  );
  animation: none;
}

:root.dark .navigation-progress-bar {
  background: linear-gradient(
    90deg,
    transparent 0%,
    theme('colors.dark.600') 20%,
    theme('colors.gray.100') 50%,
    theme('colors.dark.600') 80%,
    transparent 100%
  );
}

/* 淡入淡出过渡 */
.progress-fade-enter-active {
  transition: opacity var(--duration-overlay-enter) var(--ease-out);
}

.progress-fade-leave-active {
  transition: opacity var(--duration-overlay-exit) var(--ease-out);
}

.progress-fade-enter-from,
.progress-fade-leave-to {
  opacity: 0;
}

/* 减少动画模式 */
@media (prefers-reduced-motion: reduce) {
  .navigation-progress-bar {
    animation: none;
    opacity: 0.85;
  }
}
</style>

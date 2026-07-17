<template>
  <div
    aria-hidden="true"
    data-testid="auth-world-map-background"
    class="pointer-events-none absolute inset-0 overflow-hidden bg-[#0a0a12]"
  >
    <div
      class="world-map-track absolute inset-y-0 left-0"
      :style="{ backgroundImage: `url(${worldMapDotsUrl})` }"
    ></div>
  </div>
</template>

<script setup lang="ts">
import worldMapDotsUrl from '@/assets/auth/world-map-dots.webp'
</script>

<style scoped>
.world-map-track {
  width: calc(100% + 1800px);
  background-position: left center;
  background-repeat: repeat-x;
  background-size: 1800px 900px;
  animation: world-map-scroll 60s linear infinite;
  will-change: transform;
}

@keyframes world-map-scroll {
  from {
    transform: translate3d(0, 0, 0);
  }

  to {
    transform: translate3d(-1800px, 0, 0);
  }
}

@media (prefers-reduced-motion: reduce) {
  .world-map-track {
    animation: none;
    will-change: auto;
  }
}
</style>

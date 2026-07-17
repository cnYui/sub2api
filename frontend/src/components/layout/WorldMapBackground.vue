<template>
  <div
    aria-hidden="true"
    data-testid="auth-world-map-background"
    class="pointer-events-none absolute inset-0 overflow-hidden bg-[#0a0a12]"
  >
    <div class="world-map-track absolute left-0 flex">
      <img
        v-for="copy in 3"
        :key="copy"
        :src="worldMapDotsUrl"
        aria-hidden="true"
        alt=""
        :draggable="false"
        class="world-map-tile"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import worldMapDotsUrl from '@/assets/auth/world-map-dots.webp'
</script>

<style scoped>
.world-map-track {
  top: 50%;
  height: 125%;
  transform: translate3d(0, -50%, 0);
  animation: world-map-scroll 75s linear infinite;
  will-change: transform;
}

.world-map-tile {
  height: 100%;
  width: auto;
  max-width: none;
  flex: none;
  user-select: none;
}

@keyframes world-map-scroll {
  from {
    transform: translate3d(0, -50%, 0);
  }

  to {
    transform: translate3d(-33.333333%, -50%, 0);
  }
}

@media (prefers-reduced-motion: reduce) {
  .world-map-track {
    animation: none;
    transform: translate3d(0, -50%, 0);
    will-change: auto;
  }
}
</style>

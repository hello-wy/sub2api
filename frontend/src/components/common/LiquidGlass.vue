<template>
  <svg class="liquid-glass-definitions" aria-hidden="true" focusable="false">
    <defs>
      <filter
        :id="filterId"
        filterUnits="userSpaceOnUse"
        color-interpolation-filters="sRGB"
        x="0"
        y="0"
        :width="filterWidth"
        :height="filterHeight"
      >
        <feImage :href="displacementMap" :result="mapResult" :width="filterWidth" :height="filterHeight" />
        <feDisplacementMap
          in="SourceGraphic"
          :in2="mapResult"
          xChannelSelector="R"
          yChannelSelector="G"
          :scale="displacementScale"
        />
      </filter>
    </defs>
  </svg>

  <div
    v-bind="$attrs"
    ref="glassElement"
    class="liquid-glass"
    :style="{ '--liquid-glass-filter': `url(#${filterId})` }"
  >
    <slot />
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from 'vue'

defineOptions({ inheritAttrs: false })

const glassElement = ref<HTMLElement | null>(null)
const dimensions = ref({ width: 1, height: 1 })
const displacementMap = ref('')
const displacementScale = ref(0)
const filterId = `liquid-glass-${Math.random().toString(36).slice(2, 11)}`
const mapResult = `${filterId}-map`
let resizeObserver: ResizeObserver | null = null

const filterWidth = computed(() => dimensions.value.width)
const filterHeight = computed(() => dimensions.value.height)

function smoothStep(start: number, end: number, value: number) {
  const progress = Math.max(0, Math.min(1, (value - start) / (end - start)))
  return progress * progress * (3 - 2 * progress)
}

function roundedRectSdf(x: number, y: number, width: number, height: number, radius: number) {
  const horizontal = Math.abs(x) - width + radius
  const vertical = Math.abs(y) - height + radius
  return Math.min(Math.max(horizontal, vertical), 0) + Math.hypot(Math.max(horizontal, 0), Math.max(vertical, 0)) - radius
}

function updateDisplacementMap() {
  const element = glassElement.value
  if (!element) return

  const rect = element.getBoundingClientRect()
  const width = Math.max(1, Math.round(rect.width))
  const height = Math.max(1, Math.round(rect.height))
  dimensions.value = { width, height }

  const canvasWidth = Math.min(width, 680)
  const canvasHeight = Math.min(height, 280)
  const canvas = document.createElement('canvas')
  canvas.width = canvasWidth
  canvas.height = canvasHeight
  const context = canvas.getContext('2d')
  if (!context) return

  const imageData = context.createImageData(canvasWidth, canvasHeight)
  const halfHeight = 0.2
  const halfWidth = halfHeight * (canvasWidth / canvasHeight)
  const cornerRadius = Math.max(halfWidth, halfHeight)
  let maxScale = 0
  const offsets = new Float32Array(canvasWidth * canvasHeight * 2)

  for (let pixelIndex = 0; pixelIndex < canvasWidth * canvasHeight; pixelIndex += 1) {
    const pixelX = pixelIndex % canvasWidth
    const pixelY = Math.floor(pixelIndex / canvasWidth)
    const x = pixelX / canvasWidth - 0.5
    const y = pixelY / canvasHeight - 0.5
    const distanceToEdge = roundedRectSdf(x, y, halfWidth, halfHeight, cornerRadius)
    const displacement = smoothStep(0.8, 0, distanceToEdge - 0.15)
    const scale = smoothStep(0, 1, displacement)
    const horizontalOffset = (x * scale + 0.5) * canvasWidth - pixelX
    const verticalOffset = (y * scale + 0.5) * canvasHeight - pixelY
    const offsetIndex = pixelIndex * 2

    offsets[offsetIndex] = horizontalOffset
    offsets[offsetIndex + 1] = verticalOffset
    maxScale = Math.max(maxScale, Math.abs(horizontalOffset), Math.abs(verticalOffset))
  }

  maxScale *= 0.5
  const safeScale = Math.max(maxScale, 0.001)

  for (let pixelIndex = 0; pixelIndex < canvasWidth * canvasHeight; pixelIndex += 1) {
    const dataIndex = pixelIndex * 4
    const offsetIndex = pixelIndex * 2
    imageData.data[dataIndex] = ((offsets[offsetIndex] / safeScale) + 0.5) * 255
    imageData.data[dataIndex + 1] = ((offsets[offsetIndex + 1] / safeScale) + 0.5) * 255
    imageData.data[dataIndex + 3] = 255
  }

  context.putImageData(imageData, 0, 0)
  displacementMap.value = canvas.toDataURL()
  displacementScale.value = safeScale * (width / canvasWidth)
}

onMounted(async () => {
  await nextTick()
  updateDisplacementMap()

  if (glassElement.value && typeof ResizeObserver !== 'undefined') {
    resizeObserver = new ResizeObserver(updateDisplacementMap)
    resizeObserver.observe(glassElement.value)
  }
})

onBeforeUnmount(() => resizeObserver?.disconnect())
</script>

<style scoped>
.liquid-glass-definitions {
  position: fixed;
  width: 0;
  height: 0;
  pointer-events: none;
}

.liquid-glass {
  isolation: isolate;
  overflow: hidden;
  backdrop-filter: var(--liquid-glass-filter) blur(12px) saturate(1.18) contrast(1.04);
}

.liquid-glass::before {
  position: absolute;
  z-index: -1;
  inset: 0;
  content: '';
  pointer-events: none;
  background: linear-gradient(135deg, rgba(255, 255, 255, 0.54), rgba(255, 255, 255, 0.08) 46%, rgba(194, 225, 255, 0.22));
}

:global(.dark) .liquid-glass::before {
  background: linear-gradient(135deg, rgba(240, 249, 255, 0.16), rgba(111, 177, 255, 0.06) 46%, rgba(15, 44, 78, 0.22));
}

.liquid-glass::after {
  position: absolute;
  z-index: -1;
  inset: 1px;
  border: 1px solid rgba(255, 255, 255, 0.5);
  border-radius: inherit;
  content: '';
  pointer-events: none;
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.56), inset 0 -1px 0 rgba(9, 73, 151, 0.08);
}

:global(.dark) .liquid-glass::after {
  border-color: rgba(210, 234, 255, 0.2);
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.2), inset 0 -1px 0 rgba(75, 159, 255, 0.14);
}

:global(.liquid-glass-primary)::before {
  background: linear-gradient(135deg, rgba(86, 175, 255, 0.9), rgba(22, 119, 255, 0.72) 50%, rgba(7, 75, 185, 0.82));
}

:global(.liquid-glass-primary)::after {
  border-color: rgba(222, 243, 255, 0.46);
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.42), inset 0 -1px 0 rgba(3, 51, 139, 0.24);
}
</style>

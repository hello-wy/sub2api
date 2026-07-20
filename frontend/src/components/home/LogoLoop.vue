<template>
  <div
    ref="containerRef"
    :class="rootClasses"
    :style="containerStyle"
    role="region"
    :aria-label="ariaLabel"
  >
    <div
      ref="trackRef"
      class="relative z-0 flex w-max select-none will-change-transform motion-reduce:transform-none"
      @mouseenter="isHovered = true"
      @mouseleave="isHovered = false"
    >
      <ul
        v-for="copyIndex in copyCount"
        :key="`copy-${copyIndex}`"
        ref="sequenceRefs"
        class="flex items-center"
        role="list"
        :aria-hidden="copyIndex > 1 ? true : undefined"
      >
        <li
          v-for="(item, itemIndex) in logos"
          :key="`${copyIndex}-${itemIndex}`"
          class="mr-[var(--logoloop-gap)] flex-none text-[length:var(--logoloop-logoHeight)] leading-none"
          :class="scaleOnHover && 'group/item overflow-visible'"
          role="listitem"
        >
          <slot name="renderItem" :item="item" :index="itemIndex">
            <a
              v-if="item.href"
              class="inline-flex items-center rounded transition-opacity duration-200 hover:opacity-80 focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2"
              :href="item.href"
              :aria-label="getItemLabel(item)"
              target="_blank"
              rel="noreferrer noopener"
            >
              <LogoItemContent :item="item" :scale-on-hover="scaleOnHover" />
            </a>
            <LogoItemContent v-else :item="item" :scale-on-hover="scaleOnHover" />
          </slot>
        </li>
      </ul>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, defineComponent, h, nextTick, onBeforeUnmount, onMounted, ref, watch, type CSSProperties, type PropType } from 'vue'

export type LogoItemNode = {
  node: string
  href?: string
  title?: string
  ariaLabel?: string
}

export type LogoItemImage = {
  src: string
  alt?: string
  href?: string
  title?: string
  srcSet?: string
  sizes?: string
  width?: number
  height?: number
}

export type LogoItem = LogoItemNode | LogoItemImage

const props = withDefaults(defineProps<{
  logos: LogoItem[]
  speed?: number
  direction?: 'left' | 'right' | 'up' | 'down'
  width?: number | string
  logoHeight?: number
  gap?: number
  pauseOnHover?: boolean
  hoverSpeed?: number
  fadeOut?: boolean
  fadeOutColor?: string
  scaleOnHover?: boolean
  ariaLabel?: string
  className?: string
  style?: CSSProperties
}>(), {
  speed: 120,
  direction: 'left',
  width: '100%',
  logoHeight: 28,
  gap: 32,
  hoverSpeed: undefined,
  fadeOut: false,
  scaleOnHover: false,
  ariaLabel: 'Partner logos'
})

const containerRef = ref<HTMLDivElement | null>(null)
const trackRef = ref<HTMLDivElement | null>(null)
const sequenceRefs = ref<HTMLUListElement[]>([])
const copyCount = ref(2)
const sequenceWidth = ref(0)
const isHovered = ref(false)
const offset = ref(0)
const velocity = ref(0)
let animationFrame: number | undefined
let lastTimestamp: number | undefined
let resizeObserver: ResizeObserver | undefined

const effectiveHoverSpeed = computed(() => {
  if (props.hoverSpeed !== undefined) return props.hoverSpeed
  return props.pauseOnHover === false ? undefined : 0
})

const targetVelocity = computed(() => {
  const direction = props.direction === 'left' ? 1 : -1
  return Math.abs(props.speed) * direction * (props.speed < 0 ? -1 : 1)
})

const rootClasses = computed(() => [
  'relative overflow-x-hidden',
  props.fadeOut && 'logo-loop-fade',
  '[--logoloop-gap:32px]',
  '[--logoloop-logoHeight:28px]',
  '[--logoloop-fadeColorAuto:#f4f8ff]',
  'dark:[--logoloop-fadeColorAuto:#050b14]',
  props.scaleOnHover && 'py-[calc(var(--logoloop-logoHeight)*0.1)]',
  props.className
])

const containerStyle = computed(() => ({
  width: typeof props.width === 'number' ? `${props.width}px` : props.width,
  '--logoloop-gap': `${props.gap}px`,
  '--logoloop-logoHeight': `${props.logoHeight}px`,
  ...(props.fadeOutColor ? { '--logoloop-fadeColor': props.fadeOutColor } : {}),
  ...(props.style ?? {})
}))

function isNodeItem(item: LogoItem): item is LogoItemNode {
  return 'node' in item
}

function getItemLabel(item: LogoItem): string {
  return isNodeItem(item) ? (item.ariaLabel ?? item.title ?? 'Partner logo') : (item.alt ?? item.title ?? 'Partner logo')
}

async function updateDimensions() {
  await nextTick()
  const sequence = sequenceRefs.value[0]
  const container = containerRef.value
  if (!sequence || !container) return

  const width = Math.ceil(sequence.getBoundingClientRect().width)
  if (!width) return

  sequenceWidth.value = width
  copyCount.value = Math.max(2, Math.ceil(container.clientWidth / width) + 2)
}

function stopAnimation() {
  if (animationFrame !== undefined) cancelAnimationFrame(animationFrame)
  animationFrame = undefined
  lastTimestamp = undefined
}

function startAnimation() {
  stopAnimation()
  const track = trackRef.value
  if (!track || window.matchMedia('(prefers-reduced-motion: reduce)').matches) return

  const tick = (timestamp: number) => {
    const delta = lastTimestamp === undefined ? 0 : Math.min((timestamp - lastTimestamp) / 1000, 0.1)
    lastTimestamp = timestamp
    const target = isHovered.value && effectiveHoverSpeed.value !== undefined
      ? effectiveHoverSpeed.value
      : targetVelocity.value
    const smoothing = 1 - Math.exp(-delta / 0.25)
    velocity.value += (target - velocity.value) * smoothing

    if (sequenceWidth.value > 0) {
      offset.value = (offset.value + velocity.value * delta) % sequenceWidth.value
      if (offset.value < 0) offset.value += sequenceWidth.value
      track.style.transform = `translate3d(${-offset.value}px, 0, 0)`
    }

    animationFrame = requestAnimationFrame(tick)
  }

  animationFrame = requestAnimationFrame(tick)
}

onMounted(async () => {
  await updateDimensions()
  resizeObserver = new ResizeObserver(updateDimensions)
  if (containerRef.value) resizeObserver.observe(containerRef.value)
  if (sequenceRefs.value[0]) resizeObserver.observe(sequenceRefs.value[0])
  startAnimation()
})

onBeforeUnmount(() => {
  stopAnimation()
  resizeObserver?.disconnect()
})

watch(() => [props.logos, props.gap, props.logoHeight, props.width], async () => {
  await updateDimensions()
}, { deep: true })

const LogoItemContent = defineComponent({
  props: {
    item: { type: Object as PropType<LogoItem>, required: true },
    scaleOnHover: Boolean
  },
  setup(contentProps) {
    return () => {
      const hoverClass = contentProps.scaleOnHover
        ? 'transition-transform duration-300 group-hover/item:scale-110'
        : ''

      if (isNodeItem(contentProps.item)) {
        return h('span', {
          class: ['inline-flex items-center', hoverClass],
          innerHTML: contentProps.item.node
        })
      }

      return h('img', {
        class: ['block h-[var(--logoloop-logoHeight)] w-auto object-contain', hoverClass],
        src: contentProps.item.src,
        srcset: contentProps.item.srcSet,
        sizes: contentProps.item.sizes,
        width: contentProps.item.width,
        height: contentProps.item.height,
        alt: contentProps.item.alt ?? '',
        title: contentProps.item.title,
        loading: 'lazy',
        decoding: 'async',
        draggable: false
      })
    }
  }
})
</script>

<style scoped>
.logo-loop-fade {
  -webkit-mask-image: linear-gradient(to right, transparent 0%, #000 10%, #000 90%, transparent 100%);
  mask-image: linear-gradient(to right, transparent 0%, #000 10%, #000 90%, transparent 100%);
}
</style>

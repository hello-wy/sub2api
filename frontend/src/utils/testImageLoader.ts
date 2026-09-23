import { intelligentTestsAPI } from '@/api/intelligentTests'

const maxConcurrentImages = 4
let activeImages = 0

interface PendingImage {
  recordId: number
  publicView: boolean
  signal?: AbortSignal
  resolve: (image: Blob) => void
  reject: (reason: unknown) => void
  abort: () => void
}

const pendingImages: PendingImage[] = []
const abortReason = (signal: AbortSignal) => signal.reason ?? new DOMException('Image request aborted', 'AbortError')

async function fetchImage(request: PendingImage) {
  try {
    const image = await intelligentTestsAPI.image(request.recordId, request.publicView, request.signal)
    if (request.signal?.aborted) throw abortReason(request.signal)
    request.resolve(image)
  } catch (error) {
    request.reject(error)
  } finally {
    // Keep an aborted active request in its slot until the transport settles.
    activeImages--
    drainImages()
  }
}

function drainImages() {
  while (activeImages < maxConcurrentImages && pendingImages.length) {
    const request = pendingImages.shift()!
    request.signal?.removeEventListener('abort', request.abort)
    if (request.signal?.aborted) {
      request.reject(abortReason(request.signal))
      continue
    }
    activeImages++
    void fetchImage(request)
  }
}

/** Bound legacy-image fallback requests across every mounted result gallery. */
export function loadTestImage(recordId: number, publicView = false, signal?: AbortSignal): Promise<Blob> {
  if (signal?.aborted) return Promise.reject(abortReason(signal))
  return new Promise((resolve, reject) => {
    const request: PendingImage = {
      recordId, publicView, signal, resolve, reject,
      abort: () => {
        const index = pendingImages.indexOf(request)
        if (index < 0) return
        pendingImages.splice(index, 1)
        signal?.removeEventListener('abort', request.abort)
        reject(abortReason(signal!))
      }
    }
    pendingImages.push(request)
    signal?.addEventListener('abort', request.abort, { once: true })
    drainImages()
  })
}

import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'

import { describe, expect, it } from 'vitest'

const source = readFileSync(resolve(process.cwd(), 'src/views/ModelPlazaView.vue'), 'utf8')

describe('ModelPlazaView route scrolling', () => {
  it('uses the shared scroll container for the embedded app layout', () => {
    expect(source).toMatch(
      /import ScrollablePageLayout from ['"]@\/components\/layout\/ScrollablePageLayout\.vue['"]/,
    )
    expect(source).toMatch(
      /<AppLayout v-if="isEmbedded">\s*<ScrollablePageLayout>[\s\S]*<\/ScrollablePageLayout>\s*<\/AppLayout>/,
    )
  })
})

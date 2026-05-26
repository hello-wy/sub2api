import { describe, expect, it } from 'vitest'

import en from '../locales/en'
import zh from '../locales/zh'

type LocaleMessages = Record<string, unknown>

type LocaleMessageEntry = {
  key: string
  message: string
}

function collectMessageEntries(messages: LocaleMessages, prefix = ''): LocaleMessageEntry[] {
  return Object.entries(messages).flatMap(([key, value]) => {
    const path = prefix ? `${prefix}.${key}` : key
    if (typeof value === 'string') {
      return [{ key: path, message: value }]
    }
    if (value && typeof value === 'object' && !Array.isArray(value)) {
      return collectMessageEntries(value as LocaleMessages, path)
    }
    return []
  })
}

describe('locale messages', () => {
  it.each([
    ['en', en],
    ['zh', zh]
  ] as const)('escapes literal at signs in %s messages', (locale, messages) => {
    const errors: string[] = []

    for (const { key, message } of collectMessageEntries(messages)) {
      const unescapedMessage = message.replaceAll("{'@'}", '')
      if (unescapedMessage.includes('@')) {
        errors.push(`${locale}.${key}: escape literal @ as {'@'}`)
      }
    }

    expect(errors).toEqual([])
  })
})

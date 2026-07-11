import { describe, expect, it } from 'vitest'

import en from '../locales/en'
import zh from '../locales/zh'

type LocaleMessages = Record<string, unknown>

function readMessage(messages: LocaleMessages, path: string): unknown {
  return path.split('.').reduce<unknown>((value, key) => {
    if (!value || typeof value !== 'object' || Array.isArray(value)) {
      return undefined
    }
    return (value as LocaleMessages)[key]
  }, messages)
}

const requiredKeys = [
  'admin.settings.tabs.operations',
  'admin.settings.operations.subtabsLabel',
  'admin.settings.operations.subtabs.welfare',
  'admin.settings.operations.subtabs.membership',
  'admin.settings.operations.title',
  'admin.settings.operations.description',
  'admin.settings.operations.rankLimit',
  'admin.settings.operations.rankLimitHint',
  'admin.settings.operations.rewardRatios',
  'admin.settings.operations.rewardRatiosHint',
  'admin.settings.operations.ratioItem',
  'admin.settings.operations.ratioPlaceholder',
  'admin.settings.operations.membership.title',
  'admin.settings.operations.membership.description',
  'admin.settings.operations.membership.weeklyPlan',
  'admin.settings.operations.membership.weeklyPlanHint',
  'admin.settings.operations.membership.permanentPlan',
  'admin.settings.operations.membership.permanentPlanHint',
  'admin.settings.operations.membership.addRule',
  'admin.settings.operations.membership.removeRule',
  'admin.settings.operations.membership.rechargeAmount',
  'admin.settings.operations.membership.rebateRate',
  'admin.settings.operations.membership.saveHint',
  'keys.ccsClientSelect.codex',
  'keys.ccsClientSelect.codexDesc',
] as const

describe.each([
  ['en', en],
  ['zh', zh],
] as const)('%s settings and CC-Switch locales', (locale, messages) => {
  for (const key of requiredKeys) {
    it(`defines ${key}`, () => {
      expect(readMessage(messages, key)).toEqual(expect.any(String))
    })
  }
})

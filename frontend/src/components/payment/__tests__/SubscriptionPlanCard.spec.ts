import { mount } from "@vue/test-utils";
import { describe, expect, it } from "vitest";
import { createPinia } from "pinia";
import { createI18n } from "vue-i18n";
import SubscriptionPlanCard from "../SubscriptionPlanCard.vue";

const i18n = createI18n({
  legacy: false,
  locale: "en",
  fallbackWarn: false,
  missingWarn: false,
  messages: {
    en: {
      payment: {
        days: "days",
        models: "Models",
        actualPay: "Actual pay",
        notAvailable: "Unavailable",
        planCard: {
          quota: "Quota",
          rate: "Rate",
          unlimited: "Unlimited",
        },
      },
      wallet: {
        subscribeAction: "Subscribe",
        subscriptionPaymentRecharge: "Recharge payment",
        subscriptionPaymentBalance: "Balance payment",
        subscriptionBalanceAvailable: "Available balance",
        subscriptionBalanceInsufficient: "Insufficient balance",
      },
      common: { processing: "Processing" },
    },
  },
});

const mountPlanCard = (
  groupPlatform: string,
  subscriptionUsdToCnyRate = 0,
  overrides: Record<string, unknown> = {},
) =>
  mount(SubscriptionPlanCard, {
    props: {
      subscriptionUsdToCnyRate,
      availableBalance: 25,
      rechargeAmountLabel: "¥71.50",
      plan: {
        id: 1,
        group_id: 10,
        group_platform: groupPlatform,
        name: "Pro",
        price: 10,
        amount: 1000,
        features: [],
        rate_multiplier: 1,
        validity_days: 30,
        validity_unit: "day",
        supported_model_scopes: ["claude", "gemini_text", "gemini_image"],
        is_active: true,
      },
      ...overrides,
    },
    global: { plugins: [i18n, createPinia()] },
  });

describe("SubscriptionPlanCard", () => {
  it("does not show Antigravity model scopes for OpenAI plans", () => {
    const text = mountPlanCard("openai").text();

    expect(text).not.toContain("Claude");
    expect(text).not.toContain("Gemini");
    expect(text).not.toContain("Imagen");
  });

  it("shows model scopes for Antigravity plans", () => {
    const text = mountPlanCard("antigravity").text();

    expect(text).toContain("Claude");
    expect(text).toContain("Gemini");
    expect(text).toContain("Imagen");
  });

  it("shows the converted yuan price and always uses the subscribe action", () => {
    const wrapper = mountPlanCard("openai", 7.15);

    expect(wrapper.text()).toContain("¥71.50");
    expect(wrapper.text()).not.toContain("$10");
    expect(wrapper.findAll("button").at(-1)?.text()).toBe("wallet.subscribeAction");
  });

  it("selects the payment source and subscribes directly inside the card", async () => {
    const wrapper = mountPlanCard("openai", 7.15);
    const balanceButton = wrapper.findAll("button").find(button =>
      button.text().includes("wallet.subscriptionPaymentBalance"),
    );

    await balanceButton?.trigger("click");
    expect(wrapper.text()).toContain("$25.00");

    await wrapper.findAll("button").at(-1)?.trigger("click");
    expect(wrapper.emitted("subscribe")?.[0]?.[1]).toBe("balance");
  });

  it("disables balance subscription when the available balance is insufficient", async () => {
    const wrapper = mountPlanCard("openai", 7.15, { availableBalance: 5 });
    const balanceButton = wrapper.findAll("button").find(button =>
      button.text().includes("wallet.subscriptionPaymentBalance"),
    );

    await balanceButton?.trigger("click");
    expect(wrapper.text()).toContain("wallet.subscriptionBalanceInsufficient");
    expect(wrapper.findAll("button").at(-1)?.attributes("disabled")).toBeDefined();
  });
});

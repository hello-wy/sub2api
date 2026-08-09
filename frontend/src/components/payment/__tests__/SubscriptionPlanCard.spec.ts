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
        weeks: "weeks",
        months: "months",
        perMonth: "month",
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
        subscriptionBalanceInsufficientShort: "Insufficient balance",
        subscriptionMemberDiscount: "Member discount",
        subscriptionNoDiscount: "No discount",
        subscriptionWeeklyDiscount: "Weekly discount",
        subscriptionPermanentDiscount: "Lifetime discount",
        subscriptionBeforeDiscount: "Before discount",
        subscriptionAfterDiscount: "After discount",
        subscriptionSettlementAmount: "Settlement amount",
        subscriptionBalanceNoDiscount: "No discount",
        subscriptionBalanceSettlement: "Balance after payment",
      },
      common: { processing: "Processing" },
    },
  },
});

const mountPlanCard = (
  groupPlatform: string,
  subscriptionUsdToCnyRate: number | Record<string, unknown> = 0,
  overrides: Record<string, unknown> = {},
) => {
  const rate = typeof subscriptionUsdToCnyRate === "number" ? subscriptionUsdToCnyRate : 0;
  const planOverrides = typeof subscriptionUsdToCnyRate === "number" ? overrides : subscriptionUsdToCnyRate;

  return mount(SubscriptionPlanCard, {
    props: {
      subscriptionUsdToCnyRate: rate,
      availableBalance: 25,
      rechargeBeforeDiscountLabel: "¥71.50",
      rechargeAfterDiscountLabel: "¥71.50",
      rechargeAmountLabel: "¥71.50",
      loyaltyDiscountLabel: "No discount",
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
        ...planOverrides,
      },
      ...planOverrides,
    },
    global: { plugins: [i18n, createPinia()] },
  });
};

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

  // #4607：管理端保存的单位是复数（months/weeks），此前用户侧只匹配单数
  // 'month'，「1 个月」的套餐卡片被显示成「1天」。测试环境的 vue-i18n 为
  // runtime-only 构建，t() 原样返回 key，故按 key 断言单位分支。
  it("renders plural admin-form validity units instead of mislabeled days (#4607)", () => {
    expect(mountPlanCard("openai", 0, { validity_days: 1, validity_unit: "months" }).text()).toContain("/ payment.perMonth");
    expect(mountPlanCard("openai", 0, { validity_days: 3, validity_unit: "months" }).text()).toContain("/ 3payment.months");
    expect(mountPlanCard("openai", 0, { validity_days: 2, validity_unit: "weeks" }).text()).toContain("/ 2payment.weeks");
    expect(mountPlanCard("openai", 0, { validity_days: 30, validity_unit: "day" }).text()).toContain("/ 30payment.days");
  });

  it("uses the configured currency symbol while preserving USD for legacy plans", () => {
    const cnyPlan = mountPlanCard("openai", 0, { currency: "CNY", original_price: 20 }).text();

    expect(cnyPlan).toContain("¥10CNY");
    expect(cnyPlan).toContain("¥20CNY");
    expect(mountPlanCard("openai", 0, { currency: "USD" }).text()).toContain("$10USD");
    expect(mountPlanCard("openai").text()).toContain("$10");
  });

  it("always uses the subscribe action", () => {
    const wrapper = mountPlanCard("openai");
    expect(wrapper.findAll("button").at(-1)?.text()).toBe("wallet.subscribeAction");
  });

  it("selects the payment source and subscribes directly inside the card", async () => {
    const wrapper = mountPlanCard("openai");
    const balanceButton = wrapper.findAll("button").find(button =>
      button.text().includes("wallet.subscriptionPaymentBalance"),
    );

    await balanceButton?.trigger("click");
    expect(wrapper.text()).toContain("$15.00");

    await wrapper.findAll("button").at(-1)?.trigger("click");
    expect(wrapper.emitted("subscribe")?.[0]?.[1]).toBe("balance");
  });

  it("disables balance subscription when the available balance is insufficient", async () => {
    const wrapper = mountPlanCard("openai", 0, { availableBalance: 5 });
    const balanceButton = wrapper.findAll("button").find(button =>
      button.text().includes("wallet.subscriptionPaymentBalance"),
    );

    await balanceButton?.trigger("click");
    expect(wrapper.text()).toContain("wallet.subscriptionBalanceInsufficientShort");
    expect(wrapper.findAll("button").at(-1)?.attributes("disabled")).toBeDefined();
  });

  it("shows the plan discount for recharge and settles balance payment without it", async () => {
    const basePlan = mountPlanCard("openai").props("plan");
    const wrapper = mountPlanCard("openai", 1, {
      availableBalance: 25,
      balancePrice: 20,
      rechargeBeforeDiscountLabel: "¥10.00",
      rechargeAfterDiscountLabel: "¥9.20",
      rechargeAmountLabel: "¥9.20",
      loyaltyDiscountLabel: "Weekly L4 · 8% off",
      plan: { ...basePlan, price: 10, original_price: 20 },
    });

    expect(wrapper.text()).toContain("Weekly L4 · 8% off");
    expect(wrapper.text()).toContain("¥10.00");
    expect(wrapper.text()).toContain("¥9.20");

    const balanceButton = wrapper.findAll("button").find(button =>
      button.text().includes("wallet.subscriptionPaymentBalance"),
    );
    await balanceButton?.trigger("click");

    expect(wrapper.text()).toContain("wallet.subscriptionBalanceNoDiscount");
    expect(wrapper.text()).toContain("$25.00");
    expect(wrapper.text()).toContain("$20.00");
    expect(wrapper.text()).toContain("$5.00");
  });

  it.each([
    ["long Chinese", "企业全球加速专业订阅套餐（含高级模型与优先支持）"],
    ["long English", "Enterprise Global Acceleration Subscription with Priority Support"],
    ["unbroken token", "EnterpriseGlobalAccelerationSubscriptionWithPrioritySupport1234567890"],
  ])("keeps the full %s plan title accessible in a bounded two-line area", (_label, name) => {
    const wrapper = mountPlanCard("openai", { name });
    const title = wrapper.get("h3");

    expect(title.text()).toBe(name);
    expect(title.attributes("title")).toBe(name);
    expect(title.classes()).toEqual(expect.arrayContaining([
      "min-w-0",
      "h-12",
      "break-words",
      "line-clamp-2",
      "[overflow-wrap:anywhere]",
    ]));
    expect(title.classes()).not.toContain("truncate");
  });

  it("keeps title, badge, price, description, and purchase action in separate bounded regions", () => {
    const wrapper = mountPlanCard("openai", {
      name: "Enterprise Global Acceleration Subscription with Priority Support",
      price: 123.45,
      currency: "USD",
      description: "Includes advanced models and priority support.",
    });
    const title = wrapper.get("h3");
    const badge = wrapper.findAll("span").find((node) => node.text() === "OpenAI");
    const price = wrapper.findAll("span").find((node) => node.text() === "123.45");

    expect(title.element.parentElement?.classList).toContain("min-w-0");
    expect(title.element.parentElement?.classList).toContain("flex-1");
    expect(badge?.classes()).toContain("shrink-0");
    expect([...(badge?.element.parentElement?.classList ?? [])]).toEqual(expect.arrayContaining([
      "flex",
      "items-center",
      "justify-end",
    ]));
    expect(badge?.element.parentElement?.textContent).toContain("/ 30payment.days");
    expect(badge?.element.parentElement?.parentElement?.classList).toContain("shrink-0");
    expect(price?.element.parentElement?.parentElement?.classList).toContain("shrink-0");
    expect(wrapper.get("p").text()).toBe("Includes advanced models and priority support.");
    expect(wrapper.findAll("button").at(-1)?.text()).toBe("wallet.subscribeAction");
  });

  it("keeps short plan titles compact and aligned", () => {
    const wrapper = mountPlanCard("openai", { name: "Pro", description: "" });
    const title = wrapper.get("h3");
    const badge = wrapper.findAll("span").find((node) => node.text() === "OpenAI");

    expect(title.text()).toBe("Pro");
    expect(title.attributes("title")).toBe("Pro");
    expect(title.classes()).toEqual(expect.arrayContaining(["text-base", "font-bold", "h-12"]));
    expect([...(badge?.element.parentElement?.classList ?? [])]).toEqual(expect.arrayContaining([
      "flex",
      "items-center",
      "justify-end",
    ]));
    expect(badge?.element.parentElement?.textContent).toContain("/ 30payment.days");
  });
});

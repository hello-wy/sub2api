export default {
  accountTicketDefaults: {
    title: '新账号自动获取票据',
    off: '关闭自动获取',
    plan: '票据方案',
    pro: 'Pro · 292',
    team: 'Team · 332',
    scope: '选择 Pro 或 Team 并保存，即为今后新建、导入的账号和新增 IP 通道开启自动获取。新账号优先使用这里的方案，验证模型为 gpt-6-astra。只有取得当前凭据及固定业务 IP 验证通过的票据后，才参与调度。',
    futureOnly: '仅影响今后新建、导入和新增 IP 通道；已有账号的设置保持不变。关闭后，新账号继续按分组原有设置处理。',
    prerequisites: '网关 STATE 总开关未开启或动态 IP 池尚未配置。可以先保存默认规则，新账号将等待配置与验证完成。',
    prerequisitesUnknown: '暂时无法读取网关采集配置。仍可保存默认规则，采集是否就绪需要在网关设置中确认。',
    settings: '前往网关设置',
    saved: '默认规则已保存。实际采集和验证结果请在新账号的票据状态中查看。',
    loadFailed: '无法读取默认规则，请重试后保存。',
    saveFailed: '保存失败，已保留当前选择；请重试或重新打开确认已保存的规则。',
    reload: '重新读取',
    retryPrerequisites: '刷新网关配置',
    save: '保存默认规则'
  }
}

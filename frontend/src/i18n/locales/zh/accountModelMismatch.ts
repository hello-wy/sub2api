export default {
  accountModelMismatch: {
    label: '降智',
    legacyLabel: '历史模型记录',
    legacyExplanation: '该历史记录不属于 gpt-6-astra → gpt-5.6-luna，不再判定为降智。原有暂停状态保持，系统不会自动开启账号。',
    legacyRestoreHint: '核实后，可单独确认恢复该账号并清理历史模型标记。手动停用的 IP、认证错误和冷却限制仍保留。',
    viewDetails: '查看模型不一致记录及调度状态',
    title: '降智 · 模型不一致',
    paused: '已暂停调度',
    observed: '仅记录，未自动暂停',
    observedExplanation: '检测时“降智自动暂停调度”已关闭。本次仅记录 gpt-6-astra → gpt-5.6-luna，不更改调度状态；手动停用、限流及其他保护仍生效。',
    explanation: '实际发送 gpt-6-astra，但上游返回 gpt-5.6-luna，已暂停该账号及其全部固定 IP 通道的新请求。此标记根据上游响应中的模型字段判断。',
    expected: '实际请求模型',
    actual: '上游响应模型',
    detectedAt: '检测时间',
    requestId: '请求 ID',
    restoreHint: '核实上游恢复后，可确认恢复调度。原先手动暂停的 IP、其他错误和冷却限制会保留；自动暂停开关开启时，再次出现 gpt-6-astra → gpt-5.6-luna 会重新暂停。',
    confirmRestore: '确认恢复调度',
    restoreFailed: '恢复失败，请重试',
    channelsPaused: '降智：该账号的全部固定 IP 已暂停调度。请在账号列表点击“降智”，核实后确认恢复。',
    settings: {
      title: '降智自动暂停调度',
      description: '仅当实际发送 gpt-6-astra、上游返回 gpt-5.6-luna 时，自动暂停该账号及全部固定 IP 通道。切换即保存，默认开启。',
      existingHint: '关闭后，新检测仅保留红色“降智”记录，不自动暂停。已经被隔离的账号不会自动恢复，请在账号列表核实后单独确认恢复；手动停用和其他限制不受影响。',
      stateHint: '此开关独立于 STATE 功能。STATE 的模型校验、异常票据失效和重新采集仍按原设置执行。',
      on: '已开启：新检测会自动暂停调度。',
      off: '已关闭：新检测仅记录，不自动暂停调度。',
      loadFailed: '无法读取自动暂停设置，请重试。',
      saveFailed: '未能确认保存结果，请重新读取设置后再操作。'
    }
  }
}

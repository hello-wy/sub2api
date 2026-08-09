export default {
  welfare: {
    title: '福利发放记录',
    description: '查看及管理福利发放记录。',
    searchPlaceholder: '搜索邮箱或备注',
    type: {
      all: '全部类型',
      leaderboard: '排行榜奖励',
      checkin: '签到奖励',
      lottery: '幸运抽奖'
    },
    statusFilter: {
      all: '全部状态'
    },
    status: {
      success: '已发放',
      revoked: '已撤销'
    },
    dashboard: {
      totalRewards: '发放笔数',
      totalAmount: '发放总金额',
      breakdown: '签到：{checkin} · 排行榜：{leaderboard} · 抽奖：{lottery}'
    },
    table: {
      email: '用户邮箱',
      amount: '金额',
      type: '类型',
      remarks: '备注',
      status: '状态',
      createdAt: '发放时间',
      actions: '操作'
    },
    action: {
      revoke: '撤销发放',
      revokeButton: '撤销发放',
      revokeConfirmTitle: '确认撤销福利发放',
      revokeConfirmMessage: '确定要撤销这笔 {amount} 美元的福利发放吗？撤销后将从用户余额中扣除该金额。',
      revokeSuccess: '福利发放已撤销'
    }
  }
}

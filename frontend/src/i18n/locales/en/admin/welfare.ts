export default {
  welfare: {
    title: 'Welfare Records',
    description: 'Review and manage welfare distributions.',
    searchPlaceholder: 'Search email or remarks',
    type: {
      all: 'All types',
      leaderboard: 'Leaderboard reward',
      checkin: 'Daily check-in reward',
      lottery: 'Lucky draw'
    },
    statusFilter: {
      all: 'All statuses'
    },
    status: {
      success: 'Issued',
      revoked: 'Revoked'
    },
    dashboard: {
      totalRewards: 'Total distributions',
      totalAmount: 'Total amount',
      breakdown: 'Daily check-in: {checkin} · Leaderboard: {leaderboard} · Lottery: {lottery}'
    },
    table: {
      email: 'User email',
      amount: 'Amount',
      type: 'Type',
      remarks: 'Remarks',
      status: 'Status',
      createdAt: 'Issued at',
      actions: 'Actions'
    },
    action: {
      revoke: 'Revoke distribution',
      revokeButton: 'Revoke',
      revokeConfirmTitle: 'Revoke welfare distribution',
      revokeConfirmMessage: 'Revoke this welfare distribution of {amount}? This deducts the amount from the user balance.',
      revokeSuccess: 'Welfare distribution revoked'
    }
  }
}

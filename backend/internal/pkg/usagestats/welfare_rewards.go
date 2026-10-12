package usagestats

// Reward ranking always excludes the entire welfare exclusion list, regardless
// of each user's leaderboard visibility preference.
const WelfareRewardCandidatesSQL = `SELECT u.user_id, us.email, SUM(u.actual_cost)::text
FROM usage_logs u JOIN users us ON us.id = u.user_id
WHERE u.created_at >= $1 AND u.created_at < $2 AND us.role <> 'admin'
AND ` + WelfareEligibleUserSQL + `
GROUP BY u.user_id, us.email HAVING SUM(u.actual_cost) > 0
ORDER BY SUM(u.actual_cost) DESC,
SUM(u.input_tokens + u.output_tokens + u.cache_creation_tokens + u.cache_read_tokens) DESC, u.user_id ASC`

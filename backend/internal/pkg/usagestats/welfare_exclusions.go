package usagestats

const WelfareExcludedUserIDsSQL = `SELECT jsonb_array_elements_text(value::jsonb)::bigint
	FROM settings WHERE key = 'welfare_excluded_user_ids'`

// WelfareEligibleUserSQL filters before ranking/limiting and before settling
// daily rebates. The usage_logs table must use the alias u.
const WelfareEligibleUserSQL = `u.user_id NOT IN (` + WelfareExcludedUserIDsSQL + `)`

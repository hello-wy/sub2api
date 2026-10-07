package admin

func boolSetting(value *bool, current bool) bool {
	if value == nil {
		return current
	}
	return *value
}

package service

import (
	"strings"
)

const maxAccountTestSSELine = 12 << 20

func accountTestSSEData(line string) (string, bool) {
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "data:") {
		return "", false
	}
	return strings.TrimSpace(strings.TrimPrefix(line, "data:")), true
}

package timeutil

import (
	"fmt"
	"math"
	"time"
)

// FormatRelativeDuration formats a duration to a readable, relative duration.
//
//	FormatRelativeDuration(5 * time.Second) // in 5s
//	FormatRelativeDuration(-5 * time.Hour) // 5h ago
func FormatRelativeDuration(duration time.Duration) string {
	prefix := ""
	if duration > 0 {
		prefix = "in "
	}

	postfix := ""
	if duration < 0 {
		duration = -duration
		postfix = " ago"
	}

	if duration > 24*time.Hour {
		return fmt.Sprintf("%s%gd%s", prefix, math.Round(duration.Hours()/24), postfix)
	} else if duration > time.Hour {
		return fmt.Sprintf("%s%gh%s", prefix, math.Round(duration.Hours()), postfix)
	} else if duration > time.Minute {
		return fmt.Sprintf("%s%gmin%s", prefix, math.Round(duration.Minutes()), postfix)
	} else {
		return fmt.Sprintf("%s%gs%s", prefix, math.Round(duration.Seconds()), postfix)
	}
}

package timeutil

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestFormatRelativeDuration(t *testing.T) {
	testCases := []struct {
		Duration time.Duration
		Expected string
	}{
		{
			Duration: 5 * time.Second,
			Expected: "in 5s",
		},
		{
			Duration: -5 * time.Second,
			Expected: "5s ago",
		},
		{
			Duration: 5 * time.Minute,
			Expected: "in 5min",
		},
		{
			Duration: 5 * time.Hour,
			Expected: "in 5h",
		},
		{
			Duration: 25 * time.Hour,
			Expected: "in 1d",
		},
		{
			Duration: 46 * time.Hour,
			Expected: "in 2d",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Duration.String(), func(t *testing.T) {
			assert.Equal(t, testCase.Expected, FormatRelativeDuration(testCase.Duration))
		})
	}
}

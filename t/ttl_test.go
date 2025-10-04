package t

import (
	"testing"
	"testing/synctest"
	"time"

	adspots "github.com/coyotoid/admoai-take-home-challenge"
)

func TestExpiry(t *testing.T) {
	tests := []struct {
		name        string
		ttl         *int
		age         int
		expected    bool
		description string
	}{
		{
			name:        "no_ttl",
			ttl:         nil,
			age:         120,
			expected:    false,
			description: "Object with null TTL should never immediately",
		},
		{
			name:        "zero_ttl",
			ttl:         intPtr(0),
			age:         120,
			expected:    false,
			description: "Object with zero TTL should not expire",
		},
		{
			name:        "not_yet_expired",
			ttl:         intPtr(60),
			age:         30,
			expected:    false,
			description: "Object should not be expired if within TTL",
		},
		{
			name:        "just_expired",
			ttl:         intPtr(60),
			age:         61,
			expected:    true,
			description: "Object should be expired if past TTL",
		},
		{
			name:        "exactly_at_ttl_boundary",
			ttl:         intPtr(60),
			age:         60,
			expected:    false,
			description: "Ad at exact TTL boundary should not be expired",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				adspot := adspots.AdSpot{
					TTLMinutes: tt.ttl,
					CreatedAt:  adspots.ISO8601(time.Now()),
				}
				time.Sleep(time.Duration(tt.age) * time.Minute)
				result := adspot.IsExpired()
				if result != tt.expected {
					t.Errorf("[%s] IsExpired() = %v, expected %v", tt.description, result, tt.expected)
				}
			})
		})
	}
}

func intPtr(i int) *int { return &i }

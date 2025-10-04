package t

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/synctest"
	"time"

	adspots "github.com/coyotoid/admoai-take-home-challenge"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupDatabase(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	db.AutoMigrate(&adspots.AdSpot{})
	return db.Debug()
}

func TestFiltering(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		db := setupDatabase(t)
		sqlDB, err := db.DB()
		if err != nil {
			t.Fatal(err)
		}
		defer sqlDB.Close()

		server := adspots.NewServer(db)

		testCases := []struct {
			name        string
			ttl         *int
			status      bool
			shouldShow  bool
			description string
		}{
			{
				name:        "active_no_ttl",
				ttl:         nil,
				status:      true,
				shouldShow:  true,
				description: "Active ad with no TTL should always show",
			},
			{
				name:        "active_within_ttl",
				ttl:         intPtr(120), // 2 hours
				status:      true,
				shouldShow:  true,
				description: "Active ad within TTL should show",
			},
			{
				name:        "active_expired",
				ttl:         intPtr(30), // 30 minutes
				status:      true,
				shouldShow:  false,
				description: "Active ad past TTL should be filtered out",
			},
			{
				name:        "inactive",
				ttl:         nil, // 30 minutes
				status:      false,
				shouldShow:  false,
				description: "Inactive ad should not be shown no matter what",
			},
		}

		for _, tc := range testCases {
			st := adspots.Status(tc.status)
			ad := adspots.AdSpot{
				ID:         tc.name,
				Title:      "Test ad: " + tc.name,
				ImageURL:   "https://example.com/image.png",
				TTLMinutes: tc.ttl,
				Placement:  adspots.PlacementHomeScreen,
				CreatedAt:  adspots.ISO8601(time.Now()),
				Status:     &st,
			}
			err := gorm.G[adspots.AdSpot](db).Create(t.Context(), &ad)
			if err != nil {
				t.Fatalf("Failed to insert test case %s: %v", tc.name, err)
			}
		}

		time.Sleep(1 * time.Hour)

		req := httptest.NewRequest("GET", "/adspots?status=active", nil)
		w := httptest.NewRecorder()
		server.ListAdSpots(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("Expected 200, got %d: %s", w.Code, w.Body.String())
		}

		var adspots []adspots.AdSpot
		err = json.Unmarshal(w.Body.Bytes(), &adspots)
		if err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		expectedCount := 0
		for _, tc := range testCases {
			if tc.shouldShow {
				expectedCount++
			}
		}
		if len(adspots) != expectedCount {
			t.Errorf("Expected %d active ads, got %d", expectedCount, len(adspots))
		}

		for _, ad := range adspots {
			if ad.IsExpired() {
				t.Errorf("Found expired ad in active results: %s (TTL: %d, creation date: %v)", ad.ID, ad.TTLMinutes, ad.CreatedAt.String())
			}
		}
	})
}

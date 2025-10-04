package adspots

import (
	"database/sql/driver"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"gorm.io/gorm"
)

type (
	Placement int
	Status    bool
	ISO8601   time.Time

	Server struct {
		db  *gorm.DB
		mux *http.ServeMux
		rl  *RateLimiter
	}

	AdSpot struct {
		ID            string    `json:"id" gorm:"primaryKey"`
		Title         string    `json:"title"`
		ImageURL      string    `json:"imageUrl"`
		Placement     Placement `json:"placement"`
		Status        *Status   `json:"status"` // zero values bit my ass here lol
		TTLMinutes    *int      `json:"ttlMinutes,omitempty"`
		CreatedAt     ISO8601   `json:"createdAt"`
		DeactivatedAt *ISO8601  `json:"deactivatedAt,omitempty"`
	}
	CreatePayload struct {
		Title      *string `json:"title"`
		ImageURL   *string `json:"imageUrl"`
		Placement  *string `json:"placement"`
		TTLMinutes *int    `json:"ttlMinutes"`
	}
)

const (
	PlacementHomeScreen  Placement = 1
	PlacementRideSummary Placement = 2
	PlacementMapView     Placement = 3
)

var (
	statusActive   Status  = true
	statusInactive Status  = false
	StatusActive   *Status = &statusActive
	StatusInactive *Status = &statusInactive
)

var InvalidPlacement = errors.New("invalid placement value")
var InvalidStatus = errors.New("invalid status value")

func NewServer(db *gorm.DB) *Server {
	// Create rate limiter with sensible defaults
	rateLimiter := NewRateLimiter(RateLimiterConfig{
		RequestsPerSecond: 10,              // 10 requests per second
		BurstSize:         20,              // Allow bursts up to 20 requests
		CleanupInterval:   5 * time.Minute, // Cleanup every 5 minutes
	})

	server := &Server{
		db: db,
		rl: rateLimiter,
	}

	mux := http.NewServeMux()
	// Apply rate limiting to all endpoints
	mux.HandleFunc("POST /adspots", server.rl.RateLimitHandlerFunc(server.CreateAdSpot))
	mux.HandleFunc("GET /adspots/{id}", server.rl.RateLimitHandlerFunc(server.GetAdSpot))
	mux.HandleFunc("POST /adspots/{id}/deactivate", server.rl.RateLimitHandlerFunc(server.DeactivateAdSpot))
	mux.HandleFunc("GET /adspots", server.rl.RateLimitHandlerFunc(server.ListAdSpots))
	server.mux = mux

	return server
}

func (s *Server) Mux() *http.ServeMux {
	return s.mux
}

func (p Placement) String() string {
	switch p {
	case PlacementHomeScreen:
		return "home_screen"
	case PlacementRideSummary:
		return "ride_summary"
	case PlacementMapView:
		return "map_view"
	}
	return "invalid"
}

func (p *Placement) Parse(value string) error {
	switch value {
	case "home_screen":
		*p = PlacementHomeScreen
	case "ride_summary":
		*p = PlacementRideSummary
	case "map_view":
		*p = PlacementMapView
	default:
		return InvalidPlacement
	}
	return nil
}

func (p Placement) MarshalJSON() ([]byte, error) {
	buf := []byte{}
	if str := p.String(); str == "" {
		return buf, nil
	} else {
		return fmt.Appendf(buf, "%q", str), nil
	}
}

func (p *Placement) UnmarshalJSON(buf []byte) error {
	value := strings.Trim(string(buf), `"`)
	return p.Parse(value)
}

func (s Status) MarshalJSON() ([]byte, error) {
	if s {
		return []byte("\"active\""), nil
	}
	return []byte("\"inactive\""), nil
}

func (s *Status) UnmarshalJSON(buf []byte) error {
	value := strings.Trim(string(buf), `"`)
	switch value {
	case "active":
		*s = true
	case "inactive":
		*s = false
	default:
		return InvalidStatus
	}

	return nil
}

func (s *Status) Scan(value any) error {
	if value == nil {
		return nil
	}

	switch v := value.(type) {
	case int64:
		switch v {
		case 0:
			*s = false
		case 1:
			*s = true
		}
	case bool:
		*s = Status(v)
	default:
		return fmt.Errorf("cannot scan %T into Status", value)
	}

	return nil
}

func (s Status) Value() (driver.Value, error) {
	return bool(s), nil
}

func (tm ISO8601) String() string {
	return time.Time(tm).Format("2006-01-02T15:04:05-0700")
}

func (tm ISO8601) MarshalJSON() ([]byte, error) {
	return fmt.Appendf([]byte{}, "%q", tm.String()), nil
}

func (tm *ISO8601) UnmarshalJSON(buf []byte) error {
	value := strings.Trim(string(buf), `"`)
	tm_new, err := time.Parse("2006-01-02T15:04:05-0700", value)
	if err != nil {
		return err
	}
	*tm = ISO8601(tm_new)
	return nil
}

func (tm *ISO8601) Scan(value any) error {
	if value == nil {
		return nil
	}

	switch v := value.(type) {
	case string:
		tm_new, err := time.Parse("2006-01-02T15:04:05-0700", v)
		if err != nil {
			return err
		}
		*tm = ISO8601(tm_new)
		return nil
	case []byte:
		tm_new, err := time.Parse("2006-01-02T15:04:05-0700", string(v))
		if err != nil {
			return err
		}
		*tm = ISO8601(tm_new)
		return nil
	case time.Time:
		*tm = ISO8601(v)
		return nil
	default:
		return fmt.Errorf("cannot scan %T into ISO8601", value)
	}
}

func (tm ISO8601) Value() (driver.Value, error) {
	return tm.String(), nil
}

func (a AdSpot) IsExpired() bool {
	if a.TTLMinutes == nil || *a.TTLMinutes == 0 {
		return false
	}
	expirationTime := time.Time(a.CreatedAt).Add(time.Duration(*a.TTLMinutes) * time.Minute)
	return time.Now().After(expirationTime)
}

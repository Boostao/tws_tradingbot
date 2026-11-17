package bot

import (
	"fmt"
	"log"
	"strings"
	"time"
)

// SessionType describes the active market hours phase.
type SessionType string

const (
	SessionRTH     SessionType = "RTH"      // Regular Trading Hours: 09:30 - 16:00 ET
	SessionPreETH  SessionType = "PRE_ETH"  // Pre-Market: 04:00 - 09:30 ET
	SessionPostETH SessionType = "POST_ETH" // After-Hours: 16:00 - 20:00 ET
	SessionClosed  SessionType = "CLOSED"   // Closed (overnight or weekend)
)

// SessionGate controls market access and session verification using ET (America/New_York).
type SessionGate struct {
	loc *time.Location
}

// NewSessionGate initializes a SessionGate with America/New_York timezone.
func NewSessionGate() *SessionGate {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		log.Printf("Warning: failed to load America/New_York timezone, falling back to fixed EDT: %v", err)
		loc = time.FixedZone("EDT", -4*3600)
	}
	return &SessionGate{loc: loc}
}

// Location returns the configured timezone location.
func (sg *SessionGate) Location() *time.Location {
	return sg.loc
}

// InET converts any time to Eastern Time.
func (sg *SessionGate) InET(t time.Time) time.Time {
	return t.In(sg.loc)
}

// GetSession classifies time t into RTH, PRE_ETH, POST_ETH, or CLOSED.
func (sg *SessionGate) GetSession(t time.Time) SessionType {
	et := sg.InET(t)
	weekday := et.Weekday()

	// Closed on weekends
	if weekday == time.Saturday || weekday == time.Sunday {
		return SessionClosed
	}

	hour, min, _ := et.Clock()
	totalMinutes := hour*60 + min

	const (
		preMarketOpen  = 4 * 60        // 04:00 ET
		rthOpen        = 9*60 + 30     // 09:30 ET
		rthClose       = 16 * 60       // 16:00 ET
		postMarketClose = 20 * 60      // 20:00 ET
	)

	switch {
	case totalMinutes >= rthOpen && totalMinutes < rthClose:
		return SessionRTH
	case totalMinutes >= preMarketOpen && totalMinutes < rthOpen:
		return SessionPreETH
	case totalMinutes >= rthClose && totalMinutes < postMarketClose:
		return SessionPostETH
	default:
		return SessionClosed
	}
}

// IsRTH returns true if time t is strictly within Regular Trading Hours (Mon-Fri 09:30-16:00 ET).
func (sg *SessionGate) IsRTH(t time.Time) bool {
	return sg.GetSession(t) == SessionRTH
}

// CheckSessionHours verifies whether execution is permissible.
// Returns (true, "RTH") during regular trading hours, or (false, reason) outside RTH.
func (sg *SessionGate) CheckSessionHours(t time.Time) (bool, string) {
	et := sg.InET(t)
	weekday := et.Weekday()

	if weekday == time.Saturday || weekday == time.Sunday {
		return false, fmt.Sprintf("market closed (weekend: %s)", weekday.String())
	}

	sess := sg.GetSession(t)
	timeStr := et.Format("15:04:05 MST")

	switch sess {
	case SessionRTH:
		return true, fmt.Sprintf("RTH open (%s)", timeStr)
	case SessionPreETH:
		return false, fmt.Sprintf("pre-market ETH (%s), RTH opens at 09:30 ET", timeStr)
	case SessionPostETH:
		return false, fmt.Sprintf("after-hours ETH (%s), RTH closed at 16:00 ET", timeStr)
	default:
		return false, fmt.Sprintf("market closed (%s)", timeStr)
	}
}

// CanExecuteOrder checks order validity against current session hours.
// MKT orders are strictly rejected outside RTH.
func (sg *SessionGate) CanExecuteOrder(orderType string, t time.Time) (bool, string) {
	upperType := strings.ToUpper(strings.TrimSpace(orderType))
	isRTH := sg.IsRTH(t)

	if upperType == "MKT" || upperType == "MARKET" || upperType == "" {
		if !isRTH {
			_, reason := sg.CheckSessionHours(t)
			return false, fmt.Sprintf("market orders (MKT) strictly rejected outside RTH: %s", reason)
		}
		return true, "OK"
	}

	// Limit orders may potentially be routed during ETH if OutsideRTH is explicitly configured,
	// but default fail-closed check enforces session awareness.
	if !isRTH {
		sess := sg.GetSession(t)
		if sess == SessionClosed {
			return false, "market closed (overnight / weekend)"
		}
	}

	return true, "OK"
}

// FormatSessionStatus returns an operator-friendly NY/Toronto Eastern Time session label.
func (sg *SessionGate) FormatSessionStatus(t time.Time) string {
	et := sg.InET(t)
	sess := sg.GetSession(t)
	ts := et.Format("15:04")

	switch sess {
	case SessionRTH:
		return fmt.Sprintf("NY/Toronto RTH (%s ET)", ts)
	case SessionPreETH:
		return fmt.Sprintf("NY/Toronto Pre-ETH (%s ET)", ts)
	case SessionPostETH:
		return fmt.Sprintf("NY/Toronto Post-ETH (%s ET)", ts)
	default:
		return fmt.Sprintf("NY/Toronto CLOSED (%s ET)", ts)
	}
}

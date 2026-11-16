package bot

import (
	"testing"
	"time"
)

func TestSessionGate_RTH(t *testing.T) {
	sg := NewSessionGate()
	loc := sg.Location()

	// Wednesday at 10:30 AM ET -> strictly RTH
	wedRTH := time.Date(2026, 11, 18, 10, 30, 0, 0, loc)
	if !sg.IsRTH(wedRTH) {
		t.Errorf("expected wednesday 10:30 AM ET to be RTH")
	}
	if sg.GetSession(wedRTH) != SessionRTH {
		t.Errorf("expected SessionRTH, got %v", sg.GetSession(wedRTH))
	}

	can, reason := sg.CheckSessionHours(wedRTH)
	if !can {
		t.Errorf("expected CheckSessionHours to be true during RTH, got false: %s", reason)
	}

	canExec, execReason := sg.CanExecuteOrder("MKT", wedRTH)
	if !canExec {
		t.Errorf("expected MKT order permitted during RTH, got false: %s", execReason)
	}
}

func TestSessionGate_PreMarketETH(t *testing.T) {
	sg := NewSessionGate()
	loc := sg.Location()

	// Wednesday at 08:30 AM ET -> Pre-market ETH
	wedPre := time.Date(2026, 11, 18, 8, 30, 0, 0, loc)
	if sg.IsRTH(wedPre) {
		t.Errorf("expected 08:30 AM ET to NOT be RTH")
	}
	if sg.GetSession(wedPre) != SessionPreETH {
		t.Errorf("expected SessionPreETH, got %v", sg.GetSession(wedPre))
	}

	can, reason := sg.CheckSessionHours(wedPre)
	if can {
		t.Errorf("expected CheckSessionHours to be false during Pre-market")
	}
	t.Logf("Pre-market reject reason: %s", reason)

	// MKT order must be rejected
	canExec, _ := sg.CanExecuteOrder("MKT", wedPre)
	if canExec {
		t.Errorf("expected MKT order to be rejected outside RTH")
	}
}

func TestSessionGate_PostMarketETH(t *testing.T) {
	sg := NewSessionGate()
	loc := sg.Location()

	// Wednesday at 17:30 ET -> Post-market ETH
	wedPost := time.Date(2026, 11, 18, 17, 30, 0, 0, loc)
	if sg.IsRTH(wedPost) {
		t.Errorf("expected 17:30 ET to NOT be RTH")
	}
	if sg.GetSession(wedPost) != SessionPostETH {
		t.Errorf("expected SessionPostETH, got %v", sg.GetSession(wedPost))
	}

	canExec, _ := sg.CanExecuteOrder("MKT", wedPost)
	if canExec {
		t.Errorf("expected MKT order to be rejected in after-hours")
	}
}

func TestSessionGate_WeekendClosed(t *testing.T) {
	sg := NewSessionGate()
	loc := sg.Location()

	// Saturday at 12:00 PM ET -> strictly closed
	sat := time.Date(2026, 11, 21, 12, 0, 0, 0, loc)
	if sg.IsRTH(sat) {
		t.Errorf("expected Saturday to NOT be RTH")
	}
	if sg.GetSession(sat) != SessionClosed {
		t.Errorf("expected SessionClosed, got %v", sg.GetSession(sat))
	}

	can, reason := sg.CheckSessionHours(sat)
	if can {
		t.Errorf("expected Saturday to fail CheckSessionHours")
	}
	t.Logf("Weekend reject reason: %s", reason)

	canExec, _ := sg.CanExecuteOrder("MKT", sat)
	if canExec {
		t.Errorf("expected MKT order to be rejected on weekend")
	}
}

func TestSessionGate_BoundaryMinutes(t *testing.T) {
	sg := NewSessionGate()
	loc := sg.Location()

	// 09:29:59 ET -> NOT RTH
	t1 := time.Date(2026, 11, 18, 9, 29, 59, 0, loc)
	if sg.IsRTH(t1) {
		t.Errorf("expected 09:29:59 to not be RTH")
	}

	// 09:30:00 ET -> RTH
	t2 := time.Date(2026, 11, 18, 9, 30, 0, 0, loc)
	if !sg.IsRTH(t2) {
		t.Errorf("expected 09:30:00 to be RTH")
	}

	// 15:59:59 ET -> RTH
	t3 := time.Date(2026, 11, 18, 15, 59, 59, 0, loc)
	if !sg.IsRTH(t3) {
		t.Errorf("expected 15:59:59 to be RTH")
	}

	// 16:00:00 ET -> NOT RTH (SessionPostETH)
	t4 := time.Date(2026, 11, 18, 16, 0, 0, 0, loc)
	if sg.IsRTH(t4) {
		t.Errorf("expected 16:00:00 to not be RTH")
	}
}

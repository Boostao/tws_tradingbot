package e2e

import (
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"tws_traderbot/tests/e2e/harness"
)

// Tier identifies the testing tier in the 4-tier E2E test matrix.
type Tier string

const (
	Tier1 Tier = "Tier 1: Feature Coverage"
	Tier2 Tier = "Tier 2: Boundary & Corner Cases"
	Tier3 Tier = "Tier 3: Interactions & Combinations"
	Tier4 Tier = "Tier 4: Real-World Scenarios"
	Tier5 Tier = "Tier 5: Adversarial Hardening"
)

// TestCase defines a single opaque-box test case.
type TestCase struct {
	ID          string                                                     // e.g. "E2E-T1-F04-01"
	Tier        Tier                                                       // Tier 1..4
	FeatureID   int                                                        // 1..55
	FeatureName string                                                     // e.g. "SYMBOL.VENUE Normalization"
	Description string                                                     // Behavioral specification
	Tags        []string                                                   // e.g. []string{"watchlist", "venue"}
	Setup       func(t *testing.T, h *harness.Harness)                     // Optional pre-test setup
	Action      func(t *testing.T, h *harness.Harness) (any, error)        // Execute public contract
	Assert      func(t *testing.T, h *harness.Harness, res any, err error) // Verification logic
}

// TestResult records the execution outcome of a single test case.
type TestResult struct {
	ID          string
	Tier        Tier
	FeatureID   int
	FeatureName string
	Passed      bool
	Duration    time.Duration
	Err         error
	Diagnostic  string
}

// SuiteMetrics aggregates test results across all tiers and generates executive summaries.
type SuiteMetrics struct {
	mu           sync.Mutex
	startTime    time.Time
	endTime      time.Time
	results      []TestResult
	featuresSeen map[int]bool
}

// GlobalMetrics is the singleton metrics tracker used by TestMain and test runners.
var GlobalMetrics = &SuiteMetrics{
	startTime:    time.Now(),
	results:      make([]TestResult, 0, 600),
	featuresSeen: make(map[int]bool),
}

// Record records a test result in the global suite metrics.
func (m *SuiteMetrics) Record(res TestResult) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.results = append(m.results, res)
	if res.FeatureID > 0 {
		m.featuresSeen[res.FeatureID] = true
	}
}

// FormatSummary generates the ASCII summary table for the test suite.
func (m *SuiteMetrics) FormatSummary() string {
	m.mu.Lock()
	defer m.mu.Unlock()

	var t1Total, t1Pass, t1Fail int
	var t2Total, t2Pass, t2Fail int
	var t3Total, t3Pass, t3Fail int
	var t4Total, t4Pass, t4Fail int
	var t5Total, t5Pass, t5Fail int
	var t1Dur, t2Dur, t3Dur, t4Dur, t5Dur time.Duration

	for _, r := range m.results {
		switch r.Tier {
		case Tier1:
			t1Total++
			t1Dur += r.Duration
			if r.Passed {
				t1Pass++
			} else {
				t1Fail++
			}
		case Tier2:
			t2Total++
			t2Dur += r.Duration
			if r.Passed {
				t2Pass++
			} else {
				t2Fail++
			}
		case Tier3:
			t3Total++
			t3Dur += r.Duration
			if r.Passed {
				t3Pass++
			} else {
				t3Fail++
			}
		case Tier4:
			t4Total++
			t4Dur += r.Duration
			if r.Passed {
				t4Pass++
			} else {
				t4Fail++
			}
		case Tier5:
			t5Total++
			t5Dur += r.Duration
			if r.Passed {
				t5Pass++
			} else {
				t5Fail++
			}
		}
	}

	totalTests := len(m.results)
	totalPassed := t1Pass + t2Pass + t3Pass + t4Pass + t5Pass
	totalFailed := t1Fail + t2Fail + t3Fail + t4Fail + t5Fail
	totalDuration := time.Since(m.startTime)
	featureCoverage := len(m.featuresSeen)

	t1Pct := 0.0
	if t1Total > 0 {
		t1Pct = float64(t1Pass) / float64(t1Total) * 100.0
	}
	t2Pct := 0.0
	if t2Total > 0 {
		t2Pct = float64(t2Pass) / float64(t2Total) * 100.0
	}
	t3Pct := 0.0
	if t3Total > 0 {
		t3Pct = float64(t3Pass) / float64(t3Total) * 100.0
	}
	t4Pct := 0.0
	if t4Total > 0 {
		t4Pct = float64(t4Pass) / float64(t4Total) * 100.0
	}
	t5Pct := 0.0
	if t5Total > 0 {
		t5Pct = float64(t5Pass) / float64(t5Total) * 100.0
	}
	totalPct := 0.0
	if totalTests > 0 {
		totalPct = float64(totalPassed) / float64(totalTests) * 100.0
	}

	var sb strings.Builder
	sb.WriteString("\n")
	sb.WriteString("================================================================================\n")
	sb.WriteString("              E2E OPAQUE-BOX TEST EXECUTION SUMMARY (tws_traderbot)            \n")
	sb.WriteString("================================================================================\n")
	sb.WriteString(fmt.Sprintf("Tier 1 (Feature Coverage):        %3d / %3d PASSED (%5.1f%%) [%6.2fs]\n", t1Pass, t1Total, t1Pct, t1Dur.Seconds()))
	sb.WriteString(fmt.Sprintf("Tier 2 (Boundary & Corner):       %3d / %3d PASSED (%5.1f%%) [%6.2fs]\n", t2Pass, t2Total, t2Pct, t2Dur.Seconds()))
	sb.WriteString(fmt.Sprintf("Tier 3 (Interactions):            %3d / %3d PASSED (%5.1f%%) [%6.2fs]\n", t3Pass, t3Total, t3Pct, t3Dur.Seconds()))
	sb.WriteString(fmt.Sprintf("Tier 4 (Real-World Scenarios):    %3d / %3d PASSED (%5.1f%%) [%6.2fs]\n", t4Pass, t4Total, t4Pct, t4Dur.Seconds()))
	sb.WriteString(fmt.Sprintf("Tier 5 (Adversarial Hardening):   %3d / %3d PASSED (%5.1f%%) [%6.2fs]\n", t5Pass, t5Total, t5Pct, t5Dur.Seconds()))
	sb.WriteString("--------------------------------------------------------------------------------\n")
	sb.WriteString(fmt.Sprintf("TOTAL:                           %4d / %4d PASSED (%5.1f%%) [%6.2fs]\n", totalPassed, totalTests, totalPct, totalDuration.Seconds()))
	sb.WriteString(fmt.Sprintf("FEATURE COVERAGE:                 %2d / 55 FEATURES (%5.1f%%)\n", featureCoverage, float64(featureCoverage)/55.0*100.0))
	sb.WriteString("================================================================================\n")

	if totalFailed == 0 && totalTests > 0 {
		sb.WriteString("RESULT: ALL TESTS PASSED (EXIT CODE 0)\n")
	} else {
		sb.WriteString(fmt.Sprintf("RESULT: %d TESTS FAILED (EXIT CODE 1)\n", totalFailed))
	}
	sb.WriteString("================================================================================\n")

	return sb.String()
}

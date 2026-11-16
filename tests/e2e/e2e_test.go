package e2e

import (
	"fmt"
	"os"
	"testing"
	"time"

	"tws_traderbot/tests/e2e/harness"
)

// TestMain acts as the central harness lifecycle manager and executive metrics reporter.
func TestMain(m *testing.M) {
	// Ensure working directory is repo root so relative file tests resolve accurately
	for {
		if _, err := os.Stat("go.mod"); err == nil {
			break
		}
		if err := os.Chdir(".."); err != nil {
			break
		}
	}

	GlobalMetrics.startTime = time.Now()

	exitCode := m.Run()

	GlobalMetrics.endTime = time.Now()

	// Print executive summary table to stdout
	fmt.Print(GlobalMetrics.FormatSummary())

	os.Exit(exitCode)
}

// RunTestCase executes a single TestCase against a fresh in-memory harness with diagnostic tracking.
func RunTestCase(t *testing.T, tc TestCase) {
	t.Helper()

	h, err := harness.NewHarness()
	if err != nil {
		t.Fatalf("[%s] Failed to initialize in-memory harness: %v", tc.ID, err)
	}
	defer func() {
		_ = h.Close()
	}()

	start := time.Now()
	var actionRes any
	var actionErr error
	var assertErr error

	defer func() {
		duration := time.Since(start)
		passed := !t.Failed() && assertErr == nil

		res := TestResult{
			ID:          tc.ID,
			Tier:        tc.Tier,
			FeatureID:   tc.FeatureID,
			FeatureName: tc.FeatureName,
			Passed:      passed,
			Duration:    duration,
			Err:         assertErr,
		}

		if !passed {
			t.Fail()
			diag := fmt.Sprintf(
				"\n[DIAGNOSTIC FAILURE REPORT]\n"+
					"Test ID:      %s\n"+
					"Tier:         %s\n"+
					"Feature:      #%02d %s\n"+
					"Description:  %s\n"+
					"Duration:     %v\n"+
					"Action Err:   %v\n"+
					"Assert Err:   %v\n",
				tc.ID, tc.Tier, tc.FeatureID, tc.FeatureName, tc.Description, duration, actionErr, assertErr,
			)
			res.Diagnostic = diag
			t.Log(diag)
		}

		GlobalMetrics.Record(res)
	}()

	if tc.Setup != nil {
		func() {
			defer func() {
				if r := recover(); r != nil {
					assertErr = fmt.Errorf("panic caught during setup: %v", r)
					t.Errorf("[%s] Setup panicked: %v", tc.ID, r)
					t.Fail()
				}
			}()
			tc.Setup(t, h)
		}()
		if assertErr != nil {
			return
		}
	}

	// Execute with panic recovery for diagnostics
	func() {
		defer func() {
			if r := recover(); r != nil {
				actionErr = fmt.Errorf("panic caught during action: %v", r)
				t.Errorf("[%s] Action panicked: %v", tc.ID, r)
				t.Fail()
			}
		}()
		actionRes, actionErr = tc.Action(t, h)
	}()

	func() {
		defer func() {
			if r := recover(); r != nil {
				assertErr = fmt.Errorf("assertion failure / panic: %v", r)
				t.Errorf("[%s] Assertion panicked: %v", tc.ID, r)
				t.Fail()
			}
		}()
		if tc.Assert != nil {
			tc.Assert(t, h, actionRes, actionErr)
		} else if actionErr != nil {
			assertErr = actionErr
			t.Errorf("[%s] Action returned unhandled error: %v", tc.ID, actionErr)
			t.Fail()
		}
	}()
}

// TestTier1 executes all 275 Tier 1 Feature Coverage tests.
func TestTier1(t *testing.T) {
	for _, tc := range Tier1TestCases {
		tc := tc
		t.Run(tc.ID+"_"+sanitizeTestName(tc.Description), func(t *testing.T) {
			RunTestCase(t, tc)
		})
	}
}

// TestTier2 executes all 275 Tier 2 Boundary & Corner Case tests.
func TestTier2(t *testing.T) {
	for _, tc := range Tier2TestCases {
		tc := tc
		t.Run(tc.ID+"_"+sanitizeTestName(tc.Description), func(t *testing.T) {
			RunTestCase(t, tc)
		})
	}
}

// TestTier3 executes all 15 Tier 3 Cross-Feature Interaction test suites.
func TestTier3(t *testing.T) {
	for _, tc := range Tier3TestCases {
		tc := tc
		t.Run(tc.ID+"_"+sanitizeTestName(tc.Description), func(t *testing.T) {
			RunTestCase(t, tc)
		})
	}
}

// TestTier4 executes all 6 Tier 4 Real-World Operator Scenario tests.
func TestTier4(t *testing.T) {
	for _, tc := range Tier4TestCases {
		tc := tc
		t.Run(tc.ID+"_"+sanitizeTestName(tc.Description), func(t *testing.T) {
			RunTestCase(t, tc)
		})
	}
}

// TestTier5 executes all Tier 5 Adversarial Hardening tests.
func TestTier5(t *testing.T) {
	for _, tc := range Tier5AdversarialTestCases {
		tc := tc
		t.Run(tc.ID+"_"+sanitizeTestName(tc.Description), func(t *testing.T) {
			RunTestCase(t, tc)
		})
	}
}

func sanitizeTestName(s string) string {
	res := make([]rune, 0, len(s))
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			res = append(res, r)
		} else if r == ' ' || r == '-' || r == '_' {
			res = append(res, '_')
		}
	}
	name := string(res)
	if len(name) > 40 {
		return name[:40]
	}
	return name
}

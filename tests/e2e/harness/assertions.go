package harness

import (
	"fmt"
	"os"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/scmhub/ibapi"
)

// AssertNoError fails test if err != nil.
func AssertNoError(t *testing.T, err error, msgAndArgs ...any) {
	t.Helper()
	if err != nil {
		t.Fatalf("Expected no error, but got: %v (%s)", err, formatMsg(msgAndArgs...))
	}
}

// AssertError fails test if err == nil.
func AssertError(t *testing.T, err error, msgAndArgs ...any) {
	t.Helper()
	if err == nil {
		t.Fatalf("Expected error, but got nil (%s)", formatMsg(msgAndArgs...))
	}
}

// AssertEqual fails test if actual != expected.
func AssertEqual(t *testing.T, actual, expected any, msgAndArgs ...any) {
	t.Helper()
	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("AssertEqual failed: expected %+v (%T), got %+v (%T) (%s)", expected, expected, actual, actual, formatMsg(msgAndArgs...))
	}
}

// AssertNotEqual fails test if actual == expected.
func AssertNotEqual(t *testing.T, actual, expected any, msgAndArgs ...any) {
	t.Helper()
	if reflect.DeepEqual(actual, expected) {
		t.Fatalf("AssertNotEqual failed: expected values to differ, but both were %+v (%s)", actual, formatMsg(msgAndArgs...))
	}
}

// AssertTrue fails test if cond != true.
func AssertTrue(t *testing.T, cond bool, msgAndArgs ...any) {
	t.Helper()
	if !cond {
		t.Fatalf("Expected true, got false (%s)", formatMsg(msgAndArgs...))
	}
}

// AssertFalse fails test if cond != false.
func AssertFalse(t *testing.T, cond bool, msgAndArgs ...any) {
	t.Helper()
	if cond {
		t.Fatalf("Expected false, got true (%s)", formatMsg(msgAndArgs...))
	}
}

// AssertNil fails test if obj != nil.
func AssertNil(t *testing.T, obj any, msgAndArgs ...any) {
	t.Helper()
	if obj == nil {
		return
	}
	val := reflect.ValueOf(obj)
	switch val.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		if val.IsNil() {
			return
		}
	}
	t.Fatalf("Expected nil, got %+v (%s)", obj, formatMsg(msgAndArgs...))
}

// AssertNotNil fails test if obj == nil.
func AssertNotNil(t *testing.T, obj any, msgAndArgs ...any) {
	t.Helper()
	if obj == nil {
		t.Fatalf("Expected non-nil, got nil (%s)", formatMsg(msgAndArgs...))
	}
	val := reflect.ValueOf(obj)
	switch val.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		if val.IsNil() {
			t.Fatalf("Expected non-nil, got nil pointer/slice (%s)", formatMsg(msgAndArgs...))
		}
	}
}

// AssertContract verifies fields of an IBKR Contract.
func AssertContract(t *testing.T, c *ibapi.Contract, symbol, secType, exchange, currency, primaryExch string) {
	t.Helper()
	AssertNotNil(t, c, "contract must not be nil")
	AssertEqual(t, c.Symbol, symbol, "contract Symbol mismatch")
	AssertEqual(t, c.SecType, secType, "contract SecType mismatch")
	AssertEqual(t, c.Exchange, exchange, "contract Exchange mismatch")
	AssertEqual(t, c.Currency, currency, "contract Currency mismatch")
	if primaryExch != "" {
		AssertEqual(t, c.PrimaryExchange, primaryExch, "contract PrimaryExchange mismatch")
	}
}

// AssertWithinRange fails if val is outside [min, max].
func AssertWithinRange(t *testing.T, val, min, max float64, msgAndArgs ...any) {
	t.Helper()
	if val < min || val > max {
		t.Fatalf("Value %f outside expected range [%f, %f] (%s)", val, min, max, formatMsg(msgAndArgs...))
	}
}

func formatMsg(msgAndArgs ...any) string {
	if len(msgAndArgs) == 0 {
		return ""
	}
	if format, ok := msgAndArgs[0].(string); ok && len(msgAndArgs) > 1 {
		return fmt.Sprintf(format, msgAndArgs[1:]...)
	}
	return fmt.Sprint(msgAndArgs...)
}

// ExtractTranslationKeys parses translations.ts and returns maps of keys for EN and FR.
func ExtractTranslationKeys(filePath string) (enKeys map[string]string, frKeys map[string]string, err error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, nil, err
	}
	content := string(data)
	enIdx := strings.Index(content, "en: {")
	frIdx := strings.Index(content, "fr: {")
	if enIdx == -1 || frIdx == -1 {
		return nil, nil, fmt.Errorf("could not find en: { or fr: { in %s", filePath)
	}

	enPart := content[enIdx:frIdx]
	frPart := content[frIdx:]

	extract := func(part string) map[string]string {
		res := make(map[string]string)
		re := regexp.MustCompile(`(?m)^\s*(?:'([^']+)'|"([^"]+)"|([a-zA-Z0-9_]+))\s*:\s*(.*)$`)
		matches := re.FindAllStringSubmatch(part, -1)
		for _, m := range matches {
			k := m[1]
			if k == "" {
				k = m[2]
			}
			if k == "" {
				k = m[3]
			}
			if k == "en" || k == "fr" || k == "" {
				continue
			}
			val := strings.TrimSpace(m[4])
			val = strings.Trim(val, `,"'`)
			res[k] = val
		}
		return res
	}

	return extract(enPart), extract(frPart), nil
}


package payment

import (
	"strings"
	"testing"
)

func TestNormalizeOrderTypeDefaultsEmptyToBalance(t *testing.T) {
	got, ok := NormalizeOrderType("")
	if !ok || got != OrderTypeBalance {
		t.Fatalf("NormalizeOrderType(empty) = (%q,%v), want (%q,true)", got, ok, OrderTypeBalance)
	}
}

func TestNormalizeOrderTypeAllowsKnownValues(t *testing.T) {
	for _, input := range []string{OrderTypeBalance, OrderTypeSubscription, OrderTypeTrafficPack, " subscription "} {
		got, ok := NormalizeOrderType(input)
		if !ok {
			t.Fatalf("NormalizeOrderType(%q) rejected", input)
		}
		if got != strings.TrimSpace(input) {
			t.Fatalf("NormalizeOrderType(%q) = %q", input, got)
		}
	}
}

func TestNormalizeOrderTypeRejectsUnknownNonEmpty(t *testing.T) {
	got, ok := NormalizeOrderType("evil")
	if ok || got != "" {
		t.Fatalf("NormalizeOrderType(evil) = (%q,%v), want empty false", got, ok)
	}
}

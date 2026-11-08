//go:build linux

package core

import "testing"

func TestProcessIdentityParsesCommandParenthesesAndStartTicks(t *testing.T) {
	stat := "123 (engine (worker)) S 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15 16 17 18 999 21"
	if got := processStartTicks(stat); got != "999" {
		t.Fatalf("wrong starttime: %s", got)
	}
	if processStartTicks("invalid stat") != "" {
		t.Fatal("invalid process stat gained an identity")
	}
}

//go:build windows

package core

import (
	"path/filepath"
	"testing"
)

func TestInstanceLockCannotBeSharedAndCanReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".instance.lock")
	unlock, err := AcquireInstance(path)
	if err != nil {
		t.Fatal(err)
	}
	if second, err := AcquireInstance(path); err == nil {
		second()
		t.Fatal("two queues can open the same state")
	}
	unlock()
	again, err := AcquireInstance(path)
	if err != nil {
		t.Fatal(err)
	}
	again()
}

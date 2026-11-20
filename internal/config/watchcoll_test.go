package config

import (
	"runtime"
	"testing"
)

func TestReconcileWatchCollections(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("windows")
	}
	dirs := []WatchDir{
		{Path: `D:\papers\A\test`, Enable: true},
	}
	out, _ := ReconcileWatchCollections(dirs)
	if out[0].CollectionName != "test" {
		t.Fatalf("unique: %q", out[0].CollectionName)
	}

	dirs = []WatchDir{
		{Path: `D:\papers\A\test`, Enable: true, CollectionName: "test", CollectionID: "KEY1"},
		{Path: `D:\papers\B\test`, Enable: true},
	}
	out, renames := ReconcileWatchCollections(dirs)
	if out[0].CollectionName != "A/test" || out[1].CollectionName != "B/test" {
		t.Fatalf("clash: %#v %#v", out[0], out[1])
	}
	if len(renames) != 1 || renames[0].CollectionID != "KEY1" || renames[0].NewName != "A/test" {
		t.Fatalf("renames: %#v", renames)
	}
}

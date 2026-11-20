package index

import (
	"path/filepath"
	"testing"
)

func TestInsertAndFind(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(filepath.Join(dir, "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	r := Record{
		ServerID: "s1", FileHash: "abc", Path: "a.pdf",
		DOI: "10.1/x", TitleNorm: "hello world title long enough",
		ItemKey: "ITEM1", AttachmentKey: "ATT1", Source: "crossref",
	}
	if err := s.Insert(r); err != nil {
		t.Fatal(err)
	}
	got, err := s.FindByDOI("s1", "10.1/x")
	if err != nil || got == nil || got.ItemKey != "ITEM1" {
		t.Fatalf("%v %#v", err, got)
	}
	got, _ = s.FindByHash("s1", "abc")
	if got == nil {
		t.Fatal("hash miss")
	}
}

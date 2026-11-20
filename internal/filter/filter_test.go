package filter

import "testing"

func TestShouldProcess(t *testing.T) {
	cases := []struct {
		path string
		ok   bool
	}{
		{`D:\sync\paper.pdf`, true},
		{`D:\sync\paper.PDF`, true},
		{`D:\sync\note.docx`, true},
		{`D:\sync\old.doc`, false},
		{`D:\sync\.hidden.pdf`, false},
		{`D:\sync\foo(冲突)\a.pdf`, false},
		{`D:\sync\a.tmp`, false},
		{`D:\sync\~$draft.docx`, false},
		{`D:\sync\.nutstore\x.pdf`, false},
	}
	for _, c := range cases {
		if got := ShouldProcess(c.path, nil); got != c.ok {
			t.Fatalf("%s: got %v want %v", c.path, got, c.ok)
		}
	}
}

package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func testStore(t *testing.T) Store { return Store{Dir: filepath.Join(t.TempDir(), "loads")} }

func TestStoreRoundTrip(t *testing.T) {
	st := testStore(t)
	want := []ItemSet{{2, Dims{40, 30, 20}}, {1, Dims{10.5, 8, 6}}}
	if err := st.Save("Tuesday run", want); err != nil {
		t.Fatal(err)
	}
	got, err := st.Load("Tuesday run")
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Errorf("Load = %v, %v; want %v", got, err, want)
	}
}

func TestStoreOverwrite(t *testing.T) {
	st := testStore(t)
	st.Save("a", []ItemSet{{1, Dims{1, 1, 1}}})
	st.Save("a", []ItemSet{{5, Dims{2, 2, 2}}})
	got, _ := st.Load("a")
	if len(got) != 1 || got[0].Qty != 5 {
		t.Errorf("Load = %v, want the second save", got)
	}
	names, _ := st.List()
	if len(names) != 1 {
		t.Errorf("List = %v, want one entry (and no temp files)", names)
	}
}

func TestStoreListSortedAndMissingDir(t *testing.T) {
	st := testStore(t)
	if names, err := st.List(); err != nil || len(names) != 0 {
		t.Errorf("empty List = %v, %v", names, err)
	}
	for _, n := range []string{"b", "a", "c"} {
		st.Save(n, nil)
	}
	if names, _ := st.List(); !reflect.DeepEqual(names, []string{"a", "b", "c"}) {
		t.Errorf("List = %v", names)
	}
	if !st.Exists("a") || st.Exists("zzz") {
		t.Error("Exists is wrong")
	}
}

func TestValidateName(t *testing.T) {
	for _, bad := range []string{"", "  ", ".hidden", "a/b", `a\b`, "..", "x:y", "a\x00b"} {
		if _, err := ValidateName(bad); err == nil {
			t.Errorf("ValidateName(%q) accepted a bad name", bad)
		}
	}
	if n, err := ValidateName("  Monday #2 "); err != nil || n != "Monday #2" {
		t.Errorf("ValidateName = %q, %v", n, err)
	}
	if err := testStore(t).Save("../escape", nil); err == nil {
		t.Error("Save accepted a path-like name")
	}
}

func TestStoreRejectsCorruptAndInvalid(t *testing.T) {
	st := testStore(t)
	os.MkdirAll(st.Dir, 0o755)
	os.WriteFile(filepath.Join(st.Dir, "bad.json"), []byte("{not json"), 0o644)
	if _, err := st.Load("bad"); err == nil {
		t.Error("Load accepted corrupt JSON")
	}
	os.WriteFile(filepath.Join(st.Dir, "neg.json"), []byte(`{"sets":[{"qty":1,"l":-1,"w":1,"h":1}]}`), 0o644)
	if _, err := st.Load("neg"); err == nil {
		t.Error("Load accepted a negative dimension")
	}
}

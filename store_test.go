package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func testStore(t *testing.T) Store { return Store{Dir: filepath.Join(t.TempDir(), "loads")} }

func TestStoreRoundTrip(t *testing.T) {
	st := testStore(t)
	want := []ItemSet{{Qty: 2, Dims: Dims{40, 30, 20}}, {Qty: 1, Dims: Dims{10.5, 8, 6}}}
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
	st.Save("a", []ItemSet{{Qty: 1, Dims: Dims{1, 1, 1}}})
	st.Save("a", []ItemSet{{Qty: 5, Dims: Dims{2, 2, 2}}})
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

func TestStoreKeepsWeights(t *testing.T) {
	st := testStore(t)
	want := []ItemSet{{Qty: 2, Weight: 12.5, Dims: Dims{4, 3, 2}}, {Qty: 1, Dims: Dims{1, 1, 1}}}
	if err := st.Save("w", want); err != nil {
		t.Fatal(err)
	}
	got, err := st.Load("w")
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Errorf("Load = %v, %v; want %v", got, err, want)
	}
	raw, _ := os.ReadFile(filepath.Join(st.Dir, "w.json"))
	if strings.Count(string(raw), `"weight"`) != 1 {
		t.Errorf("an unset weight should be omitted from the file:\n%s", raw)
	}
}

func TestStoreLoadsFilesWithoutWeight(t *testing.T) {
	st := testStore(t)
	os.MkdirAll(st.Dir, 0o755)
	os.WriteFile(filepath.Join(st.Dir, "old.json"), []byte(`{"sets":[{"qty":2,"l":4,"w":3,"h":2}]}`), 0o644)
	got, err := st.Load("old")
	if err != nil || len(got) != 1 || got[0].Weight != 0 {
		t.Errorf("Load = %v, %v; want a saved load from before weights to open", got, err)
	}
}

func TestStoreRejectsNegativeWeight(t *testing.T) {
	st := testStore(t)
	os.MkdirAll(st.Dir, 0o755)
	os.WriteFile(filepath.Join(st.Dir, "bad.json"), []byte(`{"sets":[{"qty":1,"weight":-5,"l":1,"w":1,"h":1}]}`), 0o644)
	if _, err := st.Load("bad"); err == nil {
		t.Error("Load accepted a negative weight")
	}
}

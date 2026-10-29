package main

import "testing"

func TestFitsInside(t *testing.T) {
	tests := []struct {
		name string
		item Dims
		want bool
	}{
		{"exact fit", Dims{182, 70, 63}, true},
		{"rotated", Dims{70, 182, 10}, true},
		{"rotated all axes", Dims{63, 70, 182}, true},
		{"too long", Dims{183, 1, 1}, false},
		{"too wide in every orientation", Dims{100, 100, 1}, false},
		{"small", Dims{1, 1, 1}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FitsInside(tt.item, DefaultSpace); got != tt.want {
				t.Errorf("FitsInside(%v) = %v, want %v", tt.item, got, tt.want)
			}
		})
	}
}

func TestLoadVolumes(t *testing.T) {
	l := Load{Space: DefaultSpace, Sets: []ItemSet{{Qty: 2, Dims: Dims{40, 30, 20}}}}
	if got := l.Capacity(); got != 802620 {
		t.Errorf("Capacity = %v, want 802620", got)
	}
	if got := l.Used(); got != 48000 {
		t.Errorf("Used = %v, want 48000", got)
	}
	if got := l.Available(); got != 754620 {
		t.Errorf("Available = %v, want 754620", got)
	}
	if !l.Fits() {
		t.Error("Fits = false, want true")
	}
}

func TestEmptyLoad(t *testing.T) {
	l := Load{Space: DefaultSpace}
	if l.Used() != 0 || l.Available() != l.Capacity() || !l.Fits() {
		t.Errorf("empty load: used=%v available=%v fits=%v", l.Used(), l.Available(), l.Fits())
	}
}

func TestOverCapacity(t *testing.T) {
	l := Load{Space: DefaultSpace, Sets: []ItemSet{{Qty: 100, Dims: Dims{50, 50, 50}}}}
	if l.Available() >= 0 {
		t.Errorf("Available = %v, want negative", l.Available())
	}
	if l.Fits() {
		t.Error("Fits = true, want false")
	}
	if len(l.Oversized()) != 0 {
		t.Errorf("Oversized = %v, want none", l.Oversized())
	}
}

func TestOversizedItem(t *testing.T) {
	l := Load{Space: DefaultSpace, Sets: []ItemSet{
		{Qty: 1, Dims: Dims{10, 10, 10}},
		{Qty: 1, Dims: Dims{200, 10, 10}},
	}}
	got := l.Oversized()
	if len(got) != 1 || got[0] != 1 {
		t.Errorf("Oversized = %v, want [1]", got)
	}
	if l.Fits() {
		t.Error("Fits = true, want false even though volume is small")
	}
}

func TestAddMerge(t *testing.T) {
	l := Load{Space: DefaultSpace}
	l.Add(ItemSet{2, Dims{10, 10, 10}}, true)
	l.Add(ItemSet{1, Dims{20, 10, 10}}, true)
	if i := l.Add(ItemSet{3, Dims{10, 10, 10}}, true); i != 0 {
		t.Errorf("merged index = %d, want 0", i)
	}
	if len(l.Sets) != 2 || l.Sets[0].Qty != 5 {
		t.Errorf("Sets = %v, want 2 sets with first qty 5", l.Sets)
	}
}

func TestAddNoMerge(t *testing.T) {
	l := Load{Space: DefaultSpace}
	l.Add(ItemSet{2, Dims{10, 10, 10}}, false)
	l.Add(ItemSet{3, Dims{10, 10, 10}}, false)
	if len(l.Sets) != 2 {
		t.Errorf("Sets = %v, want 2 separate sets", l.Sets)
	}
}

func TestReplaceMergesIntoDuplicate(t *testing.T) {
	l := Load{Space: DefaultSpace, Sets: []ItemSet{
		{2, Dims{10, 10, 10}},
		{1, Dims{20, 10, 10}},
	}}
	i := l.Replace(1, ItemSet{4, Dims{10, 10, 10}}, true)
	if i != 0 || len(l.Sets) != 1 || l.Sets[0].Qty != 6 {
		t.Errorf("index=%d Sets=%v, want index 0, one set with qty 6", i, l.Sets)
	}
}

func TestMergeDuplicates(t *testing.T) {
	l := Load{Sets: []ItemSet{
		{1, Dims{1, 2, 3}}, {1, Dims{4, 5, 6}}, {2, Dims{1, 2, 3}},
	}}
	l.MergeDuplicates()
	if len(l.Sets) != 2 || l.Sets[0].Qty != 3 || l.Sets[1].Qty != 1 {
		t.Errorf("Sets = %v", l.Sets)
	}
}

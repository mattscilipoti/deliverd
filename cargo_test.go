package main

import "testing"

// testSpace is an arbitrary roomy box so the tests don't depend on vehicle data.
var testSpace = Dims{L: 182, W: 70, H: 63}

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
			if got := FitsInside(tt.item, testSpace); got != tt.want {
				t.Errorf("FitsInside(%v) = %v, want %v", tt.item, got, tt.want)
			}
		})
	}
}

func TestLoadVolumes(t *testing.T) {
	l := Load{Space: testSpace, Sets: []ItemSet{{Qty: 2, Dims: Dims{40, 30, 20}}}}
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
	l := Load{Space: testSpace}
	if l.Used() != 0 || l.Available() != l.Capacity() || !l.Fits() {
		t.Errorf("empty load: used=%v available=%v fits=%v", l.Used(), l.Available(), l.Fits())
	}
}

func TestOverCapacity(t *testing.T) {
	l := Load{Space: testSpace, Sets: []ItemSet{{Qty: 100, Dims: Dims{50, 50, 50}}}}
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
	l := Load{Space: testSpace, Sets: []ItemSet{
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
	l := Load{Space: testSpace}
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
	l := Load{Space: testSpace}
	l.Add(ItemSet{2, Dims{10, 10, 10}}, false)
	l.Add(ItemSet{3, Dims{10, 10, 10}}, false)
	if len(l.Sets) != 2 {
		t.Errorf("Sets = %v, want 2 separate sets", l.Sets)
	}
}

func TestReplaceMergesIntoDuplicate(t *testing.T) {
	l := Load{Space: testSpace, Sets: []ItemSet{
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

func TestDollyCountsTowardUsedVolume(t *testing.T) {
	l := Load{Space: testSpace, Sets: []ItemSet{{1, Dims{10, 10, 10}}}}
	if got := l.Used(); got != 1000 {
		t.Fatalf("Used without dolly = %v, want 1000", got)
	}
	l.Dolly = DollyWithItems
	if got := l.Used(); got != 1000+48*24*6 {
		t.Errorf("Used with dolly = %v, want %v", got, 1000+48*24*6)
	}
	if len(l.Sets) != 1 {
		t.Error("including the dolly must not modify Sets")
	}
}

func TestDollyTooBigForSpace(t *testing.T) {
	l := Load{Space: Dims{40, 20, 10}, Dolly: DollyWithItems}
	got := l.Oversized()
	if len(got) != 1 || got[0] != len(l.Sets) || l.Fits() {
		t.Errorf("Oversized=%v Fits=%v, want the dolly flagged and not fitting", got, l.Fits())
	}
}

func TestDollyBehindSeatsTakesNoCargoSpace(t *testing.T) {
	l := Load{Space: Dims{40, 20, 10}, Sets: []ItemSet{{1, Dims{10, 10, 10}}}}
	base := l.Used()
	l.Dolly = DollyBehindSeats
	// The dolly would not fit this box if it were counted.
	if l.Used() != base || len(l.All()) != 1 || len(l.Oversized()) != 0 || !l.Fits() {
		t.Errorf("Used=%v All=%v Oversized=%v Fits=%v; a stowed dolly should be ignored",
			l.Used(), l.All(), l.Oversized(), l.Fits())
	}
}

func TestDollyModeCycleAndParse(t *testing.T) {
	m := DollyOff
	var seen []DollyMode
	for i := 0; i < 4; i++ {
		m = m.Next()
		seen = append(seen, m)
	}
	if seen[0] != DollyBehindSeats || seen[1] != DollyWithItems || seen[2] != DollyOff || seen[3] != DollyBehindSeats {
		t.Errorf("cycle = %v, want behind seats, with items, off, behind seats", seen)
	}
	for in, want := range map[string]DollyMode{"off": DollyOff, "": DollyOff, "seats": DollyBehindSeats, "items": DollyWithItems} {
		if got, err := ParseDollyMode(in); err != nil || got != want {
			t.Errorf("ParseDollyMode(%q) = %v, %v; want %v", in, got, err, want)
		}
	}
	if _, err := ParseDollyMode("garage"); err == nil {
		t.Error("ParseDollyMode accepted an unknown value")
	}
}

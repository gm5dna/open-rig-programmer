// SPDX-License-Identifier: GPL-3.0-or-later

package spec

import (
	"fmt"
	"reflect"
	"testing"
)

// populate fills every field reachable from v with a non-zero value,
// allocating every slice, map and pointer it meets. It is what makes the
// aliasing check below FUTURE-PROOF: a slice field added to Capabilities
// later is populated here without anyone editing this test, so if Clone
// forgets it the check fails rather than passing quietly.
func populate(v reflect.Value) {
	switch v.Kind() {
	case reflect.Slice:
		v.Set(reflect.MakeSlice(v.Type(), 2, 2))
		for i := 0; i < v.Len(); i++ {
			populate(v.Index(i))
		}
	case reflect.Map:
		v.Set(reflect.MakeMap(v.Type()))
		key := reflect.New(v.Type().Key()).Elem()
		populate(key)
		elem := reflect.New(v.Type().Elem()).Elem()
		populate(elem)
		v.SetMapIndex(key, elem)
	case reflect.Pointer:
		v.Set(reflect.New(v.Type().Elem()))
		populate(v.Elem())
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			populate(v.Field(i))
		}
	case reflect.String:
		v.SetString("x")
	case reflect.Bool:
		v.SetBool(true)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		v.SetInt(1)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		v.SetUint(1)
	}
}

// checkNoAlias walks a populated original and its clone in step and
// reports two failures: a slice/map/pointer left empty by populate (which
// would make the aliasing check vacuous for that field), and one whose
// clone shares the original's backing array, map header or pointee.
func checkNoAlias(t *testing.T, orig, clone reflect.Value, path string) {
	t.Helper()
	switch orig.Kind() {
	case reflect.Slice, reflect.Map:
		if orig.Len() == 0 {
			t.Errorf("%s: populate left this %s empty, so nothing is proved about it", path, orig.Kind())
			return
		}
		if orig.Pointer() == clone.Pointer() {
			t.Errorf("%s: Clone aliases the original's %s", path, orig.Kind())
			return
		}
		if orig.Kind() == reflect.Slice {
			for i := 0; i < orig.Len(); i++ {
				checkNoAlias(t, orig.Index(i), clone.Index(i), fmt.Sprintf("%s[%d]", path, i))
			}
		}
	case reflect.Pointer:
		if orig.IsNil() {
			t.Errorf("%s: populate left this pointer nil, so nothing is proved about it", path)
			return
		}
		if orig.Pointer() == clone.Pointer() {
			t.Errorf("%s: Clone aliases the original's pointee", path)
			return
		}
		checkNoAlias(t, orig.Elem(), clone.Elem(), path+".*")
	case reflect.Struct:
		for i := 0; i < orig.NumField(); i++ {
			checkNoAlias(t, orig.Field(i), clone.Field(i), path+"."+orig.Type().Field(i).Name)
		}
	}
}

func TestCapabilitiesClone_SharesNothingWithTheOriginal(t *testing.T) {
	var caps Capabilities
	populate(reflect.ValueOf(&caps).Elem())

	clone := caps.Clone()
	if !reflect.DeepEqual(caps, clone) {
		t.Fatalf("Clone changed the value:\n got %+v\nwant %+v", clone, caps)
	}
	checkNoAlias(t, reflect.ValueOf(caps), reflect.ValueOf(clone), "Capabilities")
}

func TestCapabilitiesClone_NilStaysNil(t *testing.T) {
	clone := Capabilities{Model: "FT-710"}.Clone()
	if clone.Banks != nil || clone.Modes != nil || clone.CTCSSToneRange != nil || clone.AntennaOptions != nil {
		t.Errorf("Clone allocated where the original was nil: %+v", clone)
	}
}

func TestCapabilitiesBankOf(t *testing.T) {
	caps := Capabilities{Banks: []Bank{
		{ID: BankMemory, Slots: []string{"001", "002"}},
		{ID: BankPMS, Slots: []string{"P1L", "P1U"}},
		{ID: BankScan},
	}}
	for _, tc := range []struct {
		name string
		slot string
		want BankID
		ok   bool
	}{
		{"first bank", "001", BankMemory, true},
		{"later slot of the first bank", "002", BankMemory, true},
		{"second bank", "P1U", BankPMS, true},
		{"unknown slot", "999", "", false},
		{"empty slot string", "", "", false},
		{"bank with no slots claims nothing", "SCAN", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := caps.BankOf(tc.slot)
			if got != tc.want || ok != tc.ok {
				t.Errorf("BankOf(%q) = %q, %v; want %q, %v", tc.slot, got, ok, tc.want, tc.ok)
			}
		})
	}
}

func TestCapabilitiesBankOf_NoBanks(t *testing.T) {
	if got, ok := (Capabilities{}).BankOf("001"); got != "" || ok {
		t.Errorf("BankOf on a bankless Capabilities = %q, %v; want \"\", false", got, ok)
	}
}

func TestNumberedSlots(t *testing.T) {
	for _, tc := range []struct {
		name   string
		lo, hi int
		format string
		want   []string
	}{
		{"the Icom memSlots loop", 1, 3, "%03d", []string{"001", "002", "003"}},
		{"single slot", 5, 5, "%03d", []string{"005"}},
		{"zero-based", 0, 2, "%02d", []string{"00", "01", "02"}},
		{"a non-numeric prefix", 1, 2, "M-%02d", []string{"M-01", "M-02"}},
		{"empty range", 3, 2, "%03d", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := NumberedSlots(tc.lo, tc.hi, tc.format)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("NumberedSlots(%d, %d, %q) = %v; want %v", tc.lo, tc.hi, tc.format, got, tc.want)
			}
		})
	}
}

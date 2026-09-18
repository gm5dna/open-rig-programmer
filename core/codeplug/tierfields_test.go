// SPDX-License-Identifier: GPL-3.0-or-later

package codeplug

import (
	"reflect"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/spec"
)

func TestTierFields_SeventeenRowsAllDistinct(t *testing.T) {
	if len(TierFields) != 20 {
		t.Fatalf("TierFields has %d rows; want 20 — the two Icom extensions added ten then seven, and v1.10.0 added the TS-2000 satellite bank's three", len(TierFields))
	}
	names := map[string]bool{}
	fields := map[spec.Field]bool{}
	columns := map[string]bool{}
	receivers := 0
	for _, row := range TierFields {
		if names[row.Name] {
			t.Errorf("duplicate Name %q", row.Name)
		}
		if fields[row.Field] {
			t.Errorf("%s: duplicate Field %q", row.Name, row.Field)
		}
		if columns[row.Column] {
			t.Errorf("%s: duplicate Column %q", row.Name, row.Column)
		}
		names[row.Name], fields[row.Field], columns[row.Column] = true, true, true
		if row.Receiver {
			receivers++
		}
	}
	if receivers != 10 {
		t.Errorf("%d rows marked Receiver; want 10 (the D8 group's seven, plus v1.10.0's TS-2000 satellite bank three — Receiver's own doc comment)", receivers)
	}
}

// TestTierFields_AccessorsReachTheirOwnField is the check that keeps a
// copy-pasted row from silently pointing at its neighbour's field: it
// mutates through each row's accessors and then asks the STRUCT which
// field actually moved.
func TestTierFields_AccessorsReachTheirOwnField(t *testing.T) {
	for _, row := range TierFields {
		t.Run(row.Name, func(t *testing.T) {
			if _, ok := reflect.TypeOf(ChannelData{}).FieldByName(row.Name); !ok {
				t.Fatalf("ChannelData has no field named %q", row.Name)
			}

			var d ChannelData
			if got := *row.State(&d); got != Absent {
				t.Errorf("State on a zero ChannelData = %q; want Absent", got)
			}
			*row.State(&d) = Known
			if changed := changedFieldNames(t, d); len(changed) != 1 || changed[0] != row.Name {
				t.Errorf("State mutated %v; want exactly [%s]", changed, row.Name)
			}

			// SetState must replace the whole field, so no value can
			// survive a transition out of Known.
			d = ChannelData{}
			row.SetState(&d, Known)
			if changed := changedFieldNames(t, d); len(changed) != 1 || changed[0] != row.Name {
				t.Errorf("SetState mutated %v; want exactly [%s]", changed, row.Name)
			}
			if got := *row.State(&d); got != Known {
				t.Errorf("State after SetState(Known) = %q; want Known", got)
			}
			// A non-Known state must carry a zero Value: SetState replaces
			// the field rather than writing its State alone.
			d = ChannelData{}
			setSomeValue(t, &d, row.Name)
			row.SetState(&d, Unavailable)
			if v := reflect.ValueOf(d).FieldByName(row.Name).FieldByName("Value"); !v.IsZero() {
				t.Errorf("SetState(Unavailable) left the value %v in place", v)
			}

			// Equal compares the WHOLE field: a state change alone is a
			// difference, and a change to any other field is not.
			var zero ChannelData
			if !row.Equal(zero, zero) {
				t.Error("Equal reported two zero channels as different")
			}
			var moved ChannelData
			row.SetState(&moved, Unknown)
			if row.Equal(zero, moved) {
				t.Error("Equal ignored a state change")
			}
			other := zero
			other.Tag = "not a tier field"
			if !row.Equal(zero, other) {
				t.Error("Equal reacted to a change in a field that is not its own")
			}
		})
	}
}

// setSomeValue puts a non-zero Value into the named tri-state field of d,
// whatever that field's value type happens to be.
func setSomeValue(t *testing.T, d *ChannelData, name string) {
	t.Helper()
	v := reflect.ValueOf(d).Elem().FieldByName(name).FieldByName("Value")
	switch v.Kind() {
	case reflect.String:
		v.SetString("x")
	case reflect.Bool:
		v.SetBool(true)
	case reflect.Int, reflect.Int64:
		v.SetInt(1)
	case reflect.Uint, reflect.Uint64:
		v.SetUint(1)
	default:
		t.Fatalf("%s: unhandled Value kind %s", name, v.Kind())
	}
	if v.IsZero() {
		t.Fatalf("%s: failed to set a non-zero Value", name)
	}
}

// changedFieldNames names every ChannelData field that differs from the
// zero value.
func changedFieldNames(t *testing.T, d ChannelData) []string {
	t.Helper()
	var out []string
	v := reflect.ValueOf(d)
	for i := 0; i < v.NumField(); i++ {
		if !v.Field(i).IsZero() {
			out = append(out, v.Type().Field(i).Name)
		}
	}
	return out
}

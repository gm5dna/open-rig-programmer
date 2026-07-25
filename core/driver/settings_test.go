// SPDX-License-Identifier: GPL-3.0-or-later

package driver

import "testing"

// validDescriptor builds a small, structurally-valid two-menu descriptor
// for the table tests below to mutate.
func validDescriptor() SettingsDescriptor {
	return SettingsDescriptor{
		Version: "test@1",
		Menus: []SettingMenu{
			{
				ID:    "01",
				Label: "MENU ONE",
				Groups: []SettingGroup{
					{
						ID:    "0101",
						Label: "GROUP ONE-ONE",
						Items: []SettingItem{
							{ID: "010101", Label: "ITEM A", Display: "01-01-01"},
							{ID: "010102", Label: "ITEM B", Display: "01-01-02"},
						},
					},
				},
			},
			{
				ID:    "02",
				Label: "MENU TWO",
				Groups: []SettingGroup{
					{
						ID:    "0201",
						Label: "GROUP TWO-ONE",
						Items: []SettingItem{
							{ID: "020101", Label: "ITEM C", Display: "02-01-01"},
						},
					},
				},
			},
		},
	}
}

func TestSettingsDescriptor_Validate(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(d SettingsDescriptor) SettingsDescriptor
		wantErr bool
	}{
		{
			name:    "valid two-menu descriptor passes",
			mutate:  func(d SettingsDescriptor) SettingsDescriptor { return d },
			wantErr: false,
		},
		{
			name: "empty Version",
			mutate: func(d SettingsDescriptor) SettingsDescriptor {
				d.Version = ""
				return d
			},
			wantErr: true,
		},
		{
			name: "empty Menus slice",
			mutate: func(d SettingsDescriptor) SettingsDescriptor {
				d.Menus = nil
				return d
			},
			wantErr: true,
		},
		{
			name: "empty Groups slice on a menu",
			mutate: func(d SettingsDescriptor) SettingsDescriptor {
				d.Menus[0].Groups = nil
				return d
			},
			wantErr: true,
		},
		{
			name: "empty Items slice on a group",
			mutate: func(d SettingsDescriptor) SettingsDescriptor {
				d.Menus[0].Groups[0].Items = nil
				return d
			},
			wantErr: true,
		},
		{
			name: "empty menu ID",
			mutate: func(d SettingsDescriptor) SettingsDescriptor {
				d.Menus[0].ID = ""
				return d
			},
			wantErr: true,
		},
		{
			name: "empty menu Label",
			mutate: func(d SettingsDescriptor) SettingsDescriptor {
				d.Menus[0].Label = ""
				return d
			},
			wantErr: true,
		},
		{
			name: "empty group ID",
			mutate: func(d SettingsDescriptor) SettingsDescriptor {
				d.Menus[0].Groups[0].ID = ""
				return d
			},
			wantErr: true,
		},
		{
			name: "empty group Label",
			mutate: func(d SettingsDescriptor) SettingsDescriptor {
				d.Menus[0].Groups[0].Label = ""
				return d
			},
			wantErr: true,
		},
		{
			name: "empty item ID",
			mutate: func(d SettingsDescriptor) SettingsDescriptor {
				d.Menus[0].Groups[0].Items[0].ID = ""
				return d
			},
			wantErr: true,
		},
		{
			name: "empty item Label",
			mutate: func(d SettingsDescriptor) SettingsDescriptor {
				d.Menus[0].Groups[0].Items[0].Label = ""
				return d
			},
			wantErr: true,
		},
		{
			name: "duplicate menu ID",
			mutate: func(d SettingsDescriptor) SettingsDescriptor {
				d.Menus[1].ID = d.Menus[0].ID
				return d
			},
			wantErr: true,
		},
		{
			name: "duplicate group ID across different menus",
			mutate: func(d SettingsDescriptor) SettingsDescriptor {
				d.Menus[1].Groups[0].ID = d.Menus[0].Groups[0].ID
				return d
			},
			wantErr: true,
		},
		{
			name: "duplicate item ID across different groups",
			mutate: func(d SettingsDescriptor) SettingsDescriptor {
				d.Menus[1].Groups[0].Items[0].ID = d.Menus[0].Groups[0].Items[0].ID
				return d
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := tt.mutate(validDescriptor())
			err := d.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestSettingsDescriptor_CloneIndependence proves Clone is a genuine deep
// copy at every level: mutating the clone's Menus, a cloned Menu's
// Groups, or a cloned Group's Items must never reach the source.
func TestSettingsDescriptor_CloneIndependence(t *testing.T) {
	src := validDescriptor()
	clone := src.Clone()

	// Mutate the clone at every level.
	clone.Menus[0].ID = "MUTATED-MENU"
	clone.Menus[0].Groups[0].ID = "MUTATED-GROUP"
	clone.Menus[0].Groups[0].Items[0].ID = "MUTATED-ITEM"
	clone.Menus[0].Groups[0].Items[0].Label = "MUTATED-LABEL"
	clone.Menus = append(clone.Menus, SettingMenu{ID: "99", Label: "EXTRA", Groups: []SettingGroup{
		{ID: "9901", Label: "EXTRA GROUP", Items: []SettingItem{{ID: "990101", Label: "EXTRA ITEM"}}},
	}})
	clone.Menus[1].Groups = append(clone.Menus[1].Groups, SettingGroup{
		ID: "0299", Label: "EXTRA GROUP 2", Items: []SettingItem{{ID: "029901", Label: "EXTRA ITEM 2"}},
	})
	clone.Menus[1].Groups[0].Items = append(clone.Menus[1].Groups[0].Items, SettingItem{ID: "020199", Label: "EXTRA ITEM 3"})

	want := validDescriptor()
	if src.Menus[0].ID != want.Menus[0].ID {
		t.Errorf("source Menus[0].ID = %q, want unchanged %q — Clone leaked a mutation back to the source", src.Menus[0].ID, want.Menus[0].ID)
	}
	if src.Menus[0].Groups[0].ID != want.Menus[0].Groups[0].ID {
		t.Errorf("source Menus[0].Groups[0].ID = %q, want unchanged %q", src.Menus[0].Groups[0].ID, want.Menus[0].Groups[0].ID)
	}
	if src.Menus[0].Groups[0].Items[0].ID != want.Menus[0].Groups[0].Items[0].ID {
		t.Errorf("source item ID = %q, want unchanged %q", src.Menus[0].Groups[0].Items[0].ID, want.Menus[0].Groups[0].Items[0].ID)
	}
	if src.Menus[0].Groups[0].Items[0].Label != want.Menus[0].Groups[0].Items[0].Label {
		t.Errorf("source item Label = %q, want unchanged %q", src.Menus[0].Groups[0].Items[0].Label, want.Menus[0].Groups[0].Items[0].Label)
	}
	if len(src.Menus) != len(want.Menus) {
		t.Errorf("source Menus length = %d, want unchanged %d — appending to the clone's Menus leaked into the source", len(src.Menus), len(want.Menus))
	}
	if len(src.Menus[1].Groups) != len(want.Menus[1].Groups) {
		t.Errorf("source Menus[1].Groups length = %d, want unchanged %d", len(src.Menus[1].Groups), len(want.Menus[1].Groups))
	}
	if len(src.Menus[0].Groups[0].Items) != len(want.Menus[0].Groups[0].Items) {
		t.Errorf("source Menus[0].Groups[0].Items length = %d, want unchanged %d", len(src.Menus[0].Groups[0].Items), len(want.Menus[0].Groups[0].Items))
	}

	if err := src.Validate(); err != nil {
		t.Errorf("source Validate() after clone mutation = %v, want nil (source must remain untouched and valid)", err)
	}
}

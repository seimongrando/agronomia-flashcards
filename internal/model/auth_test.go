package model

import "testing"

func TestHasAnyRole(t *testing.T) {
	tests := []struct {
		name     string
		userRole []string
		check    []string
		want     bool
	}{
		{"exact match", []string{"admin"}, []string{"admin"}, true},
		{"one of many", []string{"student"}, []string{"professor", "student"}, true},
		{"no match", []string{"student"}, []string{"admin"}, false},
		{"empty user roles", nil, []string{"admin"}, false},
		{"empty check roles", []string{"admin"}, nil, false},
		{"both empty", nil, nil, false},
		{"multi-role user matches", []string{"student", "professor"}, []string{"professor"}, true},
		{"multi-role user no match", []string{"student", "professor"}, []string{"admin"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := AuthInfo{Roles: tt.userRole}
			if got := info.HasAnyRole(tt.check...); got != tt.want {
				t.Errorf("HasAnyRole(%v) = %v, want %v", tt.check, got, tt.want)
			}
		})
	}
}

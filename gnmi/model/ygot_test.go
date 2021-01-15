package model

import "testing"

func TestMO_UpdateType(t *testing.T) {
	m, err := NewModel(nil, nil, nil)
	if err != nil {
		t.Errorf("%v", err)
		return
	}

	tests := []struct {
		name    string
		wantErr bool
	}{
		{
			name:    "UpdateType",
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := m.UpdateType(); (err != nil) != tt.wantErr {
				t.Errorf("MO.UpdateType() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

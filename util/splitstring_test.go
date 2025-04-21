package util

import (
	"reflect"
	"testing"
)

func TestSplitString(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{name: "commas", input: "1,2,3,4", want: []string{"1", "2", "3", "4"}},
		{name: "spaces", input: "1 2 3 4", want: []string{"1", "2", "3", "4"}},
		{name: "semis", input: "1;2;3;4", want: []string{"1", "2", "3", "4"}},
		{name: "mixed and multiples", input: "1,;2 ;3, 4", want: []string{"1", "2", "3", "4"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SplitString(tt.input); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("SplitString() = %v, want %v", got, tt.want)
			}
		})
	}
}

package notes

import "testing"

func TestDeniedWinterTyresMeanSummerTyres(t *testing.T) {
	tests := []struct {
		note string
		want bool
	}{
		{"Talvirenkaita ei vielä vaihdettu.", true},
		{"Summer tyres on, no winters", true},
		{"Huom vielä kesärenkaat alla", true},
		{"Talvirenkaat alla.", false},
		{"kesärenkaat vanteineen takakontissa", false},
		{"Rengassarja mukaan. Ei kiirettä.", false},
		{"Extra set of tyres in the boot, take along", false},
	}
	for _, tc := range tests {
		if got := Parse(tc.note, 600).SummerTyres; got != tc.want {
			t.Errorf("%q: SummerTyres = %v, want %v", tc.note, got, tc.want)
		}
	}
}

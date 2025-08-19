package main

import "testing"

func TestAnalyzeBasic(t *testing.T) {
	w, v, c := Analyze("Hello, world!")
	if w != 2 || v != 3 || c != 7 {
		t.Fatalf("got (w=%d, v=%d, c=%d); want (2,3,7)", w, v, c)
	}
}

func TestAnalyzeEmpty(t *testing.T) {
	w, v, c := Analyze("")
	if w != 0 || v != 0 || c != 0 {
		t.Fatalf("got (w=%d, v=%d, c=%d); want (0,0,0)", w, v, c)
	}
}

func TestAnalyzeVowelsOnly(t *testing.T) {
	w, v, c := Analyze("a e i o u")
	if w != 5 || v != 5 || c != 0 {
		t.Fatalf("got (w=%d, v=%d, c=%d); want (5,5,0)", w, v, c)
	}
}

func TestAnalyzeConsonantsOnly(t *testing.T) {
	w, v, c := Analyze("rhythm")
	if w != 1 || v != 0 || c != 6 {
		t.Fatalf("got (w=%d, v=%d, c=%d); want (1,0,6)", w, v, c)
	}
}

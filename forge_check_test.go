package main

import "testing"

// self-check fase 2.9: persen sukses tempa + forge stone
func TestForgeSucc(t *testing.T) {
	cases := []struct{ lv, stones, want int }{
		{0, 0, 100}, {4, 0, 100}, {5, 0, 92},
		{7, 0, 76}, {10, 0, 52}, {15, 0, 12},
		{7, 3, 100}, {15, 2, 32}, {5, 1, 100},
	}
	for _, c := range cases {
		succ := 100 - maxInt(0, c.lv-4)*8
		succ += minInt(c.stones, 3) * 10
		if succ > 100 {
			succ = 100
		}
		if succ != c.want {
			t.Fatalf("lv+%d stones=%d: got %d want %d", c.lv, c.stones, succ, c.want)
		}
	}
}

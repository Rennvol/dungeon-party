package main

import "testing"

func TestScaleBoss(t *testing.T) {
	b := Boss{"x", "x", "gelap", 350, 16, 5, 5, 1500, 400}
	sb := scaleBoss(b, 3)
	// mult = 1 + 0.35*3 = 2.05; GoldWin int64 trunc: 1500*2.05 = 3074.99… → 3074
	if sb.HP != 717 || sb.ATK != 33 || sb.GoldWin != 3074 {
		t.Fatalf("scaling salah: %+v", sb)
	}
}

func TestSkillCost(t *testing.T) {
	if skillCost(SKILLS[0], 0) != 200 {
		t.Fatalf("cost base salah")
	}
	if c := skillCost(SKILLS[0], 2); c != int64(200*1.6*1.6) {
		t.Fatalf("cost lv2 salah: %d", c)
	}
}

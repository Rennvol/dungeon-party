package main

// FASE 2.7 — Dojo: latihan & upgrade skill (gold-based), boss scaling per kill

import (
	"encoding/json"
	"net/http"
)

type Skill struct {
	ID   string `json:"id"`
	Nama string `json:"nama"`
	Desc string `json:"desc"`
	Max  int    `json:"max"`
	Base int64  `json:"base"` // biaya dasar latihan
}

var SKILLS = []Skill{
	{"power_strike", "💪 Power Strike", "ATK +6% per level", 10, 200},
	{"iron_skin", "🛡️ Iron Skin", "DEF +2 per level", 10, 180},
	{"vitality", "❤️ Vitality", "HP maks +8% per level", 10, 220},
	{"cooking", "🍳 Cooking", "Panen kebun: Herbal +20% per level", 8, 250},
}

func skillLv(p *Player, id string) int {
	m, _ := p.Data["skills"].(map[string]any)
	f, _ := m[id].(float64)
	return int(f)
}

func skillCost(s Skill, lv int) int64 {
	c := s.Base
	for i := 0; i < lv; i++ {
		c = c * 16 / 10
	}
	return c
}

// GET /api/skills — daftar skill + lv + harga; POST {train:id} — latihan
func handleSkills(w http.ResponseWriter, r *http.Request) {
	pid := parseID(r.Header.Get("X-Player-ID"))
	p, err := loadPlayer(pid)
	if err != nil || p.Data["hero"] == nil {
		writeJSON(w, 400, map[string]string{"err": "belum siap"})
		return
	}
	if r.Method == http.MethodPost {
		var req struct {
			Train string `json:"train"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		var sk *Skill
		for i := range SKILLS {
			if SKILLS[i].ID == req.Train {
				sk = &SKILLS[i]
			}
		}
		if sk == nil {
			writeJSON(w, 404, map[string]string{"err": "skill gak ada"})
			return
		}
		lv := skillLv(p, sk.ID)
		if lv >= sk.Max {
			writeJSON(w, 400, map[string]string{"err": "skill sudah MAX"})
			return
		}
		cost := skillCost(*sk, lv)
		if p.Gold < cost {
			writeJSON(w, 400, map[string]string{"err": "gold kurang (butuh " + itoa(int(cost)) + ")"})
			return
		}
		p.Gold -= cost
		m, _ := p.Data["skills"].(map[string]any)
		if m == nil {
			m = map[string]any{}
		}
		m[sk.ID] = float64(lv + 1)
		p.Data["skills"] = m
		recomputeStats(p)
		savePlayerData(p)
		writeJSON(w, 200, map[string]any{"player": p, "msg": "🥋 " + sk.Nama + " naik ke lv." + itoa(lv+1) + "!"})
		return
	}
	out := []map[string]any{}
	gold := p.Gold
	for _, s := range SKILLS {
		lv := skillLv(p, s.ID)
		var cost int64
		afford := gold >= skillCost(s, lv)
		can := lv < s.Max && afford
		if can {
			cost = skillCost(s, lv)
			gold -= cost // perkiraan belanja berantai biar list jujur
		}
		out = append(out, map[string]any{
			"id": s.ID, "nama": s.Nama, "desc": s.Desc,
			"lv": lv, "max": s.Max, "cost": cost, "can": can,
			"need_gold": !afford && lv < s.Max,
		})
	}
	writeJSON(w, 200, out)
}

// ---------- BOSS SCALING ----------
// Tiap kali boss dikalahkan, kill counter naik → stat & hadiah boss berikutnya
// lebih besar (×1.35 per kill). Anti-farm: gak bisa ngunci di boss yang sama.

func bossKills(p *Player, id string) int {
	m, _ := p.Data["boss_kills"].(map[string]any)
	if m == nil {
		return 0
	}
	f, _ := m[id].(float64)
	return int(f)
}

func scaleBoss(b Boss, k int) Boss {
	mult := 1.0 + 0.35*float64(k)
	b.HP = int(float64(b.HP) * mult)
	b.ATK = int(float64(b.ATK)*mult + 0.5)
	b.GoldWin = int64(float64(b.GoldWin) * mult)
	b.XPWin = int(float64(b.XPWin) * mult)
	return b
}

func bumpKill(p *Player, id string) {
	km, _ := p.Data["boss_kills"].(map[string]any)
	if km == nil {
		km = map[string]any{}
	}
	km[id] = float64(bossKills(p, id) + 1)
	p.Data["boss_kills"] = km
}

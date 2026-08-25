package main

// FASE 3 — JOURNEY SURVIVAL: ekspedisi real-time. Party berangkat N menit,
// nguras bekal (🍞 1 + 🧪 1 per X menit). Bekal habis → HP gerus per tick.
// Selesai → pulang bawa loot gede (XP + gold + drop epic chance).
// Skill Cooking (lv) bikin bekal lebih awet: interval makan = base / (1+0.15*cookLv).

import (
	"encoding/json"
	"math/rand"
	"net/http"
	"time"
)

// konfigurasi journey
type Journey struct {
	ID      string `json:"id"`
	Nama    string `json:"nama"`
	Menit   int    `json:"menit"`
	MinLvl  int    `json:"min_lvl"`
	XP      int    `json:"xp"`
	GoldMin int64  `json:"gold_min"`
	GoldMax int64  `json:"gold_max"`
	DropPct int    `json:"drop_pct"` // % dapet drop epic-tier
}

var JOURNEYS = []Journey{
	{"j_hutan", "🌲 Hutan Kabut", 15, 1, 120, 800, 1500, 35},
	{"j_lembah", "🏔️ Lembah Pasir Berbisik", 30, 8, 320, 2500, 5000, 55},
	{"j_reruntuhan", "🏚️ Reruntuhan Kuno", 60, 15, 800, 7000, 14000, 75},
}

const (
	jrnEatMin     = 5 // interval makan dasar (menit)
	jrnHungerDmg  = 0.02
)

func cookLvOf(p *Player) int { return skillLv(p, "cooking") }

// eatEvery: interval makan efektif (detik) — cooking memperlambat lapar
func eatEvery(p *Player) float64 {
	return float64(jrnEatMin*60) / (1 + 0.15*float64(cookLvOf(p)))
}

// jrnTick: dipanggil di loadPlayer — proses ekspedisi berjalan
func jrnTick(p *Player) {
	j, ok := p.Data["journey"].(map[string]any)
	if !ok || j == nil {
		return
	}
	done, _ := j["done"].(bool)
	if done {
		return
	}
	now := float64(time.Now().Unix())
	end := toFv(j["end"])
	if now < end {
		// masih di jalan: cek kelaparan tiap interval makan
		lastEat := toFv(j["last_eat"])
		if lastEat == 0 {
			j["last_eat"] = now
			p.Data["journey"] = j
			return
		}
		for now-lastEat >= eatEvery(p) {
			// makan 1 bekal + 1 potion; kalau gak ada → HP gerus
			if takeStackInv(p, "bekal", 1) && takeStackInv(p, "potion_kecil", 1) {
				// kenyang
			} else {
				h := p.Data["hero"].(map[string]any)
				mx, _ := toF(h["hp_max"])
				cur := heroHP(p)
				h["hp"] = cur - mx*jrnHungerDmg
			}
			lastEat += eatEvery(p)
		}
		j["last_eat"] = lastEat
		p.Data["journey"] = j
		return
	}
	// selesai! hitung loot
	jr := JOURNEY_MAP[j["id"].(string)]
	lvl := int(toFv(p.Data["hero"].(map[string]any)["lvl"]))
	if lvl < jr.MinLvl { // safety
		delete(p.Data, "journey")
		return
	}
	gold := jr.GoldMin + rand.Int63n(jr.GoldMax-jr.GoldMin+1)
	xp := jr.XP * (1 + rand.Intn(3))
	p.Gold += gold
	gainXP(p, xp)
	msg := "🧭 Ekspedisi " + jr.Nama + " selesai! +" + itoa(int(gold)) + "g +" + itoa(xp) + " XP"
	drops := []string{}
	if rand.Intn(100) < jr.DropPct {
		id := rollLoot()
		if id != "" && bagHasRoom(p) {
			invE := normInv(p.Data["inv"])
			addItemSrv(invE, id, 1)
			p.Data["inv"] = invE
			drops = append(drops, ITEMS[id].Nama)
		}
	}
	delete(p.Data, "journey")
	p.Data["jrn_log"] = append(jrnLog(p), msg)
	savePlayerData(p)
}

func jrnLog(p *Player) []string {
	out := []string{}
	if a, ok := p.Data["jrn_log"].([]any); ok {
		for _, x := range a {
			if s, ok := x.(string); ok {
				out = append(out, s)
			}
		}
	}
	if len(out) > 10 {
		out = out[len(out)-10:]
	}
	return out
}

var JOURNEY_MAP = func() map[string]Journey {
	m := map[string]Journey{}
	for _, j := range JOURNEYS {
		m[j.ID] = j
	}
	return m
}()

// POST /api/journey {action:"start", id} | {action:"claim"} | {} → status
func handleJourney(w http.ResponseWriter, r *http.Request) {
	pid := parseID(r.Header.Get("X-Player-ID"))
	var req struct {
		Action string `json:"action"`
		ID     string `json:"id"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	p, err := loadPlayer(pid)
	if err != nil || p.Data["hero"] == nil {
		writeJSON(w, 400, map[string]string{"err": "belum siap"})
		return
	}
	jrnTick(p) // proses dulu sebelum balas

	switch req.Action {
	case "start":
		if raidLocked(p) {
			writeBusy(w, "ekspedisi")
			return
		}
		if _, on := p.Data["journey"]; on {
			writeJSON(w, 400, map[string]string{"err": "party masih di jalan!"})
			return
		}
		jr, ok := JOURNEY_MAP[req.ID]
		if !ok {
			writeJSON(w, 400, map[string]string{"err": "ekspedisi gak ada"})
			return
		}
		lvl := int(toFv(p.Data["hero"].(map[string]any)["lvl"]))
		if lvl < jr.MinLvl {
			writeJSON(w, 400, map[string]string{"err": "🔒 butuh Lv." + itoa(jr.MinLvl)})
			return
		}
		// cek bekal minimum: 1 bekal per interval makan × jumlah makan perkiraan
		eats := jr.Menit*60/int(eatEvery(p)) + 1
		inv := normInv(p.Data["inv"])
		st, _ := inv["stack"].(map[string]any)
		bekal, _ := st["bekal"].(float64)
		pot, _ := st["potion_kecil"].(float64)
		if bekal < float64(eats) || pot < float64(eats) {
			writeJSON(w, 400, map[string]string{
				"err": "bekal kurang! butuh 🍞" + itoa(eats) + " + 🧪" + itoa(eats) + " (" + itoa(jr.Menit) + " menit, cooking lv." + itoa(cookLvOf(p)) + ")"})
			return
		}
		now := float64(time.Now().Unix())
		p.Data["journey"] = map[string]any{
			"id": jr.ID, "start": now, "end": now + float64(jr.Menit)*60,
			"last_eat": now,
		}
		savePlayerData(p)
		writeJSON(w, 200, map[string]any{"player": p,
			"msg": "🧭 Berangkat ke " + jr.Nama + "! Balik dalam " + itoa(jr.Menit) + " menit."})
	case "claim":
		if _, on := p.Data["journey"]; on {
			writeJSON(w, 400, map[string]string{"err": "party belum pulang"})
			return
		}
		log := jrnLog(p)
		if len(log) == 0 {
			writeJSON(w, 400, map[string]string{"err": "belum ada hasil ekspedisi"})
			return
		}
		writeJSON(w, 200, map[string]any{"player": p, "log": log})
	default:
		// status: journey aktif? log?
		var cur map[string]any
		if j, ok := p.Data["journey"].(map[string]any); ok {
			cur = map[string]any{
				"id": j["id"], "start": j["start"], "end": j["end"],
				"left": int(toFv(j["end"]) - float64(time.Now().Unix())),
			}
		}
		writeJSON(w, 200, map[string]any{"player": p, "active": cur, "log": jrnLog(p)})
	}
}

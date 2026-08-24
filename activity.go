package main

// FASE 2.5 — pemisahan aktivitas (desain user):
//   🌿 KEBUN  = farm GOLD + herbal (tanaman tumbuh real-time)
//   ⚔️ DUNGEON = farm XP (+ drop kecil), turn-based ringan
//   👑 BOSS   = dadu d20, pakai potion & skill, element weakness

import (
	"encoding/json"
	"math/rand"
	"net/http"
	"time"
)

// ---------- KEBUN (gold) ----------

// state kebun di data.garden: {lv, planted_at}
// gold menumpuk = elapsed * rate; herbal tiap 45s max cap 8 jam.
func handleGarden(w http.ResponseWriter, r *http.Request) {
	pid := parseID(r.Header.Get("X-Player-ID"))
	var req struct {
		Action  string `json:"action"` // harvest | upgrade
		Collect bool   `json:"collect"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	p, err := loadPlayer(pid)
	if err != nil || p.Data["hero"] == nil {
		writeJSON(w, 400, map[string]string{"err": "belum siap"})
		return
	}

	switch req.Action {
	case "upgrade":
		lv := gardenLv(p)
		cost := int64(150)
		for i := 0; i < lv; i++ {
			cost = cost * 16 / 10
		}
		if p.Gold < cost {
			writeJSON(w, 400, map[string]string{"err": "gold kurang (butuh " + itoa(int(cost)) + ")"})
			return
		}
		harvestGarden(p) // auto-panen sisa sebelum upgrade
		p.Gold -= cost
		p.Data["garden_lv"] = float64(lv + 1)
		savePlayerData(p)
		writeJSON(w, 200, map[string]any{"player": p, "msg": "🌿 Kebun lv." + itoa(lv+1)})
	default: // collect/preview
		gold, herbs := gardenPending(p)
		if req.Collect || req.Action == "collect" {
			p.Gold += int64(gold)
			addItemSrvInv(p, "herbal", herbs)
			resetGardenClock(p)
			savePlayerData(p)
			writeJSON(w, 200, map[string]any{"player": p,
				"msg": "🌿 Panen: " + itoa(gold) + " gold" + (map[bool]string{true: " + " + itoa(herbs) + " 🌿 Herbal"})[herbs > 0]})
			return
		}
		writeJSON(w, 200, map[string]any{"player": p, "gold": gold, "herbs": herbs})
	}
}

func gardenLv(p *Player) int {
	f, _ := p.Data["garden_lv"].(float64)
	if f < 1 {
		return 1
	}
	return int(f)
}

func gardenPending(p *Player) (int, int) {
	t0f, _ := p.Data["garden_at"].(float64)
	if t0f == 0 {
		return 0, 0
	}
	elapsed := time.Now().Unix() - int64(t0f)
	if elapsed > gardenCapHours*3600 {
		elapsed = gardenCapHours * 3600
	}
	gold := int(float64(elapsed) * gardenRateGoldPerSec * float64(gardenLv(p)))
	herbs := int(elapsed / gardenHerbEverySec)
	return gold, herbs
}

func harvestGarden(p *Player) {
	g, h := gardenPending(p)
	p.Gold += int64(g)
	for ; h > 0; h-- {
		addItemSrvInv(p, "herbal", 1)
	}
	resetGardenClock(p)
}

func resetGardenClock(p *Player) { p.Data["garden_at"] = float64(time.Now().Unix()) }

// ---------- DUNGEON (XP) ----------

// POST /api/dungeon {dive:"gua_goblin"} — dive 1x per 30s window, hasil instan:
// XP sesuai power gap + chance drop kecil. Ini pengganti tick lama.
func handleDungeon(w http.ResponseWriter, r *http.Request) {
	pid := parseID(r.Header.Get("X-Player-ID"))
	var req struct{ Dive string }
	json.NewDecoder(r.Body).Decode(&req)

	p, err := loadPlayer(pid)
	if err != nil || p.Data["hero"] == nil {
		writeJSON(w, 400, map[string]string{"err": "belum siap"})
		return
	}
	d, ok := DUNGEONS[req.Dive]
	if !ok {
		writeJSON(w, 400, map[string]string{"err": "dungeon gak ada"})
		return
	}
	// cooldown antar dive
	now := time.Now().Unix()
	lastD, _ := p.Data["last_dive"].(float64)
	if now-int64(lastD) < 25 {
		writeJSON(w, 429, map[string]string{"err": "party masih istirahat (" + itoa(int(25-(now-int64(lastD)))) + "s)"})
		return
	}
	p.Data["last_dive"] = float64(now)

	xp := d.XP * (1 + rand.Intn(3)) // 2-6 xp dasar
	gainXP(p, xp)
	msg := "⚔️ Dive " + d.Nama + ": +" + itoa(xp) + " XP"

	drops := []string{}
	if rand.Intn(100) < 15 {
		id := rollLoot()
		if id != "" && addItemSrvInv(p, id, 1) {
			drops = append(drops, id)
			msg += ", drop " + ITEMS[id].Nama
		}
	}
	savePlayerData(p)
	writeJSON(w, 200, map[string]any{"player": p, "msg": msg, "drops": drops})
}

// ---------- BOSS (dadu d20 + potion) ----------

// GET /api/boss — list boss
func handleBossList(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, BOSSES)
}

// POST /api/boss {id, potion:"potion_besar"} — fight turn-based singkat:
// server simulasi: hero vs boss, roll d20 tiap giliran, crit 20, miss 1.
func handleBossFight(w http.ResponseWriter, r *http.Request) {
	pid := parseID(r.Header.Get("X-Player-ID"))
	var req struct {
		ID     string `json:"id"`
		Potion string `json:"potion"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	p, err := loadPlayer(pid)
	if err != nil || p.Data["hero"] == nil {
		writeJSON(w, 400, map[string]string{"err": "belum siap"})
		return
	}
	var boss *Boss
	for i := range BOSSES {
		if BOSSES[i].ID == req.ID {
			boss = &BOSSES[i]
		}
	}
	if boss == nil {
		writeJSON(w, 404, map[string]string{"err": "boss gak ada"})
		return
	}
	h := p.Data["hero"].(map[string]any)
	lvl := int(h["lvl"].(float64))
	if lvl < boss.MinLvl {
		writeJSON(w, 400, map[string]string{"err": "butuh level " + itoa(boss.MinLvl)})
		return
	}
	// potion wajib dibawa? opsional tapi disarankan — heal otomatis saat HP kritis
	maxHP := int(h["hp_max"].(float64))
	atk := int(h["atk"].(float64))
	def := lvl
	hp := maxHP

	// elemen hero vs boss
	heroElem := CLASSES[h["class"].(string)].Element
	mult := elemMult(heroElem, boss.Element)
	if mult > 1 {
		atk = atk * 3 / 2
	} else if mult < 1 {
		atk = atk * 3 / 4
	}

	// potion pre-fight: otomatis dipakai dari tas saat HP <= 30%
	usePotion := func() {
		for hp*10 <= maxHP*3 {
			if takeStackInv(p, "potion_kecil", 1) {
				hp += ITEMS["potion_kecil"].HP
				continue
			}
			if takeStackInv(p, "potion_besar", 1) {
				hp += ITEMS["potion_besar"].HP
				continue
			}
			break
		}
		if hp > maxHP {
			hp = maxHP
		}
	}

	logs := []string{}
	bossHP := boss.HP
	turn := 0
	for bossHP > 0 && hp > 0 && turn < 50 {
		turn++
		// hero attack — d20
		roll := rand.Intn(20) + 1
		switch {
		case roll == 20:
			dmg := atk * 2
			bossHP -= dmg
			logs = append(logs, "🎲 NAT 20! CRIT "+itoa(dmg)+" dmg!")
		case roll == 1:
			logs = append(logs, "🎲 Nat 1... meleset.")
		default:
			dmg := atk * (7 + roll) / 14 // skala roll
			bossHP -= dmg
			logs = append(logs, "🎲 "+itoa(roll)+" → hit "+itoa(dmg))
		}
		if bossHP <= 0 {
			break
		}
		// boss attack
		hp -= boss.ATK - def/2
		if hp < 0 {
			hp = 0
		}
		usePotion()
		logs = append(logs, "👹 Boss hit! HP kamu "+itoa(hp)+"/"+itoa(maxHP))
	}

	if bossHP <= 0 {
		p.Gold += boss.GoldWin
		gainXP(p, boss.XPWin)
		savePlayerData(p)
		writeJSON(w, 200, map[string]any{"win": true, "logs": logs, "player": p,
			"msg": "🏆 " + boss.Nama + " dikalahkan! +" + itoa(int(boss.GoldWin)) + "g +" + itoa(boss.XPWin) + "xp"})
		return
	}
	// kalah: tetap simpen (hp reset), gold penalty kecil
	p.Gold -= p.Gold / 20
	savePlayerData(p)
	writeJSON(w, 200, map[string]any{"win": false, "logs": logs, "player": p,
		"msg": "💀 Kalah... coba lagi nanti (-5% gold)"})
}

func takeStackInv(p *Player, id string, n int) bool {
	inv := normInv(p.Data["inv"])
	st, _ := inv["stack"].(map[string]any)
	f, ok := st[id].(float64)
	if !ok || f < float64(n) {
		return false
	}
	st[id] = f - float64(n)
	p.Data["inv"] = inv
	return true
}

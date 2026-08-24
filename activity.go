package main

// FASE 2.6 — boss turn-based (d20 per serangan), HP persisten + regen,
// potion bisa dipakai kapan pun, dive dungeon nguras HP dikit.

import (
	"encoding/json"
	"math/rand"
	"net/http"
	"time"
)

// takeStackInv pindah ke sini (dipake activity + boss)
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

// ---------- REGEN HP ----------
// 1% hp_max tiap 3 detik di luar pertarungan.
func applyRegen(p *Player) {
	hm, ok := p.Data["hero"].(map[string]any)
	if !ok || hm == nil {
		return
	}
	mx, _ := toF(hm["hp_max"])
	if mx <= 0 {
		return
	}
	cur, ok := toF(hm["hp"])
	if !ok || cur <= 0 {
		hm["hp"] = mx
		p.Data["hp_at"] = float64(time.Now().Unix())
		return
	}
	now := time.Now().Unix()
	t0, _ := toF(p.Data["hp_at"])
	if t0 == 0 {
		p.Data["hp_at"] = float64(now)
		return
	}
	el := now - int64(t0)
	if el < 0 {
		el = 0
	}
	if el >= 3 && cur < mx {
		cur += mx * 0.01 * float64(el/3)
		if cur > mx {
			cur = mx
		}
		hm["hp"] = cur
	}
	p.Data["hp_at"] = float64(now)
}

func heroHP(p *Player) float64 {
	h := p.Data["hero"].(map[string]any)
	f, _ := toF(h["hp"])
	return f
}

// ---------- KEBUN (gold) ----------

func handleGarden(w http.ResponseWriter, r *http.Request) {
	pid := parseID(r.Header.Get("X-Player-ID"))
	var req struct {
		Action string `json:"action"`
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
		harvestGarden(p)
		p.Gold -= cost
		p.Data["garden_lv"] = float64(lv + 1)
		savePlayerData(p)
		writeJSON(w, 200, map[string]any{"player": p, "msg": "🌿 Kebun lv." + itoa(lv+1)})
	case "collect":
		gold, herbs := gardenPending(p)
		p.Gold += int64(gold)
		addItemSrvInv(p, "herbal", herbs)
		resetGardenClock(p)
		savePlayerData(p)
		msg := "🌿 Panen: " + itoa(gold) + " gold"
		if herbs > 0 {
			msg += " +" + itoa(herbs) + " 🌿 Herbal"
		}
		writeJSON(w, 200, map[string]any{"player": p, "msg": msg})
	default:
		gold, herbs := gardenPending(p)
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

// ---------- DUNGEON (XP) — dive nguras HP 8% ----------

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
	now := time.Now().Unix()
	lastD, _ := p.Data["last_dive"].(float64)
	if now-int64(lastD) < 25 {
		writeJSON(w, 429, map[string]string{"err": "party masih istirahat (" + itoa(int(25-(now-int64(lastD)))) + "s)"})
		return
	}
	h := p.Data["hero"].(map[string]any)
	mx, _ := toF(h["hp_max"])
	cur := heroHP(p)
	if cur < mx*0.15 {
		writeJSON(w, 400, map[string]string{"err": "❤️ HP terlalu rendah — pakai potion atau istirahat dulu"})
		return
	}
	// dive nguras HP
	h["hp"] = cur - mx*0.08
	p.Data["last_dive"] = float64(now)

	xp := d.XP * (1 + rand.Intn(3))
	gainXP(p, xp)
	msg := "⚔️ Dive " + d.Nama + ": +" + itoa(xp) + " XP"

	drops := []string{}
	if rand.Intn(100) < 15 {
		id := rollLoot()
		if id != "" {
			if isEquipID(id) {
				if bagHasRoom(p) {
					invE := normInv(p.Data["inv"])
					addItemSrv(invE, id, 1)
					p.Data["inv"] = invE
					drops = append(drops, id)
				}
			} else if addItemSrvInv(p, id, 1) {
				drops = append(drops, id)
			}
		}
	}
	savePlayerData(p)
	writeJSON(w, 200, map[string]any{"player": p, "msg": msg, "drops": drops})
}

// ---------- BOSS — TURN-BASED, state tersimpan ----------

// POST /api/boss {action:"start"|"attack"|"potion"|"flee", id}
func handleBossFight(w http.ResponseWriter, r *http.Request) {
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

	switch req.Action {
	case "start":
		bossStart(w, p, req.ID)
	case "attack":
		bossAttack(w, p)
	case "potion":
		bossPotion(w, p)
	case "flee":
		p.Data["battle"] = nil
		savePlayerData(p)
		writeJSON(w, 200, map[string]any{"player": p, "msg": "🏃 Kabur dari pertarungan"})
	default:
		writeJSON(w, 400, map[string]string{"err": "aksi gak dikenal"})
	}
}

// GET /api/bosses — list boss TERSCALING sesuai kill count player
func handleBossList(w http.ResponseWriter, r *http.Request) {
	pid := parseID(r.Header.Get("X-Player-ID"))
	p, err := loadPlayer(pid)
	if err != nil {
		writeJSON(w, 200, BOSSES)
		return
	}
	out := []Boss{}
	for _, b := range BOSSES {
		out = append(out, scaleBoss(b, bossKills(p, b.ID)))
	}
	writeJSON(w, 200, out)
}

func findBoss(id string) *Boss {
	for i := range BOSSES {
		if BOSSES[i].ID == id {
			return &BOSSES[i]
		}
	}
	return nil
}

func bossStart(w http.ResponseWriter, p *Player, id string) {
	if _, busy := p.Data["battle"].(map[string]any); busy {
		writeJSON(w, 400, map[string]string{"err": "masih ada pertarungan aktif"})
		return
	}
	boss := findBoss(id)
	if boss == nil {
		writeJSON(w, 404, map[string]string{"err": "boss gak ada"})
		return
	}
	h := p.Data["hero"].(map[string]any)
	lvl := int(toFv(h["lvl"]))
	if lvl < boss.MinLvl {
		writeJSON(w, 400, map[string]string{"err": "butuh level " + itoa(boss.MinLvl)})
		return
	}
	if heroHP(p) < toFv(h["hp_max"])*0.3 {
		writeJSON(w, 400, map[string]string{"err": "❤️ HP di bawah 30% — sembuhkan dulu"})
		return
	}
	p.Data["battle"] = map[string]any{"boss": boss.ID, "bhp": float64(boss.HP)}
	savePlayerData(p)
	writeJSON(w, 200, map[string]any{"player": p,
		"log": []string{"⚔️ " + boss.Nama + " muncul! Giliranmu — tekan SERANG."}})
}

func toFv(v any) float64 { f, _ := v.(float64); return f }

func bossAttack(w http.ResponseWriter, p *Player) {
	bt, ok := p.Data["battle"].(map[string]any)
	if !ok {
		writeJSON(w, 400, map[string]string{"err": "gak ada pertarungan aktif"})
		return
	}
	boss := findBoss(bt["boss"].(string))
	if boss == nil {
		p.Data["battle"] = nil
		savePlayerData(p)
		writeJSON(w, 400, map[string]string{"err": "data boss rusak, battle dibatalkan"})
		return
	}
	h := p.Data["hero"].(map[string]any)
	atk := toFv(h["atk"]) * elemMult(CLASSES[h["class"].(string)].Element, boss.Element)
	def := toFv(h["def"])

	logs := []string{}

	// giliran hero: d20
	roll := rand.Intn(20) + 1
	bhp := toFv(bt["bhp"])
	switch {
	case roll == 20:
		dmg := atk * 2
		bhp -= dmg
		logs = append(logs, "🎲 NAT 20!! CRIT "+itoa(int(dmg))+" dmg 🔥")
	case roll == 1:
		logs = append(logs, "🎲 Nat 1... meleset 😅")
	default:
		dmg := atk * float64(7+roll) / 14
		bhp -= dmg
		logs = append(logs, "🎲 "+itoa(roll)+" → hit "+itoa(int(dmg)))
	}

	// menang? — boss scaling anti-farm: kill counter naik, boss berikutnya lebih kuat
	if bhp <= 0 {
		k := bossKills(p, boss.ID)
		bumpKill(p, boss.ID)
		p.Data["battle"] = nil
		p.Gold += boss.GoldWin
		gainXP(p, boss.XPWin)
		savePlayerData(p)
		logs = append(logs, "🏆 "+boss.Nama+" DIKALAHKAN! +"+itoa(int(boss.GoldWin))+"g +"+itoa(boss.XPWin)+"xp")
		nb := scaleBoss(*boss, k+1)
		logs = append(logs, "⚠️ Boss membangkitkan kekuatan baru! HP "+itoa(nb.HP)+", ATK "+itoa(nb.ATK)+" — hadiah ×1.35")
		writeJSON(w, 200, map[string]any{"player": p, "log": logs, "win": true})
		return
	}
	bt["bhp"] = bhp

	// giliran boss
	mx, _ := toF(h["hp_max"])
	cur := heroHP(p)
	dmgIn := float64(boss.ATK) - def/2
	if dmgIn < 2 {
		dmgIn = 2
	}
	cur -= dmgIn
	logs = append(logs, "👹 "+boss.Nama+" menyerang! -"+itoa(int(dmgIn))+" HP")

	// auto-potion saat kritis kalau ada stok (biar gak mati tanpa sempat klik)
	if cur > 0 && cur <= mx*0.3 {
		if takeStackInv(p, "potion_kecil", 1) {
			cur += float64(ITEMS["potion_kecil"].HP)
			logs = append(logs, "🧪 AUTO-POTION! +30 HP")
		} else if takeStackInv(p, "potion_besar", 1) {
			cur += float64(ITEMS["potion_besar"].HP)
			logs = append(logs, "🧴 AUTO-POTION BESAR! +80 HP")
		} else {
			logs = append(logs, "⚠️ Gak ada potion cadangan!")
		}
	}
	if cur > mx {
		cur = mx
	}
	h["hp"] = cur

	// kalah?
	if cur <= 0 {
		h["hp"] = 1
		p.Data["battle"] = nil
		p.Gold -= p.Gold / 20
		savePlayerData(p)
		logs = append(logs, "💀 KAMU TUMBANG... -5% gold, coba lagi!")
		writeJSON(w, 200, map[string]any{"player": p, "log": logs, "win": false})
		return
	}

	savePlayerData(p)
	writeJSON(w, 200, map[string]any{"player": p, "log": logs})
}

// potion manual — bisa dipakai di dalam ATAU luar pertarungan
func bossPotion(w http.ResponseWriter, p *Player) {
	mx, _ := toF(p.Data["hero"].(map[string]any)["hp_max"])
	cur := heroHP(p)
	if cur >= mx {
		writeJSON(w, 400, map[string]string{"err": "❤️ HP sudah penuh"})
		return
	}
	if takeStackInv(p, "potion_kecil", 1) {
		cur += float64(ITEMS["potion_kecil"].HP)
	} else if takeStackInv(p, "potion_besar", 1) {
		cur += float64(ITEMS["potion_besar"].HP)
	} else {
		writeJSON(w, 400, map[string]string{"err": "gak ada potion! beli di shop"})
		return
	}
	if cur > mx {
		cur = mx
	}
	p.Data["hero"].(map[string]any)["hp"] = cur
	p.Data["hp_at"] = float64(time.Now().Unix())
	savePlayerData(p)
	writeJSON(w, 200, map[string]any{"player": p, "msg": "🧪 HP pulih ke " + itoa(int(cur)) + "/" + itoa(int(mx))})
}

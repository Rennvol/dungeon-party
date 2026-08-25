package main

// Fase 1: pilih class, farm idle, level up, shop potion

import (
	"encoding/json"
	"math/rand"
	"net/http"
	"time"
)

func handleClasses(w http.ResponseWriter, r *http.Request) {
	list := []Class{}
	for _, c := range CLASSES {
		list = append(list, c)
	}
	writeJSON(w, 200, list)
}

// POST /api/choose {class:"warrior"} — sekali saja, kunci hero utama
func handleChoose(w http.ResponseWriter, r *http.Request) {
	pid := parseID(r.Header.Get("X-Player-ID"))
	var req struct{ Class string }
	json.NewDecoder(r.Body).Decode(&req)
	c, ok := CLASSES[req.Class]
	if !ok {
		writeJSON(w, 400, map[string]string{"err": "class gak ada"})
		return
	}
	p, err := loadPlayer(pid)
	if err != nil {
		writeJSON(w, 500, map[string]string{"err": "player gak ada"})
		return
	}
	if _, taken := p.Data["hero"]; taken {
		writeJSON(w, 409, map[string]string{"err": "hero udah dipilih"})
		return
	}
	p.Data["hero"] = map[string]any{
		"class": c.ID, "lvl": 1, "xp": 0,
		"hp_max": c.HP, "atk": c.ATK,
	}
	savePlayerData(p)
	writeJSON(w, 200, p)
}

func heroPower(h map[string]any) int {
	atk, _ := h["atk"].(float64)
	hp, _ := h["hp_max"].(float64)
	lvl, _ := h["lvl"].(float64)
	return int(atk*2 + hp/10 + lvl*5)
}

func gainXP(p *Player, xp int) {
	h := p.Data["hero"].(map[string]any)
	lvl, _ := h["lvl"].(float64)
	xpNow, _ := h["xp"].(float64)
	xpNow += float64(xp)
	for xpNow >= float64(xpNeed(int(lvl))) {
		xpNow -= float64(xpNeed(int(lvl)))
		lvl++
		c := CLASSES[h["class"].(string)]
		h["hp_max"] = c.HP + int(lvl)*8
		h["atk"] = c.ATK + int(lvl)*2
	}
	h["lvl"], h["xp"] = lvl, xpNow
	p.Data["hero"] = h
	p.Power = heroPower(h)
}

// farm tick — dipanggil client tiap detik via /api/save? TIDAK. Server-side ringan:
// client hitung sendiri offline-safe, save bawa hasil. Validasi: max 8 jam income.
func clampOffline(elapsedSec float64) float64 {
	const cap = 8 * 3600
	if elapsedSec > cap {
		return cap
	}
	if elapsedSec < 0 {
		return 0
	}
	return elapsedSec
}

var rng = rand.New(rand.NewSource(1))

func farmGold(d Dungeon) int { return d.GoldMin + rand.Intn(d.GoldMax-d.GoldMin+1) }

// POST /api/shop {item} — beli pakai gold; {item, pay:"herbal"} — tukar herbal
var SHOP = map[string]struct {
	Nama string `json:"nama"`
	Harga int64  `json:"harga"`
}{
	"potion_kecil": {"🧪 Potion HP Kecil (+30)", 50},
	"potion_besar": {"🧴 Potion HP Besar (+80)", 180},
}

// tukar herbal → potion (resep alchemy)
var BREW = map[string]int{"potion_kecil": 2, "potion_besar": 5}

func handleShop(w http.ResponseWriter, r *http.Request) {
	pid := parseID(r.Header.Get("X-Player-ID"))
	var req struct {
		Item string
		Pay  string `json:"pay"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	// 🍞 craft bekal — cek SEBELUM lookup SHOP (bekal gak dijual gold)
	if req.Item == "bekal" && req.Pay == "craft" {
		p, err := loadPlayer(pid)
		if err != nil {
			writeJSON(w, 500, map[string]string{"err": "gak ada"})
			return
		}
		inv := normInv(p.Data["inv"])
		st, _ := inv["stack"].(map[string]any)
		hb, _ := st["herbal"].(float64)
		kl, _ := st["kulit_goblin"].(float64)
		if hb < 3 || kl < 2 {
			writeJSON(w, 400, map[string]string{"err": "butuh 🌿3 + 🟫2 buat 1 bekal"})
			return
		}
		st["herbal"] = hb - 3
		if st["herbal"].(float64) <= 0 {
			delete(st, "herbal")
		}
		st["kulit_goblin"] = kl - 2
		if st["kulit_goblin"].(float64) <= 0 {
			delete(st, "kulit_goblin")
		}
		bonus := 1 + cookLvOf(p)/3 // cooking bikin adonan efisien
		addItemSrv(inv, "bekal", bonus)
		p.Data["inv"] = inv
		savePlayerData(p)
		writeJSON(w, 200, map[string]any{"player": p,
			"msg": "🍞 Masak " + itoa(bonus) + " bekal! (cooking lv." + itoa(cookLvOf(p)) + ")"})
		return
	}

	item, ok := SHOP[req.Item]
	if !ok {
		// list shop
		writeJSON(w, 200, SHOP)
		return
	}
	p, err := loadPlayer(pid)
	if err != nil {
		writeJSON(w, 500, map[string]string{"err": "gak ada"})
		return
	}
	if raidLocked(p) && (req.Item != "" || req.Pay != "") {
		writeBusy(w, "belanja")
		return
	}
	inv := normInv(p.Data["inv"])

	// bayar pakai herbal
	if req.Pay == "herbal" {
		need, ok := BREW[req.Item]
		if !ok {
			writeJSON(w, 400, map[string]string{"err": "gak bisa dibrEW dari herbal"})
			return
		}
		st, _ := inv["stack"].(map[string]any)
		f, _ := st["herbal"].(float64)
		if f < float64(need) {
			writeJSON(w, 400, map[string]string{"err": "🌿 herbal kurang (butuh " + itoa(need) + ")"})
			return
		}
		st["herbal"] = f - float64(need)
		if st["herbal"].(float64) <= 0 {
			delete(st, "herbal")
		}
		addItemSrv(inv, req.Item, 1)
		p.Data["inv"] = inv
		savePlayerData(p)
		writeJSON(w, 200, p)
		return
	}

	if p.Gold < item.Harga {
		writeJSON(w, 400, map[string]string{"err": "gold kurang"})
		return
	}
	p.Gold -= item.Harga
	if bagUsed(inv) >= bagSlots(p.Data) {
		writeJSON(w, 400, map[string]string{"err": "tas penuh! Upgrade tas dulu"})
		return
	}
	addItemSrv(inv, req.Item, 1)
	p.Data["inv"] = inv
	savePlayerData(p)
	writeJSON(w, 200, p)
}

func savePlayerData(p *Player) {
	dj, _ := json.Marshal(p.Data)
	db.Exec(`UPDATE players SET gold=?, data=?, power=? WHERE id=?`,
		p.Gold, string(dj), p.Power, p.ID)
}

// POST /api/farm {gold_delta, xp_delta} — client kirim hasil farm 30 detik.
// Server clamp: max gold_delta = 3 * elapsed sejak last_farm (anti cheat), max 8 jam.
// loot roll tiap save (30s) — drop chance 25% per window
func handleFarm(w http.ResponseWriter, r *http.Request) {
	pid := parseID(r.Header.Get("X-Player-ID"))
	var req struct {
		GoldDelta int `json:"gold_delta"`
		XPDelta   int `json:"xp_delta"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	p, err := loadPlayer(pid)
	if err != nil {
		writeJSON(w, 500, map[string]string{"err": "gak ada"})
		return
	}
	h, ok := p.Data["hero"].(map[string]any)
	if !ok {
		writeJSON(w, 400, map[string]string{"err": "belum ada hero"})
		return
	}
	_ = h // hero ada; detail stat dipake gainXP

	// hitung batas wajar dari waktu sejak farm terakhir (disimpan di data.farm_at unix)
	now := time.Now().Unix()
	last, _ := toF(p.Data["farm_at"])
	if last == 0 {
		last = float64(now - 30)
	}
	elapsed := now - int64(last)
	if elapsed > 8*3600 {
		elapsed = 8 * 3600 // offline cap
	}
	maxGold := elapsed * 3 // dungeon max 3 gold/tick
	gd := int64(req.GoldDelta)
	if gd < 0 {
		gd = 0
	}
	if gd > maxGold {
		gd = maxGold
	}
	p.Gold += gd

	// XP: max 2/tick
	maxXP := int(elapsed) * 2
	xd := req.XPDelta
	if xd < 0 {
		xd = 0
	}
	if xd > maxXP {
		xd = maxXP
	}
	if xd > 0 {
		gainXP(p, xd)
	}
	p.Data["farm_at"] = now

	// LOOT ROLL fase 2: 25% chance dapat 1 item dari loot table per window
	// equip → slot terpisah dgn uid; material/potion → stack
	drops := []string{}
	if rand.Intn(100) < 25 {
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
	writeJSON(w, 200, map[string]any{"player": p, "drops": drops})
}

// addItemSrvInv: validasi tas + add; return false kalau tas penuh
func addItemSrvInv(p *Player, id string, qty int) bool {
	if !bagHasRoom(p) {
		return false
	}
	inv := normInv(p.Data["inv"])
	addItemSrv(inv, id, qty)
	p.Data["inv"] = inv
	return true
}

func toF(v any) (float64, bool) {
	f, ok := v.(float64)
	return f, ok
}

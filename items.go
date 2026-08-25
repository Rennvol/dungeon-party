package main

// FASE 2 — katalog item, loot table, rarity, class restriction, upgrade

import (
	"encoding/json"
	"math/rand"
	"net/http"
	"strings"
)

type Item struct {
	ID       string   `json:"id"`
	Nama     string   `json:"nama"`
	Kind     string   `json:"kind"` // wep cap hel boo arm | mat pot
	Rarity   string   `json:"rarity"`
	Element  string   `json:"element,omitempty"`
	ATK      int      `json:"atk,omitempty"`
	DEF      int      `json:"def,omitempty"`
	HP       int      `json:"hp,omitempty"`
	ClassReq []string `json:"class_req,omitempty"` // kosong = semua class
	Harga    int64    `json:"harga"`               // harga beli (jual = 1/4)
}

var ITEMS = map[string]Item{
	// ---- MATERIAL (stack) ----
	"kulit_goblin": {"kulit_goblin", "🟫 Kulit Goblin", "mat", "common", "", 0, 0, 0, nil, 12},
	"bijih_besi":   {"bijih_besi", "⛏️ Bijih Besi", "mat", "common", "", 0, 0, 0, nil, 20},
	"herbal":       {"herbal", "🌿 Herbal", "mat", "common", "", 0, 0, 0, nil, 15},
	"forge_stone":  {"forge_stone", "🪨 Forge Stone (+10% tempa)", "mat", "rare", "", 0, 0, 0, nil, 120},

	// ---- WEAPON (per class!) ----
	"wep_besi":     {"wep_besi", "🗡️ Pedang Besi", "wep", "common", "api", 8, 0, 0, []string{"warrior"}, 150},
	"wep_flame":    {"wep_flame", "🔥 Pedang Api Naga", "wep", "epic", "api", 45, 0, 10, []string{"warrior"}, 2500},
	"staf_pemula":  {"staf_pemula", "🪄 Staf Pemula", "wep", "common", "listrik", 11, 0, 0, []string{"mage"}, 150},
	"staf_badai":   {"staf_badai", "⚡ Staf Badai Abadi", "wep", "epic", "listrik", 52, 0, 0, []string{"mage"}, 2600},
	"bow_rimba":    {"bow_rimba", "🏹 Busur Rimba", "wep", "common", "alam", 9, 0, 2, []string{"ranger"}, 150},
	"bow_fajar":    {"bow_fajar", "🏹 Busur Fajar Kembar", "wep", "epic", "cahaya", 48, 0, 5, []string{"ranger"}, 2550},
	"tongkat_suci": {"tongkat_suci", "✨ Tongkat Suci", "wep", "common", "cahaya", 7, 3, 4, []string{"cleric"}, 150},
	"tongkat_dawn": {"tongkat_dawn", "🌟 Tongkat Cahaya Fajar", "wep", "epic", "cahaya", 38, 10, 20, []string{"cleric"}, 2700},

	// ---- ARMOR SET (semua class) ----
	"arm_kulit": {"arm_kulit", "🥋 Armur Kulit", "arm", "common", "", 0, 5, 10, nil, 120},
	"arm_baja":  {"arm_baja", "🛡️ Armur Baja Berat", "arm", "rare", "", 0, 14, 25, nil, 600},
	"hel_baja":  {"hel_baja", "⛑️ Helm Baja", "hel", "common", "", 0, 4, 6, nil, 100},
	"cap_alam":  {"cap_alam", "🍃 Cape Rimba", "cap", "rare", "alam", 3, 6, 8, nil, 450},
	"boot_kulit": {"boot_kulit", "👢 Sepatu Kulit", "boo", "common", "", 1, 2, 4, nil, 90},

	// ---- POTION (stack) ----
	"potion_kecil": {"potion_kecil", "🧪 Potion HP Kecil", "pot", "common", "", 0, 0, 30, nil, 50},
	"potion_besar": {"potion_besar", "🧴 Potion HP Besar", "pot", "common", "", 0, 0, 80, nil, 180},

	// ---- BEKAL JOURNEY ----
	"bekal": {"bekal", "🍞 Roti Panggang (bekal)", "mat", "common", "", 0, 0, 0, nil, 40},

	// ---- TIKET ----
	"raid_ticket": {"raid_ticket", "🎟️ Tiket Raid Instan", "mat", "rare", "", 0, 0, 0, nil, 400},
}

var RARITY_COLOR = map[string]string{
	"common": "#9aa0a6", "rare": "#4fc3f7", "epic": "#ba68c8", "legendary": "#ffb300",
}

// loot table gua goblin — weight-based roll
var LOOT_GOBLIN = []struct {
	ID     string
	Weight int
}{
	{"kulit_goblin", 40}, {"bijih_besi", 22},
	{"forge_stone", 3},
	{"boot_kulit", 8}, {"hel_baja", 6}, {"arm_kulit", 8},
	{"wep_besi", 4}, {"staf_pemula", 4}, {"bow_rimba", 4}, {"tongkat_suci", 4},
	{"arm_baja", 2}, {"cap_alam", 2},
	{"wep_flame", 1}, {"staf_badai", 1}, {"bow_fajar", 1}, {"tongkat_dawn", 1},
}

func rollLoot() string {
	total := 0
	for _, l := range LOOT_GOBLIN {
		total += l.Weight
	}
	r := rand.Intn(total)
	for _, l := range LOOT_GOBLIN {
		if r < l.Weight {
			return l.ID
		}
		r -= l.Weight
	}
	return ""
}

func itemPower(it Item, lv int) int {
	mul := 1.0 + 0.08*float64(lv)
	return int((float64(it.ATK)*2 + float64(it.DEF)*3 + float64(it.HP)/2) * mul)
}

// ---------- API ----------

// GET /api/items — katalog lengkap buat UI; ?type=dungeons → list dungeon
func handleItems(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Get("type") == "dungeons" {
		out := []Dungeon{}
		for _, d := range DUNGEONS {
			out = append(out, d)
		}
		writeJSON(w, 200, out)
		return
	}
	list := []Item{}
	for _, it := range ITEMS {
		list = append(list, it)
	}
	writeJSON(w, 200, list)
}

// POST /api/equip {uid} — pakai equipment (dari tas), atau {"uid":""} lepas slot
// POST /api/equip {"slot":"wep"} — lepas slot itu
func handleEquip(w http.ResponseWriter, r *http.Request) {
	pid := parseID(r.Header.Get("X-Player-ID"))
	var req struct {
		UID  string `json:"uid"`
		Slot string `json:"slot"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	p, err := loadPlayer(pid)
	if err != nil || p.Data["hero"] == nil {
		writeJSON(w, 400, map[string]string{"err": "belum siap"})
		return
	}
	h := p.Data["hero"].(map[string]any)
	eq, _ := h["equip"].(map[string]any)
	if eq == nil {
		eq = map[string]any{}
	}
	inv := normInv(p.Data["inv"])

	// lepas slot
	if req.UID == "" && req.Slot != "" {
		delete(eq, req.Slot)
		h["equip"] = eq
		recomputeStats(p)
		savePlayerData(p)
		writeJSON(w, 200, p)
		return
	}

	// cari item di tas by uid
	var found map[string]any
	var foundSlot string
	for _, e := range inv["equip"].([]any) {
		if m, ok := e.(map[string]any); ok && m["uid"] == req.UID {
			found = m
			break
		}
	}
	if found == nil {
		writeJSON(w, 404, map[string]string{"err": "item gak ada di tas"})
		return
	}
	it, ok := ITEMS[found["id"].(string)]
	if !ok || !isEquipKind(it.Kind) {
		writeJSON(w, 400, map[string]string{"err": "bukan equipment"})
		return
	}
	// CLASS RESTRICTION
	if len(it.ClassReq) > 0 {
		cls := h["class"].(string)
		okCls := false
		for _, c := range it.ClassReq {
			if c == cls {
				okCls = true
			}
		}
		if !okCls {
			writeJSON(w, 400, map[string]string{"err": "❌ " + it.Nama + " cuma buat " + strings.Join(it.ClassReq, "/")})
			return
		}
	}
	// lepas yang lama di slot itu (kembali ke tas)
	if oldUID, exists := eq[it.Kind]; exists {
		for _, e := range inv["equip"].([]any) {
			if m, ok := e.(map[string]any); ok && m["uid"] == oldUID {
				foundSlot = "" // lama tetap di tas, tinggal tandai tidak terpakai
			}
		}
	}
	eq[it.Kind] = req.UID
	h["equip"] = eq
	recomputeStats(p)
	savePlayerData(p)
	writeJSON(w, 200, p)
	_ = foundSlot
}

func isEquipKind(k string) bool {
	switch k {
	case "wep", "cap", "hel", "boo", "arm":
		return true
	}
	return false
}

// equipLv ambil level upgrade item terpasang
func equipLv(h map[string]any, slot, uid string) int {
	eq, _ := h["equip"].(map[string]any)
	if eq[slot] != uid {
		return 0
	}
	inv, _ := h["__lv"].(map[string]any) // upgrade lv disimpan per uid di data.upg
	if inv == nil {
		return 0
	}
	f, _ := inv[uid].(float64)
	return int(f)
}

func recomputeStats(p *Player) {
	h := p.Data["hero"].(map[string]any)
	cls := CLASSES[h["class"].(string)]
	lvl := 1
	if f, ok := h["lvl"].(float64); ok {
		lvl = int(f)
	}
	skl := func(id string, def float64) float64 {
		m, _ := p.Data["skills"].(map[string]any)
		f, _ := m[id].(float64)
		return f + def
	}
	atk := float64(cls.ATK+(lvl-1)*2) * (1 + 0.06*skl("power_strike", 0))
	hp := float64(cls.HP+(lvl-1)*8) * (1 + 0.08*skl("vitality", 0))
	def := lvl + int(skl("iron_skin", 0))

	upg, _ := p.Data["upg"].(map[string]any)
	eqMap, _ := h["equip"].(map[string]any)
	for slot, uidV := range eqMap {
		uid, _ := uidV.(string)
		var it *Item
		var lv int
		// temukan item by uid di tas
		if inv, ok := p.Data["inv"].(map[string]any); ok {
			if arr, ok2 := inv["equip"].([]any); ok2 {
				for _, e := range arr {
					if m, ok3 := e.(map[string]any); ok3 && m["uid"] == uid {
						idStr, _ := m["id"].(string)
						c := ITEMS[idStr]
						it = &c
					}
				}
			}
		}
		if it == nil {
			continue
		}
		lvf, _ := upg[uid].(float64)
		lv = int(lvf)
		mul := 1.0 + 0.08*float64(lv)
		atk += float64(it.ATK) * mul
		hp += float64(it.HP) * mul
		def += int(float64(it.DEF) * mul)
		_ = slot
	}
	h["atk"], h["hp_max"], h["def"] = int(atk), int(hp), def
	p.Power = int(atk*2 + hp/10 + float64(def)*2 + float64(lvl)*5)
}

// POST /api/upgrade {uid} — naikkan lv equipment (+8%/lv), +10 bisa gagal
func handleUpgrade(w http.ResponseWriter, r *http.Request) {
	pid := parseID(r.Header.Get("X-Player-ID"))
	var req struct {
		UID string `json:"uid"`
		UseProt bool `json:"use_prot"`
		UseStone int `json:"use_stone"`
		}
	json.NewDecoder(r.Body).Decode(&req)

	p, err := loadPlayer(pid)
	if err != nil || p.Data["hero"] == nil {
		writeJSON(w, 400, map[string]string{"err": "belum siap"})
		return
	}
	if raidLocked(p) {
		writeBusy(w, "nempa")
		return
	}
	// cari item & lv
	var it Item
	var curLv int
	{
		inv := normInv(p.Data["inv"])
		for _, e := range inv["equip"].([]any) {
			if m, ok := e.(map[string]any); ok && m["uid"] == req.UID {
				it = ITEMS[m["id"].(string)]
			}
		}
	}
	if it.ID == "" {
		writeJSON(w, 404, map[string]string{"err": "equipment gak ada"})
		return
	}
	upg := normMap(p.Data["upg"])
	if f, ok := upg[req.UID].(float64); ok {
		curLv = int(f)
	}
	if curLv >= 15 {
		writeJSON(w, 400, map[string]string{"err": "max +15"})
		return
	}
	costGold := int64(float64(it.UpCostHarga()) * pow16(float64(curLv)))
	costOre := curLv + 1
	inv := normInv(p.Data["inv"])
	st, _ := inv["stack"].(map[string]any)
	if st == nil {
		st = map[string]any{}
	}
	// persen sukses: 100% sampai +5, turun 8%/lv setelahnya; Forge Stone +10%/batu (max 3)
	succ := 100 - maxInt(0, curLv-4)*8
	useStone := 0
	if req.UseStone > 3 {
		req.UseStone = 3
	}
	if succ < 100 && req.UseStone > 0 {
		st0, _ := st["forge_stone"].(float64)
		useStone = minInt(req.UseStone, int(st0))
		succ += useStone * 10
	}
	if succ > 100 {
		succ = 100
	}
	if p.Gold < costGold {
		writeJSON(w, 400, map[string]string{"err": "gold kurang (butuh " + itoa(int(costGold)) + ")"})
		return
	}
	ore, _ := st["bijih_besi"].(float64)
	if ore < float64(costOre) {
		writeJSON(w, 400, map[string]string{"err": "butuh ⛏️ Bijih Besi ×" + itoa(costOre)})
		return
	}

	p.Gold -= costGold
	st["bijih_besi"] = ore - float64(costOre)
	if useStone > 0 {
		takeStack(inv, "forge_stone", useStone)
	}

	newLv := curLv + 1
	failMsg := ""
	if rand.Intn(100) >= succ {
		if req.UseProt && takeStack(inv, "prot_stone", 1) {
			newLv = curLv // gagal tapi gak turun
			failMsg = " ❌ GAGAL (dilindungi Protection Stone, level bertahan)"
		} else {
			newLv = curLv - 1
			if newLv < 0 {
				newLv = 0
			}
			failMsg = " ❌ GAGAL! Turun ke +" + itoa(newLv)
		}
	}
	upg[req.UID] = float64(newLv)
	p.Data["upg"] = upg
	recomputeStats(p)
	savePlayerData(p)
	msg := "🔨 +" + itoa(newLv)
	if newLv > curLv {
		msg = "🔨 SUKSES (" + itoa(succ) + "%) → +" + itoa(newLv) + " — stat naik!"
	} else if failMsg == "" {
		newLv = curLv
		msg = "🔨 tetap +" + itoa(curLv)
	} else {
		msg += " (" + itoa(succ) + "%)"
	}
	writeJSON(w, 200, map[string]any{"player": p, "msg": msg, "new_lv": newLv})
}

func takeStack(inv map[string]any, id string, n int) bool {
	st, _ := inv["stack"].(map[string]any)
	f, ok := st[id].(float64)
	if !ok || f < float64(n) {
		return false
	}
	f -= float64(n)
	if f <= 0 {
		delete(st, id) // bersihkan entry value 0 biar gak numpuk di tas
	} else {
		st[id] = f
	}
	return true
}

func normMap(v any) map[string]any {
	m, _ := v.(map[string]any)
	if m == nil {
		return map[string]any{}
	}
	return m
}

func pow16(n float64) float64 {
	r := 1.0
	for i := 0; i < int(n); i++ {
		r *= 1.5
	}
	return r
}

// POST /api/sell {"uid":"..."} atau {"stack_id":"kulit_goblin","qty":5}
func handleSell(w http.ResponseWriter, r *http.Request) {
	pid := parseID(r.Header.Get("X-Player-ID"))
	var req struct {
		UID     string `json:"uid"`
		StackID string `json:"stack_id"`
		Qty     int    `json:"qty"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	p, err := loadPlayer(pid)
	if err != nil || p.Data["hero"] == nil {
		writeJSON(w, 400, map[string]string{"err": "belum siap"})
		return
	}
	inv := normInv(p.Data["inv"])

	if req.StackID != "" {
		st := inv["stack"].(map[string]any)
		f, ok := st[req.StackID].(float64)
		qty := float64(req.Qty)
		if !ok || f < qty || qty <= 0 {
			writeJSON(w, 400, map[string]string{"err": "jumlah gak cukup"})
			return
		}
		gold := ITEMS[req.StackID].Harga / 4 * int64(qty)
		if gold < 1 {
			gold = 1
		}
		st[req.StackID] = f - qty
		if st[req.StackID].(float64) <= 0 {
			delete(st, req.StackID)
		}
		p.Gold += gold
		savePlayerData(p)
		writeJSON(w, 200, p)
		return
	}

	// sell equipment by uid (juga melepas dari slot kalau lagi dipake)
	arr, _ := inv["equip"].([]any)
	out := []any{}
	sold := ""
	for _, e := range arr {
		m, ok := e.(map[string]any)
		if !ok {
			continue
		}
		if m["uid"] == req.UID {
			it := ITEMS[m["id"].(string)]
			p.Gold += it.Harga / 4
			sold = it.Nama
			// lepas dari slot kalau terpasang
			if h, ok := p.Data["hero"].(map[string]any); ok {
				if eq, ok2 := h["equip"].(map[string]any); ok2 {
					for slot, u := range eq {
						if s, _ := u.(string); s == req.UID {
							delete(eq, slot)
						}
					}
				}
			}
			continue
		}
		out = append(out, m)
	}
	if sold == "" {
		writeJSON(w, 404, map[string]string{"err": "item gak ketemu"})
		return
	}
	inv["equip"] = out
	recomputeStats(p)
	savePlayerData(p)
	writeJSON(w, 200, p)
}

// UpCostHarga: biaya upgrade dasar = 60% harga beli
func (it Item) UpCostHarga() int64 {
	if it.Harga < 100 {
		return 60
	}
	return it.Harga * 6 / 10
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

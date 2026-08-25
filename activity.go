package main

// FASE 2.6 — boss turn-based (d20 per serangan), HP persisten + regen,
// potion bisa dipakai kapan pun, dive dungeon nguras HP dikit.

import (
	"encoding/json"
	"fmt"
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

// AUTO RAID TICK — server-side: tiap load, kalau raid_on & sudah lewat CD 25s,
// party "meraid" dungeon yang dipilih (data.raid_dg): XP + drop otomatis.
func raidTick(p *Player) {
	on, _ := p.Data["raid_on"].(bool)
	if !on {
		return
	}
	dgID, _ := p.Data["raid_dg"].(string)
	d, ok := DUNGEONS[dgID]
	if !ok {
		// belum pilih dungeon → raid idle, tapi tetap terkunci (toggle menyimpan raid_dg juga)
		return
	}
	now := time.Now().Unix()
	last, _ := toF(p.Data["last_raid"])
	if now-int64(last) < 25 {
		return // masih cooldown
	}
	h := p.Data["hero"].(map[string]any)
	lvl := int(toFv(h["lvl"]))
	if lvl < d.MinLvl {
		p.Data["raid_on"] = false
		return
	}
	mx, _ := toF(h["hp_max"])
	cur := heroHP(p)
	if cur <= mx*0.15 {
		p.Data["raid_on"] = false // HP kritis → raid berhenti sendiri
		return
	}
	h["hp"] = cur - mx*0.08
	p.Data["last_raid"] = float64(now)
	xp := d.XP * (1 + rand.Intn(3))
	gainXP(p, xp)
	msg := "⚔️ Raid " + d.Nama + ": +" + itoa(xp) + " XP"
	// LOG RAID: simpan exp + item biar user tau progress
	log := []string{}
	if a, ok := p.Data["raid_log"].([]any); ok {
		for _, x := range a {
			if s, ok := x.(string); ok {
				log = append(log, s)
			}
		}
	}
	log = append(log, msg)
	if rand.Intn(100) < d.DropPct {
		id := rollLoot()
		if id != "" && isEquipID(id) && bagHasRoom(p) {
			invE := normInv(p.Data["inv"])
			addItemSrv(invE, id, 1)
			p.Data["inv"] = invE
			log = append(log, "🎁 Drop: "+(ITEMS[id].Nama))
		}
	}
	if len(log) > 25 {
		log = log[len(log)-25:] // ring buffer
	}
	p.Data["raid_log"] = log
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
	if raidLocked(p) && req.Action != "raid" {
		writeBusy(w, "panen kebun")
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

// RAID LOCK: saat auto-raid aktif, party ada di dungeon — gak bisa aktivitas lain.
func raidLocked(p *Player) bool {
	on, _ := p.Data["raid_on"].(bool)
	return on
}

func writeBusy(w http.ResponseWriter, wkt string) {
	writeJSON(w, 429, map[string]string{"err": "⚔️ Party lagi RAID di dungeon! Matiin Auto Raid dulu buat " + wkt})
}

// ---------- DUNGEON (XP) — dive nguras HP 8% ----------

func handleDungeon(w http.ResponseWriter, r *http.Request) {
	pid := parseID(r.Header.Get("X-Player-ID"))
	var req struct {
		Dive    string `json:"dive"`
		Raid    string `json:"raid"`    // "on" | "off" — auto raid
		Instant string `json:"instant"` // dungeon id → raid instan (tiket)
	}
	json.NewDecoder(r.Body).Decode(&req)

	p, err := loadPlayer(pid)
	if err != nil || p.Data["hero"] == nil {
		writeJSON(w, 400, map[string]string{"err": "belum siap"})
		return
	}

	// toggle auto raid — server yang jalanin, bukan client
	if req.Raid == "on" || req.Raid == "off" {
		on := req.Raid == "on"
		p.Data["raid_on"] = on
		if on {
			dg := req.Dive
			if dg == "" {
				dg = "gua_goblin"
			}
			p.Data["raid_dg"] = dg
		}
		savePlayerData(p)
		msg := "🛑 Auto Raid MATI — party balik ke kamp"
		if on {
			msg = "⚔️ AUTO RAID AKTIF! Party terus-terusan ngeraid dungeon. Panen/boss/tempa/toko DIKUNCI."
		}
		writeJSON(w, 200, map[string]any{"player": p, "msg": msg})
		return
	}
	if raidLocked(p) {
		writeBusy(w, "raid manual")
		return
	}
	// 🎟️ raid instan via /api/dungeon {instant:"id"} — tiket dari toko
	if req.Instant != "" {
		instantRaid(w, p, req.Instant)
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
	lvl := int(toFv(h["lvl"]))
	if lvl < d.MinLvl {
		writeJSON(w, 400, map[string]string{"err": "🔒 butuh Lv." + itoa(d.MinLvl) + " buat " + d.Nama})
		return
	}
	mx, _ := toF(h["hp_max"])
	cur := heroHP(p)
	if cur < mx*0.15 {
		writeJSON(w, 400, map[string]string{"err": "❤️ HP terlalu rendah — pakai potion atau istirahat dulu"})
		return
	}
	// dive nguras HP
	h["hp"] = cur - mx*0.08
	p.Data["last_dive"] = float64(now)

	// POWER & CLEAR RATE: roll internal — power rendah = bisa gagal (XP/gold setengah)
	myP := p.Power
	chance := clearChance(myP, d.EnemyPow)
	success := rand.Intn(100) < int(chance*100)

	xp := d.XP * (1 + rand.Intn(3))
	gold := int64(0)
	if success {
		gold = int64(float64(farmGold(d)) * incomeMult(p) * elemMult(CLASSES[h["class"].(string)].Element, d.Element))
		p.Gold += gold
	} else {
		xp /= 2 // gagal dive — XP sedikit, no gold/drop
	}
	gainXP(p, xp)
	msg := ""
	if !success {
		msg = "⚔️ " + d.Nama + ": party kewalahan... mundur (+ " + itoa(xp) + " XP)"
	} else {
		msg = "⚔️ Raid " + d.Nama + ": +" + itoa(xp) + " XP"
		if gold > 0 {
			msg += " +" + itoa(int(gold)) + "g"
		}
	}
	if story := storyFor(p, req.Dive); story != nil {
		for _, s := range story {
			msg += "\n📖 " + s
		}
	}

	drops := []string{}
	if success && rand.Intn(100) < d.DropPct {
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
	case "potion":
		bossPotion(w, p)
	case "flee":
		p.Data["battle"] = nil
		savePlayerData(p)
		writeJSON(w, 200, map[string]any{"player": p, "msg": "🏃 Kabur dari pertarungan"})
	default:
		// attack / power_strike / guard / wind → kontes dadu
		bossAttack(w, p, req.Action)
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
	if raidLocked(p) {
		writeBusy(w, "lawan boss")
		return
	}
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
	p.Data["battle"] = map[string]any{
		"boss": boss.ID, "bhp": float64(boss.HP), "wind": 2.0,
		"nama": boss.Nama + " " + BOSS_TITLES[rand.Intn(len(BOSS_TITLES))],
	}
	savePlayerData(p)
	btl := p.Data["battle"].(map[string]any)
	writeJSON(w, 200, map[string]any{"player": p,
		"log": []string{"⚔️ " + btl["nama"].(string) + " (Lv." + itoa(boss.MinLvl+bossKills(p, boss.ID)) + ") muncul! Giliranmu — tekan SERANG."}})
}

func toFv(v any) float64 { f, _ := v.(float64); return f }

// giliran hero: KONTES DADU ala DnD — roll hero vs roll boss, yang lebih besar menang ronde.
// skill aktif: power_strike (dmg x2), guard (blokir 70%), wind (heal 25%, 2x per battle)
func bossAttack(w http.ResponseWriter, p *Player, useSkill string) {
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
	atkBase := toFv(h["atk"]) * elemMult(CLASSES[h["class"].(string)].Element, boss.Element)
	def := toFv(h["def"])
	mx := toFv(h["hp_max"])

	cds, _ := bt["cds"].(map[string]any)
	if cds == nil {
		cds = map[string]any{}
	}
	windLeft := int(toFv(bt["wind"]))
	logs := []string{}

	// --- aksi spesial sebelum kontes dadu ---
	dmgMul, guardPct, heal := 1.0, 0.0, 0.0
	switch useSkill {
	case "", "none":
	case "power_strike":
		if skillLv(p, "power_strike") < 1 {
			writeJSON(w, 400, map[string]string{"err": "belajar dulu di Dojo!"})
			return
		}
		if toFv(cds["ps"]) > 0 {
			writeJSON(w, 400, map[string]string{"err": "Power Strike cooldown " + itoa(int(toFv(cds["ps"]))) + " giliran lagi"})
			return
		}
		dmgMul = 2.0
		cds["ps"] = 3.0
		logs = append(logs, "💪 POWER STRIKE!")
	case "guard":
		if skillLv(p, "iron_skin") < 1 {
			writeJSON(w, 400, map[string]string{"err": "belajar Iron Skin dulu di Dojo!"})
			return
		}
		if toFv(cds["gr"]) > 0 {
			writeJSON(w, 400, map[string]string{"err": "Guard cooldown " + itoa(int(toFv(cds["gr"]))) + " giliran lagi"})
			return
		}
		guardPct = 0.7
		cds["gr"] = 3.0
		logs = append(logs, "🛡️ GUARD — siap menahan serangan")
	case "wind":
		if skillLv(p, "vitality") < 1 {
			writeJSON(w, 400, map[string]string{"err": "belajar Vitality dulu di Dojo!"})
			return
		}
		if windLeft <= 0 {
			writeJSON(w, 400, map[string]string{"err": "Second Wind habis untuk battle ini"})
			return
		}
		heal = mx * 0.25
		windLeft--
		logs = append(logs, "❤️‍🔥 SECOND WIND! +"+itoa(int(heal))+" HP")
	case "attack":
		// serangan normal
	default:
		writeJSON(w, 400, map[string]string{"err": "skill gak dikenal"})
		return
	}

	bhp := toFv(bt["bhp"])

	if heal > 0 {
		cur := heroHP(p) + heal
		if cur > mx {
			cur = mx
		}
		h["hp"] = cur
	}

	// --- KONTES DADU: hero d20 vs boss d20 ---
	hr := rand.Intn(20) + 1
	br := rand.Intn(20) + 1

	if hr == 20 || (hr > br && hr != 1) {
		// hero menang ronde — dmg dikurangi DEF boss
		margin := float64(hr-br) / 18.0 // 0..1
		dmg := (atkBase*dmgMul - float64(boss.DEF)) * (0.7 + 0.6*margin)
		if dmg < atkBase*0.2 {
			dmg = atkBase * 0.2 // minimal 20% ATK biar gak nol
		}
		if hr == 20 {
			dmg = atkBase * dmgMul * 2
			logs = append(logs, fmt.Sprintf("🎲 NAT 20!! (boss %d) CRIT %d dmg 🔥", br, int(dmg)))
		} else {
			logs = append(logs, fmt.Sprintf("🎲 kamu %d vs boss %d → HIT %d", hr, br, int(dmg)))
		}
		bhp -= dmg
	} else if br == 20 || hr < br {
		// boss menang ronde
		cur := heroHP(p)
		dmgIn := (float64(boss.ATK)*2 - def) * (1 - guardPct)
		if dmgIn < 2 {
			dmgIn = 2
		}
		if br == 20 {
			dmgIn *= 2
			logs = append(logs, fmt.Sprintf("🎲 kamu %d vs BOSS NAT 20!! CRIT -%d HP 💀", hr, int(dmgIn)))
		} else {
			logs = append(logs, fmt.Sprintf("🎲 kamu %d vs boss %d → kena hit -%d HP", hr, br, int(dmgIn)))
		}
		cur -= dmgIn
		if guardPct > 0 {
			logs = append(logs, "🛡️ Guard menahan 70% serangan!")
		}
		if cur < 0 {
			cur = 0
		}
		h["hp"] = cur
	} else {
		logs = append(logs, fmt.Sprintf("🎲 seri %d-%d — saling menghindar", hr, br))
	}

	// auto-potion saat kritis
	if hpNow := heroHP(p); hpNow > 0 && hpNow <= mx*0.3 && heal == 0 {
		if takeStackInv(p, "potion_kecil", 1) {
			hpN := heroHP(p) + float64(ITEMS["potion_kecil"].HP)
			if hpN > mx {
				hpN = mx
			}
			h["hp"] = hpN
			logs = append(logs, "🧪 AUTO-POTION! +30 HP")
		} else if takeStackInv(p, "potion_besar", 1) {
			hpN := heroHP(p) + float64(ITEMS["potion_besar"].HP)
			if hpN > mx {
				hpN = mx
			}
			h["hp"] = hpN
			logs = append(logs, "🧴 AUTO-POTION BESAR! +80 HP")
		}
	}

	// tick cooldown
	for k, v := range cds {
		f := toFv(v)
		if f > 0 {
			cds[k] = f - 1
		}
	}
	bt["cds"], bt["wind"] = cds, float64(windLeft)

	// menang? — hadiah dari boss TERSCALING (bukan nilai dasar)
	if bhp <= 0 {
		k := bossKills(p, boss.ID)
		bumpKill(p, boss.ID)
		sb := scaleBoss(*boss, k) // reward sesuai level scaling yang dilawan
		p.Data["battle"] = nil
		p.Gold += sb.GoldWin
		gainXP(p, sb.XPWin)
		savePlayerData(p)
		logs = append(logs, "🏆 "+boss.Nama+" DIKALAHKAN! +"+itoa(int(sb.GoldWin))+"g +"+itoa(sb.XPWin)+"xp")
		nb := scaleBoss(*boss, k+1)
		logs = append(logs, "⚠️ Boss membangkitkan kekuatan baru! HP "+itoa(nb.HP)+", ATK "+itoa(nb.ATK)+" — hadiah ×1.35")
		writeJSON(w, 200, map[string]any{"player": p, "log": logs, "win": true})
		return
	}
	bt["bhp"] = bhp

	// kalah?
	if heroHP(p) <= 0 {
		h["hp"] = 1.0
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

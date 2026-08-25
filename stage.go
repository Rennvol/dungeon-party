package main

// FASE 4 — stage unlock, power/clear-rate, element dungeon, story engine,
// raid instan (tiket), prestige (reset → income permanen naik).

import (
	"math/rand"
	"net/http"
	"time"
)

// ---------- UNLOCK STAGE ----------
func heroLvlOf(p *Player) int {
	h, _ := p.Data["hero"].(map[string]any)
	return int(toFv(h["lvl"]))
}

// "" = terbuka, selain itu teks alasan terkunci
func dgnLockedReason(p *Player, d Dungeon) string {
	if heroLvlOf(p) < d.MinLvl {
		return "🔒 butuh Lv." + itoa(d.MinLvl)
	}
	if d.UnlockGold > 0 && p.LifetimeGold < d.UnlockGold {
		return "🔒 butuh " + itoa(int(d.UnlockGold)) + "g gold lifetime"
	}
	if d.UnlockBoss != "" {
		nm := d.UnlockBoss
		if b := findBoss(d.UnlockBoss); b != nil {
			nm = b.Nama
		}
		if bossKills(p, d.UnlockBoss) < d.UnlockBossN {
			return "🔒 kalahkan " + nm + " ×" + itoa(d.UnlockBossN)
		}
	}
	if d.PrestigeReq > 0 && p.Prestige < d.PrestigeReq {
		return "🔒 butuh prestige ×" + itoa(d.PrestigeReq)
	}
	return ""
}

// ---------- POWER / CLEAR RATE ----------
// rumus roadmap: chance = clamp(0.5 + (myP-enemyP)/enemyP*0.5, 0.05, 0.99)
func clearChance(myP, enP int) float64 {
	if enP <= 0 {
		return 1
	}
	c := 0.5 + float64(myP-enP)/float64(enP)*0.5
	if c > 0.99 {
		c = 0.99
	}
	if c < 0.05 {
		c = 0.05
	}
	return c
}

// prestige bikin income makin gede permanen (+25%/prestige)
func incomeMult(p *Player) float64 { return 1 + 0.25*float64(p.Prestige) }

// ---------- STORY ENGINE ----------
// Dialog visual-novel per stage — muncul SEKALI saat pertama kali dive ke stage.
var STORY = map[string][]string{
	"gua_goblin": {
		"Elara: \"Gerbang gua itu berlumur lumpur segar... mereka baru pindah.\"",
		"Gronnak: \"Hmph. Goblin kampung biasa. Bagus buat latihan pedang.\"",
		"Party melangkah masuk — kegelapan menelan cahaya obor.",
	},
	"tambang_runtuh": {
		"Elara: \"Tambang ini runtuh 50 tahun lalu. Tapi aku dengar suara palu dari dalam...\"",
		"Gronnak: \"Hantu gak pegang palu, Elara. Sesuatu LAIN yang pegang.\"",
	},
	"neraka_kegelapan": {
		"Raja Hantu: \"...aku menunggu. Berabad-abad. Di sini.\"",
		"Elara: \"Jangan dengarkan suaranya! Terus jalan!\"",
	},
	"kuburan_terkutuk": {
		"Nisan-nisan retak bergeser sendiri. Tanah mengembang seperti bernapas.",
		"Gronnak: \"Yang mati di sini gak mau ditinggal lagi. Siapkan senjata.\"",
	},
	"rawa_bandit": {
		"Bendera sobek tergantung di tiang kayu — markas Bandit Rawa.",
		"Elara: \"Mereka merampas karavan warga. Hari ini tagihan ditagih.\"",
	},
	"lahar_naga": {
		"Gronnak: \"Gunung ini SARANG. Aku cuma kenal satu makhluk yang bikin sarang sebesar itu...\"",
		"Elara: \"Naga Bara. Kalau kita sampai ke takhtanya, cerita tentang kita akan dibawakan para bard.\"",
		"Udaranya bisa membakar paru-paru. Partai dimulai.",
	},
}

// balikin baris story kalau pertama kali, sekalian tandain udah lihat
func storyFor(p *Player, id string) []string {
	seen, _ := p.Data["story_seen"].(map[string]any)
	if seen == nil {
		seen = map[string]any{}
	}
	if _, ok := seen[id]; ok {
		return nil
	}
	lines := STORY[id]
	if lines == nil {
		return nil
	}
	seen[id] = true
	p.Data["story_seen"] = seen
	return lines
}

// ---------- RAID INSTAN ----------
// butuh 🎟️ raid_ticket (stack) + cooldown 120s; hasil instan besar tanpa roll dadu.
func instantRaid(w http.ResponseWriter, p *Player, dgID string) {
	if raidLocked(p) {
		writeBusy(w, "raid instan")
		return
	}
	d, ok := DUNGEONS[dgID]
	if !ok {
		writeJSON(w, 400, map[string]string{"err": "dungeon gak ada"})
		return
	}
	if s := dgnLockedReason(p, d); s != "" {
		writeJSON(w, 400, map[string]string{"err": s})
		return
	}
	now := time.Now().Unix()
	last := toFv(p.Data["last_instant"])
	if now-int64(last) < 120 {
		writeJSON(w, 429, map[string]string{"err": "🎟️ tiket istirahat (" + itoa(int(120-(now-int64(last)))) + "s)"})
		return
	}
	if !takeStackInv(p, "raid_ticket", 1) {
		writeJSON(w, 400, map[string]string{"err": "butuh 🎟️ Tiket Raid — beli di Toko"})
		return
	}
	p.Data["last_instant"] = float64(now)
	mult := incomeMult(p)
	xp := int(float64(d.XP) * 4 * mult)
	gold := int64(float64(d.XP) * 15 * mult)
	p.Gold += gold
	gainXP(p, xp)
	msg := "🎟️ RAID INSTAN " + d.Nama + ": +" + itoa(int(gold)) + "g +" + itoa(xp) + " XP"
	// drop dua roll — instan harus kerasa worth it
	for i := 0; i < 2; i++ {
		if rand.Intn(100) < d.DropPct+30 {
			id := rollLoot()
			if id == "" {
				continue
			}
			if isEquipID(id) {
				if bagHasRoom(p) {
					invE := normInv(p.Data["inv"])
					addItemSrv(invE, id, 1)
					p.Data["inv"] = invE
					msg += " · 🎁 " + ITEMS[id].Nama
				}
			} else if addItemSrvInv(p, id, 1) {
				msg += " · 🎁 " + ITEMS[id].Nama
			}
		}
	}
	savePlayerData(p)
	writeJSON(w, 200, map[string]any{"player": p, "msg": msg})
}

// ---------- PRESTIGE ----------
// Lv.30 + semua aktivitas idle → reset lvl/gold, prestige+1, income ×1.25 permanen.
func handlePrestige(w http.ResponseWriter, r *http.Request) {
	p, err := loadPlayer(parseID(r.Header.Get("X-Player-ID")))
	if err != nil || p.Data["hero"] == nil {
		writeJSON(w, 400, map[string]string{"err": "belum siap"})
		return
	}
	if raidLocked(p) {
		writeBusy(w, "prestige")
		return
	}
	if _, busy := p.Data["battle"]; busy {
		writeJSON(w, 400, map[string]string{"err": "selesaikan battle dulu"})
		return
	}
	if _, on := p.Data["journey"]; on {
		writeJSON(w, 400, map[string]string{"err": "party masih di ekspedisi!"})
		return
	}
	if heroLvlOf(p) < 30 {
		writeJSON(w, 400, map[string]string{"err": "♻️ prestige butuh Lv.30"})
		return
	}
	h := p.Data["hero"].(map[string]any)
	h["lvl"], h["xp"] = 1, 0.0
	p.Gold = 0
	p.Prestige++
	recomputeStats(p)
	savePlayerData(p)
	writeJSON(w, 200, map[string]any{"player": p,
		"msg": "♻️ PRESTIGE ×" + itoa(p.Prestige) + "! Income +25% permanen. Petualangan dimulai lagi — kali ini lebih kuat."})
}

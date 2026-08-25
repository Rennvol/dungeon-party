package main

import (
	"encoding/json"
	"math/rand"
	"net/http"
	"time"
)

// TAMBANG — sewa penambang NPC dengan gold. Tiap penambang ngasih ⛏️ Bijih Besi
// pasif + chance dapet 🪨 Forge Stone (langka, buat nempa). Sewa bisa di-stack
// (tambah jumlah & durasi). Tick jalan di loadPlayer (sama kayak raid/garden).
//
// Balance:
//   rate besi/detik/penambang = 0.15  → 1 penambang ~9/menit
//   chance forge_stone per menit/penambang = 8%
//   harga = 50 gold × jumlah × (durasi_jam)

const (
	mineRateOrePerSec  = 0.15
	mineStonePctPerMin = 8
	mineCostPerMinerHr = 50 // gold per penambang per jam
	mineMaxMiners      = 100
	mineMaxHours       = 100
	mineCapBase        = 300 // kapasitas penampungan × scope_lv
)

// 📐 Scope: upgrade rate & kapasitas penampungan
func mineUpCost(scopeLv int) int { return 300 * scopeLv * scopeLv }
func mineCap(scopeLv int) float64 { return float64(mineCapBase) * float64(scopeLv) }

// POST /api/mine {action:"hire", count, hours} | {action:"status"}
func handleMine(w http.ResponseWriter, r *http.Request) {
	pid := parseID(r.Header.Get("X-Player-ID"))
	var req struct {
		Action string `json:"action"`
		Count   int    `json:"count"`
		Hours   int    `json:"hours"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	p, err := loadPlayer(pid)
	if err != nil || p.Data["hero"] == nil {
		writeJSON(w, 400, map[string]string{"err": "belum siap"})
		return
	}
	if raidLocked(p) {
		writeBusy(w, "nyewa penambang")
		return
	}

	switch req.Action {
	case "hire":
		if req.Count < 1 || req.Count > mineMaxMiners {
			writeJSON(w, 400, map[string]string{"err": "jumlah 1-" + itoa(mineMaxMiners)})
			return
		}
		if req.Hours < 1 || req.Hours > mineMaxHours {
			writeJSON(w, 400, map[string]string{"err": "durasi 1-" + itoa(mineMaxHours) + " jam"})
			return
		}
		cost := int64(req.Count * req.Hours * mineCostPerMinerHr)
		if p.Gold < cost {
			writeJSON(w, 400, map[string]string{"err": "gold kurang (butuh " + itoa(int(cost)) + ")"})
			return
		}
		p.Gold -= cost
		m := normMine(p.Data["mine"])
		now := float64(time.Now().Unix())
		// stack: perpanjang until & tambah count
		until := now + float64(req.Hours)*3600
		if m["until"].(float64) > now {
			until = m["until"].(float64) + float64(req.Hours)*3600
		}
		m["count"] = toFv(m["count"]) + float64(req.Count)
		m["at"] = now
		m["until"] = until
		p.Data["mine"] = m
		savePlayerData(p)
		writeJSON(w, 200, map[string]any{"player": p,
			"msg": "⛏️ Nyewa " + itoa(req.Count) + " penambang " + itoa(req.Hours) + " jam! Mereka nambang otomatis."})
	case "collect":
		// ambil 50% penampungan (dibulatkan) — sisanya tetap nambang
		m := normMine(p.Data["mine"])
		pend := toFv(m["pend"])
		stones := toFv(m["pend_stone"])
		ore := int(pend / 2)
		st := int(stones / 2)
		if pend < 2 && stones < 1 {
			writeJSON(w, 400, map[string]string{"err": "penampungan masih kosong — tunggu penambang bekerja dulu"})
			return
		}
		if ore > 0 {
			addItemSrvInv(p, "bijih_besi", ore)
		}
		if st > 0 {
			addItemSrvInv(p, "forge_stone", st)
		}
		m["pend"] = pend - float64(ore)
		m["pend_stone"] = stones - float64(st)
		p.Data["mine"] = m
		savePlayerData(p)
		writeJSON(w, 200, map[string]any{"player": p,
			"msg": "🧺 Panen " + itoa(ore) + " Bijih Besi" + func() string {
			if st > 0 {
				return " + " + itoa(st) + " Forge Stone!"
			}
			return "!"
		}()})
	case "upgrade":
		// 📐 upgrade scope: rate ×scope & kapasitas penampungan ×scope
		scope := 1
		if v := p.Data["mine_scope"]; v != nil {
			scope = int(toFv(v))
		}
		cost := int64(mineUpCost(scope))
		if p.Gold < cost {
			writeJSON(w, 400, map[string]string{"err": "gold kurang (butuh " + itoa(int(cost)) + ")"})
			return
		}
		p.Gold -= cost
		scope++
		p.Data["mine_scope"] = float64(scope)
		savePlayerData(p)
		writeJSON(w, 200, map[string]any{"player": p,
			"msg": "📐 Scope naik ke Lv." + itoa(scope) + "! Rate nambang & kapasitas penampungan meningkat."})
	default:
		m := normMine(p.Data["mine"])
		writeJSON(w, 200, map[string]any{"player": p, "mine": m})
	}
}

func normMine(v any) map[string]any {
	m, _ := v.(map[string]any)
	if m == nil {
		m = map[string]any{}
	}
	if _, ok := m["count"]; !ok {
		m["count"] = 0.0
	}
	if _, ok := m["at"]; !ok {
		m["at"] = 0.0
	}
	if _, ok := m["until"]; !ok {
		m["until"] = 0.0
	}
	return m
}

// mineTick: akumulasi hasil ke PENAMPUNGAN (cap), bukan langsung masuk tas.
// Player tekan PANEN buat ambil 50% penampungan.
func mineTick(p *Player) {
	m := normMine(p.Data["mine"])
	cnt := toFv(m["count"])
	if cnt <= 0 {
		return
	}
	scope := 1
	if v := p.Data["mine_scope"]; v != nil {
		scope = int(toFv(v))
	}
	now := float64(time.Now().Unix())
	until := toFv(m["until"])
	if now >= until {
		m["count"] = 0.0 // sewa habis — hasil tetap aman di penampungan
		p.Data["mine"] = m
		return
	}
	at := toFv(m["at"])
	if at == 0 {
		at = now
	}
	elapsed := now - at
	if elapsed <= 0 {
		return
	}
	capv := mineCap(scope)
	ore := elapsed * mineRateOrePerSec * float64(scope) * cnt
	m["pend"] = minFloat(toFv(m["pend"])+ore, capv)
	// chance stone: 8%/penambang/menit
	rolls := int(elapsed / 60 * cnt)
	for i := 0; i < rolls; i++ {
		if rand.Intn(100) < mineStonePctPerMin {
			m["pend_stone"] = toFv(m["pend_stone"]) + 1
		}
	}
	m["at"] = now
	p.Data["mine"] = m
}

func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

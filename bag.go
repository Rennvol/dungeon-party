package main

// Tas / inventory — server-side source of truth.
// Stack rules (permintaan user): potion, material farm/nempa/bahan potion = 1 slot
// gabung (stack). Equipment: weapon, cape, helm, boot, armor = tiap satuan 1 slot.
//
// Format inv di players.data JSON:
//   {"stack": {"potion_kecil": 3}, "equip": [{"id":"wep_besi","uid":"..."}]}
// bag_lv = level tas; slots = 20 + bag_lv*5.

import (
	"encoding/json"
	"fmt"
	"net/http"
)

const (
	bagBase     = 20
	bagPerLv    = 5
	bagUpCost   = 200 // gold naik ×1.6 tiap level
)

func isEquipID(id string) bool {
	if it, ok := ITEMS[id]; ok && (it.Kind == "wep" || it.Kind == "cap" || it.Kind == "hel" || it.Kind == "boo" || it.Kind == "arm") {
		return true
	}
	for _, p := range []string{"wep", "cap", "hel", "boo", "arm"} {
		if len(id) >= 3 && id[:3] == p {
			return true
		}
	}
	return false
}

func normInv(v any) map[string]any {
	m, _ := v.(map[string]any)
	if m == nil {
		return map[string]any{"stack": map[string]any{}, "equip": []any{}}
	}
	if m["stack"] == nil {
		m["stack"] = map[string]any{}
	}
	if m["equip"] == nil {
		m["equip"] = []any{}
	}
	// bersihkan stack value <=0 (jangan biarkan item "0" numpuk di tas)
	if st, ok := m["stack"].(map[string]any); ok {
		for k, val := range st {
			if f, ok := val.(float64); ok && f <= 0 {
				delete(st, k)
			}
		}
	}
	return m
}

func bagSlots(data map[string]any) int {
	lv := 0
	if f, ok := data["bag_lv"].(float64); ok {
		lv = int(f)
	}
	return bagBase + lv*bagPerLv
}

func bagUsed(inv map[string]any) int {
	n := 0
	if s, ok := inv["stack"].(map[string]any); ok {
		n += len(s)
	}
	if e, ok := inv["equip"].([]any); ok {
		n += len(e)
	}
	return n
}

func addItemSrv(inv map[string]any, id string, qty int) bool {
	slots := 0 // diisi caller via closure? tidak — caller pass slots. Simpel: hitung di sini dari p.Data tak tersedia, jadi caller validasi bagFull dulu.
	_ = slots
	if isEquipID(id) {
		arr, _ := inv["equip"].([]any)
		for i := 0; i < qty; i++ {
			arr = append(arr, map[string]any{"id": id, "uid": id + "-" + newUID()})
		}
		inv["equip"] = arr
		return true
	}
	st, _ := inv["stack"].(map[string]any)
	cur := 0.0
	if f, ok := st[id].(float64); ok {
		cur = f
	}
	st[id] = cur + float64(qty)
	return true
}

func bagHasRoom(p *Player) bool {
	return bagUsed(normInv(p.Data["inv"])) < bagSlots(p.Data)
}

// cek ruang dari inv yang sudah dinormalisasi (dipakai redeem: jangan save setengah2)
func bagHasRoomI(inv map[string]any) bool {
	lv := 0
	if f, ok := inv["_bag_lv"].(float64); ok {
		lv = int(f)
	}
	return bagUsed(inv) < bagBase+lv*bagPerLv
}

// POST /api/bag {action:"upgrade"} — beli slot tas dengan gold
func handleBag(w http.ResponseWriter, r *http.Request) {
	pid := parseID(r.Header.Get("X-Player-ID"))
	var req struct{ Action string }
	json.NewDecoder(r.Body).Decode(&req)

	p, err := loadPlayer(pid)
	if err != nil || p.Data["hero"] == nil {
		writeJSON(w, 400, map[string]string{"err": "belum siap"})
		return
	}
	switch req.Action {
	case "upgrade":
		lv := 0.0
		if f, ok := p.Data["bag_lv"].(float64); ok {
			lv = f
		}
		cost := int64(float64(bagUpCost))
		for i := 0; i < int(lv); i++ {
			cost = cost * 8 / 5 // ×1.6 integer
		}
		if p.Gold < cost {
			writeJSON(w, 400, map[string]string{"err": "gold kurang (butuh " + itoa(int(cost)) + ")"})
			return
		}
		p.Gold -= cost
		p.Data["bag_lv"] = lv + 1
	default:
		writeJSON(w, 400, map[string]string{"err": "aksi gak dikenal"})
		return
	}
	savePlayerData(p)
	writeJSON(w, 200, p)
}

func itoa(n int) string {
	b, _ := json.Marshal(n)
	return string(b)
}

func newUID() string {
	b := make([]byte, 4)
	readURandom(b)
	return fmt.Sprintf("%x", b)
}

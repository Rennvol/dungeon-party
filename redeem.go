package main

// FASE 5 — REDEEM CODE + ADMIN
// Tabel: redeem_codes (code PK, type, payload, max_uses, used_count, expires_at)
//        redemptions (code+player_id UNIQUE) — 1 player = 1 klaim per kode

import (
	"crypto/rand"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

func adminAuth(r *http.Request) bool {
	key := os.Getenv("ADMIN_KEY")
	return key != "" && r.Header.Get("X-Admin-Key") == key
}

// GET/POST /api/admin/codes
func handleAdminCodes(w http.ResponseWriter, r *http.Request) {
	if !adminAuth(r) {
		writeJSON(w, 403, map[string]string{"err": "forbidden"})
		return
	}
	if r.Method == "GET" {
		rows, err := db.Query(`SELECT code, type, payload, max_uses, used_count, IFNULL(DATE_FORMAT(expires_at,'%Y-%m-%d %H:%i'),'') FROM redeem_codes ORDER BY id DESC LIMIT 100`)
		if err != nil {
			writeJSON(w, 500, map[string]string{"err": err.Error()})
			return
		}
		defer rows.Close()
		out := []map[string]any{}
		for rows.Next() {
			var code, typ, payload, e string
			var maxU, used int
			rows.Scan(&code, &typ, &payload, &maxU, &used, &e)
			out = append(out, map[string]any{"code": code, "type": typ, "payload": payload, "max_uses": maxU, "used": used, "expires": e})
		}
		writeJSON(w, 200, out)
		return
	}
	// POST: bikin kode
	var req struct {
		Type  string `json:"type"`  // gold | item
		Value string `json:"value"` // gold: angka; item: item_id
		Qty   int64  `json:"qty"`   // item qty (default 1)
		Max   int    `json:"max"`   // max uses (0 = unlimited)
		Days  int    `json:"days"`  // expire dalam N hari (0 = gak expire)
	}
	json.NewDecoder(r.Body).Decode(&req)
	if req.Type != "gold" && req.Type != "item" {
		writeJSON(w, 400, map[string]string{"err": "type harus gold|item"})
		return
	}
	if req.Type == "item" {
		if req.Value != "rare_all" {
			if _, ok := ITEMS[req.Value]; !ok {
				writeJSON(w, 400, map[string]string{"err": "item id gak ada"})
				return
			}
		}
		if req.Qty < 1 {
			req.Qty = 1
		}
	}
	if req.Value == "" || req.Value != strings.TrimSpace(req.Value) {
		writeJSON(w, 400, map[string]string{"err": "value kosong/ada spasi"})
		return
	}
	code := genCode()
	payload := req.Value
	if req.Type == "item" && req.Qty != 1 {
		payload = req.Value + ":" + strconv.FormatInt(req.Qty, 10)
	}
	var exp any
	if req.Days > 0 {
		exp = time.Now().Add(time.Duration(req.Days) * 24 * time.Hour)
	}
	if _, err := db.Exec(`INSERT INTO redeem_codes(code,type,payload,max_uses,expires_at) VALUES(?,?,?,?,?)`,
		code, req.Type, payload, req.Max, exp); err != nil {
		writeJSON(w, 500, map[string]string{"err": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]any{"code": code, "qty": req.Qty})
}

func migrateRedeem() {
	qs := []string{
		`CREATE TABLE IF NOT EXISTS redeem_codes(
			id BIGINT AUTO_INCREMENT PRIMARY KEY,
			code VARCHAR(32) UNIQUE NOT NULL,
			type VARCHAR(16) NOT NULL,
			payload VARCHAR(64) NOT NULL,
			max_uses INT DEFAULT 0,
			used_count INT DEFAULT 0,
			expires_at TIMESTAMP NULL)`,
		`CREATE TABLE IF NOT EXISTS redemptions(
			code VARCHAR(32) NOT NULL,
			player_id BIGINT NOT NULL,
			claimed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			UNIQUE KEY uk_claim(code, player_id))`,
	}
	for _, q := range qs {
		if _, err := db.Exec(q); err != nil {
			log.Fatal("migrate redeem:", err)
		}
	}
}

func genCode() string {
	const abc = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789" // tanpa I/O/0/1 biar gak membingungkan
	b := make([]byte, 8)
	rand.Read(b)
	for i := range b {
		b[i] = abc[int(b[i])%len(abc)]
	}
	return string(b)
}

// POST /api/redeem {code}
func handleRedeem(w http.ResponseWriter, r *http.Request) {
	pid := parseID(r.Header.Get("X-Player-ID"))
	var req struct{ Code string }
	json.NewDecoder(r.Body).Decode(&req)
	code := strings.ToUpper(strings.TrimSpace(req.Code))
	if code == "" {
		writeJSON(w, 400, map[string]string{"err": "masukkan kode"})
		return
	}
	var typ, payload string
	var maxU, used int
	var expired bool
	err := db.QueryRow(`SELECT type,payload,max_uses,used_count,COALESCE(expires_at IS NOT NULL AND expires_at < NOW(), 0) FROM redeem_codes WHERE code=?`, code).
		Scan(&typ, &payload, &maxU, &used, &expired)
	if err != nil {
		writeJSON(w, 404, map[string]string{"err": "kode gak ada / sudah ditarik"})
		return
	}
	if expired {
		writeJSON(w, 400, map[string]string{"err": "kode sudah expired"})
		return
	}
	// klaim slot atomik (race-safe): cuma nambah kalau masih ada kuota
	res, err := db.Exec(`UPDATE redeem_codes SET used_count=used_count+1 WHERE code=? AND (max_uses=0 OR used_count<max_uses)`, code)
	if err != nil {
		writeJSON(w, 500, map[string]string{"err": err.Error()})
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		writeJSON(w, 400, map[string]string{"err": "kode sudah habis diklaim"})
		return
	}
	// catat klaim player (unique code+player → gak bisa dobel)
	if _, err := db.Exec(`INSERT INTO redemptions(code,player_id) VALUES(?,?)`, code, pid); err != nil {
		db.Exec(`UPDATE redeem_codes SET used_count=used_count-1 WHERE code=?`, code) // balikin slot
		writeJSON(w, 400, map[string]string{"err": "kode sudah kamu klaim sebelumnya"})
		return
	}
	// terapkan reward
	p, err := loadPlayer(pid)
	if err != nil {
		writeJSON(w, 400, map[string]string{"err": "player error"})
		return
	}
	msg := ""
	switch typ {
	case "gold":
		var n int64
		if payload != "" && payload[0] != '{' && payload[0] != '[' {
			if v, e := strconv.ParseInt(payload, 10, 64); e == nil {
				n = v
			}
		}
		if n <= 0 {
			writeJSON(w, 500, map[string]string{"err": "payload kode rusak"})
			return
		}
		p.Gold += n
		p.LifetimeGold += n
		msg = "🪙 +" + itoa(int(n)) + " gold"
	case "item":
		// payload spesial "rare_all:qty" = semua item rare+ (equip 1 pcs, stack sesuai qty)
		if strings.HasPrefix(payload, "rare_all") {
			qty := 1
			if i := strings.LastIndex(payload, ":"); i > 0 {
				if v, e := strconv.ParseInt(payload[i+1:], 10, 64); e == nil && v > 0 {
					qty = int(v)
				}
			}
			names := []string{}
			inv := normInv(p.Data["inv"])
			for id, it := range ITEMS {
				if it.Rarity != "rare" && it.Rarity != "epic" && it.Rarity != "legendary" {
					continue
				}
				addItemSrv(inv, id, qty)
				names = append(names, it.Nama)
			}
			p.Data["inv"] = inv
			msg = "🎁 " + itoa(len(names)) + " item langka ×" + itoa(qty)
			break
		}
		qty := 1 // qty disimpan di suffix payload "item_id:qty"
		itemID := payload
		if i := strings.LastIndex(payload, ":"); i > 0 {
			itemID = payload[:i]
			if v, e := strconv.ParseInt(payload[i+1:], 10, 64); e == nil && v > 0 {
				qty = int(v)
			}
		}
		it, ok := ITEMS[itemID]
		if !ok {
			writeJSON(w, 500, map[string]string{"err": "item di kode gak ada"})
			return
		}
		if isEquipID(itemID) {
			for i := 0; i < qty; i++ {
				if !bagHasRoom(p) {
					break
				}
				inv := normInv(p.Data["inv"])
				addItemSrv(inv, itemID, 1)
				p.Data["inv"] = inv
			}
		} else {
			addItemSrvInv(p, itemID, qty)
		}
		msg = "🎁 " + it.Nama + " ×" + itoa(qty)
	}
	savePlayerData(p)
	writeJSON(w, 200, map[string]any{"player": p, "msg": "🎉 Kode ditukar! " + msg})
}

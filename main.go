package main

import (
	"database/sql"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"strings"

	_ "github.com/go-sql-driver/mysql"
	"golang.org/x/crypto/bcrypt"
)

// ponytail: statik tanpa cache header — file kecil, 1 user; tambah ETag kalau berat

//go:embed static
var staticFS embed.FS

var db *sql.DB

type Player struct {
	ID           int64          `json:"id"`
	Username     string         `json:"username"`
	Gold         int64          `json:"gold"`
	Data         map[string]any `json:"data"`
	Power        int            `json:"power"`
	LifetimeGold int64          `json:"lifetime_gold"`
}

func main() {
	dsn := os.Getenv("DB_DSN")
	if dsn == "" {
		log.Fatal("DB_DSN kosong")
	}
	var err error
	db, err = sql.Open("mysql", dsn)
	if err != nil {
		log.Fatal(err)
	}
	if err = db.Ping(); err != nil {
		log.Fatal(err)
	}
	migrate()

	mux := http.NewServeMux()
	sub, _ := fs.Sub(staticFS, "static")
	mux.Handle("/", http.FileServer(http.FS(sub)))
	mux.HandleFunc("/api/register", handleRegister)
	mux.HandleFunc("/api/login", handleLogin)
	mux.HandleFunc("/api/classes", handleClasses)
	mux.HandleFunc("/api/choose", auth(handleChoose))
	mux.HandleFunc("/api/state", auth(handleState))
	mux.HandleFunc("/api/save", auth(handleSave))
	mux.HandleFunc("/api/shop", auth(handleShop))
	mux.HandleFunc("/api/farm", auth(handleFarm))
	mux.HandleFunc("/api/bag", auth(handleBag))
	mux.HandleFunc("/api/items", handleItems)
	mux.HandleFunc("/api/garden", auth(handleGarden))
	mux.HandleFunc("/api/dungeon", auth(handleDungeon))
	mux.HandleFunc("/api/boss", auth(handleBossFight))
	mux.HandleFunc("/api/bosses", auth(handleBossList))
	mux.HandleFunc("/api/skills", auth(handleSkills))
	mux.HandleFunc("/api/equip", auth(handleEquip))
	mux.HandleFunc("/api/upgrade", auth(handleUpgrade))
	mux.HandleFunc("/api/sell", auth(handleSell))

	addr := ":30512"
	log.Println("Dungeon Party listening on", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}

func migrate() {
	qs := []string{
		`CREATE TABLE IF NOT EXISTS players(
			id BIGINT AUTO_INCREMENT PRIMARY KEY,
			username VARCHAR(32) UNIQUE NOT NULL,
			pass_hash VARCHAR(255) NOT NULL,
			tg_id BIGINT NULL,
			gold BIGINT DEFAULT 0,
			gems INT DEFAULT 0,
			data JSON,
			power INT DEFAULT 0,
			lifetime_gold BIGINT DEFAULT 0,
			prestige INT DEFAULT 0,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP)`,
		`CREATE TABLE IF NOT EXISTS sessions(
			token VARCHAR(64) PRIMARY KEY,
			player_id BIGINT NOT NULL,
			expires_at TIMESTAMP NOT NULL)`,
	}
	for _, q := range qs {
		if _, err := db.Exec(q); err != nil {
			log.Fatal("migrate:", err)
		}
	}
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func handleRegister(w http.ResponseWriter, r *http.Request) {
	var req struct{ User, Pass string }
	json.NewDecoder(r.Body).Decode(&req)
	req.User = strings.TrimSpace(req.User)
	if len(req.User) < 3 || len(req.Pass) < 6 {
		writeJSON(w, 400, map[string]string{"err": "user min 3, pass min 6"})
		return
	}
	h, _ := bcrypt.GenerateFromPassword([]byte(req.Pass), bcrypt.DefaultCost)
	res, err := db.Exec(`INSERT INTO players(username, pass_hash, data) VALUES(?,?,?)`,
		req.User, string(h), `{}`)
	if err != nil {
		writeJSON(w, 409, map[string]string{"err": "username sudah dipakai"})
		return
	}
	id, _ := res.LastInsertId()
	token := newSession(id)
	writeJSON(w, 200, map[string]any{"token": token, "username": req.User})
}

func handleLogin(w http.ResponseWriter, r *http.Request) {
	var req struct{ User, Pass string }
	json.NewDecoder(r.Body).Decode(&req)
	var id int64
	var hash string
	err := db.QueryRow(`SELECT id, pass_hash FROM players WHERE username=?`, req.User).
		Scan(&id, &hash)
	if err != nil || bcrypt.CompareHashAndPassword([]byte(hash), []byte(req.Pass)) != nil {
		writeJSON(w, 401, map[string]string{"err": "login salah"})
		return
	}
	token := newSession(id)
	writeJSON(w, 200, map[string]any{"token": token, "username": req.User})
}

func newSession(playerID int64) string {
	b := make([]byte, 24)
	readURandom(b)
	token := fmt.Sprintf("%x", b)
	db.Exec(`INSERT INTO sessions(token, player_id, expires_at) VALUES(?,?,NOW()+INTERVAL 30 DAY)`,
		token, playerID)
	return token
}

func auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("Authorization")
		var pid int64
		err := db.QueryRow(`SELECT player_id FROM sessions WHERE token=? AND expires_at>NOW()`, token).Scan(&pid)
		if err != nil {
			writeJSON(w, 401, map[string]string{"err": "unauthorized"})
			return
		}
		r.Header.Set("X-Player-ID", fmt.Sprint(pid))
		next(w, r)
	}
}

func loadPlayer(pid int64) (*Player, error) {
	p := &Player{}
	var data []byte
	err := db.QueryRow(`SELECT id, username, gold, data, power, lifetime_gold FROM players WHERE id=?`, pid).
		Scan(&p.ID, &p.Username, &p.Gold, &data, &p.Power, &p.LifetimeGold)
	if err != nil {
		return nil, err
	}
	if len(data) > 0 {
		json.Unmarshal(data, &p.Data)
	}
	applyRegen(p) // HP regen pasca-pertarungan (1%/3s)
	return p, nil
}

func handleState(w http.ResponseWriter, r *http.Request) {
	p, err := loadPlayer(parseID(r.Header.Get("X-Player-ID")))
	if err != nil {
		writeJSON(w, 500, map[string]string{"err": "player gak ada"})
		return
	}
	writeJSON(w, 200, p)
}

func handleSave(w http.ResponseWriter, r *http.Request) {
	pid := parseID(r.Header.Get("X-Player-ID"))
	var in Player
	json.NewDecoder(r.Body).Decode(&in)
	dj, _ := json.Marshal(in.Data)
	// lifetime_gold dihitung server: selisih gold positif diakumulasi (anti cheat minim)
	_, err := db.Exec(`UPDATE players SET gold=?, data=?, power=?, lifetime_gold=lifetime_gold+GREATEST(?-gold,0) WHERE id=?`,
		in.Gold, string(dj), in.Power, in.Gold, pid)
	if err != nil {
		writeJSON(w, 500, map[string]string{"err": "save gagal"})
		return
	}
	writeJSON(w, 200, map[string]string{"ok": "1"})
}

func parseID(s string) int64 {
	var id int64
	fmt.Sscanf(s, "%d", &id)
	return id
}

func readURandom(b []byte) {
	f, _ := os.Open("/dev/urandom")
	defer f.Close()
	f.Read(b)
}

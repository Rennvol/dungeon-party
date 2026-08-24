# ROADMAP — Dungeon Party: Idle DnD Web Game

> Game idle RPG bertema Dungeons & Dragons. Hero farming gold otomatis, drag & drop
> party management, skill non-combat (masak!), survival journey, database MariaDB.
> Host: Oracle ARM (Go + MariaDB + static HTML). Repo GitHub per fase, rollback aman.

---

## GAMBARAN BESAR

```
Pemain → Browser (HTML+JS+DnD) → Go API (:30010) → MariaDB
                                      │
                                      └── static/ (game files)
```

- 1 binary Go: serve static + REST API + game logic server-side
- MariaDB: state pemain saja (1 JSON blob/player). Item/skill/dungeon = config di kode
- Frontend vanilla JS — tanpa framework. DnD pakai Pointer Events (mouse+touch)
- Auth: username dulu (fase 1), Telegram Login nanti (fase 4)

## LOOP INTI PEMAIN

```
Pilih hero & class → Dive dungeon (idle farm) → Gold + Loot drop
      ↑                                            │
      │                                    Shop: potion/ransel
   Prestige ← Stage unlock ← Journey survival ← Skill tree (Cooking!)
```

---

# FASE 0 — FONDASI (setup, ±1 sesi)

Tujuan: kerangka jalan, belum ada game.

- [ ] Repo GitHub `dungeon-party`, struktur:
  ```
  dungeon-party/
  ├── main.go            # HTTP server + embed static
  ├── db.go              # koneksi MariaDB
  ├── api.go             # endpoint save/load
  ├── config/            # items.go, classes.go, dungeons.go (data game)
  └── static/
      ├── index.html
      ├── style.css
      └── game.js
  ```
- [ ] DB: `CREATE DATABASE dnd;` tabel `players (id BIGINT AUTO_INCREMENT PK,
  username VARCHAR(32) UNIQUE, pass_hash VARCHAR(255), data JSON, updated_at)`
- [ ] Register/login (bcrypt), session token sederhana
- [ ] Endpoint: POST /api/register, /api/login, GET /api/state, POST /api/save
- [ ] Halaman kosong yang bisa login dan nyimpen 1 string ke DB
- **Selesai kalau:** daftar → login → reload → data masih ada
- Commit tag: `fase0`

---

# FASE 1 — CORE IDLE LOOP (jantung game)

Tujuan: bisa maem loop dasarnya.

- [ ] Pilih 1 hero utama: class Warrior/Mage/Ranger/Cleric (stats beda: ATK/HP)
- [ ] 1 dungeon "Gua Goblin": hero auto-farm → gold/tick + XP/tick
  - tick = 1 detik, dihitung client + divalidasi server saat save (anti cheat minim:
    simpan last_seen, income dibatasi elapsed real-time max 8 jam offline)
- [ ] Level hero: XP naik → level up → stats naik (kurva: xpNeed = 50 * lvl^1.5)
- [ ] Shop v1: Potion HP kecil/besar (buat nanti), harga gold flat
- [ ] Inventory sederhana (list item id + qty)
- [ ] Save: tombol manual + auto tiap 30 detik + on unload
- **Selesai kalau:** farm gold 5 menit → level up → beli potion → logout → login → semua masih ada
- Commit tag: `fase1`
- Detail angka awal (config/classes.go):
  - Warrior HP 100 ATK 12 | Mage HP 60 ATK 18 | Ranger HP 80 ATK 15 | Cleric HP 90 ATK 10(+heal)
  - Goblin dungeon: gold 1-3/tick, XP 2/tick, drop chance 5% (fase 3 baru kepake)

---

# FASE 2 — DUNGEON DROP & SHOP LENGKAP

- [ ] Tabel loot table per dungeon (config): common 70% / rare 25% / epic 5%
  - item: material (kulit goblin, bijih besi), equipment (pedang besi...), potion drop
- [ ] Equipment bisa di-equip ke hero (slot: weapon, armor, accessory) → stats nambah
- [ ] Shop lengkap: potion HP, potion stamina, ransel makanan (untuk journey fase 3),
  equipment dasar beli langsung
- [ ] Sell item (jual loot gak kepake → gold)
- [ ] Rarity color: abu/hijau/biru/ungu (common/rare/epic/legendary)
- **Selesai kalau:** drop pedang rare → equip → ATK naik keliatan di DPS farm
- Commit tag: `fase2`

---

# FASE 3 — JOURNEY SURVIVAL + SKILL TREE (identitas game ini)

- [ ] **Journey mechanic:** dungeon jauh butuh waktu tempuh (misal 30 menit real time).
  Selama journey party nguras bekal: 1 makanan + 1 potion per X menit.
  - Bekal habis → HP party gerus terus → bisa gagal dive (gold dikit, no drop)
  - Sebelum berangkat: checklist "bekal cukup?" — Cooking skill bikin bekal bagus
- [ ] **Skill tree per class** + skill umum:
  - Combat: Warrior(Berserk, Taunt), Mage(Fireball, Arcane Intellect), Ranger(multishot),
    Cleric(Heal, Bless)
  - **Non-combat: Cooking** (bikin bekal dari hasil hunt; level naik = bekal lebih awet),
    Herbalist (bikin potion dari herb drop), Blacksmith (repair durability, enchant)
  - Skill belajar pake gold + XP; level skill 1-10
- [ ] Hero rekrutan (party 3): slot tank/DPS/support — komposisi pengaruh clear rate
- [ ] Drag & drop tahap 1: drag equipment → hero portrait; drag hero → slot party
- **Selesai kalau:** masak 3 roti → journey 30 menit → pulang bawa loot epic
- Commit tag: `fase3`

---

# FASE 4 — STAGE UNLOCK, DICE, PRESTIGE

- [ ] 6 stage dungeon (syarat unlock campuran: total gold lifetime / party lv /
    item langka / boss kill):
  1. Gua Goblin (awal) · 2. Kuburan Terkutuk (500g) · 3. Rawa Bandit (25rb,
     1 epic) · 4. Kastil Necromancer (boss kill st.3) · 5. Kawah Naga (100rb) ·
  6. Lair Naga Purba (prestige 1x)
- [ ] Boss fight tiap stage akhir: sekali dive, d20 roll vs boss AC — crit/miss animasi dadu
- [ ] Prestige: reset gold/level → dapat Soul Crystal (×income permanen) + syarat stage 6
- [ ] Diamond dari first-clear stage → upgrade QoL permanen (offline cap naik dll)
- [ ] Drag & drop tahap 2: drag hero ke dungeon card buat assign dive
- **Commit tag: `fase4`**

---

# FASE 5 — TELEGRAM INTEGRATION & POLISH

- [ ] Login/bind via bot Telegram (@GetInfo kamu): /bind → web link token sekali pakai
  → players.tg_id terisi → login tanpa password seterusnya
- [ ] Notif TG: dive selesai, boss ready, journey pulang
- [ ] Leaderboard (total gold lifetime) — query JSON blob, index kolom generated
- [ ] Polish: sound effect mini, toast, dark fantasy theme CSS, responsive mobile
- [ ] Offline earnings modal ("Selama kamu pergi: +X gold")
- **Commit tag: `v1.0`**

---

# YANG SENGAJA DITUNDA (jangan dibikin sebelum v1.0)

- Sprite/gambar art — emoji dulu, ganti belakangan
- Guild/PVP/trade antar pemain — butuh validasi server kompleks
- Normalisasi DB multi-tabel — JSON blob cukup sampai leaderboard besar
- Framework frontend — vanilla kuat untuk ini

# RISIKO & CATATAN

- ARM Oracle RAM terbatas: Go + MariaDB ringan, aman bersama treasury+telecloud
- Anti-cheat: ini single-player idle, cheat cuma merugikan diri sendiri — validasi
  wajar saja (elapsed clamp), jangan overengineer
- Backup: mysqldump cron mingguan → folder backup existing

# DEFINITION OF DONE PER FITUR

Kode jalan di server + commit push GitHub + bisa dimainkan dari HP browser +
angka balance masuk akal (farm 1 jam ≠ langsung tamat).

# ROADMAP — Dungeon Party: Idle DnD Web Game (v2 AMBITIOUS)

> Game idle RPG bertema Dungeons & Dragons. Hero farming gold otomatis, drag & drop
> party management, skill non-combat (masak!), survival journey, database MariaDB.
> **v2 tambahan user:** story/dialog per progress, redeem code (admin tools),
> guild + leaderboard, power system hero vs musuh, instant raid farm.
> Host: Oracle ARM (Go + MariaDB + static HTML). Repo GitHub per fase, rollback aman.

---

## GAMBARAN BESAR

```
Pemain → Browser (HTML+JS+DnD) → Go API (:30010) → MariaDB
                                      │
                                      └── static/ (game files)
```

- 1 binary Go: serve static + REST API + game logic server-side
- MariaDB: tabel relasional penuh (bukan blob) — game ini dirancang besar & jangka panjang
- Frontend vanilla JS modular (ES modules per fitur) — tanpa framework, tapi terstruktur
- DnD pakai Pointer Events (mouse+touch)
- Auth: username dulu (fase 1), Telegram Login nanti
- **Skala target:** ratusan pemain, puluhan dungeon, ratusan item/skill — config-driven,
  nambah konten = edit file config, bukan rewrite kode

---

## SISTEM INTI YANG DIRANCANG SEJAK AWAL

1. **Power System**
   - `hero.power = ATK*2 + HP/10 + DEF*3 + skillBonus` (rumus di 1 tempat, server-side)
   - Musuh punya power juga. Dungeon butuh min-power buat clear rate 100%
   - Power gap → clear rate turun (d20-style roll internal): `chance = clamp(0.5 + (myP-enemyP)/enemyP * 0.5, 0.05, 0.99)`
   - Ditampilkan di UI: "Party Power 1.240 vs Dungeon 900 ✅"

2. **Story & Dialog Engine**
   - Cerita per arc (6 arc = 6 stage). Tiap arc punya chapter dialog:
     `config/story.json` → array {speaker, portrait_emoji, text, trigger}
   - Trigger: first-enter-stage, boss-intro, boss-defeat, prestige
   - Dialog box visual-novel style di bawah layar, tap untuk lanjut, bisa di-skip
   - Arc contoh: Arc 1 "Bayangan di Gua Goblin" → Arc 6 "Takhta Sang Naga"
   - NPC tetap: Elara si penjual misterius, Gronnak orc pandai besi, Raja Hantu

3. **Redeem Code (admin tools)**
   - Tabel DB: `codes (code PK, type, payload JSON, max_uses, uses, expires_at, created_by)`
   - Type: gold, item(id+qty), gem, ticket_raid
   - Admin panel sederhana (hanya dari IP owner / admin_key): generate code random,
     set payload & limit → kasih code ke player → player input di menu Redeem
   - Contoh: `GOLD500` = 50.000 gold sekali pakai, `TESTALL` = semua item qty 99

4. **Instant Raid**
   - Dungeon yang sudah 100% cleared bisa di-*instant raid*: langsung dapat hasil
     rata-rata tanpa animasi, tapi butuh **Raid Ticket** (drop jarang / beli / redeem)
   - Anti-abuse: cooldown per dungeon + ticket consumption server-side
   - Ini jadi sink utama Raid Ticket + QoL endgame farming

5. **Guild & Leaderboard**
   - Tabel: `guilds(id, name, desc, leader_id, level, exp)`, `guild_members(guild_id, player_id, role)`
   - Guild perk per level: +% gold, +% drop rate, slot member naik
   - Guild weekly contribution (gold donasi) → guild exp
   - Leaderboard: global (power, total gold lifetime), guild (guild power total)
   - Halaman leaderboard auto-refresh 30 detik

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
  ├── main.go            # HTTP server + embed static + router
  ├── db/                # koneksi & migrasi MariaDB
  ├── api/               # handler per domain (auth, player, raid, guild, admin)
  ├── game/              # logic: power, loot, combat, journey
  ├── config/            # items.json, classes.json, dungeons.json, story.json, skills.json
  └── static/
      ├── index.html
      ├── style.css
      └── js/            # ES modules: state.js, ui.js, dnd.js, api.js, dialog.js, ...
  ```
- [ ] DB relasional (migrasi otomatis saat start):
  ```sql
  players(id PK, username UNIQUE, pass_hash, tg_id NULL, gold BIGINT, gems INT,
          data JSON,          -- inventory, party, progress ringan
          power INT, lifetime_gold BIGINT, prestige INT,
          created_at, updated_at)
  codes(code PK, type, payload JSON, max_uses, uses, expires_at, created_by)
  code_redemptions(code, player_id, redeemed_at, UNIQUE(code,player_id))
  guilds(id PK, name UNIQUE, level, exp, leader_id)
  guild_members(guild_id, player_id PK, role, contribution_weekly)
  sessions(token PK, player_id, expires_at)
  ```
- [ ] Register/login (bcrypt), session token sederhana
- [ ] Endpoint: POST /api/register, /api/login, GET /api/state, POST /api/save
- [ ] Halaman kosong yang bisa login dan nyimpen state ke DB
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

# FASE 4 — STAGE, STORY, POWER & RAID

- [ ] 6 stage dungeon (syarat unlock campuran: total gold lifetime / party lv /
    item langka / boss kill):
  1. Gua Goblin (awal) · 2. Kuburan Terkutuk (500g) · 3. Rawa Bandit (25rb,
     1 epic) · 4. Kastil Necromancer (boss kill st.3) · 5. Kawah Naga (100rb) ·
  6. Lair Naga Purba (prestige 1x)
- [ ] **Power system aktif:** hero.power dihitung server, dungeon punya enemy power,
  UI tampilkan perbandingan party power vs dungeon power + clear rate %
- [ ] **Story engine:** config/story.json — dialog visual-novel per arc
  (first-enter, boss-intro, boss-defeat). NPC: Elara, Gronnak, Raja Hantu.
  Dialog box bawah layar, tap-next, skip-able
- [ ] Boss fight tiap stage akhir: sekali dive, roll internal vs power gap — crit/miss
- [ ] **Instant Raid:** dungeon cleared → tombol Raid (butuh Raid Ticket) → hasil
  instan rata-rata ×multiplier guild perk. Cooldown + konsumsi ticket server-side
- [ ] Prestige: reset gold/level → Soul Crystal (×income permanen) + syarat stage 6
- [ ] Diamond dari first-clear stage → upgrade QoL permanen
- [ ] Drag & drop tahap 2: drag hero ke dungeon card buat assign dive
- **Commit tag: `fase4`**

---

# FASE 5 — REDEEM CODE + ADMIN PANEL

- [ ] Admin panel `/admin` (auth admin_key env, akses dari IP owner saja):
  - Generate redeem code: pilih type (gold/item/gem/raid_ticket), payload,
    max_uses, expired
  - List code + usage count
- [ ] Player UI: menu Redeem → input code → validasi server (uses < max_uses,
  belum dipakai player ini, belum expire) → terapkan reward → catat redemption
- [ ] Contoh pakai: `TESTGOLD` 50rb, `ALLITEM99` semua item qty 99 buat test
- **Commit tag: `fase5`**

---

# FASE 6 — GUILD & LEADERBOARD

- [ ] Buat guild (biaya gold), invite via nama player, role leader/officer/member
- [ ] Guild donasi gold harian → guild exp → level naik → perk (+%gold, +%drop,
  slot member)
- [ ] Leaderboard halaman khusus: Top 50 global by power / lifetime gold;
  top guild by total member power; posisi player sendiri selalu keliatan
- [ ] Guild chat sederhana? — TUNDA sampai v1.x (polling 5 detik cukup nanti)
- **Commit tag: `fase6`**

---

# FASE 7 — TELEGRAM INTEGRATION & POLISH

- [ ] Login/bind via bot Telegram (@GetInfo kamu): /bind → web token sekali pakai
  → players.tg_id terisi → login tanpa password seterusnya
- [ ] Notif TG: dive selesai, boss ready, journey pulang, raid ticket full
- [ ] Polish: sound effect mini, toast, dark fantasy CSS final, responsive mobile
- [ ] Offline earnings modal ("Selama kamu pergi: +X gold")
- [ ] Balance pass: kurva gold vs item price vs stage difficulty
- **Commit tag: `v1.0`**

---

# YANG SENGAJA DITUNDA (jangan dibikin sebelum v1.0)

- Sprite/gambar art — emoji dulu, ganti belakangan
- PVP antar pemain — butuh matchmaking & validasi ketat
- Realtime guild chat — polling cukup
- Framework frontend / bundler — vanilla modular kuat untuk ini

# RISIKO & CATATAN

- ARM Oracle RAM terbatas: Go + MariaDB ringan, aman bersama treasury+telecloud.
  Port baru :30010 → buka iptables + pm2 save
- Anti-cheat: server-side validation untuk gold/raid/redeem; client cuma render
- Backup: mysqldump cron harian dungeon-party DB
- Semua balance number di config JSON — tuning gak perlu recompile

# DEFINITION OF DONE PER FASE

Kode jalan di server (pm2) + commit+push GitHub + bisa dimainkan dari HP browser +
fitur fase itu teruji end-to-end + ROADMAP.md checkbox dicentang.

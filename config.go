package main

// Config fase 1 — semua angka balance di sini (nanti dipindah ke config/*.json)

type Class struct {
	ID      string `json:"id"`
	Nama    string `json:"nama"`
	HP      int    `json:"hp"`
	ATK     int    `json:"atk"`
	Element string `json:"element"`
	Lore    string `json:"lore"`
}

var CLASSES = map[string]Class{
	"warrior": {"warrior", "🛡️ Warrior", 100, 12, "api",
		"Pejuang garis depan dari kota Benteng Merah. Kehilangan saudaranya saat Gua Goblin runtuh — kini ia turun gunung membawa kapak dan dendam."},
	"mage": {"mage", "🧙 Mage", 60, 18, "listrik",
		"Murid menara Arcane yang diusir karena eksperimen terlarang. Badai mengikutinya kemana pun ia melangkah."},
	"ranger": {"ranger", "🏹 Ranger", 80, 15, "alam",
		"Pemburu hutan Rimba Akhir. Bicara dengan serigala lebih sering daripada dengan manusia, dan tak pernah meleset."},
	"cleric": {"cleric", "✨ Cleric", 90, 10, "cahaya",
		"Imamat Kuil Fajar. Suaranya menenangkan luka — tapi jangan salah, tongkat sucinya sama mematikan dengan doanya."},
}

var DUNGEONS = map[string]Dungeon{
	"gua_goblin":       {ID: "gua_goblin", Nama: "🕳️ Gua Goblin", MinLvl: 1, XP: 2, DropPct: 15, Element: "gelap", EnemyPow: 40},
	"tambang_runtuh":   {ID: "tambang_runtuh", Nama: "⛏️ Tambang Runtuh", MinLvl: 5, XP: 8, DropPct: 35, Element: "alam", EnemyPow: 120},
	"neraka_kegelapan": {ID: "neraka_kegelapan", Nama: "💀 Neraka Kegelapan", MinLvl: 12, XP: 22, DropPct: 60, Element: "gelap", EnemyPow: 320},
	"kuburan_terkutuk": {ID: "kuburan_terkutuk", Nama: "🪦 Kuburan Terkutuk", MinLvl: 18, XP: 45, DropPct: 65, Element: "gelap", EnemyPow: 650, UnlockGold: 500},
	"rawa_bandit":      {ID: "rawa_bandit", Nama: "🐊 Rawa Bandit", MinLvl: 25, XP: 90, DropPct: 70, Element: "air", EnemyPow: 1300, UnlockGold: 25000, UnlockBoss: "ratu_labalaba", UnlockBossN: 3},
	"lahar_naga":       {ID: "lahar_naga", Nama: "🌋 Kawah Naga", MinLvl: 32, XP: 180, DropPct: 80, Element: "api", EnemyPow: 2600, UnlockGold: 100000, UnlockBoss: "naga_bara", UnlockBossN: 5},
}

type Dungeon struct {
	ID       string `json:"id"`
	Nama     string `json:"nama"`
	MinPower int    `json:"min_power"`
	MinLvl   int    `json:"min_lvl"`
	GoldMin  int    `json:"gold_min"`
	GoldMax  int    `json:"gold_max"`
	XP       int    `json:"xp"`
	DropPct  int    `json:"drop_pct"`
	Element  string `json:"element,omitempty"`
	EnemyPow int    `json:"enemy_pow"` // power musuh buat clear-rate
	// unlock requirements (0/kosong = skip)
	UnlockGold   int64  `json:"unlock_gold,omitempty"`
	UnlockBoss   string `json:"unlock_boss,omitempty"`
	UnlockBossN  int    `json:"unlock_boss_n,omitempty"`
	PrestigeReq  int    `json:"prestige_req,omitempty"`
}

// KEBUN HERBAL — sumber gold (panen tanaman)
const (
	gardenRateGoldPerSec = 0.7 // per detik per lv kebun
	gardenHerbEverySec   = 45  // tiap 45 detik tumbuh 1 herbal
	gardenCapHours       = 8
)

// BOSS — dadu d20 + potion
type Boss struct {
	ID      string `json:"id"`
	Nama    string `json:"nama"`
	Element string `json:"element"`
	HP      int    `json:"hp"`
	ATK     int    `json:"atk"`
	DEF     int    `json:"def"`
	MinLvl  int    `json:"min_lvl"` // syarat unlock
	GoldWin int64  `json:"gold_win"`
	XPWin   int    `json:"xp_win"`
}

var BOSSES = []Boss{
	{"raja_goblin", "👑 Raja Goblin", "gelap", 350, 16, 5, 5, 1500, 400},
	{"ratu_labalaba", "🕷️ Ratu Laba-laba", "alam", 520, 22, 9, 10, 2600, 700},
	{"naga_bara", "🐉 Naga Bara", "api", 800, 30, 14, 16, 4200, 1200},
	{"sang_pemakai", "☠️ Sang Pemakai", "gelap", 1300, 42, 20, 24, 7500, 2200},
}

// gelar boss acak — nama berubah tiap pertarungan
var BOSS_TITLES = []string{
	"Ancaman", "Penghancur", "Sang Kejam", "Tirani", "Kegelapan",
	"Badai", "Petaka", "Raja", "Penguasa", "Kutukan",
}

// ELEMEN: api>alam>listrik>air>api ; cahaya<->gelap
func elemMult(a, b string) float64 {
	wheel := map[string]string{"api": "alam", "alam": "listrik", "listrik": "air", "air": "api"}
	if wheel[a] == b {
		return 1.5
	}
	if wheel[b] == a {
		return 0.75
	}
	if (a == "cahaya" && b == "gelap") || (a == "gelap" && b == "cahaya") {
		return 1.5
	}
	return 1.0
}

func xpNeed(lvl int) int {
	base := 50.0
	for i := 1; i < lvl; i++ {
		base *= 1.35
	}
	return int(base)
}

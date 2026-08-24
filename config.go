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
	"gua_goblin": {ID: "gua_goblin", Nama: "🕳️ Gua Goblin", MinPower: 0,
		GoldMin: 1, GoldMax: 3, XP: 2},
}

type Dungeon struct {
	ID       string `json:"id"`
	Nama     string `json:"nama"`
	MinPower int    `json:"min_power"`
	GoldMin  int    `json:"gold_min"`
	GoldMax  int    `json:"gold_max"`
	XP       int    `json:"xp"`
}

func xpNeed(lvl int) int {
	base := 50.0
	for i := 1; i < lvl; i++ {
		base *= 1.35
	}
	return int(base)
}

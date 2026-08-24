package main

// Config fase 1 — semua angka balance di sini (nanti dipindah ke config/*.json)

type Class struct {
	ID   string `json:"id"`
	Nama string `json:"nama"`
	HP   int    `json:"hp"`
	ATK  int    `json:"atk"`
}

var CLASSES = map[string]Class{
	"warrior": {"warrior", "🛡️ Warrior", 100, 12},
	"mage":    {"mage", "🧙 Mage", 60, 18},
	"ranger":  {"ranger", "🏹 Ranger", 80, 15},
	"cleric":  {"cleric", "✨ Cleric", 90, 10},
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

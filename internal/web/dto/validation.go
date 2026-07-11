package dto

import "f1/internal/models"

type Setup struct {
	Name          string               `json:"name"`
	AeroDynamic   int                  `json:"aero_dynamic"`
	Engine        int                  `json:"engine"`
	Chassis       int                  `json:"chassis"`
	Floor         int                  `json:"floor"`
	Tyres         int                  `json:"tyres"`
	Reliability   int                  `json:"reliability"` // 35 tokens = 0% DNF chance
	SettingsAngle models.SettingsAngle `json:"settings_angle"`
}

type UpdateType int

const (
	CarUpdate UpdateType = iota
	SynergyUpdate
)

type Updates struct {
	Type  UpdateType `json:"type"`
	Coast int        `json:"coast"`
	Stage int64      `json:"stage"`
}

type RaceSetup struct {
	Setup string `json:"setup"`
}

type BaseUpdate struct {
	Base     int `json:"base"`
	Engineer int `json:"engineer"`
	Tube     int `json:"tube"`
	Sim      int `json:"sim"`
}

type DraftItem int

const (
	DraftPilot DraftItem = iota
	DraftTeam
	DraftPrincipal
)

type Draft struct {
	Pick   DraftItem       `json:"pick"`
	ItemID int64           `json:"item_id"`
	Engine *models.ICEName `json:"engine,omitempty"` // обязателен для DraftTeam
}

// DraftBotSwap — тело запроса на обмен пилотами между командами-ботами.
type DraftBotSwap struct {
	TeamA  int64 `json:"team_a"`
	TeamB  int64 `json:"team_b"`
	PilotA int64 `json:"pilot_a"`
	PilotB int64 `json:"pilot_b"`
}

type Group struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Password string `json:"password"`
}

type PilotTransfer struct {
	PilotID int64 `json:"pilot_id"`
	Price   int   `json:"price"`
}

type PrincipalTransfer struct {
	PrincipalID int64 `json:"principal_id"`
	Price       int   `json:"price"`
}

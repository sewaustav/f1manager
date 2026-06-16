package models

type DrivingStyle int

const (
	Aggressive DrivingStyle = iota
	Balance
	Smooth
)

type SettingsAngle int

const (
	Rear SettingsAngle = iota
	Front
)

type DriverEmotion int

const (
	Emotional DriverEmotion = iota
	Consistent
	Calm
)

type DriverStability int

const (
	High DriverStability = iota
	Medium
	Low
)

type RainDriving int

const (
	MasterOfRain RainDriving = iota
	Normal
	Slow
)

type Pilot struct {
	ID                 int64
	Name               string
	Garage             *int64
	Team               *int64
	Rating             int // max 100
	QualifyingRating   int
	DrivingStyle       DrivingStyle
	Experience         int // max 100, +5 after season
	Adaptiveness       int // percentage
	Emotions           DriverEmotion
	Stability          DriverStability
	Rain               RainDriving
	SettingsAngle      SettingsAngle
	Starting           int // position retention skill
	TyreManagement     int
	MistakePossibility int // 1-20%
	Price              int // millions
	Sponsors           int // millions
}

type DownForce int

const (
	HighDownforce DownForce = iota
	MediumDownForce
	HighDrag
)

type TrackType int

const (
	Classic TrackType = iota
	City
)

type QualifyingImpact int

const (
	HighImpact QualifyingImpact = iota
	DecentImpact
	LowImpact
)

type Track struct {
	ID               int64
	Name             string
	DownForceLevel   DownForce
	Type             TrackType
	Difficulty       int // 50-85
	QualifyingImpact QualifyingImpact
	RainPossibility  int // percentage
	Tyre             int
}

type PilotTrack struct {
	ID      int64
	PilotID int64
	TrackID int64
	Level   int // 0-20
}

type ICEName int

const (
	Ferrari ICEName = iota
	Mercedes
	RBPT
	Honda
	Audi
	BMW
	Toyota
	Cadillac
	Renaute
	Self
)

type IsManufacturer int

const (
	Manufacture IsManufacturer = iota
	Semi
	Client
)

type Team struct {
	ID             int64
	Name           string
	ICE            ICEName
	CarLevel       int
	BaseLevel      int
	Engineer       int
	SimLevel       int
	TubeLevel      int
	UpdateRating   int // up to 10
	Tokens         int
	Budget         int // millions
	IsManufacturer IsManufacturer
}

type Car struct {
	TeamID      int64
	AeroDynamic int
	Engine      int
	Chassis     int
	Floor       int
	Tyres       int
	Reliability int // 55 tokens = 0% DNF chance
	SettingsAngle  SettingsAngle
}

type TeamPrincipal struct {
	ID     int64
	Name   string
	Price  int
	TeamID int64
	Level  int // 10-30
}

type Engine struct {
	ID        int64
	Engine    ICEName
	Price     int
	BaseLevel int
}

type Player struct {
	ID            int64
	Name          string
	TeamPrincipal int64
	Team          int64
	Budget        int
	Tokens        int
}

type PlayerProfile struct {
	ID            int64
	Name          string
	TeamPrincipal string
	Team          int64
	Pilot1        string
	Pilot2        string
	Budget        int
	Tokens        int
}

// Структуры для вывода результатов гонки
type RaceResult struct {
	PilotID       int64
	GarageID int64
	PilotName     string
	TeamName      string
	QualiPosition int
	RacePosition  int
	Points        int
	IsDNF         bool
	DNFReason     string
}

package main

import (
	"database/sql"
	"encoding/csv"
	"errors"
	data "f1/initial_data"
	"f1/internal/models"
	"io"
	"strconv"
	
	_ "github.com/mattn/go-sqlite3"
)

type Seed struct {
	DB *sql.DB	
}

func NewSeed(db *sql.DB) *Seed {
	return &Seed{DB: db}
}

func main() {
	db, err := sql.Open("sqlite3", "./f1_simulation.db?_foreign_keys=on")
	if err != nil {
		panic(err)
	}
	defer db.Close()
	
	seed := NewSeed(db)
	
	seed.createTables()
	seed.parseBaseData()
}

func (s *Seed) createTables() {
	pilotTable := `
	CREATE TABLE IF NOT EXISTS pilots_initial (
	    id INTEGER PRIMARY KEY AUTOINCREMENT,
	    name TEXT,
	    rating INTEGER,
	    quali_rating INTEGER,
	    style INTEGER,
	    expirince INTEGER,
	    adaptiveness INTEGER,
	    emotions INTEGER,
	    stability INTEGER,
	    rain INTEGER,
	    settings_angle INTEGER,
	    starting INTEGER,
	    tyre_management INTEGER,
	    mistake_possibility INTEGER,
	    price INTEGER,
	    sponsors INTEGER
	)`
	
	if _, err := s.DB.Exec(pilotTable); err != nil {
		panic(err)
	}
	
	currentPilotTable := `
	CREATE TABLE IF NOT EXISTS pilots (
	    id INTEGER PRIMARY KEY AUTOINCREMENT,
	    name TEXT,
	    rating INTEGER,
	    quali_rating INTEGER,
	    style INTEGER,
	    expirince INTEGER,
	    adaptiveness INTEGER,
	    emotions INTEGER,
	    stability INTEGER,
	    rain INTEGER,
	    settings_angle INTEGER,
	    starting INTEGER,
	    tyre_management INTEGER,
	    mistake_possibility INTEGER,
	    price INTEGER,
	    sponsors INTEGER
	)`
	
	if _, err := s.DB.Exec(currentPilotTable); err != nil {
		panic(err)
	}
	
	pilotTrackTable := `
	CREATE TABLE IF NOT EXISTS pilots_track_initial (
    	id INTEGER PRIMARY KEY AUTOINCREMENT,
    	pilot_id INTEGER,
    	track_id INTEGER,
    	FOREIGN KEY(pilot_id) REFERENCES pilots(id),
    	FOREIGN KEY(track_id) REFERENCES tracks(id)
	)
	`
	
	if _, err := s.DB.Exec(pilotTrackTable); err != nil {
		panic(err)
	}
	
	currentPilotTrackTable := `
	CREATE TABLE IF NOT EXISTS pilots_track (
	    id INTEGER PRIMARY KEY AUTOINCREMENT,
	    pilot_id INTEGER,
	    track_id INTEGER,
	    FOREIGN KEY(pilot_id) REFERENCES pilots(id),
	    FOREIGN KEY(track_id) REFERENCES tracks(id)
	)
	`
	
	if _, err := s.DB.Exec(currentPilotTrackTable); err != nil {
		panic(err)
	}
	
	trackTable := `
	CREATE TABLE IF NOT EXISTS tracks (
	    id INTEGER PRIMARY KEY AUTOINCREMENT,
	    name TEXT,
	    downforce INTEGER,
	    type INTEGER,
	    difficulity INTEGER,
	    quali_impact INTEGER,
	    rain INTEGER,
	    tyre INTEGER
	)
	`
	
	if _, err := s.DB.Exec(trackTable); err != nil {
		panic(err)
	}
	
	teamPrincipalsTable := `
	CREATE TABLE IF NOT EXISTS teams_principals (
	    id INTEGER PRIMARY KEY AUTOINCREMENT,
	    name TEXT,
	    price INTEGER,
	    level INTEGER
	)
	`
	if _, err := s.DB.Exec(teamPrincipalsTable); err != nil {
		panic(err)
	}
	
	engineTable := `
	CREATE TABLE IF NOT EXISTS engine (
	    id INTEGER PRIMARY KEY AUTOINCREMENT,
	    manufacturer TEXT,
	    price INTEGER,
	    power INTEGER
	)
	`
	
	if _, err := s.DB.Exec(engineTable); err != nil {
		panic(err)
	}
	
	baseTeamTable := `
	CREATE TABLE IF NOT EXISTS base_team (
	    id INTEGER PRIMARY KEY AUTOINCREMENT,
	    name TEXT,
	    car_lvl INTEGER,
	    ice INTEGER,
	    base_lvl INTEGER,
	    engineer INTEGER,
	    tube INTEGER,
	    sim INTEGER,
	    update_rtg INTEGER,
	    is_manuf INTEGER
	)
	`
	
	if _, err := s.DB.Exec(baseTeamTable); err != nil {
		panic(err)
	}
	
	currentTeamTable := `
	CREATE TABLE IF NOT EXISTS teams (
	    id INTEGER PRIMARY KEY AUTOINCREMENT,
	    name TEXT,
	    car_lvl INTEGER,
	    ice INTEGER,
	    base_lvl INTEGER,
	    engineer INTEGER,
	    tube INTEGER,
	    sim INTEGER,
	    update_rtg INTEGER,
	    is_manuf INTEGER
	)
	`
	
	if _, err := s.DB.Exec(currentTeamTable); err != nil {
		panic(err)
	}
	
	playerTable := `
	CREATE TABLE IF NOT EXISTS players (
	    id INTEGER PRIMARY KEY AUTOINCREMENT,
	    name TEXT,
	    team_id INTEGER,
	    pilot1_id INTEGER,
	    pilot2_id INTEGER,
	    principal_id INTEGER,
	    FOREIGN KEY(team_id) REFERENCES teams(id),
	    FOREIGN KEY(pilot1_id) REFERENCES pilots(id),
	    FOREIGN KEY(pilot2_id) REFERENCES pilots(id),
	    FOREIGN KEY(principal_id) REFERENCES teams_principals(id)
	)
	`
	
	if _, err := s.DB.Exec(playerTable); err != nil {
		panic(err)
	}
	
}

func (s *Seed) seedData() {
	
}

func (s *Seed) parseBaseData() []models.Team {
	base, err := data.DataFS.Open("base.csv")
	if err != nil {
		panic(err)
	}
	defer base.Close()
	
	reader := csv.NewReader(base)
	
	var baseData []models.Team
	
	for {
		row, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			panic(err)
		}
		
		if row[0] == "" {
			continue
		}
		
		carLevel, _ := strconv.Atoi(row[1])
		baseLevel, _ := strconv.Atoi(row[2])
		engineer, _ := strconv.Atoi(row[3])
		tube, _ := strconv.Atoi(row[4])
		sim, _ := strconv.Atoi(row[5])
		updateRTG, _ := strconv.Atoi(row[6])
		isManufacture, _ := strconv.Atoi(row[7])
		ice, _ := strconv.Atoi(row[8])
		budget, _ := strconv.Atoi(row[9])
		
		baseData = append(baseData, models.Team{
			Name:           row[0],
			CarLevel:       carLevel,
			BaseLevel:      baseLevel,
			Engineer:       engineer,
			TubeLevel:      tube,
			SimLevel:       sim,
			UpdateRating:   updateRTG,
			IsManufacturer: models.IsManufacturer(isManufacture),
			ICE:            models.ICEName(ice),
			Budget:         budget,
			Tokens:         100,
		})
		
	}
	
	return baseData
}

func (s *Seed) parseEngineData() []models.Engine {
	engine, err := data.DataFS.Open("engine.csv")
	if err != nil {
		panic(err)
	}
	defer engine.Close()
	
	reader := csv.NewReader(engine)
	
	var engineData []models.Engine
	
	for {
		row, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			panic(err)
		}
		
		if row[0] == "" {
			continue
		}
		
		var engine models.ICEName
		
		switch row[0] {
		case "Ferrari": engine = models.Ferrari
		case "Mercedes": engine = models.Mercedes
		case "RBPT": engine = models.RBPT
		case "Honda": engine = models.Honda
		case "Audi": engine = models.Audi
		case "BMW": engine = models.BMW
		case "Toyota": engine = models.Toyota
		case "Cadillac": engine = models.Cadillac
		case "Renate": engine = models.Renaute
		case "Self": engine = models.Self
		}
		
		price, _ := strconv.Atoi(row[1])
		power, _ := strconv.Atoi(row[2])
		
		engineData = append(engineData, models.Engine{
			Engine: engine,
			Price: price,
			BaseLevel: power,
			
		})
		
	}
	
	return engineData
}

func (s *Seed) parseTrackData() []models.Track {
	track, err := data.DataFS.Open("track.csv")
	if err != nil {
		panic(err)
	}
	defer track.Close()
	
	reader := csv.NewReader(track)
	
	var trackData []models.Track
	
	for {
		row, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			panic(err)
		}
		
		if row[0] == "" {
			continue
		}
		
		downForce, _ := strconv.Atoi(row[1])
		typeTrack, _ := strconv.Atoi(row[2])
		difficulty, _ := strconv.Atoi(row[3])
		qualifyingImpact, _ := strconv.Atoi(row[4])
		rain, _ := strconv.Atoi(row[5])
		tyre, _ := strconv.Atoi(row[6])
		
		trackData = append(trackData, models.Track{
			Name:           row[0],
			DownForceLevel: models.DownForce(downForce),
			Type: models.TrackType(typeTrack),
			Difficulty: difficulty,
			QualifyingImpact: models.QualifyingImpact(qualifyingImpact),
			RainPossibility: rain,
			Tyre: tyre,
		})
	}
	
	return trackData
}

func (s *Seed) parsePrincipalData() []models.TeamPrincipal {
	principal, err := data.DataFS.Open("team_principal.csv")
	if err != nil {
		panic(err)
	}
	defer principal.Close()
	
	reader := csv.NewReader(principal)
	
	var principalData []models.TeamPrincipal
	
	for {
		row, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			panic(err)
		}
		
		if row[0] == "" {
			continue
		}
		
		price, _ := strconv.Atoi(row[1])
		level, _ := strconv.Atoi(row[2])
		
		principalData = append(principalData, models.TeamPrincipal{
			Name: row[0],
			Price: price,
			Level: level,
		})
	}
	
	return principalData
}

func (s *Seed) parsePilotData() []models.Pilot {
	pilot, err := data.DataFS.Open("pilot.csv")
	if err != nil {
		panic(err)
	}
	defer pilot.Close()
	
	reader := csv.NewReader(pilot)
	
	var pilotData []models.Pilot
	
	for {
		row, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			panic(err)
		}
		
		if row[0] == "" {
			continue
		}
		
		rating, _ := strconv.Atoi(row[1])
		qualifyingRating, _ := strconv.Atoi(row[2])
		style, _ := strconv.Atoi(row[3])
		experience, _ := strconv.Atoi(row[4])
		adaptiveness, _ := strconv.Atoi(row[5])
		emotions, _ := strconv.Atoi(row[6])
		stability, _ := strconv.Atoi(row[7])
		rain, _ := strconv.Atoi(row[8])
		angle, _ := strconv.Atoi(row[9])
		starting, _ := strconv.Atoi(row[10])
		tyre, _ := strconv.Atoi(row[11])
		mistakes, _ := strconv.Atoi(row[12])
		price, _ := strconv.Atoi(row[13])
		sponsors, _ := strconv.Atoi(row[14])
		
		pilotData = append(pilotData, models.Pilot{
			Name: row[0],
			Rating: rating,
			QualifyingRating: qualifyingRating,
			DrivingStyle: models.DrivingStyle(style),
			Experience: experience,
			Adaptiveness: adaptiveness,
			Emotions: models.DriverEmotion(emotions),
			Stability: models.DriverStability(stability),
			Rain: models.RainDriving(rain),
			SettingsAngle: models.SettingsAngle(angle),
			Starting: starting,
			TyreManagement: tyre,
			MistakePossibility: mistakes,
			Price: price,
			Sponsors: sponsors,
		})
	}
	
	return pilotData
}

func (s *Seed) parsePilotTrackData() []models.PilotTrack {
	pilotTrack, err := data.DataFS.Open("pilot_track.csv")
	if err != nil {
		panic(err)
	}
	defer pilotTrack.Close()
	
	reader := csv.NewReader(pilotTrack)
	
	var pilotTrackData []models.PilotTrack
	
	for {
		row, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			panic(err)
		}
		
		if row[0] == "" {
			continue
		}
		
		pilotTrackData = append(pilotTrackData, models.PilotTrack{
		})
		
	}
	
	return pilotTrackData
}
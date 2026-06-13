package main

import (
	"database/sql"
	"encoding/csv"
	"errors"
	data "f1/initial_data"
	"f1/internal/models"
	"fmt"
	"io"
	"strconv"
	"strings"
	"flag"
	
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
	
	drop := flag.Bool("d", false, "drop tables")
	create := flag.Bool("c", false, "create tables")
	seedData := flag.Bool("s", false, "seed data")
	check := flag.Bool("check", false, "check data")
	
	flag.Parse()
	
	if *drop {
		seed.dropTables()
	}
	
	if *create {
		seed.createTables()
	}
	
	if *seedData {
		seed.seedData()
	}
	
	if *check {
		seed.checkData()
	}
	
}

func (s *Seed) seedData() {
	if err := s.seedEngines(s.parseEngineData()); err != nil {
		panic(err)
	}
	if err := s.seedTeams(s.parseBaseData()); err != nil {
		panic(err)
	}
	if err := s.seedTracks(s.parseTrackData()); err != nil {
		panic(err)
	}
	if err := s.seedPrincipals(s.parsePrincipalData()); err != nil {
		panic(err)
	}
	if err := s.seedPilots(s.parsePilotData()); err != nil {
		panic(err)
	}
	
	if err := s.seedPilotTracks(s.parsePilotTrackData()); err != nil {
		panic(err)
	}
}

func (s *Seed) checkData() {
	tables := []string{
		"engine",
		"base_team",
		"teams",
		"tracks",
		"teams_principals",
		"pilots_initial",
		"pilots",
		"pilots_track_initial",
		"pilots_track",
		"players",
	}
	
	for _, table := range tables {
		fmt.Printf("\n=== %s ===\n", table)
		
		rows, err := s.DB.Query("SELECT * FROM " + table)
		if err != nil {
			fmt.Println("error:", err)
			continue
		}
		
		cols, err := rows.Columns()
		if err != nil {
			rows.Close()
			fmt.Println("error:", err)
			continue
		}
		
		fmt.Println(strings.Join(cols, " | "))
		
		values := make([]interface{}, len(cols))
		ptrs := make([]interface{}, len(cols))
		for i := range values {
			ptrs[i] = &values[i]
		}
		
		for rows.Next() {
			if err := rows.Scan(ptrs...); err != nil {
				fmt.Println("error:", err)
				continue
			}
			
			parts := make([]string, len(cols))
			for i, v := range values {
				switch val := v.(type) {
				case []byte:
					parts[i] = string(val)
				case nil:
					parts[i] = "NULL"
				default:
					parts[i] = fmt.Sprintf("%v", val)
				}
			}
			fmt.Println(strings.Join(parts, " | "))
		}
		
		rows.Close()
	}
}

func (s *Seed) dropTables() {
	// Сначала удаляем таблицы со внешними ключами (дочерние)
	tables := []string{
		`DROP TABLE IF EXISTS players`,
		`DROP TABLE IF EXISTS pilots_track_initial`,
		`DROP TABLE IF EXISTS pilots_track`,
		`DROP TABLE IF EXISTS pilots_initial`,
		`DROP TABLE IF EXISTS pilots`,
		`DROP TABLE IF EXISTS tracks`,
		`DROP TABLE IF EXISTS teams_principals`,
		`DROP TABLE IF EXISTS engine`,
		`DROP TABLE IF EXISTS base_team`,
		`DROP TABLE IF EXISTS teams`,
	}
	
	for _, query := range tables {
		if _, err := s.DB.Exec(query); err != nil {
			panic(err)
		}
	}
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
	
	pilotTrackTable := `
	CREATE TABLE IF NOT EXISTS pilots_track_initial (
    	id INTEGER PRIMARY KEY AUTOINCREMENT,
    	pilot_id INTEGER,
    	track_id INTEGER,
    	level INTEGER,
    	FOREIGN KEY(pilot_id) REFERENCES pilots_initial(id),
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
	    level INTEGER,
	    FOREIGN KEY(pilot_id) REFERENCES pilots_initial(id),
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
	    is_manufacturer INTEGER
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
	    is_manufacturer INTEGER
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
-- 	    pilot1_id INTEGER,
-- 	    pilot2_id INTEGER,
	    principal_id INTEGER,
	    budget INTEGER,
	    FOREIGN KEY(team_id) REFERENCES teams(id),
	    FOREIGN KEY(principal_id) REFERENCES teams_principals(id)
	)
	`
	
	
	if _, err := s.DB.Exec(playerTable); err != nil {
		panic(err)
	}
	
	currentPilotTable := `
	CREATE TABLE IF NOT EXISTS pilots (
	    id INTEGER PRIMARY KEY AUTOINCREMENT,
	    name TEXT,
	    team_id INTEGER,
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
	    sponsors INTEGER,
	    FOREIGN KEY(team_id) REFERENCES players(id)
	)`
	
	if _, err := s.DB.Exec(currentPilotTable); err != nil {
		panic(err)
	}
	
	carTable := `
	CREATE TABLE IF NOT EXISTS car (
	    id INTEGER PRIMARY KEY AUTOINCREMENT,
	    team_id INTEGER,
	    aerodynamic INTEGER,
	    engine INTEGER,
	    chassis INTEGER,
	    floor INTEGER,
	    tyres INTEGER,
	    reliability INTEGER,
	    FOREIGN KEY(team_id) REFERENCES teams(id)
	)
	`
	if _, err := s.DB.Exec(carTable); err != nil {
		panic(err)
	}
}

func (s *Seed) seedEngines(engines []models.Engine) error {
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	
	stmt, err := tx.Prepare(`INSERT INTO engine (manufacturer, price, power) VALUES (?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	
	for _, e := range engines {
		if _, err := stmt.Exec(e.Engine, e.Price, e.BaseLevel); err != nil {
			return err
		}
	}
	
	return tx.Commit()
}

func (s *Seed) seedTeams(base []models.Team) error {
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	
	stmtBase, err := tx.Prepare(`
		INSERT INTO base_team (name, car_lvl, ice, base_lvl, engineer, tube, sim, update_rtg, is_manufacturer)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmtBase.Close()
	
	stmtTeam, err := tx.Prepare(`
		INSERT INTO teams (name, car_lvl, ice, base_lvl, engineer, tube, sim, update_rtg, is_manufacturer)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmtTeam.Close()
	
	for _, t := range base {
		if _, err := stmtBase.Exec(t.Name, t.CarLevel, int(t.ICE), t.BaseLevel, t.Engineer, t.TubeLevel, t.SimLevel, t.UpdateRating, int(t.IsManufacturer)); err != nil {
			return err
		}
		if _, err := stmtTeam.Exec(t.Name, t.CarLevel, int(t.ICE), t.BaseLevel, t.Engineer, t.TubeLevel, t.SimLevel, t.UpdateRating, int(t.IsManufacturer)); err != nil {
			return err
		}
	}
	
	return tx.Commit()
}

func (s *Seed) seedTracks(tracks []models.Track) error {
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	
	stmt, err := tx.Prepare(`
		INSERT INTO tracks (name, downforce, type, difficulity, quali_impact, rain, tyre)
		VALUES (?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	
	for _, tr := range tracks {
		if _, err := stmt.Exec(tr.Name, int(tr.DownForceLevel), int(tr.Type), tr.Difficulty, int(tr.QualifyingImpact), tr.RainPossibility, tr.Tyre); err != nil {
			return err
		}
	}
	
	return tx.Commit()
}

func (s *Seed) seedPrincipals(principals []models.TeamPrincipal) error {
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	
	stmt, err := tx.Prepare(`INSERT INTO teams_principals (name, price, level) VALUES (?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	
	for _, p := range principals {
		if _, err := stmt.Exec(p.Name, p.Price, p.Level); err != nil {
			return err
		}
	}
	
	return tx.Commit()
}

func (s *Seed) seedPilots(pilots []models.Pilot) error {
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	
	stmt, err := tx.Prepare(`
		INSERT INTO pilots_initial (name, rating, quali_rating, style, expirince, adaptiveness, emotions, stability, rain, settings_angle, starting, tyre_management, mistake_possibility, price, sponsors)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	
	for _, pl := range pilots {
		vals := []interface{}{
			pl.Name, pl.Rating, pl.QualifyingRating, int(pl.DrivingStyle), pl.Experience,
			pl.Adaptiveness, int(pl.Emotions), int(pl.Stability), int(pl.Rain), int(pl.SettingsAngle),
			pl.Starting, pl.TyreManagement, pl.MistakePossibility, pl.Price, pl.Sponsors,
		}
		if _, err := stmt.Exec(vals...); err != nil {
			return err
		}
	}
	
	return tx.Commit()
}

func (s *Seed) seedPilotTracks(pilotTrack []models.PilotTrack) error {
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	
	stmt, err := tx.Prepare(`INSERT INTO pilots_track_initial (pilot_id, track_id, level) VALUES (?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	
	for _, pt := range pilotTrack {
		if _, err := stmt.Exec(pt.PilotID, pt.TrackID, pt.Level); err != nil {
			return err
		}
	}
	
	return tx.Commit()
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
	
	var tracks []string
	
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
			for i := 0; i < len(row); i++ {
				tracks = append(tracks, row[i])
			}
			continue
		}
		
		query := `SELECT id FROM pilots_initial WHERE name = ?`
		pilot, err := s.DB.Query(query, row[0])
		if err != nil {
			panic(err)
		}
		defer pilot.Close()
		
		var pilotID int
		for pilot.Next() {
			err := pilot.Scan(&pilotID)
			if err != nil {
				panic(err)
			}
		}
		
		for i := 1; i < len(row); i++ {
			level, _ := strconv.Atoi(row[i])
			
			query := `SELECT id FROM tracks WHERE name = ?`
			track, err := s.DB.Query(query, tracks[i])
			if err != nil {
				panic(err)
			}
			defer track.Close()
			
			var trackID int
			for track.Next() {
				err := track.Scan(&trackID)
				if err != nil {
					panic(err)
				}
			}
			
			pilotTrackData = append(pilotTrackData, models.PilotTrack{
				PilotID: int64(pilotID),
				TrackID: int64(trackID),
				Level:   level,
				
			})
		}
		
		
		
	}
	
	return pilotTrackData
}
package storage

import (
	"database/sql"
	"f1/internal/models"
)

type Storage struct {
	DB *sql.DB
}

func NewStorage(db *sql.DB) *Storage {
	return &Storage{DB: db}
}

func (s *Storage) InitSchema() error {
	schema := `
	-- Базовые справочники (static)
	CREATE TABLE IF NOT EXISTS base_pilots (
		id INTEGER PRIMARY KEY, name TEXT, rating INTEGER, quali_rating INTEGER,
		style INTEGER, exp INTEGER, adaptive INTEGER, emotions INTEGER, stability INTEGER,
		rain INTEGER, angle INTEGER, starting INTEGER, tyre_mng INTEGER, mistake INTEGER, price INTEGER, sponsors INTEGER
	);
	CREATE TABLE IF NOT EXISTS base_teams (
		id INTEGER PRIMARY KEY, name TEXT, ice INTEGER, base_lvl INTEGER, eng INTEGER, sim INTEGER, tube INTEGER, update_rtg INTEGER, tokens INTEGER, budget INTEGER, angle INTEGER, is_manuf INTEGER
	);
	CREATE TABLE IF NOT EXISTS base_tracks (
		id INTEGER PRIMARY KEY, name TEXT, downforce INTEGER, type INTEGER, diff INTEGER, quali_impact INTEGER, rain INTEGER, tyre INTEGER
	);

	-- Сессионные таблицы текущего сохранения
	CREATE TABLE IF NOT EXISTS session_players (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT, pilot1 INTEGER, pilot2 INTEGER, principal INTEGER, team INTEGER);
	CREATE TABLE IF NOT EXISTS session_teams (id INTEGER PRIMARY KEY, name TEXT, ice INTEGER, car_lvl INTEGER, base_lvl INTEGER, eng INTEGER, sim INTEGER, tube INTEGER, update_rtg INTEGER, tokens INTEGER, budget INTEGER, angle INTEGER, is_manuf INTEGER);
	CREATE TABLE IF NOT EXISTS session_pilots (id INTEGER PRIMARY KEY, name TEXT, team_name TEXT, rating INTEGER, quali_rating INTEGER, style INTEGER, exp INTEGER, adaptive INTEGER, emotions INTEGER, stability INTEGER, rain INTEGER, angle INTEGER, starting INTEGER, tyre_mng INTEGER, mistake INTEGER, price INTEGER, sponsors INTEGER);
	CREATE TABLE IF NOT EXISTS session_cars (team_id INTEGER PRIMARY KEY, aero INTEGER, engine INTEGER, chassis INTEGER, floor INTEGER, tyres INTEGER, reliability INTEGER);
	CREATE TABLE IF NOT EXISTS session_principals (id INTEGER PRIMARY KEY, name TEXT, price INTEGER, team_id INTEGER, level INTEGER);
	CREATE TABLE IF NOT EXISTS session_pilot_tracks (pilot_id INTEGER, track_id INTEGER, level INTEGER, PRIMARY KEY(pilot_id, track_id));
	`
	_, err := s.DB.Exec(schema)
	return err
}

func (s *Storage) SeedData() error {
	// Проверяем, есть ли уже данные в base_pilots, чтобы не дублировать
	var count int
	s.DB.QueryRow("SELECT COUNT(*) FROM base_pilots").Scan(&count)
	if count > 0 {
		return nil
	}

	// Сид Пилотов
	_, err := s.DB.Exec(`
		INSERT INTO base_pilots VALUES 
		(1, 'Max Verstappen', 96, 98, 0, 85, 90, 2, 0, 0, 1, 95, 90, 3, 55, 40),
		(2, 'Lewis Hamilton', 93, 92, 2, 98, 95, 2, 0, 0, 0, 92, 94, 2, 45, 50),
		(3, 'Charles Leclerc', 91, 97, 0, 75, 85, 0, 2, 1, 1, 88, 80, 7, 35, 25),
		(4, 'Lando Norris', 89, 91, 1, 70, 80, 1, 1, 1, 0, 85, 85, 5, 25, 15);
	`)
	if err != nil { return err }

	// Сид Команд
	_, err = s.DB.Exec(`
		INSERT INTO base_teams VALUES 
		(1, 'Red Bull Racing', 2, 90, 92, 95, 93, 8, 100, 150, 1, 1),
		(2, 'Mercedes F1', 1, 88, 90, 92, 91, 7, 100, 150, 0, 1),
		(3, 'Ferrari', 0, 89, 87, 90, 89, 7, 100, 150, 1, 1),
		(4, 'McLaren', 1, 85, 88, 87, 86, 9, 100, 150, 0, 0);
	`)
	if err != nil { return err }

	// Сид Трасс
	_, err = s.DB.Exec(`
		INSERT INTO base_tracks VALUES 
		(1, 'Monaco GP', 0, 1, 85, 0, 20, 80),
		(2, 'Monza GP', 2, 0, 60, 2, 10, 40),
		(3, 'Spa-Francorchamps', 1, 0, 75, 1, 40, 65);
	`)
	return err
}

func (s *Storage) ResetSession() error {
	s.DB.Exec("DELETE FROM session_players")
	s.DB.Exec("DELETE FROM session_teams")
	s.DB.Exec("DELETE FROM session_pilots")
	s.DB.Exec("DELETE FROM session_cars")
	s.DB.Exec("DELETE FROM session_principals")
	s.DB.Exec("DELETE FROM session_pilot_tracks")

	// Копируем базовые данные в сессию
	_, err := s.DB.Exec(`
		INSERT INTO session_teams SELECT id, name, ice, base_lvl, base_lvl, eng, sim, tube, update_rtg, tokens, budget, angle, is_manuf FROM base_teams;
		INSERT INTO session_pilots SELECT * FROM base_pilots;
	`)
	if err != nil { return err }

	// Генерируем дефолтные машины для всех команд
	_, err = s.DB.Exec(`
		INSERT INTO session_cars SELECT id, 50, 50, 50, 50, 50, 40 FROM base_teams;
	`)
	return err
}

func (s *Storage) GetTeams() ([]models.Team, error) {
	rows, err := s.DB.Query("SELECT id, name, ice, car_lvl, base_lvl, eng, sim, tube, update_rtg, tokens, budget, angle, is_manuf FROM session_teams")
	if err != nil { return nil, err }
	defer rows.Close()

	var teams []models.Team
	for rows.Next() {
		var t models.Team
		var isManuf int
		err := rows.Scan(&t.ID, &t.Name, &t.ICE, &t.CarLevel, &t.BaseLevel, &t.Engineer, &t.SimLevel, &t.TubeLevel, &t.UpdateRating, &t.Tokens, &t.Budget, &t.SettingsAngle, &isManuf)
		if err != nil { return nil, err }
		t.IsManufacturer = isManuf == 1
		teams = append(teams, t)
	}
	return teams, nil
}

func (s *Storage) GetPilots() ([]models.Pilot, error) {
	rows, err := s.DB.Query("SELECT id, name, team_name, rating, quali_rating, style, exp, adaptive, emotions, stability, rain, angle, starting, tyre_mng, mistake, price, sponsors FROM session_pilots")
	if err != nil { return nil, err }
	defer rows.Close()

	var pilots []models.Pilot
	for rows.Next() {
		var p models.Pilot
		err := rows.Scan(&p.ID, &p.Name, &p.Team, &p.Rating, &p.QualifyingRating, &p.DrivingStyle, &p.Experience, &p.Adaptiveness, &p.Emotions, &p.Stability, &p.Rain, &p.SettingsAngle, &p.Starting, &p.TyreManagement, &p.MistakePossibility, &p.Price, &p.Sponsors)
		if err != nil { return nil, err }
		pilots = append(pilots, p)
	}
	return pilots, nil
}

func (s *Storage) GetTracks() ([]models.Track, error) {
	rows, err := s.DB.Query("SELECT id, name, downforce, type, diff, quali_impact, rain, tyre FROM base_tracks")
	if err != nil { return nil, err }
	defer rows.Close()

	var tracks []models.Track
	for rows.Next() {
		var t models.Track
		err := rows.Scan(&t.ID, &t.Name, &t.DownForceLevel, &t.Type, &t.Difficulty, &t.QualifyingImpact, &t.RainPossibility, &t.Tyre)
		if err != nil { return nil, err }
		tracks = append(tracks, t)
	}
	return tracks, nil
}

func (s *Storage) SavePlayer(p models.Player) error {
	_, err := s.DB.Exec("INSERT INTO session_players (name, pilot1, pilot2, principal, team) VALUES (?, ?, ?, ?, ?)", p.Name, p.Pilot1, p.Pilot2, p.TeamPrincipal, p.Team)
	return err
}

func (s *Storage) UpdateTeamTokensAndBudget(teamID int64, tokens, budget int) error {
	_, err := s.DB.Exec("UPDATE session_teams SET tokens = ?, budget = ? WHERE id = ?", tokens, budget, teamID)
	return err
}

func (s *Storage) UpdateCar(car models.Car) error {
	_, err := s.DB.Exec("UPDATE session_cars SET aero = ?, engine = ?, chassis = ?, floor = ?, tyres = ?, reliability = ? WHERE team_id = ?",
		car.AeroDynamic, car.Engine, car.Chassis, car.Floor, car.Tyres, car.Reliability, car.TeamID)
	return err
}

func (s *Storage) ExecuteTransfer(pilotID int64, fromTeamID, toTeamID int64, cost int) error {
	var toTeamName string
	_ = s.DB.QueryRow("SELECT name FROM session_teams WHERE id = ?", toTeamID).Scan(&toTeamName)

	tx, err := s.DB.Begin()
	if err != nil { return err }

	// Снятие/начисление бюджета
	_, _ = tx.Exec("UPDATE session_teams SET budget = budget - ? WHERE id = ?", cost, toTeamID)
	if fromTeamID > 0 {
		_, _ = tx.Exec("UPDATE session_teams SET budget = budget + ? WHERE id = ?", cost, fromTeamID)
	}
	// Смена команды пилота
	_, _ = tx.Exec("UPDATE session_pilots SET team_name = ? WHERE id = ?", toTeamName, pilotID)

	return tx.Commit()
}
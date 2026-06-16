package storage

import (
	"context"
	"database/sql"
	"errors"
	"f1/internal/models"
	"fmt"
)

type SqliteF1Repo struct {
	db DBTX
	tx Tx
}

func NewSqliteF1Repo(db *sql.DB) *SqliteF1Repo {
	return &SqliteF1Repo{db: db}
}

func (s *SqliteF1Repo) Begin(ctx context.Context) (Tx, error) {
	base, ok := s.db.(*sql.DB)
	if !ok {
		return nil, fmt.Errorf("begin: underlying db is not *sql.DB")
	}
	
	tx, err := base.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	
	return tx, nil
}

func (s *SqliteF1Repo) WithTx(tx Tx) F1Repo {
	return &SqliteF1Repo{
		db: s.db,
		tx: tx,
	}
}

func (s *SqliteF1Repo) GetPlayers(ctx context.Context) ([]models.PlayerProfile, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, team_id, budget, tokens, principal_id FROM players`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var players []models.PlayerProfile
	for rows.Next() {
		var p models.PlayerProfile
		var pId sql.NullInt64
		
		if err := rows.Scan(&p.ID, &p.Name, &p.Team, &p.Budget, &p.Tokens, &pId); err != nil {
			return nil, err
		}
		
		if pId.Valid {
			id := pId.Int64
			p.TeamPrincipal = &id
		} else {
			p.TeamPrincipal = nil
		}
		
		players = append(players, p)
	}
	return players, nil
}

func (s *SqliteF1Repo) GetPlayer(ctx context.Context, id int64) (models.PlayerProfile, error) {
	var p models.PlayerProfile
	row := s.db.QueryRowContext(ctx, `SELECT id, name, team_id, budget, tokens, principal_id FROM players WHERE id = ?`, id)
	if err := row.Scan(&p.ID, &p.Name, &p.Team, &p.Budget, &p.Tokens, &p.TeamPrincipal); err != nil {
		return models.PlayerProfile{}, fmt.Errorf("ошибка при получении игрока: %w", err)
	}
	
	rows, err := s.db.QueryContext(ctx, `SELECT name FROM pilots WHERE team_id = ?`, id)
	if err != nil {
		return models.PlayerProfile{}, fmt.Errorf("ошибка при получении пилотов: %w", err)
	}
	defer rows.Close()
	
	pilots := make([]string, 2)
	
	for rows.Next() {
		var pilotName string
		if err := rows.Scan(&pilotName); err != nil {
			return models.PlayerProfile{}, fmt.Errorf("ошибка при получении пилотовd: %w", err)
		}
		pilots = append(pilots, pilotName)
	}
	
	p.Pilot1 = pilots[0]
	p.Pilot2 = pilots[1]
	
	return p, nil
}

func (s *SqliteF1Repo) GetTeam(ctx context.Context, teamID int64) (models.Team, error) {
	var t models.Team
	row := s.db.QueryRowContext(ctx, `SELECT id, name, car_lvl, ice, base_lvl, engineer, tube, sim, update_rtg, is_manufacturer, budget FROM teams WHERE id = ?`, teamID)
	if err := row.Scan(&t.ID, &t.Name, &t.CarLevel, &t.ICE, &t.BaseLevel, &t.Engineer, &t.TubeLevel, &t.SimLevel, &t.UpdateRating, &t.IsManufacturer, &t.Budget); err != nil {
		return models.Team{}, err
	}
	return t, nil
}

func (s *SqliteF1Repo) ResetSession(ctx context.Context) error {
	s.db.ExecContext(ctx, `DELETE FROM pilots`)
	s.db.ExecContext(ctx, `DELETE FROM pilots_track`)
	s.db.ExecContext(ctx, `DELETE FROM teams`)
	s.db.ExecContext(ctx, `DELETE FROM players`)
	return nil
}

func (s *SqliteF1Repo) GetBudget(ctx context.Context, teamID int64) (int, error) {
	var budget int
	row := s.db.QueryRowContext(ctx, `SELECT budget FROM players WHERE id = ?`, teamID)
	if err := row.Scan(&budget); err != nil {
		return 0, err
	}
	return budget, nil
}

func (s *SqliteF1Repo) GetTeams(ctx context.Context) ([]models.Team, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, car_lvl, ice, base_lvl, engineer, tube, sim, update_rtg, is_manufacturer, budget
		FROM teams`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var teams []models.Team
	for rows.Next() {
		var t models.Team
		var ice, isManufacturer int
		
		if err := rows.Scan(&t.ID, &t.Name, &t.CarLevel, &ice, &t.BaseLevel, &t.Engineer, &t.TubeLevel, &t.SimLevel, &t.UpdateRating, &isManufacturer, &t.Budget); err != nil {
			return nil, err
		}
		
		t.ICE = models.ICEName(ice)
		t.IsManufacturer = models.IsManufacturer(isManufacturer)
		
		teams = append(teams, t)
	}
	
	if err := rows.Err(); err != nil {
		return nil, err
	}
	
	return teams, nil
}

func (s *SqliteF1Repo) GetPilots(ctx context.Context) ([]models.Pilot, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, rating, quali_rating, style, expirince, adaptiveness, emotions, stability, rain, settings_angle, starting, tyre_management, mistake_possibility, price, sponsors, team_id, garage_id
		FROM pilots`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var pilots []models.Pilot
	for rows.Next() {
		var p models.Pilot
		var style, emotions, stability, rain, angle int
		
		if err := rows.Scan(&p.ID, &p.Name, &p.Rating, &p.QualifyingRating, &style, &p.Experience, &p.Adaptiveness, &emotions, &stability, &rain, &angle, &p.Starting, &p.TyreManagement, &p.MistakePossibility, &p.Price, &p.Sponsors, &p.Team, &p.Garage); err != nil {
			return nil, err
		}
		
		p.DrivingStyle = models.DrivingStyle(style)
		p.Emotions = models.DriverEmotion(emotions)
		p.Stability = models.DriverStability(stability)
		p.Rain = models.RainDriving(rain)
		p.SettingsAngle = models.SettingsAngle(angle)
		
		pilots = append(pilots, p)
	}
	
	if err := rows.Err(); err != nil {
		return nil, err
	}
	
	return pilots, nil
}

func (s *SqliteF1Repo) GetTracks(ctx context.Context) ([]models.Track, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, downforce, type, difficulity, quali_impact, rain, tyre
		FROM tracks`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var tracks []models.Track
	for rows.Next() {
		var t models.Track
		var downforce, trackType, qualiImpact int
		
		if err := rows.Scan(&t.ID, &t.Name, &downforce, &trackType, &t.Difficulty, &qualiImpact, &t.RainPossibility, &t.Tyre); err != nil {
			return nil, err
		}
		
		t.DownForceLevel = models.DownForce(downforce)
		t.Type = models.TrackType(trackType)
		t.QualifyingImpact = models.QualifyingImpact(qualiImpact)
		
		tracks = append(tracks, t)
	}
	
	if err := rows.Err(); err != nil {
		return nil, err
	}
	
	return tracks, nil
}

func (s *SqliteF1Repo) GetPilot(ctx context.Context, id int64) (models.Pilot, error) {
	var p models.Pilot
	var style, emotions, stability, rain, angle int
	
	row := s.db.QueryRowContext(ctx, `
		SELECT id, name, rating, quali_rating, style, expirince, adaptiveness, emotions, stability, rain, settings_angle, starting, tyre_management, mistake_possibility, price, sponsors, team_id, garage_id
		FROM pilots WHERE id = ?`, id)
	
	if err := row.Scan(&p.ID, &p.Name, &p.Rating, &p.QualifyingRating, &style, &p.Experience, &p.Adaptiveness, &emotions, &stability, &rain, &angle, &p.Starting, &p.TyreManagement, &p.MistakePossibility, &p.Price, &p.Sponsors, &p.Team, &p.Garage); err != nil {
		return models.Pilot{}, err
	}
	
	p.DrivingStyle = models.DrivingStyle(style)
	p.Emotions = models.DriverEmotion(emotions)
	p.Stability = models.DriverStability(stability)
	p.Rain = models.RainDriving(rain)
	p.SettingsAngle = models.SettingsAngle(angle)
	
	return p, nil
}

func (s *SqliteF1Repo) GetPilotTrack(ctx context.Context, pilotID, trackID int64) (models.PilotTrack, error) {
	var pt models.PilotTrack
	
	row := s.db.QueryRowContext(ctx, `
		SELECT id, pilot_id, track_id, level
		FROM pilots_track WHERE pilot_id = ? AND track_id = ?`, pilotID, trackID)
	
	if err := row.Scan(&pt.ID, &pt.PilotID, &pt.TrackID, &pt.Level); err != nil {
		return models.PilotTrack{}, err
	}
	
	return pt, nil
}

func (s *SqliteF1Repo) SavePlayer(ctx context.Context, player models.Player) (int64, error) {
	pl, err := s.db.ExecContext(ctx, `INSERT INTO players (name, team_id, budget, tokens) VALUES (?, ?, ?, ?)`, player.Name, player.Team, player.Budget, 120)
	if err != nil {
		return 0, err
	}
	
	id, err := pl.LastInsertId()
	if err != nil {
		return 0, err
	}
	
	return id, nil
}

func (s *SqliteF1Repo) UpdateTeamTokensAndBudget(ctx context.Context, teamID int64, tokens, budget int) error {
	if _, err := s.db.ExecContext(ctx, `UPDATE players SET tokens = ?, budget = ? WHERE id = ?`, tokens, budget, teamID); err != nil {
		return err
	}
	return nil
}

func (s *SqliteF1Repo) UpdatePilot(ctx context.Context, pilot models.Pilot) error {
	if _, err := s.db.ExecContext(ctx, `UPDATE pilots SET rating = ?, quali_rating = ?, style = ?, expirince = ?, adaptiveness = ?, emotions = ?, stability = ?, rain = ?, settings_angle = ?, starting = ?, tyre_management = ?, mistake_possibility = ?, price = ?, sponsors = ? WHERE id = ?`, pilot.Rating, pilot.QualifyingRating, pilot.DrivingStyle, pilot.Experience, pilot.Adaptiveness, pilot.Emotions, pilot.Stability, pilot.Rain, pilot.SettingsAngle, pilot.Starting, pilot.TyreManagement, pilot.MistakePossibility, pilot.Price, pilot.Sponsors, pilot.ID); err != nil {
		return err
	}
	return nil
}

func (s *SqliteF1Repo) UpdatePilotTrack(ctx context.Context, pt models.PilotTrack) error {
	if _, err := s.db.ExecContext(ctx, `UPDATE pilots_track SET level = ? WHERE id = ?`, pt.Level, pt.ID); err != nil {
		return err
	}
	return nil
}

func (s *SqliteF1Repo) CreateTeams(ctx context.Context) ([]models.Team, error) {
	query := `INSERT INTO teams SELECT * FROM base_team`
	if _, err := s.db.ExecContext(ctx, query); err != nil {
		return nil, err
	}
	teams, err := s.GetTeams(ctx)
	if err != nil {
		return nil, err
	}
	return teams, nil
}

func (s *SqliteF1Repo) CreatePilots(ctx context.Context) error {
	// Явно перечисляем колонки для пилотов
	queryPilots := `
		INSERT INTO pilots (
			id, name, garage_id, rating, quali_rating, style, expirince, 
			adaptiveness, emotions, stability, rain, settings_angle, 
			starting, tyre_management, mistake_possibility, price, sponsors
		) 
		SELECT 
			id, name, garage_id, rating, quali_rating, style, expirince, 
			adaptiveness, emotions, stability, rain, settings_angle, 
			starting, tyre_management, mistake_possibility, price, sponsors 
		FROM pilots_initial`
	
	if _, err := s.db.ExecContext(ctx, queryPilots); err != nil {
		return fmt.Errorf("ошибка импорта пилотов: %w", err)
	}
	
	// Явно перечисляем колонки для треков пилотов
	queryTracks := `
		INSERT INTO pilots_track (pilot_id, track_id, level) 
		SELECT pilot_id, track_id, level 
		FROM pilots_track_initial`
	
	if _, err := s.db.ExecContext(ctx, queryTracks); err != nil {
		return fmt.Errorf("ошибка импорта треков пилотов: %w", err)
	}
	
	return nil
}

func (s *SqliteF1Repo) UpdateCar(ctx context.Context, car models.Car) error {
	if _, err := s.db.ExecContext(ctx, `UPDATE car SET aerodynamic = ?, engine = ?, chassis = ?, floor = ?, tyres = ?, reliability = ?, settings_angle = ? WHERE team_id = ?`, car.AeroDynamic, car.Engine, car.Chassis, car.Floor, car.Tyres, car.Reliability, int(car.SettingsAngle), car.TeamID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			if _, err := s.db.ExecContext(ctx, `INSERT INTO car (team_id, aerodynamic, engine, chassis, floor, tyres, reliability, settings_angle) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, car.TeamID, car.AeroDynamic, car.Engine, car.Chassis, car.Floor, car.Tyres, car.Reliability, int(car.SettingsAngle)); err != nil {
				return err
			}
			return nil
		}
		return err
	}
	return nil
}

func (s *SqliteF1Repo) ExecuteTransfer(ctx context.Context, pilotID, fromTeamID, teamID int64, cost int) error {
	
	var playerTeamID int64
	if err := s.db.QueryRowContext(ctx, `SELECT team_id FROM players WHERE id = ?`, teamID).Scan(&playerTeamID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			if err := s.fillTeams(ctx, pilotID, teamID); err != nil {
				return err
			}
			
			return nil
		}
		return err
	}
	
	
	if err := s.checkBudget(ctx, teamID, cost); err != nil {
		return err
	}
	
	
	tx, err := s.Begin(ctx)
	if err != nil { return err }
	
	if _, err := tx.ExecContext(ctx, `UPDATE players SET budget = budget - ? WHERE id = ?`, cost, teamID); err != nil {
		return err
	}
	
	if fromTeamID > 0 {
		_, err := tx.ExecContext(ctx, `UPDATE players SET budget = budget + ? WHERE id = ?`, cost, fromTeamID)
		if err != nil {
			return err
		}
	}
	
	if _, err := tx.ExecContext(ctx, `UPDATE pilots SET team_id = ?, garage_id = ? WHERE id = ?`, teamID, playerTeamID, pilotID); err != nil {
		return err
	}
	
	if err := tx.Commit(); err != nil {
		return err
	}
	
	return nil
}

func (s *SqliteF1Repo) fillTeams(ctx context.Context, pilotID, teamID int64) error {
	if _, err := s.db.ExecContext(ctx, `UPDATE pilots SET garage_id = ? WHERE id = ?`, teamID, pilotID); err != nil {
		return err
	}
	return nil
}

func (s *SqliteF1Repo) TeamPrincipalTransfer(ctx context.Context, teamPrincipalID, fromTeamID, teamID int64, cost int) error {
	fmt.Println(cost, fromTeamID, teamID, teamPrincipalID)
	if err := s.checkBudget(ctx, teamID, cost); err != nil {
		return fmt.Errorf("not enough money to transfer: %w", err)
	}
	
	tx, err := s.Begin(ctx)
	if err != nil { return err }
	defer tx.Rollback()
	
	if _, err := tx.ExecContext(ctx, `UPDATE players SET budget = budget - ? WHERE id = ?`, cost, fromTeamID); err != nil {
		return fmt.Errorf("hhahahah %w", err)
	}
	if fromTeamID > 0 {
		_, err := tx.ExecContext(ctx, `UPDATE players SET budget = budget + ? WHERE id = ?`, cost, teamID)
		if err != nil {
			return fmt.Errorf("tttttt %w", err)
		}
	}
	
	if _, err := tx.ExecContext(ctx, `UPDATE players SET principal_id = ? WHERE id = ?`, teamPrincipalID, teamID); err != nil {
		return err
	}
	
	if err := tx.Commit(); err != nil {
		return err
	}
	
	return nil
}

func (s *SqliteF1Repo) GetTeamPrincipals(ctx context.Context) ([]models.TeamPrincipal, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, price, level
		FROM teams_principals`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var principals []models.TeamPrincipal
	for rows.Next() {
		var p models.TeamPrincipal
		if err := rows.Scan(&p.ID, &p.Name, &p.Price, &p.Level); err != nil {
			return nil, err
		}
		principals = append(principals, p)
	}
	return principals, nil
}

var ErrNotEnoughMoney = fmt.Errorf("you don't have enough money to transfer")

func (s *SqliteF1Repo) checkBudget(ctx context.Context, teamID int64, cost int) error {
	budget, err := s.GetBudget(ctx, teamID)
	if err != nil {
		return err
	}
	
	if budget < cost {
		return ErrNotEnoughMoney
	}
	return nil
	
}

func (s *SqliteF1Repo) UpdateBudget(ctx context.Context, playerID int64, cost int) error {
	if err := s.checkBudget(ctx, playerID, cost); err != nil {
		return fmt.Errorf("вот где собака зарыта: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE players SET budget = budget - ? WHERE id = ?`, cost, playerID); err != nil {
		return err
	}
	return nil
}

func (s *SqliteF1Repo) GetActivePilots(ctx context.Context) ([]models.Pilot, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, rating, quali_rating, style, expirince, adaptiveness, emotions, stability, rain, settings_angle, starting, tyre_management, mistake_possibility, price, sponsors, team_id, garage_id
		FROM pilots WHERE garage_id IS NOT NULL`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var pilots []models.Pilot
	for rows.Next() {
		var p models.Pilot
		var style, emotions, stability, rain, angle int
		
		if err := rows.Scan(&p.ID, &p.Name, &p.Rating, &p.QualifyingRating, &style, &p.Experience, &p.Adaptiveness, &emotions, &stability, &rain, &angle, &p.Starting, &p.TyreManagement, &p.MistakePossibility, &p.Price, &p.Sponsors, &p.Team, &p.Garage); err != nil {
			return nil, err 
		}
		
		p.DrivingStyle = models.DrivingStyle(style)
		p.Emotions = models.DriverEmotion(emotions)
		p.Stability = models.DriverStability(stability)
		p.Rain = models.RainDriving(rain)
		p.SettingsAngle = models.SettingsAngle(angle)
		pilots = append(pilots, p)
	}
	return pilots, nil
}

func (s *SqliteF1Repo) UpdateTeam(ctx context.Context, team models.Team) error {
	if _, err := s.db.ExecContext(ctx, `UPDATE teams SET ice = ? WHERE id = ?`, team.ICE, team.ID); err != nil {
		return err
	}
	return nil
}

func (s *SqliteF1Repo) GetEngines(ctx context.Context) ([]models.Engine, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, manufacturer, price, power
		FROM engine`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var engines []models.Engine
	for rows.Next() {
		var e models.Engine
		if err := rows.Scan(&e.ID, &e.Engine, &e.Price, &e.BaseLevel); err != nil {
			return nil, err
		}
		engines = append(engines, e)
	}
	return engines, nil
}

func (s *SqliteF1Repo) GetTokens(ctx context.Context, playerID int64) (int, error) {
	var tokens int
	if err := s.db.QueryRowContext(ctx, `SELECT tokens FROM players WHERE id = ?`, playerID).Scan(&tokens); err != nil {
		return 0, err
	}
	return tokens, nil
}

func (s *SqliteF1Repo) UpdateTokens(ctx context.Context, playerID int64, tokens int) error {
	if _, err := s.db.ExecContext(ctx, `UPDATE players SET tokens = tokens + ? WHERE id = ?`, tokens, playerID); err != nil {
		return err
	}
	return nil
}

func (s *SqliteF1Repo) Fire(ctx context.Context, userID, pilotID int64, who string) error {
	tx, err := s.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	
	if who == "pilot" {
		pilot, err := s.GetPilot(ctx, pilotID)
		if err != nil {
			return fmt.Errorf("failed to get pilot: %w", err)
		}
		
		
		if _, err := tx.ExecContext(ctx, `UPDATE pilots SET garage_id = NULL, team_id = NULL WHERE id = ?`, pilotID); err != nil {
			return fmt.Errorf("failed to update pilot: %w", err)
		}
		
		if _, err := tx.ExecContext(ctx, `UPDATE players SET budget = budget + ? - ? WHERE id = ?`, pilot.Price, pilot.Sponsors, userID); err != nil {
			return fmt.Errorf("failed to update player: %w", err)
		}
		
	} else if who == "principal" {
		var principalPrice int
		fmt.Println("------")
		fmt.Println(pilotID)
		fmt.Println("------")
		if err := s.db.QueryRowContext(ctx, `SELECT price FROM teams_principals WHERE id = ?`, pilotID).Scan(&principalPrice); err != nil {
			return fmt.Errorf("failed to get principal price: %w", err)
		}
		
		if _, err := tx.ExecContext(ctx, `UPDATE players SET budget = budget + ? WHERE id = ?`, principalPrice, userID); err != nil {
			return fmt.Errorf("failed to update player: %w", err)
		}
		
		if _, err := tx.ExecContext(ctx, `UPDATE players SET principal_id = NULL WHERE id = ?`, userID); err != nil {
			return fmt.Errorf("failed to update principal: %w", err)
		}
	} else {
		return fmt.Errorf("unknown who: %s", who)
	}
	
	
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	return nil
}
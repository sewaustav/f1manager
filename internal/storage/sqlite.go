package storage

import (
	"context"
	"database/sql"
	"f1/internal/models"
	"fmt"
)



type SqliteF1Repo struct {
	db *sql.DB
}

func NewSqliteF1Repo(db *sql.DB) *SqliteF1Repo {
	return &SqliteF1Repo{db: db}
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
		SELECT id, name, car_lvl, ice, base_lvl, engineer, tube, sim, update_rtg, is_manufacturer
		FROM teams`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var teams []models.Team
	for rows.Next() {
		var t models.Team
		var ice, isManufacturer int
		
		if err := rows.Scan(&t.ID, &t.Name, &t.CarLevel, &ice, &t.BaseLevel, &t.Engineer, &t.TubeLevel, &t.SimLevel, &t.UpdateRating, &isManufacturer); err != nil {
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
		SELECT id, name, rating, quali_rating, style, expirince, adaptiveness, emotions, stability, rain, settings_angle, starting, tyre_management, mistake_possibility, price, sponsors
		FROM pilots`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var pilots []models.Pilot
	for rows.Next() {
		var p models.Pilot
		var style, emotions, stability, rain, angle int
		
		if err := rows.Scan(&p.ID, &p.Name, &p.Rating, &p.QualifyingRating, &style, &p.Experience, &p.Adaptiveness, &emotions, &stability, &rain, &angle, &p.Starting, &p.TyreManagement, &p.MistakePossibility, &p.Price, &p.Sponsors); err != nil {
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
		SELECT id, name, rating, quali_rating, style, expirince, adaptiveness, emotions, stability, rain, settings_angle, starting, tyre_management, mistake_possibility, price, sponsors
		FROM pilots WHERE id = ?`, id)
	
	if err := row.Scan(&p.ID, &p.Name, &p.Rating, &p.QualifyingRating, &style, &p.Experience, &p.Adaptiveness, &emotions, &stability, &rain, &angle, &p.Starting, &p.TyreManagement, &p.MistakePossibility, &p.Price, &p.Sponsors); err != nil {
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

func (s *SqliteF1Repo) SavePlayer(ctx context.Context, player models.Player) error {
	if _, err := s.db.ExecContext(ctx,
		`INSERT INTO players (name, pilot1, pilot2, principal, team) VALUES (?, ?, ?, ?, ?)`, player.Name, player.Pilot1, player.Pilot2, player.TeamPrincipal, player.Team); err != nil {
		return err
	}
	return nil
}

func (s *SqliteF1Repo) UpdateTeamTokensAndBudget(ctx context.Context, teamID int64, tokens, budget int) error {
	if _, err := s.db.ExecContext(ctx, `UPDATE teams SET tokens = ?, budget = ? WHERE id = ?`, tokens, budget, teamID); err != nil {
		return err
	}
	return nil
}

func (s *SqliteF1Repo) UpdatePilot(ctx context.Context, pilot models.Pilot) error {
	if _, err := s.db.ExecContext(ctx, `UPDATE pilots SET rating = ?, quali_rating = ?, style = ?, exp = ?, adaptiveness = ?, emotions = ?, stability = ?, rain = ?, settings_angle = ?, starting = ?, tyre_management = ?, mistake_possibility = ?, price = ?, sponsors = ? WHERE id = ?`, pilot.Rating, pilot.QualifyingRating, pilot.DrivingStyle, pilot.Experience, pilot.Adaptiveness, pilot.Emotions, pilot.Stability, pilot.Rain, pilot.SettingsAngle, pilot.Starting, pilot.TyreManagement, pilot.MistakePossibility, pilot.Price, pilot.Sponsors, pilot.ID); err != nil {
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

func (s *SqliteF1Repo) CreatePilot(ctx context.Context, pilot models.Pilot, pilotTrack models.PilotTrack) error {
	if _, err := s.db.ExecContext(ctx, `INSERT INTO pilots (name, rating, quali_rating, style, exp, adaptiveness, emotions, stability, rain, settings_angle, starting, tyre_management, mistake_possibility, price, sponsors) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, pilot.Name, pilot.Rating, pilot.QualifyingRating, pilot.DrivingStyle, pilot.Experience, pilot.Adaptiveness, pilot.Emotions, pilot.Stability, pilot.Rain, pilot.SettingsAngle, pilot.Starting, pilot.TyreManagement, pilot.MistakePossibility, pilot.Price, pilot.Sponsors); err != nil {
		return err
	}
	
	if _, err := s.db.ExecContext(ctx, `INSERT INTO pilots_track (pilot_id, track_id, level) VALUES (?, ?, ?)`, pilotTrack.PilotID, pilotTrack.TrackID, pilotTrack.Level); err != nil {
		return err
	}
	return nil
}

func (s *SqliteF1Repo) UpdateCar(ctx context.Context, car models.Car) error {
	if _, err := s.db.ExecContext(ctx, `UPDATE car SET aerodynamic = ?, engine = ?, chassis = ?, floor = ?, tyres = ?, reliability = ? WHERE team_id = ?`, car.AeroDynamic, car.Engine, car.Chassis, car.Floor, car.Tyres, car.Reliability, car.TeamID); err != nil {
		return err
	}
	return nil
}

func (s *SqliteF1Repo) ExecuteTransfer(ctx context.Context, pilotID, fromTeamID, teamID int64, cost int) error {
	
	var teamName string
	if err := s.db.QueryRowContext(ctx, `SELECT name FROM teams WHERE id = ?`, teamID).Scan(&teamName); err != nil {
		return err
	}
	
	budget, err := s.GetBudget(ctx, fromTeamID)
	if err != nil {
		return err
	}
	
	if budget < cost {
		return fmt.Errorf("you don't have enough money to transfer")
	}
	
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil { return err }
	defer tx.Rollback()
	
	if _, err := tx.ExecContext(ctx, `UPDATE players SET budget = budget - ? WHERE id = ?`, cost, fromTeamID); err != nil {
		return err
	}
	
	if fromTeamID > 0 {
		_, err := tx.ExecContext(ctx, `UPDATE players SET budget = budget + ? WHERE id = ?`, cost, fromTeamID)
		if err != nil {
			return err
		}
	}
	
	if _, err := tx.ExecContext(ctx, `UPDATE pilots SET team_id = ? WHERE id = ?`, teamID, pilotID); err != nil {
		return err
	}
	
	return nil
}

	
package storage

import (
	"context"
	"database/sql"
	"f1/internal/models"
)

type F1Repo interface {
	GetTeams(ctx context.Context) ([]models.Team, error)
	GetPilots(ctx context.Context) ([]models.Pilot, error)
	GetTracks(ctx context.Context) ([]models.Track, error)
	GetPilot(ctx context.Context, id int64) (models.Pilot, error)
	GetPilotTrack(ctx context.Context, id int64) (models.PilotTrack, error)
}

type SqliteF1Repo struct {
	db *sql.DB
}

func NewSqliteF1Repo(db *sql.DB) *SqliteF1Repo {
	return &SqliteF1Repo{db: db}
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

func (s *SqliteF1Repo) GetPilotTrack(ctx context.Context, id int64) (models.PilotTrack, error) {
	var pt models.PilotTrack
	
	row := s.db.QueryRowContext(ctx, `
		SELECT id, pilot_id, track_id, level
		FROM pilots_track WHERE id = ?`, id)
	
	if err := row.Scan(&pt.ID, &pt.PilotID, &pt.TrackID, &pt.Level); err != nil {
		return models.PilotTrack{}, err
	}
	
	return pt, nil
}
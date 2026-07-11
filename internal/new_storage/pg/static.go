package pg

import (
	"context"
	"database/sql"
	"errors"

	"f1/internal/models"
	repo "f1/internal/new_storage"

	_ "github.com/lib/pq"
)

// Static реализует repo.StaticRepo поверх PostgreSQL. Хранит неизменяемые
// в ходе игры данные: шаблоны пилотов, трассы, боссов команд и двигатели.
type Static struct {
	db *sql.DB
}

// NewStatic создаёт репозиторий статических данных.
func NewStatic(db *sql.DB) *Static {
	return &Static{db: db}
}

var _ repo.StaticRepo = (*Static)(nil)

// ErrNotFound возвращается одиночными геттерами, когда строка не найдена.
var ErrNotFound = errors.New("not found")

// scanPilot читает пилота из pilots_initial. Enum-поля скользят через
// временные int-переменные, garage_id — через sql.NullInt64.
func scanPilot(scan func(dest ...any) error) (models.Pilot, error) {
	var p models.Pilot
	var style, emotions, stability, rain, angle int
	var garage sql.NullInt64

	if err := scan(
		&p.ID,
		&p.Name,
		&p.Rating,
		&p.QualifyingRating,
		&style,
		&p.Experience,
		&p.Adaptiveness,
		&emotions,
		&stability,
		&rain,
		&angle,
		&p.Starting,
		&p.TyreManagement,
		&p.MistakePossibility,
		&p.Price,
		&p.Sponsors,
		&p.CarFit,
		&garage,
	); err != nil {
		return models.Pilot{}, err
	}

	p.DrivingStyle = models.DrivingStyle(style)
	p.Emotions = models.DriverEmotion(emotions)
	p.Stability = models.DriverStability(stability)
	p.Rain = models.RainDriving(rain)
	p.SettingsAngle = models.SettingsAngle(angle)

	if garage.Valid {
		g := garage.Int64
		p.Garage = &g
	}
	// pilots_initial — это шаблоны: команда пилоту ещё не назначена.
	p.Team = nil

	return p, nil
}

const pilotColumns = `id, name, rating, quali_rating, style, expirince, adaptiveness, emotions, stability, rain, settings_angle, starting, tyre_management, mistake_possibility, price, sponsors, car_fit, garage_id`

func (s *Static) GetPilot(ctx context.Context, pilotID int64) (models.Pilot, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT `+pilotColumns+` FROM pilots_initial WHERE id = $1`, pilotID)

	p, err := scanPilot(row.Scan)
	if errors.Is(err, sql.ErrNoRows) {
		return models.Pilot{}, ErrNotFound
	}
	if err != nil {
		return models.Pilot{}, err
	}
	return p, nil
}

func (s *Static) GetPilots(ctx context.Context) ([]models.Pilot, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+pilotColumns+` FROM pilots_initial`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var pilots []models.Pilot
	for rows.Next() {
		p, err := scanPilot(rows.Scan)
		if err != nil {
			return nil, err
		}
		pilots = append(pilots, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return pilots, nil
}

func (s *Static) GetPilotTrack(ctx context.Context, pilotID, trackID int64) (models.PilotTrack, error) {
	var pt models.PilotTrack
	err := s.db.QueryRowContext(ctx,
		`SELECT id, pilot_id, track_id, level
		 FROM pilots_track_initial WHERE pilot_id = $1 AND track_id = $2`,
		pilotID, trackID,
	).Scan(&pt.ID, &pt.PilotID, &pt.TrackID, &pt.Level)

	if errors.Is(err, sql.ErrNoRows) {
		return models.PilotTrack{}, ErrNotFound
	}
	if err != nil {
		return models.PilotTrack{}, err
	}
	return pt, nil
}

// scanTrack читает трассу из tracks с кастом enum-полей.
func scanTrack(scan func(dest ...any) error) (models.Track, error) {
	var t models.Track
	var downforce, trackType, qualiImpact int

	if err := scan(
		&t.ID,
		&t.Name,
		&downforce,
		&trackType,
		&t.Difficulty,
		&qualiImpact,
		&t.RainPossibility,
		&t.Tyre,
	); err != nil {
		return models.Track{}, err
	}

	t.DownForceLevel = models.DownForce(downforce)
	t.Type = models.TrackType(trackType)
	t.QualifyingImpact = models.QualifyingImpact(qualiImpact)

	return t, nil
}

const trackColumns = `id, name, downforce, type, difficulity, quali_impact, rain, tyre`

func (s *Static) GetTrack(ctx context.Context, trackID int64) (models.Track, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT `+trackColumns+` FROM tracks WHERE id = $1`, trackID)

	t, err := scanTrack(row.Scan)
	if errors.Is(err, sql.ErrNoRows) {
		return models.Track{}, ErrNotFound
	}
	if err != nil {
		return models.Track{}, err
	}
	return t, nil
}

func (s *Static) GetTracks(ctx context.Context) ([]models.Track, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+trackColumns+` FROM tracks`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tracks []models.Track
	for rows.Next() {
		t, err := scanTrack(rows.Scan)
		if err != nil {
			return nil, err
		}
		tracks = append(tracks, t)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return tracks, nil
}

func (s *Static) GetTeamPrincipal(ctx context.Context, principalID int64) (models.TeamPrincipal, error) {
	var tp models.TeamPrincipal
	err := s.db.QueryRowContext(ctx,
		`SELECT id, name, price, level FROM teams_principals WHERE id = $1`,
		principalID,
	).Scan(&tp.ID, &tp.Name, &tp.Price, &tp.Level)

	if errors.Is(err, sql.ErrNoRows) {
		return models.TeamPrincipal{}, ErrNotFound
	}
	if err != nil {
		return models.TeamPrincipal{}, err
	}
	return tp, nil
}

func (s *Static) GetTeamPrincipals(ctx context.Context) ([]models.TeamPrincipal, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, price, level FROM teams_principals`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var principals []models.TeamPrincipal
	for rows.Next() {
		var tp models.TeamPrincipal
		if err := rows.Scan(&tp.ID, &tp.Name, &tp.Price, &tp.Level); err != nil {
			return nil, err
		}
		principals = append(principals, tp)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return principals, nil
}

// GetEngine сохраняет семантику sqlite-версии: поиск по manufacturer, а не по id.
func (s *Static) GetEngine(ctx context.Context, id int64) (models.Engine, error) {
	var e models.Engine
	err := s.db.QueryRowContext(ctx,
		`SELECT id, manufacturer, price, power FROM engine WHERE manufacturer = $1`,
		id,
	).Scan(&e.ID, &e.Engine, &e.Price, &e.BaseLevel)

	if errors.Is(err, sql.ErrNoRows) {
		return models.Engine{}, ErrNotFound
	}
	if err != nil {
		return models.Engine{}, err
	}
	return e, nil
}

func (s *Static) GetEngines(ctx context.Context) ([]models.Engine, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, manufacturer, price, power FROM engine`)
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
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return engines, nil
}

package engine

import (
	"context"
	"database/sql"
	"f1/internal/models"
	"f1/internal/storage"
	"math/rand"
	"sort"
	"time"
)

type Engine struct {
	db *sql.DB
	r  *rand.Rand
	repo storage.F1Repo
}

func NewEngine(db *sql.DB) *Engine {
	return &Engine{
		db: db,
		r:  rand.New(rand.NewSource(time.Now().UnixNano())),
		repo: storage.NewSqliteF1Repo(db),
	}
}

// Вычисление базовых модификаторов
func (e *Engine) calcModifiers(pilot models.Pilot, team models.Team, car models.Car, track models.Track, principal models.TeamPrincipal, isRain bool) (float64, float64) {
	// 1. Штраф за углы настроек
	synergyPenalty := 0.0
	if pilot.SettingsAngle != car.SettingsAngle {
		synergyPenalty = 10.0 * (1.0 - float64(pilot.Adaptiveness)/100.0)
	}
	
	ctx := context.Background()
	pilotTrack, err := e.repo.GetPilotTrack(ctx, pilot.ID, track.ID)
	if err != nil {
		return 0, 0
	}
	
	pilotTrackLvl := pilotTrack.Level
	
	diffPenalty := 0.0
	if pilotTrackLvl < 15 {
		if pilot.Rating < track.Difficulty && pilot.Experience < track.Difficulty {
			diffPenalty = float64(track.Difficulty - pilot.Rating)
		}
	}
	
	// 3. Бонус шефа
	principalBonus := float64(principal.Level) / 5.0
	
	// 4. Соответствие машины типу трассы
	carFit := 0.0
	switch track.DownForceLevel {
	case models.HighDownforce:
		carFit = float64(car.AeroDynamic+car.Floor) / 2.0
	case models.HighDrag:
		carFit = float64(car.Engine)
	case models.MediumDownForce:
		carFit = float64(car.Chassis+car.Tyres) / 2.0
	}
	carFit *= 2.5
	
	// 5. Погода
	weatherMod := 0.0
	if isRain {
		switch pilot.Rain {
		case models.MasterOfRain:
			weatherMod = 10.0
		case models.Normal:
			weatherMod = 0.0
		case models.Slow:
			weatherMod = -10.0
		}
	}
	
	carBonus := float64(team.CarLevel)*1.5
	
	totalPaceBonus := carBonus + carFit + float64(pilotTrackLvl) + principalBonus + weatherMod - synergyPenalty - diffPenalty
	return totalPaceBonus, float64(pilotTrackLvl)
}

func (e *Engine) getVariance(p models.Pilot) float64 {
	switch p.Stability {
	case models.High:
		return float64(e.r.Intn(3) - 1) // -1..1
	case models.Medium:
		return float64(e.r.Intn(7) - 3) // -3..3
	default:
		return float64(e.r.Intn(11) - 6) // -6..4
	}
}

func (e *Engine) SimulateWeekend(ctx context.Context, track models.Track, pilots []models.Pilot, teams map[int64]models.Team, cars map[int64]models.Car, principals map[int64]models.TeamPrincipal) []models.RaceResult {
	isRain := e.r.Intn(100) < track.RainPossibility
	
	type tempResult struct {
		pilot      models.Pilot
		team       models.Team
		qualiScore float64
		raceScore  float64
		isDNF      bool
		dnfReason  string
		qualiPos   int
	}
	
	results := make([]*tempResult, len(pilots))
	for i, p := range pilots {
		var t models.Team
		if p.Garage != nil {
			t = teams[*p.Garage]
		}
		c := cars[t.ID]
		tp := principals[t.ID]
		
		bonus, trackLvl := e.calcModifiers(p, t, c, track, tp, isRain)
		
		// Квалификация
		qualiPace := float64(p.QualifyingRating) + bonus
		variance := e.getVariance(p)
		qualiScore := qualiPace + variance
		
		results[i] = &tempResult{
			pilot:      p,
			team:       t,
			qualiScore: qualiScore,
		}
		
		// Расчет DNF (Сходов)
		mechDNFChance := 0.0
		if c.Reliability < 35 {
			mechDNFChance = float64(35-c.Reliability) / 2.0
		}
		pilotErrorChance := float64(p.MistakePossibility)
		if track.Type == models.City {
			pilotErrorChance *= 1.5
		}
		if p.DrivingStyle == models.Smooth {
			pilotErrorChance /= 2.0
		}
		if isRain {
			pilotErrorChance *= 1.3
		}
		
		if float64(e.r.Intn(100)) < mechDNFChance {
			results[i].isDNF = true
			results[i].dnfReason = "Mechanical Failure"
		} else if float64(e.r.Intn(100)) < pilotErrorChance {
			results[i].isDNF = true
			results[i].dnfReason = "Crash / Driver Error"
		}
		
		// Гонка (базовый темп)
		racePace := float64(p.Rating) + bonus
		tyrePenalty := 0.0
		if track.Tyre > p.TyreManagement {
			tyrePenalty = float64(track.Tyre-p.TyreManagement) * 0.5
		}
		
		startBonus := float64(p.Starting) / 5.0
		styleBonus := 0.0
		penaltyLoss := 0.0
		
		switch p.DrivingStyle {
		case models.Aggressive:
			styleBonus = 5.0
			if e.r.Intn(100) < 25 {
				penaltyLoss = 8.0
			}
		case models.Balance:
			styleBonus = 2.0
			if e.r.Intn(100) < 10 {
				penaltyLoss = 4.0
			}
		}
		
		results[i].raceScore = racePace - tyrePenalty + startBonus + styleBonus - penaltyLoss + trackLvl + (variance * 1.2)
	}
	
	// Сортировка квалификации
	sort.Slice(results, func(i, j int) bool {
		return results[i].qualiScore > results[j].qualiScore
	})
	
	// Применяем влияние стартовой позиции на гоночный результат
	for pos, res := range results {
		res.qualiPos = pos + 1
		weight := 15.0
		switch track.QualifyingImpact {
		case models.HighImpact:
			weight = 25.0
		case models.LowImpact:
			weight = 5.0
		}
		if track.Type == models.City {
			weight += 5.0
		}
		
		startingAdvantage := float64(21-res.qualiPos) * weight * 0.1
		res.raceScore += startingAdvantage
		
		if res.isDNF {
			res.raceScore = -9999.0 // Скинули в конец
		}
	}
	
	// Сортировка гонки
	sort.Slice(results, func(i, j int) bool {
		return results[i].raceScore > results[j].raceScore
	})
	
	// Начисление очков
	pointsTable := []int{25, 18, 15, 12, 10, 8, 6, 4, 2, 1}
	finalResults := make([]models.RaceResult, len(results))
	
	for pos, res := range results {
		pts := 0
		if pos < len(pointsTable) && !res.isDNF {
			pts = pointsTable[pos]
		}
		
		var garageID int64
		if res.pilot.Garage != nil {
			garageID = *res.pilot.Garage
		}
		
		finalResults[pos] = models.RaceResult{
			PilotID:       res.pilot.ID,
			GarageID:      garageID,
			PilotName:     res.pilot.Name,
			TeamName:      res.team.Name,
			QualiPosition: res.qualiPos,
			RacePosition:  pos + 1,
			Points:        pts,
			IsDNF:         res.isDNF,
			DNFReason:     res.dnfReason,
		}
	}
	
	e.updateAfterRace(ctx, track, finalResults, pilots)
	
	return finalResults
}

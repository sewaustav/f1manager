package engine

import (
	"context"
	"database/sql"
	"f1/internal/models"
	"f1/internal/storage"
	"math"
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

func (e *Engine) calcSetupBonus(pilot models.Pilot, team models.Team, car models.Car) float64 {
	// Ширина рабочего окна: среднее шасси и CarLevel, оба с равным весом.
	// Чем шире окно — тем выше потолок возможного бонуса.
	// Диапазон chassis: 0-35 (токены), CarLevel: 0-100
	// Нормализуем chassis к той же шкале: chassis/35 * 100
	chassisNorm := float64(car.Chassis) / 35.0 * 100.0
	windowWidth := (chassisNorm + float64(team.CarLevel)) / 2.0
 
	// Максимально возможный бонус от ширины окна: 0..20
	maxBonus := windowWidth / 100.0 * 20.0
 
	// Шанс на крит: среднее SimLevel и Experience пилота, оба 0-100
	// При значении 100+100 шанс крита = 100%, при 0+0 = 0%
	critChance := (float64(team.SimLevel) + float64(pilot.Experience)) / 2.0
 
	// Бросок: если выпало меньше critChance — крит, иначе обычная настройка
	roll := float64(e.r.Intn(100))
 
	var setupQuality float64
	if roll < critChance {
		// Крит: качество настройки от 0.75 до 1.0
		setupQuality = 0.75 + e.r.Float64()*0.25
	} else {
		// Обычная настройка: качество от 0.2 до 0.75
		setupQuality = 0.2 + e.r.Float64()*0.55
	}
 
	return maxBonus * setupQuality
}

func calcCarScore(car models.Car, track models.Track) float64 {
	components := []float64{
		float64(car.AeroDynamic),
		float64(car.Engine),
		float64(car.Chassis),
		float64(car.Floor),
		float64(car.Tyres),
	}
	
	// Среднее всех компонентов
	sum := 0.0
	for _, v := range components {
		sum += v
	}
	avg := sum / float64(len(components))
	
	// Трек-специфичный компонент
	var trackSpecific float64
	switch track.DownForceLevel {
	case models.HighDownforce:
		trackSpecific = (float64(car.AeroDynamic) + float64(car.Floor)) / 2.0
	case models.HighDrag:
		trackSpecific = float64(car.Engine)
	case models.MediumDownForce:
		trackSpecific = (float64(car.Chassis) + float64(car.Tyres)) / 2.0
	}
	
	// Базовый счёт: 60% общий уровень + 40% специализация
	baseScore := avg*0.6 + trackSpecific*0.4
	
	// Штраф за разбаланс: дисперсия компонентов
	variance := 0.0
	for _, v := range components {
		diff := v - avg
		variance += diff * diff
	}
	variance /= float64(len(components))
	stdDev := math.Sqrt(variance)
	
	balancePenalty := stdDev * 0.2
	
	return baseScore - balancePenalty
}

// Вычисление базовых модификаторов
func (e *Engine) calcModifiers(pilot models.Pilot, team models.Team, car models.Car, track models.Track, principal models.TeamPrincipal, isRain bool) (float64, float64) {
	// 1. Штраф за углы настроек
	synergyPenalty := 0.0
	if pilot.SettingsAngle != car.SettingsAngle {
		synergyPenalty = 10.0 * (1.0 - float64(pilot.Adaptiveness)/100.0)
	}
	
	carFitPenalty := 0.0    
	delta := abs(team.CarSettings - pilot.CarFit)
	if delta > 3 {
		carFitPenalty = float64(delta - 3) * 0.3
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
	carScore := calcCarScore(car, track)
	carBonus := float64(team.CarLevel)*5.5 + carScore*2.4
	
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
	
	
	totalPaceBonus := carBonus + float64(pilotTrackLvl) + principalBonus + weatherMod - synergyPenalty - diffPenalty - carFitPenalty
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

func (e *Engine) SimulateWeekend(ctx context.Context, track models.Track, pilots []models.Pilot, teams map[int64]models.Team, cars map[int64]models.Car, principals map[int64]models.TeamPrincipal, pilotsStanding, teamStanding map[int64]int) []models.RaceResult {
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
		settings := e.calcSetupBonus(p, t, c)
		
		// Квалификация
		qualiPace := float64(p.QualifyingRating)*1.5 + bonus + settings
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
			if c.Reliability > 35 {
				c.Reliability = 35
			}
			mechDNFChance = float64(36-c.Reliability) / 2.0
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
		racePace := float64(p.Rating) + bonus + settings
		tyrePenalty := 0.0
		if track.Tyre > p.TyreManagement {
			tyrePenalty = float64(track.Tyre-p.TyreManagement) * 0.5
		}
		
		tyreBonus := 0.0
		if p.Garage != nil && cars[*p.Garage].Tyres > 15 {
			tyreBonus = float64(cars[*p.Garage].Tyres-15) * 0.4
		}
		
		tyrePenalty = max(0, tyrePenalty - tyreBonus)
		
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
		case models.DecentImpact:
			weight = 15.0
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

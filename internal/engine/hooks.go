package engine

import (
	"context"
	"f1/internal/models"
	"fmt"
	"sort"
)

// updateAfterRace обновляет рейтинги пилотов и их уровни на трассе после гонки.
// Вызывается после SimulateWeekend.
func (e *Engine) updateAfterRace(
	ctx context.Context,
	track models.Track,
	results []models.RaceResult,
	pilots []models.Pilot,
) {
	pilotByID := make(map[int64]models.Pilot, len(pilots))
	for _, p := range pilots {
		pilotByID[p.ID] = p
	}
	
	resultByPilotID := make(map[int64]models.RaceResult, len(results))
	for _, r := range results {
		resultByPilotID[r.PilotID] = r
	}
	
	// Группируем результаты по команде (garage_id) для сравнения напарников
	type teamPair struct {
		results []*models.RaceResult
	}
	teamResults := make(map[int64]*teamPair)
	for i := range results {
		r := &results[i]
		p, ok := pilotByID[r.PilotID]
		if !ok || p.Garage == nil {
			continue
		}
		gid := *p.Garage
		if teamResults[gid] == nil {
			teamResults[gid] = &teamPair{}
		}
		teamResults[gid].results = append(teamResults[gid].results, r)
	}
	
	for _, pilot := range pilots {
		res, ok := resultByPilotID[pilot.ID]
		if !ok {
			continue
		}
		
		ratingDelta := 0
		startingDelta := 0
		qualiRatingDelta := 0
		
		// ── Квалификация ────────────────────────────────────────────────────────
		qualiPos := res.QualiPosition
		qr := pilot.QualifyingRating
		
		switch {
		case qr >= 90:
			if qualiPos == 1 {
				qualiRatingDelta += 1
			} else if qualiPos > 5 {
				qualiRatingDelta -= 1
			}
		
		case qr >= 85:
			if qualiPos == 1 {
				qualiRatingDelta += 2
			} else if qualiPos <= 6 {
				qualiRatingDelta += 1
			} else if qualiPos > 10 {
				qualiRatingDelta -= 1
			}
		
		case qr >= 70:
			switch {
			case qualiPos == 1:
				qualiRatingDelta += 5
			case qualiPos <= 3:
				qualiRatingDelta += 4
			case qualiPos <= 6:
				qualiRatingDelta += 3
			case qualiPos <= 10:
				qualiRatingDelta += 1
			case qualiPos > 15:
				qualiRatingDelta -= 1
			}
		}
		
		// ── Гонка: победа / подиум ───────────────────────────────────────────
		if !res.IsDNF {
			switch res.RacePosition {
			case 1:
				ratingDelta += 3
			case 2, 3:
				ratingDelta += 2
			}
		}
		
		// ── Финиш вне очков для сильного пилота ─────────────────────────────
		if !res.IsDNF && res.RacePosition > 10 && pilot.Rating > 87 {
			ratingDelta -= 1
		}
		
		// ── Сход по вине пилота ──────────────────────────────────────────────
		if res.IsDNF && res.DNFReason == "Crash / Driver Error" {
			ratingDelta -= 1
		}
		
		// ── Сравнение с напарником ───────────────────────────────────────────
		if pilot.Garage != nil {
			pair := teamResults[*pilot.Garage]
			if pair != nil && len(pair.results) == 2 {
				var teammate *models.RaceResult
				for _, r := range pair.results {
					if r.PilotID != pilot.ID {
						teammate = r
						break
					}
				}
				if teammate != nil && !res.IsDNF && !teammate.IsDNF {
					// diff > 0 — мы впереди напарника
					diff := teammate.RacePosition - res.RacePosition
					if diff >= 2 {
						ratingDelta += 1
					} else if diff <= -3 {
						ratingDelta -= 1
					}
				}
			}
		}
		
		// ── Позиция в гонке vs квала ─────────────────────────────────────────
		posDiff := res.QualiPosition - res.RacePosition // > 0 — улучшили
		if res.QualiPosition == 1 && res.RacePosition == 1 {
			startingDelta += 2
		} else if posDiff > 2 {
			startingDelta += 2
		} else if posDiff < -2 && !res.IsDNF {
			ratingDelta -= 1
		}
		
		// ── Применяем дельты ─────────────────────────────────────────────────
		if ratingDelta == 0 && startingDelta == 0 && qualiRatingDelta == 0 {
			continue
		}
		
		updated := pilot
		updated.Rating = clamp(pilot.Rating+ratingDelta, 0, 100)
		updated.QualifyingRating = clamp(pilot.QualifyingRating+qualiRatingDelta, 0, 100)
		updated.Starting = clamp(pilot.Starting+startingDelta, 0, 100)
		
		_ = e.repo.UpdatePilot(ctx, updated)
		
		// ── pilots_track: победа +3, подиум +2 ───────────────────────────────
		if !res.IsDNF {
			trackDelta := 0
			switch res.RacePosition {
			case 1:
				trackDelta = 3
			case 2, 3:
				trackDelta = 2
			}
			if trackDelta > 0 {
				pt, err := e.repo.GetPilotTrack(ctx, pilot.ID, track.ID)
				if err == nil {
					pt.Level = clamp(pt.Level+trackDelta, 0, 20)
					_ = e.repo.UpdatePilotTrack(ctx, pt)
				}
			}
		}
	}
}

func clamp(v, min, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

// PilotSeasonSnapshot фиксирует рейтинг пилота на начало сезона.
// Создаётся до первой гонки и передаётся в UpdateAfterSeason.
type PilotSeasonSnapshot struct {
	PilotID         int64
	BaseRating      int // рейтинг на старте сезона
	BaseQualiRating int
}

// SeasonStandings итоговые данные сезона, которые cli собирает после всех гонок.
type SeasonStandings struct {
	// DriverPoints: pilotID -> суммарные очки за сезон
	DriverPoints map[int64]int
	// TeamPoints: teamID (garage_id) -> суммарные очки за сезон
	TeamPoints map[int64]int
	// Pilots — актуальный срез пилотов (с накопленными за сезон рейтингами)
	Pilots []models.Pilot
	// Snapshots — рейтинги пилотов ДО начала сезона
	Snapshots []PilotSeasonSnapshot
}

// WCCRank возвращает место команды в кубке конструкторов (1 = победитель).
// Принимает teamPoints и нужный teamID.
func WCCRank(teamPoints map[int64]int, teamID int64) int {
	type entry struct {
		id  int64
		pts int
	}
	list := make([]entry, 0, len(teamPoints))
	for id, pts := range teamPoints {
		list = append(list, entry{id, pts})
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].pts > list[j].pts
	})
	for rank, e := range list {
		if e.id == teamID {
			return rank + 1
		}
	}
	return len(list) + 1
}

// UpdateAfterSeason — публичный хук, вызывается из cli после подсчёта всех очков.
// standings.TeamPoints должен быть заполнен итоговыми очками КК.
func (e *Engine) UpdateAfterSeason(ctx context.Context, standings SeasonStandings) {
	if len(standings.DriverPoints) == 0 {
		return
	}
	
	fmt.Println("Updating after season")
	
	// Чемпион — пилот с максимальными очками
	championPoints := 0
	for _, pts := range standings.DriverPoints {
		if pts > championPoints {
			championPoints = pts
		}
	}
	
	// Снэпшоты для быстрого доступа
	snapByID := make(map[int64]PilotSeasonSnapshot, len(standings.Snapshots))
	for _, s := range standings.Snapshots {
		snapByID[s.PilotID] = s
	}
	
	// Актуальные пилоты
	pilotByID := make(map[int64]models.Pilot, len(standings.Pilots))
	for _, p := range standings.Pilots {
		pilotByID[p.ID] = p
	}
	
	// Группируем по команде для дуэлей
	type teamEntry struct {
		pilotID int64
		points  int
	}
	teamDuels := make(map[int64][]teamEntry) // garageID -> пара
	for _, p := range standings.Pilots {
		if p.Garage == nil {
			continue
		}
		pts := standings.DriverPoints[p.ID]
		teamDuels[*p.Garage] = append(teamDuels[*p.Garage], teamEntry{p.ID, pts})
	}
	
	// Место в КК для ожидаемой позиции в гонке:
	// топ-1 КК → ожидаем 1-2 место; топ-2 → 3-4; топ-3 → 5-6 и т.д.
	// Формула: expectedRacePos = (wccRank-1)*2 + 1  (середина пары)
	expectedRacePos := func(wccRank int) int {
		return (wccRank-1)*2 + 1
	}
	
	// Средняя гоночная позиция пилота за сезон — нужна для сравнения с КК.
	// Считаем как: (кол-во финишей всех - сумма позиций) / финишей нет смысла,
	// используем очки как прокси: больше очков = выше позиция условно.
	// Проще: ранг пилота в чемпионате vs ожидаемый ранг от КК.
	pilotChampRank := func(pilotID int64) int {
		type entry struct {
			id  int64
			pts int
		}
		list := make([]entry, 0, len(standings.DriverPoints))
		for id, pts := range standings.DriverPoints {
			list = append(list, entry{id, pts})
		}
		sort.Slice(list, func(i, j int) bool {
			return list[i].pts > list[j].pts
		})
		for rank, e := range list {
			if e.id == pilotID {
				return rank + 1
			}
		}
		return len(list) + 1
	}
	
	for _, pilot := range standings.Pilots {
		snap, hasSnap := snapByID[pilot.ID]
		if !hasSnap {
			continue
		}
		
		ratingDelta := 0
		pilotPts := standings.DriverPoints[pilot.ID]
		
		// ── Победа в чемпионате ──────────────────────────────────────────────
		if pilotPts == championPoints {
			baseR := snap.BaseRating
			switch {
			case baseR > 90:
				ratingDelta += 3
			case baseR > 85:
				ratingDelta += 5
			case baseR > 80:
				ratingDelta += 10
			}
		}
		
		// ── Финиш в пределах 50 очков от чемпиона ───────────────────────────
		if championPoints-pilotPts <= 50 {
			ratingDelta += 1
		}
		
		// ── Дуэль напарников ─────────────────────────────────────────────────
		if pilot.Garage != nil {
			duel := teamDuels[*pilot.Garage]
			if len(duel) == 2 {
				var me, opp teamEntry
				if duel[0].pilotID == pilot.ID {
					me, opp = duel[0], duel[1]
				} else {
					me, opp = duel[1], duel[0]
				}
				
				oppPilot := pilotByID[opp.pilotID]
				oppSnap, hasOppSnap := snapByID[opp.pilotID]
				
				ptsDiff := me.points - opp.points // > 0 — я впереди
				ratingDiff := snap.BaseRating - oppSnap.BaseRating // > 0 — я сильнее по базе
				
				switch {
				case abs(ratingDiff) <= 3:
					// Пилоты примерно равны по рейтингу
					if ptsDiff > 2 {
						ratingDelta += 1
					} else if ptsDiff < -2 {
						ratingDelta -= 1
					}
				
				default:
					// Разница рейтингов > 3
					if ptsDiff > 0 {
						// Я впереди
						if hasOppSnap && snap.BaseRating < oppSnap.BaseRating-3 {
							// Я слабее по рейтингу, но обошёл — неожиданная победа
							ratingDelta += 2
							// Напарник теряет — применим ниже через отдельный проход,
							// здесь ставим маркер через переменную oppLoss
							_ = oppPilot
							applyDelta(ctx, e, pilotByID[opp.pilotID], -2)
						} else {
							ratingDelta += 1
						}
					}
					// Если я проигрываю при разнице > 3 — без штрафа (по условию)
				}
			}
		}
		
		// ── Позиция пилота в чемпионате vs ожидаемая от КК команды ─────────
		if pilot.Garage != nil {
			wccRank := WCCRank(standings.TeamPoints, *pilot.Garage)
			expPos := expectedRacePos(wccRank)
			champRank := pilotChampRank(pilot.ID)
			
			if champRank < expPos {
				ratingDelta += 1 // финишировал выше ожидания
			} else if champRank > expPos+1 {
				ratingDelta -= 1 // финишировал ниже ожидания
			}
		}
		
		// ── Применяем ────────────────────────────────────────────────────────
		if ratingDelta != 0 {
			applyDelta(ctx, e, pilot, ratingDelta)
		}
	}
}

// applyDelta применяет дельту рейтинга к пилоту и сохраняет в БД.
func applyDelta(ctx context.Context, e *Engine, pilot models.Pilot, delta int) {
	if delta == 0 {
		return
	}
	pilot.Rating = clamp(pilot.Rating+delta, 0, 100)
	_ = e.repo.UpdatePilot(ctx, pilot)
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
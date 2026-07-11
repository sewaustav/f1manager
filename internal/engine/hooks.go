package engine

import (
	"context"
	"f1/internal/models"
	"fmt"
	"sort"
)

// updateAfterRace обновляет только уровень пилота на трассе.
func (e *Engine) updateAfterRace(
	ctx context.Context,
	groupID int64,
	track models.Track,
	results []models.RaceResult,
	pilots []models.Pilot,
) {
	pilotByID := make(map[int64]models.Pilot, len(pilots))
	for _, p := range pilots {
		pilotByID[p.ID] = p
	}
	
	for _, res := range results {
		pilot, ok := pilotByID[res.PilotID]
		if !ok {
			continue
		}
		
		trackDelta := 0
		if !res.IsDNF {
			switch res.RacePosition {
			case 1:
				trackDelta = 3
			case 2, 3:
				trackDelta = 2
			}
		} else if res.DNFReason == "Crash / Driver Error" {
			trackDelta = -1
		}
		
		if trackDelta == 0 {
			continue
		}
		
		pt, err := e.repo.GetPilotTrack(ctx, groupID, pilot.ID, track.ID)
		if err != nil {
			fmt.Println("Error getting pilot track", err)
			continue
		}
		pt.Level = clamp(pt.Level+trackDelta, 0, 20)
		if err := e.repo.UpdatePilotTrack(ctx, groupID, pt); err != nil {
			fmt.Println("Error updating pilot track", err)
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
type PilotSeasonSnapshot struct {
	PilotID         int64
	BaseRating      int
	BaseQualiRating int
}

// SeasonStandings итоговые данные сезона.
type SeasonStandings struct {
	DriverPoints map[int64]int
	TeamPoints   map[int64]int
	Pilots       []models.Pilot
	Snapshots    []PilotSeasonSnapshot
}

// WCCRank возвращает место команды в кубке конструкторов.
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

// ratingDeltaForElite возвращает дельту с жёсткими правилами для пилотов 95+.
// Они почти не растут, но активно падают за провалы.
func ratingDeltaForElite(
	pilotID int64,
	pilotPts, championPoints int,
	isChampion bool,
	teamDuel teamDuelResult,
	champRank, expPos int,
) int {
	delta := 0
	
	// Чемпионство — минимальный бонус
	if isChampion {
		delta += 1
	}
	
	// Финиш в пределах 50 очков от чемпиона — без бонуса для элиты
	// (убираем +1 который есть для остальных)
	
	// Дуэль напарников — жёсткие штрафы
	if teamDuel.valid {
		ptsDiff := teamDuel.myPoints - teamDuel.oppPoints
		ratingDiff := teamDuel.myBaseRating - teamDuel.oppBaseRating
		
		if ptsDiff < 0 {
			// Проиграл напарнику — всегда -2 для элиты
			delta -= 2
		} else if ptsDiff > 0 && abs(ratingDiff) <= 3 {
			// Обошёл примерно равного — минимальный бонус
			delta += 1
		}
		// Обошёл слабого напарника — без бонуса
	}
	
	// Позиция в чемпионате vs ожидания
	if champRank > expPos+1 {
		delta -= 2 // финишировал ниже ожидания — двойной штраф
	} else if champRank < expPos {
		// финишировал выше — без бонуса, это норма для элиты
	}
	
	// Вне топ-3 WDC — штраф
	if champRank > 3 {
		delta -= 1
	}
	
	return delta
}

type teamDuelResult struct {
	valid         bool
	myPoints      int
	oppPoints     int
	myBaseRating  int
	oppBaseRating int
}

// UpdateAfterSeason — хук после подсчёта всех очков.
func (e *Engine) UpdateAfterSeason(ctx context.Context, groupID int64, standings SeasonStandings) {
	if len(standings.DriverPoints) == 0 {
		return
	}
	
	fmt.Println("Updating after season")
	
	championPoints := 0
	for _, pts := range standings.DriverPoints {
		if pts > championPoints {
			championPoints = pts
		}
	}
	
	snapByID := make(map[int64]PilotSeasonSnapshot, len(standings.Snapshots))
	for _, s := range standings.Snapshots {
		snapByID[s.PilotID] = s
	}
	
	pilotByID := make(map[int64]models.Pilot, len(standings.Pilots))
	for _, p := range standings.Pilots {
		pilotByID[p.ID] = p
	}
	
	type teamEntry struct {
		pilotID int64
		points  int
	}
	teamDuels := make(map[int64][]teamEntry)
	for _, p := range standings.Pilots {
		if p.Garage == nil {
			continue
		}
		pts := standings.DriverPoints[p.ID]
		teamDuels[*p.Garage] = append(teamDuels[*p.Garage], teamEntry{p.ID, pts})
	}
	
	expectedRacePos := func(wccRank int) int {
		return (wccRank-1)*2 + 1
	}
	
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
		
		pilotPts := standings.DriverPoints[pilot.ID]
		isChampion := pilotPts == championPoints
		champRank := pilotChampRank(pilot.ID)
		
		// Собираем данные о дуэли
		duel := teamDuelResult{}
		if pilot.Garage != nil {
			pair := teamDuels[*pilot.Garage]
			if len(pair) == 2 {
				var me, opp teamEntry
				if pair[0].pilotID == pilot.ID {
					me, opp = pair[0], pair[1]
				} else {
					me, opp = pair[1], pair[0]
				}
				oppSnap := snapByID[opp.pilotID]
				duel = teamDuelResult{
					valid:         true,
					myPoints:      me.points,
					oppPoints:     opp.points,
					myBaseRating:  snap.BaseRating,
					oppBaseRating: oppSnap.BaseRating,
				}
			}
		}
		
		var expPos int
		if pilot.Garage != nil {
			wccRank := WCCRank(standings.TeamPoints, *pilot.Garage)
			expPos = expectedRacePos(wccRank)
		}
		
		var ratingDelta int
		
		if snap.BaseRating >= 95 {
			ratingDelta = ratingDeltaForElite(
				pilot.ID,
				pilotPts, championPoints,
				isChampion,
				duel,
				champRank, expPos,
			)
		} else {
			if isChampion {
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
			
			if championPoints-pilotPts <= 50 {
				ratingDelta += 1
			}
			
			if duel.valid {
				ptsDiff := duel.myPoints - duel.oppPoints
				ratingDiff := duel.myBaseRating - duel.oppBaseRating
				
				switch {
				case abs(ratingDiff) <= 3:
					if ptsDiff > 2 {
						ratingDelta += 1
					} else if ptsDiff < -2 {
						ratingDelta -= 1
					}
				default:
					if ptsDiff > 0 {
						if duel.myBaseRating < duel.oppBaseRating-3 {
							ratingDelta += 2
							applyDelta(ctx, groupID, e, pilotByID[snapByID[pilot.ID].PilotID], -2)
						} else {
							ratingDelta += 1
						}
					}
				}
			}
			
			if champRank < expPos {
				ratingDelta += 1
			} else if champRank > expPos+1 {
				ratingDelta -= 1
			}
		}
		
		if ratingDelta != 0 {
			applyDelta(ctx, groupID, e, pilot, ratingDelta)
		}
	}
}

// applyDelta применяет дельту рейтинга к пилоту и сохраняет в БД.
func applyDelta(ctx context.Context, groupID int64, e *Engine, pilot models.Pilot, delta int) {
	if delta == 0 {
		return
	}
	pilot.Rating = clamp(pilot.Rating+delta, 0, 100)
	_ = e.repo.UpdatePilot(ctx, groupID, pilot)
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
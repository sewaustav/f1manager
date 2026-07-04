package service

import (
	"context"
	"f1/internal/models"
	"math"
	"math/rand"
)

func (s *Service) calculateUpdate(team models.Team, investment int, stage int64) *models.Updates {
	components := []int{team.BaseLevel, team.Engineer, team.SimLevel, team.TubeLevel}
	
	sum := 0
	minComp := components[0]
	maxComp := components[0]
	
	for _, val := range components {
		sum += val
		if val < minComp {
			minComp = val
		}
		if val > maxComp {
			maxComp = val
		}
	}
	
	avgBase := float64(sum) / float64(len(components))
	delta := float64(maxComp - minComp)
	
	minBonus := -5.0
	maxBonus := 3.0
	
	investmentModifier := ((float64(investment) / 15.0) * 3.0) - 1.5
	baseModifier := ((avgBase / 100.0) * 2.0) - 1.0
	deltaPenalty := (delta / 100.0) * 4.5
	
	if team.CarLevel > 95 {
		penaltyRatio := float64(team.CarLevel-95) / 5.0
		if penaltyRatio > 1.0 {
			penaltyRatio = 1.0
		}
		maxBonus = 3.0 - (penaltyRatio * 3.0)
	}
	
	randomValue := rand.Float64()
	rawBonus := minBonus + randomValue*(maxBonus-minBonus)
	
	finalRawBonus := rawBonus + investmentModifier + baseModifier - deltaPenalty
	
	if finalRawBonus > maxBonus {
		finalRawBonus = maxBonus
	}
	if finalRawBonus < minBonus {
		finalRawBonus = minBonus
	}
	
	roundedBonus := int(math.Round(finalRawBonus))
	
	return &models.Updates{
		Team:    team,
		Bonus:   roundedBonus + team.UpdateRating,
		Stage:   stage,
		Synergy: 0,
	}
}

func (s *Service) bringUpdate(ctx context.Context, groupID, stage int64)  {
	updates, err := s.updateCache.GetUpdates(ctx, groupID)
	if err != nil {
		return
	}

	for _, update := range updates {
		if stage == update.Stage {
			s.updateCache.DeleteUpdate(ctx, update.Key) // if update can`t be deleted, it shouldn`t broke application 
			team, err := s.dynamic.GetTeamByGroup(ctx, update.TeamID, groupID)
			if err != nil {
				continue
			}
			
			if update.Type == Car {
				team.CarLevel += update.Bonus
			} else if update.Type == Synergy {
				team.CarSettings += update.Bonus
				
			} else {
				continue
			}
			if err = s.dynamic.UpdateTeam(ctx, update.PlayerID, team); err != nil {
				continue
			}
		}
	}
}

func (s *Service) calcBonus(input int) int {
	weights := map[int][]int{
		0:  {100, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		1:  {45, 35, 15, 5, 0, 0, 0, 0, 0, 0, 0},
		2:  {25, 45, 20, 7, 3, 0, 0, 0, 0, 0, 0},
		3:  {12, 25, 40, 15, 6, 2, 0, 0, 0, 0, 0},
		4:  {6, 12, 25, 35, 15, 5, 2, 0, 0, 0, 0},
		5:  {3, 6, 15, 40, 25, 8, 3, 0, 0, 0, 0},
		6:  {1, 4, 10, 25, 40, 15, 5, 0, 0, 0, 0},
		7:  {0, 2, 5, 13, 45, 25, 10, 0, 0, 0, 0},
		8:  {0, 0, 3, 7, 15, 50, 25, 0, 0, 0, 0},
		9:  {0, 0, 0, 4, 11, 40, 45, 0, 0, 0, 0},
		10: {0, 0, 0, 5, 10, 30, 55, 0, 0, 0, 0},
	}
	
	currentWeights, exists := weights[input]
	if !exists {
		return 0
	}
	
	roll := rand.Intn(100)
	accumulator := 0
	
	for choice, weight := range currentWeights {
		accumulator += weight
		if roll < accumulator {
			return choice
		}
	}
	
	return input
}

func (s *Service) getBonus(price, maxPrice int) int {
	if price == 0 {
		return s.calcBonus(maxPrice/2) * -1
	} else if price > maxPrice {
		return s.calcBonus(maxPrice)
	}
	
	return s.calcBonus(price)
}
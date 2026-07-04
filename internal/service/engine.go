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
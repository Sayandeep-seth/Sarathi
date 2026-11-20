package dispatch

import (
	"sort"
	"strconv"

	"sarathi/internal/common/utils"
)

// Rank scores candidates: closeness matters most, then rating and acceptance rate.
//
//	score = 0.6 × (1 − dist/maxDist) + 0.25 × rating/5 + 0.15 × acceptance
func Rank(cands []Candidate, acceptance func(driverID string) float64) []Candidate {
	if len(cands) == 0 {
		return cands
	}
	maxDist := 0.0
	for _, c := range cands {
		if c.DistKm > maxDist {
			maxDist = c.DistKm
		}
	}
	if maxDist == 0 {
		maxDist = 1
	}
	for i := range cands {
		c := &cands[i]
		c.Acceptance = acceptance(c.DriverID)
		c.Score = utils.Round(0.6*(1-c.DistKm/maxDist)+0.25*c.Rating/5+0.15*c.Acceptance, 4)
	}
	sort.SliceStable(cands, func(i, j int) bool { return cands[i].Score > cands[j].Score })
	return cands
}

func parseFloat(s string) float64 {
	f, _ := strconv.ParseFloat(s, 64)
	return f
}

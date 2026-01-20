package main

import (
	"fmt"
	"math"
	"sort"
	"time"
)

type GameState struct {
	Industries []IndustryState
	Resources  map[string]int
	BuyModeMax bool
}

type IndustryState struct {
	Key      string
	Name     string
	Resource string
	Workers  []WorkerState
}

type WorkerState struct {
	Definition WorkerConfig
	Owned      int
	Tier       int
	Running    bool
	EndsAt     time.Time
	Auto       bool
}

func BuildGame(cfg GameConfig) (*GameState, error) {
	resources := make(map[string]int)
	for key, value := range cfg.StartingResources {
		resources[key] = value
	}

	industries := make([]IndustryState, 0, len(cfg.Industries))
	for _, industry := range cfg.Industries {
		workers := make([]WorkerState, 0, len(industry.Workers))
		for _, worker := range industry.Workers {
			workers = append(workers, WorkerState{Definition: worker})
		}
		industries = append(industries, IndustryState{
			Key:      industry.Key,
			Name:     industry.Name,
			Resource: industry.Resource,
			Workers:  workers,
		})
	}

	if len(industries) > 5 {
		return nil, fmt.Errorf("too many industries: %d (max 5)", len(industries))
	}

	return &GameState{
		Industries: industries,
		Resources:  resources,
		BuyModeMax: false,
	}, nil
}

func (g *GameState) Update(now time.Time) {
	for industryIndex := range g.Industries {
		industry := &g.Industries[industryIndex]
		for workerIndex := range industry.Workers {
			worker := &industry.Workers[workerIndex]
			if !worker.Running {
				continue
			}
			if now.Before(worker.EndsAt) {
				continue
			}
			g.applyProduction(industry, worker)
			worker.Running = false
			if worker.Auto {
				worker.Running = true
				worker.EndsAt = now.Add(worker.Definition.ProdRate)
			}
		}
	}
}

func (g *GameState) StartRun(industryIndex, workerIndex int, now time.Time) string {
	worker := &g.Industries[industryIndex].Workers[workerIndex]
	if worker.Owned == 0 {
		return "need at least 1 worker"
	}
	if worker.Running {
		return "already running"
	}
	worker.Running = true
	worker.EndsAt = now.Add(worker.Definition.ProdRate)
	return "cycle started"
}

func (g *GameState) BuyWorker(industryIndex, workerIndex int) string {
	worker := &g.Industries[industryIndex].Workers[workerIndex]
	cost := worker.Definition.Cost
	count := 1
	if g.BuyModeMax {
		count = maxAffordable(cost, g.Resources)
	}
	if count <= 0 {
		return "cannot afford"
	}
	for resource, amount := range cost {
		g.Resources[resource] -= amount * count
	}
	worker.Owned += count
	return fmt.Sprintf("bought %d", count)
}

func (g *GameState) UpgradeWorker(industryIndex, workerIndex int) string {
	worker := &g.Industries[industryIndex].Workers[workerIndex]
	cost := scaledCost(worker.Definition.Cost, worker.Definition.UpgradeMult, worker.Tier)
	if !canAfford(cost, g.Resources) {
		return "cannot afford upgrade"
	}
	for resource, amount := range cost {
		g.Resources[resource] -= amount
	}
	worker.Tier++
	if worker.Definition.AutoTier > 0 && worker.Tier >= worker.Definition.AutoTier {
		worker.Auto = true
	}
	return "upgraded"
}

func (g *GameState) applyProduction(industry *IndustryState, worker *WorkerState) {
	if worker.Owned == 0 {
		return
	}
	produced := worker.Definition.ProdQuant * worker.Owned
	switch worker.Definition.Produces {
	case industry.Resource:
		g.Resources[industry.Resource] += produced
	default:
		g.Resources[worker.Definition.Produces] += produced
	}
}

func canAfford(cost, resources map[string]int) bool {
	for resource, amount := range cost {
		if resources[resource] < amount {
			return false
		}
	}
	return true
}

func maxAffordable(cost, resources map[string]int) int {
	limit := math.MaxInt
	for resource, amount := range cost {
		if amount <= 0 {
			continue
		}
		limit = minInt(limit, resources[resource]/amount)
	}
	if limit == math.MaxInt {
		return 0
	}
	return limit
}

func scaledCost(base map[string]int, multiplier float64, tier int) map[string]int {
	cost := make(map[string]int, len(base))
	factor := math.Pow(multiplier, float64(tier))
	for resource, amount := range base {
		cost[resource] = int(math.Ceil(float64(amount) * factor))
	}
	return cost
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (g *GameState) ResourceSummary() []string {
	if len(g.Resources) == 0 {
		return []string{"no resources"}
	}
	keys := make([]string, 0, len(g.Resources))
	for key := range g.Resources {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	lines := make([]string, 0, len(keys))
	for _, key := range keys {
		lines = append(lines, fmt.Sprintf("%s: %d", key, g.Resources[key]))
	}
	return lines
}

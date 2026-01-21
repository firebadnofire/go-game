package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
	"time"
)

type GameState struct {
	Industries []IndustryState
	Resources  map[string]int
	Production []PassiveProductionState
	BuyModeMax bool
	DevMode    bool
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

type PassiveProductionState struct {
	Definition PassiveProductionSpec
	NextAt     time.Time
}

type saveGame struct {
	Industries []saveIndustry   `json:"industries"`
	Resources  map[string]int   `json:"resources"`
	Production []saveProduction `json:"production"`
	BuyModeMax bool             `json:"buyModeMax"`
	DevMode    bool             `json:"devMode"`
	SavedAt    time.Time        `json:"savedAt"`
	Version    int              `json:"version"`
}

type saveIndustry struct {
	Key     string       `json:"key"`
	Workers []saveWorker `json:"workers"`
}

type saveWorker struct {
	Key   string `json:"key"`
	Owned int    `json:"owned"`
	Tier  int    `json:"tier"`
	Auto  bool   `json:"auto"`
}

type saveProduction struct {
	NextAt time.Time `json:"nextAt"`
}

func BuildGame(cfg GameConfig) (*GameState, error) {
	resources := make(map[string]int)
	for key, value := range cfg.StartingResources {
		resources[key] = value
	}

	industries := make([]IndustryState, 0, len(cfg.Industries))
	for _, industry := range cfg.Industries {
		workers := make([]WorkerState, 0, len(industry.Workers))
		for index, worker := range industry.Workers {
			owned := 0
			if index == 0 {
				owned = 1
			}
			workers = append(workers, WorkerState{
				Definition: worker,
				Owned:      owned,
				Tier:       1,
			})
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
		Production: buildPassiveProduction(cfg.StartingProduction),
		BuyModeMax: false,
	}, nil
}

func (g *GameState) Update(now time.Time) {
	for index := range g.Production {
		g.Production[index].apply(now, g.Resources)
	}
	for industryIndex := range g.Industries {
		industry := &g.Industries[industryIndex]
		for workerIndex := range industry.Workers {
			worker := &industry.Workers[workerIndex]
			if worker.Auto && !worker.Running && worker.Owned > 0 {
				worker.Running = true
				worker.EndsAt = now.Add(worker.Definition.ProdRate)
			}
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
	if g.DevMode {
		if g.BuyModeMax {
			count = maxAffordable(cost, g.Resources)
			if count < 1 {
				count = 1
			}
		}
	} else if g.BuyModeMax {
		count = maxAffordable(cost, g.Resources)
	} else if !canAfford(cost, g.Resources) {
		return "cannot afford"
	}
	if count <= 0 {
		return "cannot afford"
	}
	if !g.DevMode {
		for resource, amount := range cost {
			g.Resources[resource] -= amount * count
		}
	}
	worker.Owned += count
	return fmt.Sprintf("bought %d", count)
}

func (g *GameState) UpgradeWorker(industryIndex, workerIndex int) string {
	worker := &g.Industries[industryIndex].Workers[workerIndex]
	cost := scaledCost(worker.Definition.Cost, worker.Definition.UpgradeMult, worker.Tier)
	if !g.DevMode && !canAfford(cost, g.Resources) {
		return "cannot afford upgrade"
	}
	if !g.DevMode {
		for resource, amount := range cost {
			g.Resources[resource] -= amount
		}
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
	if targetIndex, ok := findWorkerIndex(industry.Workers, worker.Definition.Produces); ok {
		industry.Workers[targetIndex].Owned += produced
		return
	}
	if worker.Definition.Produces == industry.Resource {
		g.Resources[industry.Resource] += produced
		return
	}
	g.Resources[worker.Definition.Produces] += produced
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
	factor := math.Pow(multiplier, float64(maxInt(tier-1, 0)))
	for resource, amount := range base {
		cost[resource] = int(math.Ceil(float64(amount) * factor))
	}
	return cost
}

func findWorkerIndex(workers []WorkerState, key string) (int, bool) {
	for index, worker := range workers {
		if worker.Definition.Key == key {
			return index, true
		}
	}
	return 0, false
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
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

func buildPassiveProduction(definitions []PassiveProductionSpec) []PassiveProductionState {
	if len(definitions) == 0 {
		return nil
	}
	production := make([]PassiveProductionState, 0, len(definitions))
	for _, definition := range definitions {
		production = append(production, PassiveProductionState{
			Definition: definition,
			NextAt:     time.Now().Add(definition.ProdRate),
		})
	}
	return production
}

func (p *PassiveProductionState) apply(now time.Time, resources map[string]int) {
	if now.Before(p.NextAt) {
		return
	}
	if p.Definition.ProdRate <= 0 || p.Definition.ProdQuant <= 0 {
		return
	}
	for !now.Before(p.NextAt) {
		resources[p.Definition.Resource] += p.Definition.ProdQuant
		p.NextAt = p.NextAt.Add(p.Definition.ProdRate)
	}
}

func (g *GameState) SaveToFile(path string) error {
	snapshot := g.snapshot()
	payload, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return fmt.Errorf("serialize save: %w", err)
	}
	if err := os.WriteFile(path, payload, 0o644); err != nil {
		return fmt.Errorf("write save: %w", err)
	}
	return nil
}

func (g *GameState) LoadFromFile(path string) error {
	payload, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read save: %w", err)
	}
	var snapshot saveGame
	if err := json.Unmarshal(payload, &snapshot); err != nil {
		return fmt.Errorf("parse save: %w", err)
	}
	if err := g.applySnapshot(snapshot); err != nil {
		return fmt.Errorf("apply save: %w", err)
	}
	return nil
}

func (g *GameState) snapshot() saveGame {
	industries := make([]saveIndustry, 0, len(g.Industries))
	for _, industry := range g.Industries {
		workers := make([]saveWorker, 0, len(industry.Workers))
		for _, worker := range industry.Workers {
			workers = append(workers, saveWorker{
				Key:   worker.Definition.Key,
				Owned: worker.Owned,
				Tier:  worker.Tier,
				Auto:  worker.Auto,
			})
		}
		industries = append(industries, saveIndustry{
			Key:     industry.Key,
			Workers: workers,
		})
	}

	production := make([]saveProduction, 0, len(g.Production))
	for _, entry := range g.Production {
		production = append(production, saveProduction{
			NextAt: entry.NextAt,
		})
	}

	resources := make(map[string]int, len(g.Resources))
	for key, value := range g.Resources {
		resources[key] = value
	}

	return saveGame{
		Industries: industries,
		Resources:  resources,
		Production: production,
		BuyModeMax: g.BuyModeMax,
		DevMode:    g.DevMode,
		SavedAt:    time.Now(),
		Version:    1,
	}
}

func (g *GameState) applySnapshot(snapshot saveGame) error {
	if snapshot.Resources == nil {
		return fmt.Errorf("save missing resources")
	}
	if len(snapshot.Industries) != len(g.Industries) {
		return fmt.Errorf("save industries mismatch")
	}
	if len(snapshot.Production) != len(g.Production) {
		return fmt.Errorf("save production mismatch")
	}

	industryLookup := make(map[string]saveIndustry, len(snapshot.Industries))
	for _, industry := range snapshot.Industries {
		industryLookup[industry.Key] = industry
	}

	for industryIndex := range g.Industries {
		industry := &g.Industries[industryIndex]
		savedIndustry, ok := industryLookup[industry.Key]
		if !ok {
			return fmt.Errorf("save missing industry %s", industry.Key)
		}

		workerLookup := make(map[string]saveWorker, len(savedIndustry.Workers))
		for _, worker := range savedIndustry.Workers {
			workerLookup[worker.Key] = worker
		}

		for workerIndex := range industry.Workers {
			worker := &industry.Workers[workerIndex]
			savedWorker, ok := workerLookup[worker.Definition.Key]
			if !ok {
				return fmt.Errorf("save missing worker %s", worker.Definition.Key)
			}
			worker.Owned = savedWorker.Owned
			worker.Tier = savedWorker.Tier
			worker.Auto = savedWorker.Auto || (worker.Definition.AutoTier > 0 && savedWorker.Tier >= worker.Definition.AutoTier)
			worker.Running = false
			worker.EndsAt = time.Time{}
		}
	}

	g.Resources = make(map[string]int, len(snapshot.Resources))
	for key, value := range snapshot.Resources {
		g.Resources[key] = value
	}

	for index := range g.Production {
		g.Production[index].NextAt = snapshot.Production[index].NextAt
	}

	g.BuyModeMax = snapshot.BuyModeMax
	g.DevMode = snapshot.DevMode
	return nil
}

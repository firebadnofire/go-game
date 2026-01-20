package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/gdamore/tcell/v2"
	"golang.org/x/term"
	"gopkg.in/yaml.v3"
)

const (
	minWidth  = 135
	minHeight = 35
)

type Config struct {
	Industries []IndustryDef `yaml:"industry"`
}

type IndustryDef struct {
	Name     string      `yaml:"name"`
	Resource string      `yaml:"resource"`
	Workers  []WorkerDef `yaml:"workers"`
}

type WorkerDef struct {
	WorkerName  string  `yaml:"workerName"`
	ProdRateSec float64 `yaml:"prodRate"`
	ProdQuant   int64   `yaml:"prodQuant"`
	UpgradeMult float64 `yaml:"upgradeMult"`
}

type Game struct {
	Industries []IndustryState
}

type IndustryState struct {
	Def         IndustryDef
	ResourceQty int64
	Workers     []WorkerState
}

type WorkerState struct {
	Def      WorkerDef
	Running  bool
	Level    int
	Progress time.Duration
}

type UIState struct {
	SelectedIndustry int
	TabOffset        int
	SelectedWorker   int
	WorkerOffset     int
	MinSize          bool
}

func main() {
	configPath := flag.String("config", "game.yaml", "Path to the YAML config")
	flag.Parse()

	config, err := loadConfig(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}

	game := buildGame(config)
	ui := UIState{}

	screen, err := tcell.NewScreen()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to init screen: %v\n", err)
		os.Exit(1)
	}
	if err := screen.Init(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to start screen: %v\n", err)
		os.Exit(1)
	}
	defer screen.Fini()

	screen.Clear()

	quit := make(chan struct{})
	resize := make(chan struct{}, 1)
	input := make(chan *tcell.EventKey, 8)

	go func() {
		for {
			ev := screen.PollEvent()
			switch e := ev.(type) {
			case *tcell.EventResize:
				resize <- struct{}{}
			case *tcell.EventKey:
				input <- e
			}
		}
	}()

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	lastTick := time.Now()
	running := true
	for running {
		select {
		case <-quit:
			running = false
		case <-resize:
			applyMinSize(&ui)
		case ev := <-input:
			running = handleInput(ev, &game, &ui, quit)
		case <-ticker.C:
			now := time.Now()
			delta := now.Sub(lastTick)
			lastTick = now
			updateGame(&game, delta)
		}

		draw(screen, &game, &ui)
	}
}

func loadConfig(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func buildGame(cfg Config) Game {
	industries := make([]IndustryState, 0, len(cfg.Industries))
	for _, industry := range cfg.Industries {
		workers := make([]WorkerState, 0, len(industry.Workers))
		for _, worker := range industry.Workers {
			workers = append(workers, WorkerState{
				Def:   worker,
				Level: 0,
			})
		}
		industries = append(industries, IndustryState{
			Def:     industry,
			Workers: workers,
		})
	}
	return Game{Industries: industries}
}

func applyMinSize(ui *UIState) {
	width, height, err := term.GetSize(int(os.Stdout.Fd()))
	ui.MinSize = err != nil || width < minWidth || height < minHeight
}

func handleInput(ev *tcell.EventKey, game *Game, ui *UIState, quit chan struct{}) bool {
	if ev.Key() == tcell.KeyCtrlC {
		close(quit)
		return false
	}

	switch ev.Key() {
	case tcell.KeyLeft:
		moveIndustry(-1, game, ui)
	case tcell.KeyRight:
		moveIndustry(1, game, ui)
	case tcell.KeyUp:
		moveWorker(-1, game, ui)
	case tcell.KeyDown:
		moveWorker(1, game, ui)
	case tcell.KeyEnter:
		toggleWorker(game, ui)
	case tcell.KeyRune:
		switch ev.Rune() {
		case 'a':
			moveIndustry(-1, game, ui)
		case 'd':
			moveIndustry(1, game, ui)
		case 'w':
			moveWorker(-1, game, ui)
		case 's':
			moveWorker(1, game, ui)
		case 'r':
			toggleWorker(game, ui)
		case 'u':
			upgradeWorker(game, ui)
		case 'q':
			close(quit)
			return false
		}
	}

	return true
}

func moveIndustry(delta int, game *Game, ui *UIState) {
	if len(game.Industries) == 0 {
		return
	}
	ui.SelectedIndustry = clamp(ui.SelectedIndustry+delta, 0, len(game.Industries)-1)
	ui.SelectedWorker = 0
	ui.WorkerOffset = 0
	ui.TabOffset = tabOffset(ui.SelectedIndustry, len(game.Industries), 5)
}

func moveWorker(delta int, game *Game, ui *UIState) {
	if len(game.Industries) == 0 {
		return
	}
	workers := game.Industries[ui.SelectedIndustry].Workers
	if len(workers) == 0 {
		return
	}
	ui.SelectedWorker = clamp(ui.SelectedWorker+delta, 0, len(workers)-1)
	ui.WorkerOffset = workerOffset(ui.SelectedWorker, len(workers), 10, ui.WorkerOffset)
}

func toggleWorker(game *Game, ui *UIState) {
	if len(game.Industries) == 0 {
		return
	}
	industry := &game.Industries[ui.SelectedIndustry]
	if len(industry.Workers) == 0 {
		return
	}
	worker := &industry.Workers[ui.SelectedWorker]
	worker.Running = !worker.Running
}

func upgradeWorker(game *Game, ui *UIState) {
	if len(game.Industries) == 0 {
		return
	}
	industry := &game.Industries[ui.SelectedIndustry]
	if len(industry.Workers) == 0 {
		return
	}
	worker := &industry.Workers[ui.SelectedWorker]
	cost := upgradeCost(worker)
	if industry.ResourceQty < cost {
		return
	}
	industry.ResourceQty -= cost
	worker.Level++
}

func updateGame(game *Game, delta time.Duration) {
	for i := range game.Industries {
		industry := &game.Industries[i]
		for j := range industry.Workers {
			worker := &industry.Workers[j]
			if !worker.Running {
				continue
			}
			worker.Progress += delta
			cycle := time.Duration(worker.Def.ProdRateSec * float64(time.Second))
			if cycle <= 0 {
				continue
			}
			for worker.Progress >= cycle {
				worker.Progress -= cycle
				industry.ResourceQty += workerYield(worker)
			}
		}
	}
}

func workerYield(worker *WorkerState) int64 {
	multiplier := 1.0 + float64(worker.Level)*worker.Def.UpgradeMult
	if multiplier < 1.0 {
		multiplier = 1.0
	}
	return int64(float64(worker.Def.ProdQuant) * multiplier)
}

func upgradeCost(worker *WorkerState) int64 {
	base := int64(10)
	if worker.Def.ProdQuant > 0 {
		base = worker.Def.ProdQuant
	}
	return base * int64(10+worker.Level*5)
}

func draw(screen tcell.Screen, game *Game, ui *UIState) {
	screen.Clear()
	applyMinSize(ui)
	width, height := screen.Size()

	if ui.MinSize {
		drawCentered(screen, width, height, "Terminal too small", "Needs at least 135x35.", "Resize to continue.")
		screen.Show()
		return
	}

	drawHeader(screen, width, game, ui)
	drawIndustry(screen, width, height, game, ui)
	screen.Show()
}

func drawHeader(screen tcell.Screen, width int, game *Game, ui *UIState) {
	style := tcell.StyleDefault.Background(tcell.ColorBlack).Foreground(tcell.ColorWhite)
	status := "a/d or ←/→ to switch industries | w/s or ↑/↓ to select worker | Enter/r to run | u to upgrade | q to quit"
	drawText(screen, 1, 0, style, truncate(status, width-2))

	tabs := visibleTabs(ui.SelectedIndustry, len(game.Industries), 5)
	x := 1
	for _, idx := range tabs {
		if idx < 0 || idx >= len(game.Industries) {
			continue
		}
		industry := game.Industries[idx]
		name := fmt.Sprintf(" %s ", industry.Def.Name)
		tabStyle := style
		if idx == ui.SelectedIndustry {
			tabStyle = tabStyle.Background(tcell.ColorBlue).Foreground(tcell.ColorBlack)
		}
		drawText(screen, x, 2, tabStyle, name)
		x += len(name) + 1
	}
}

func drawIndustry(screen tcell.Screen, width, height int, game *Game, ui *UIState) {
	if len(game.Industries) == 0 {
		drawText(screen, 2, 4, tcell.StyleDefault, "No industries configured.")
		return
	}
	industry := game.Industries[ui.SelectedIndustry]
	info := fmt.Sprintf("Industry: %s | Resource: %s | Stored: %d", industry.Def.Name, industry.Def.Resource, industry.ResourceQty)
	drawText(screen, 2, 4, tcell.StyleDefault.Foreground(tcell.ColorGreen), truncate(info, width-4))

	startY := 6
	visibleRows := height - startY - 2
	if visibleRows < 1 {
		return
	}

	workers := industry.Workers
	end := ui.WorkerOffset + visibleRows
	if end > len(workers) {
		end = len(workers)
	}
	for i := ui.WorkerOffset; i < end; i++ {
		worker := workers[i]
		cursor := " "
		if i == ui.SelectedWorker {
			cursor = ">"
		}
		status := "idle"
		if worker.Running {
			status = "running"
		}
		line := fmt.Sprintf("%s %-14s | rate: %.1fs | yield: %d | lvl: %d | %s | upgrade cost: %d",
			cursor,
			worker.Def.WorkerName,
			worker.Def.ProdRateSec,
			workerYield(&worker),
			worker.Level,
			status,
			upgradeCost(&worker),
		)
		drawText(screen, 2, startY+(i-ui.WorkerOffset), tcell.StyleDefault, truncate(line, width-4))
	}
}

func drawCentered(screen tcell.Screen, width, height int, lines ...string) {
	startY := height/2 - len(lines)/2
	for i, line := range lines {
		x := width/2 - len(line)/2
		if x < 0 {
			x = 0
		}
		drawText(screen, x, startY+i, tcell.StyleDefault.Foreground(tcell.ColorRed), line)
	}
}

func drawText(screen tcell.Screen, x, y int, style tcell.Style, text string) {
	for i, ch := range text {
		screen.SetContent(x+i, y, ch, nil, style)
	}
}

func truncate(value string, max int) string {
	if max <= 0 || len(value) <= max {
		return value
	}
	if max < 3 {
		return value[:max]
	}
	return value[:max-3] + "..."
}

func clamp(value, min, max int) int {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func visibleTabs(selected, total, maxTabs int) []int {
	if total == 0 {
		return nil
	}
	start := tabOffset(selected, total, maxTabs)
	end := start + maxTabs
	if end > total {
		end = total
	}
	indices := make([]int, 0, end-start)
	for i := start; i < end; i++ {
		indices = append(indices, i)
	}
	return indices
}

func tabOffset(selected, total, maxTabs int) int {
	if total <= maxTabs {
		return 0
	}
	start := selected - maxTabs/2
	if start < 0 {
		start = 0
	}
	if start+maxTabs > total {
		start = total - maxTabs
	}
	return start
}

func workerOffset(selected, total, visible, current int) int {
	if total <= visible {
		return 0
	}
	if selected < current {
		return selected
	}
	if selected >= current+visible {
		return selected - visible + 1
	}
	return current
}

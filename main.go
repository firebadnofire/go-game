package main

import (
	"fmt"
	"os"
	"time"

	"github.com/gdamore/tcell/v2"
)

const (
	minWidth  = 135
	minHeight = 35
)

type ResourceConfig struct {
	ID   string
	Name string
}

type WorkerConfig struct {
	ID                 string
	Name               string
	ProducesResourceID string
	ProducesWorkerID   string
	Interval           time.Duration
	Yield              int
}

type IndustryConfig struct {
	ID       string
	Name     string
	Resource ResourceConfig
	Workers  []WorkerConfig
}

type Config struct {
	Industries []IndustryConfig
}

type Industry struct {
	ID       string
	Name     string
	Resource ResourceConfig
	Workers  []WorkerConfig
}

type Game struct {
	Industries       []Industry
	SelectedIndustry int
	WorkerScroll     int
	TabOffset        int
}

func BuildGameFromConfig(cfg Config) Game {
	industries := make([]Industry, 0, len(cfg.Industries))
	for _, industry := range cfg.Industries {
		workers := make([]WorkerConfig, len(industry.Workers))
		copy(workers, industry.Workers)
		industries = append(industries, Industry{
			ID:       industry.ID,
			Name:     industry.Name,
			Resource: industry.Resource,
			Workers:  workers,
		})
	}

	return Game{
		Industries:       industries,
		SelectedIndustry: 0,
		WorkerScroll:     0,
		TabOffset:        0,
	}
}

func defaultConfig() Config {
	return Config{
		Industries: []IndustryConfig{
			{
				ID:   "industry1",
				Name: "Coal Production",
				Resource: ResourceConfig{
					ID:   "resource1",
					Name: "Coal",
				},
				Workers: []WorkerConfig{
					{
						ID:                 "worker1",
						Name:               "Miner",
						ProducesResourceID: "resource1",
						Interval:           time.Second,
						Yield:              25,
					},
					{
						ID:                 "worker2",
						Name:               "Driller",
						ProducesResourceID: "resource1",
						ProducesWorkerID:   "worker1",
						Interval:           5 * time.Second,
						Yield:              50,
					},
				},
			},
			{
				ID:   "industry2",
				Name: "Iron Production",
				Resource: ResourceConfig{
					ID:   "resource2",
					Name: "Iron",
				},
				Workers: []WorkerConfig{
					{
						ID:                 "worker3",
						Name:               "Prospector",
						ProducesResourceID: "resource2",
						Interval:           2 * time.Second,
						Yield:              15,
					},
					{
						ID:                 "worker4",
						Name:               "Extractor",
						ProducesResourceID: "resource2",
						ProducesWorkerID:   "worker3",
						Interval:           6 * time.Second,
						Yield:              40,
					},
				},
			},
		},
	}
}

func main() {
	screen, err := tcell.NewScreen()
	if err != nil {
		fmt.Println("failed to create screen:", err)
		os.Exit(1)
	}
	if err := screen.Init(); err != nil {
		fmt.Println("failed to initialize screen:", err)
		os.Exit(1)
	}
	defer screen.Fini()

	game := BuildGameFromConfig(defaultConfig())
	style := tcell.StyleDefault.Foreground(tcell.ColorWhite).Background(tcell.ColorBlack)

	screen.Clear()
	draw(screen, style, game)

	for {
		switch event := screen.PollEvent().(type) {
		case *tcell.EventResize:
			screen.Sync()
			draw(screen, style, game)
		case *tcell.EventKey:
			if handleKey(event, &game) {
				return
			}
			draw(screen, style, game)
		}
	}
}

func handleKey(event *tcell.EventKey, game *Game) bool {
	switch event.Key() {
	case tcell.KeyEscape:
		return true
	case tcell.KeyLeft:
		selectIndustry(game, game.SelectedIndustry-1)
	case tcell.KeyRight:
		selectIndustry(game, game.SelectedIndustry+1)
	case tcell.KeyUp:
		scrollWorkers(game, -1)
	case tcell.KeyDown:
		scrollWorkers(game, 1)
	}

	switch event.Rune() {
	case 'q':
		return true
	case 'a':
		selectIndustry(game, game.SelectedIndustry-1)
	case 'd':
		selectIndustry(game, game.SelectedIndustry+1)
	case 'w':
		scrollWorkers(game, -1)
	case 's':
		scrollWorkers(game, 1)
	}

	return false
}

func selectIndustry(game *Game, index int) {
	if len(game.Industries) == 0 {
		return
	}
	if index < 0 {
		index = 0
	}
	if index >= len(game.Industries) {
		index = len(game.Industries) - 1
	}
	game.SelectedIndustry = index
	game.WorkerScroll = 0
	if index < game.TabOffset {
		game.TabOffset = index
	}
	if index >= game.TabOffset+maxTabs() {
		game.TabOffset = index - maxTabs() + 1
	}
}

func scrollWorkers(game *Game, delta int) {
	workers := game.Industries[game.SelectedIndustry].Workers
	if len(workers) == 0 {
		game.WorkerScroll = 0
		return
	}
	game.WorkerScroll += delta
	if game.WorkerScroll < 0 {
		game.WorkerScroll = 0
	}
	if game.WorkerScroll > len(workers)-1 {
		game.WorkerScroll = len(workers) - 1
	}
}

func maxTabs() int {
	return 5
}

func draw(screen tcell.Screen, style tcell.Style, game Game) {
	width, height := screen.Size()
	screen.Clear()
	if width < minWidth || height < minHeight {
		drawSmallScreen(screen, style, width, height)
		screen.Show()
		return
	}

	drawText(screen, style, 1, 0, "Exponential Ladder Game")
	drawTabs(screen, style, game, width)
	drawWorkers(screen, style, game, width, height)
	screen.Show()
}

func drawSmallScreen(screen tcell.Screen, style tcell.Style, width, height int) {
	message := fmt.Sprintf("Terminal too small (%dx%d). Minimum size is %dx%d.", width, height, minWidth, minHeight)
	x := max(0, (width-len(message))/2)
	y := height / 2
	drawText(screen, style, x, y, message)
}

func drawTabs(screen tcell.Screen, style tcell.Style, game Game, width int) {
	start := game.TabOffset
	end := min(len(game.Industries), start+maxTabs())
	x := 1
	for i := start; i < end; i++ {
		tabStyle := style
		if i == game.SelectedIndustry {
			tabStyle = tabStyle.Reverse(true)
		}
		label := fmt.Sprintf("[%s]", game.Industries[i].Name)
		drawText(screen, tabStyle, x, 2, label)
		x += len(label) + 1
		if x >= width-1 {
			break
		}
	}
	if len(game.Industries) > maxTabs() {
		drawText(screen, style, width-10, 2, "< a/d >")
	}
}

func drawWorkers(screen tcell.Screen, style tcell.Style, game Game, width, height int) {
	if len(game.Industries) == 0 {
		return
	}
	industry := game.Industries[game.SelectedIndustry]
	drawText(screen, style, 1, 4, fmt.Sprintf("Industry: %s", industry.Name))
	drawText(screen, style, 1, 5, fmt.Sprintf("Resource: %s", industry.Resource.Name))
	drawText(screen, style, 1, 7, "Workers (scroll with w/s or ↑/↓):")

	availableLines := height - 9
	if availableLines <= 0 {
		return
	}
	start := game.WorkerScroll
	end := min(len(industry.Workers), start+availableLines)
	line := 8
	for i := start; i < end; i++ {
		worker := industry.Workers[i]
		produces := worker.ProducesResourceID
		if worker.ProducesWorkerID != "" {
			produces = fmt.Sprintf("%s + %s", produces, worker.ProducesWorkerID)
		}
		text := fmt.Sprintf("- %s | every %s | yield %d | produces %s", worker.Name, worker.Interval, worker.Yield, produces)
		drawText(screen, style, 3, line, truncate(text, width-4))
		line++
	}
}

func drawText(screen tcell.Screen, style tcell.Style, x, y int, text string) {
	for i, r := range text {
		screen.SetContent(x+i, y, r, nil, style)
	}
}

func truncate(text string, maxWidth int) string {
	if maxWidth <= 0 || len(text) <= maxWidth {
		return text
	}
	if maxWidth <= 3 {
		return text[:maxWidth]
	}
	return text[:maxWidth-3] + "..."
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

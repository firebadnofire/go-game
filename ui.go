package main

import (
	"fmt"
	"time"

	"github.com/gdamore/tcell/v2"
)

const (
	minWidth  = 85
	minHeight = 22
	saveFile  = "savegame.json"
)

type UI struct {
	screen         tcell.Screen
	game           *GameState
	activeIndustry int
	selectedWorker int
	statusMessage  string
	lastStatusAt   time.Time
	workerScroll   int
}

func NewUI(game *GameState) (*UI, error) {
	screen, err := tcell.NewScreen()
	if err != nil {
		return nil, err
	}
	if err := screen.Init(); err != nil {
		return nil, err
	}

	screen.SetStyle(tcell.StyleDefault.Foreground(tcell.ColorWhite).Background(tcell.ColorBlack))
	return &UI{screen: screen, game: game}, nil
}

func (ui *UI) Close() {
	ui.screen.Fini()
}

func (ui *UI) Run() error {
	defer ui.Close()

	tick := time.NewTicker(100 * time.Millisecond)
	defer tick.Stop()

	eventCh := make(chan tcell.Event)
	done := make(chan struct{})
	go func() {
		for {
			select {
			case <-done:
				return
			case eventCh <- ui.screen.PollEvent():
			}
		}
	}()
	defer close(done)

	for {
		ui.draw()
		select {
		case <-tick.C:
			ui.game.Update(time.Now())
		case ev := <-eventCh:
			switch event := ev.(type) {
			case *tcell.EventResize:
				ui.screen.Sync()
			case *tcell.EventKey:
				if ui.handleKey(event) {
					return nil
				}
			}
		}
	}
}

func (ui *UI) handleKey(event *tcell.EventKey) bool {
	switch event.Key() {
	case tcell.KeyEscape, tcell.KeyCtrlC:
		return true
	case tcell.KeyLeft:
		ui.shiftIndustry(-1)
	case tcell.KeyRight:
		ui.shiftIndustry(1)
	case tcell.KeyUp:
		ui.shiftWorker(-1)
	case tcell.KeyDown:
		ui.shiftWorker(1)
	default:
		switch event.Rune() {
		case 'a':
			ui.shiftIndustry(-1)
		case 'd':
			ui.shiftIndustry(1)
		case 'w':
			ui.shiftWorker(-1)
		case 's':
			ui.shiftWorker(1)
		case 'b':
			ui.setStatus(ui.game.BuyWorker(ui.activeIndustry, ui.selectedWorker))
		case 'r', ' ':
			ui.setStatus(ui.game.StartRun(ui.activeIndustry, ui.selectedWorker, time.Now()))
		case 'u':
			ui.setStatus(ui.game.UpgradeWorker(ui.activeIndustry, ui.selectedWorker))
		case 'm':
			ui.game.BuyModeMax = !ui.game.BuyModeMax
			ui.setStatus(ui.buyModeLabel())
		case 'q':
			ui.setStatus(ui.runLowestAvailable(time.Now()))
		case 't':
			ui.setStatus(ui.guardDevMode("save", ui.saveGame))
		case 'y':
			ui.setStatus(ui.guardDevMode("load", ui.loadGame))
		}
	}

	return false
}

func (ui *UI) shiftIndustry(delta int) {
	count := len(ui.game.Industries)
	if count == 0 {
		return
	}
	ui.activeIndustry = (ui.activeIndustry + delta + count) % count
	ui.selectedWorker = 0
	ui.workerScroll = 0
}

func (ui *UI) shiftWorker(delta int) {
	workers := ui.game.Industries[ui.activeIndustry].Workers
	if len(workers) == 0 {
		return
	}
	ui.selectedWorker = clamp(ui.selectedWorker+delta, 0, len(workers)-1)
}

func (ui *UI) runLowestAvailable(now time.Time) string {
	industry := ui.game.Industries[ui.activeIndustry]
	for index, worker := range industry.Workers {
		if worker.Auto || worker.Running {
			continue
		}
		if worker.Owned == 0 {
			continue
		}
		return ui.game.StartRun(ui.activeIndustry, index, now)
	}
	return "no manual workers available"
}

func (ui *UI) draw() {
	ui.screen.Clear()
	width, height := ui.screen.Size()
	if width < minWidth || height < minHeight {
		ui.drawTooSmall(width, height)
		ui.screen.Show()
		return
	}

	ui.drawHeader(width)
	ui.drawResources(2, 4, width)
	ui.drawWorkers(2, 8, width, height-10)
	ui.drawFooter(2, height-2, width)
	ui.screen.Show()
}

func (ui *UI) drawTooSmall(width, height int) {
	message := fmt.Sprintf("Terminal too small (%dx%d). Need at least %dx%d.", width, height, minWidth, minHeight)
	ui.drawTextCentered(width, height/2, message, tcell.StyleDefault.Foreground(tcell.ColorRed))
	ui.drawTextCentered(width, height/2+2, "Resize the window to continue.", tcell.StyleDefault.Foreground(tcell.ColorWhite))
}

func (ui *UI) drawHeader(width int) {
	ui.drawText(2, 1, "Go Game - Industry Ladder", tcell.StyleDefault.Bold(true))
	if ui.game.DevMode {
		label := "developer mode"
		startX := width - len(label) - 2
		if startX > 2 {
			ui.drawText(startX, 1, label, tcell.StyleDefault.Bold(true))
		}
	}

	industryLabels := make([]string, 0, len(ui.game.Industries))
	for _, industry := range ui.game.Industries {
		label := industry.Name
		if label == "" {
			label = industry.Key
		}
		industryLabels = append(industryLabels, label)
	}

	visible := industryLabels
	if len(visible) > 5 {
		visible = visible[:5]
	}

	startX := 2
	for idx, label := range visible {
		style := tcell.StyleDefault
		if idx == ui.activeIndustry {
			style = style.Reverse(true)
		}
		ui.drawText(startX, 2, fmt.Sprintf("[%s]", label), style)
		startX += len(label) + 3
	}
}

func (ui *UI) drawResources(x, y, width int) {
	ui.drawText(x, y, "Resources:", tcell.StyleDefault.Bold(true))
	lines := ui.game.ResourceSummary()
	for i, line := range lines {
		ui.drawText(x+2, y+1+i, truncate(line, width-x-4), tcell.StyleDefault)
	}
}

func (ui *UI) drawWorkers(x, y, width, height int) {
	industry := ui.game.Industries[ui.activeIndustry]
	ui.drawText(x, y, fmt.Sprintf("Workers - %s", industry.Name), tcell.StyleDefault.Bold(true))
	start := ui.workerScroll
	end := minInt(len(industry.Workers), start+height-2)
	if ui.selectedWorker >= end {
		start = ui.selectedWorker - (height - 3)
		ui.workerScroll = start
		end = minInt(len(industry.Workers), start+height-2)
	}
	if ui.selectedWorker < start {
		start = ui.selectedWorker
		ui.workerScroll = start
		end = minInt(len(industry.Workers), start+height-2)
	}

	for i := start; i < end; i++ {
		worker := industry.Workers[i]
		status := "idle"
		if worker.Running {
			remaining := time.Until(worker.EndsAt).Truncate(time.Second)
			if remaining < 0 {
				remaining = 0
			}
			status = fmt.Sprintf("running %s", remaining)
		}
		autoLabel := "manual"
		if worker.Auto {
			autoLabel = "auto"
		}
		line := fmt.Sprintf("%s | owned %d | tier %d | %s | %s", worker.Definition.WorkerName, worker.Owned, worker.Tier, status, autoLabel)
		style := tcell.StyleDefault
		if i == ui.selectedWorker {
			style = style.Reverse(true)
		}
		ui.drawText(x+2, y+1+(i-start), truncate(line, width-x-4), style)
	}
}

func (ui *UI) drawFooter(x, y, width int) {
	controlsTop := "a/d or ←/→ switch industry | w/s or ↑/↓ select worker | b buy"
	controlsBottom := "r run | q global run | u upgrade | m toggle buy mode | t save | y load | esc quit"
	ui.drawText(x, y-1, truncate(controlsTop, width-x-2), tcell.StyleDefault)
	ui.drawText(x, y, truncate(controlsBottom, width-x-2), tcell.StyleDefault)
	status := ui.statusMessage
	if time.Since(ui.lastStatusAt) > 5*time.Second {
		status = ui.buyModeLabel()
	}
	ui.drawText(x, y-2, truncate(status, width-x-2), tcell.StyleDefault.Foreground(tcell.ColorGreen))
}

func (ui *UI) setStatus(message string) {
	ui.statusMessage = message
	ui.lastStatusAt = time.Now()
}

func (ui *UI) buyModeLabel() string {
	if ui.game.BuyModeMax {
		return "buy mode: 100%"
	}
	return "buy mode: 1x"
}

func (ui *UI) guardDevMode(action string, fn func() string) string {
	if ui.game.DevMode {
		return fmt.Sprintf("%s disabled in developer mode", action)
	}
	return fn()
}

func (ui *UI) saveGame() string {
	if err := ui.game.SaveToFile(saveFile); err != nil {
		return fmt.Sprintf("save failed: %v", err)
	}
	return fmt.Sprintf("saved to %s", saveFile)
}

func (ui *UI) loadGame() string {
	if err := ui.game.LoadFromFile(saveFile); err != nil {
		return fmt.Sprintf("load failed: %v", err)
	}
	ui.activeIndustry = clamp(ui.activeIndustry, 0, len(ui.game.Industries)-1)
	ui.selectedWorker = 0
	ui.workerScroll = 0
	return fmt.Sprintf("loaded %s", saveFile)
}

func (ui *UI) drawText(x, y int, text string, style tcell.Style) {
	for i, char := range text {
		ui.screen.SetContent(x+i, y, char, nil, style)
	}
}

func (ui *UI) drawTextCentered(width, y int, text string, style tcell.Style) {
	start := (width - len(text)) / 2
	ui.drawText(start, y, text, style)
}

func truncate(text string, width int) string {
	if width <= 0 {
		return ""
	}
	if len(text) <= width {
		return text
	}
	if width <= 3 {
		return text[:width]
	}
	return text[:width-3] + "..."
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

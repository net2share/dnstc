package handlers

import (
	"fmt"
	"strings"

	"github.com/net2share/dnstc/internal/actions"
	"github.com/net2share/go-corelib/tui"
)

// TUIOutput implements OutputWriter using the tui package.
type TUIOutput struct {
	progressView *tui.ProgressView
}

// NewTUIOutput creates a new TUI output writer.
func NewTUIOutput() *TUIOutput {
	return &TUIOutput{}
}

func (t *TUIOutput) Print(msg string) {
	if t.progressView != nil {
		t.progressView.AddText(msg)
		return
	}
	fmt.Print(msg)
}

func (t *TUIOutput) Printf(format string, args ...interface{}) {
	if t.progressView != nil {
		t.progressView.AddText(fmt.Sprintf(format, args...))
		return
	}
	fmt.Printf(format, args...)
}

func (t *TUIOutput) Println(args ...interface{}) {
	if t.progressView != nil {
		if len(args) == 0 {
			t.progressView.AddText("")
		} else {
			t.progressView.AddText(fmt.Sprint(args...))
		}
		return
	}
	fmt.Println(args...)
}

func (t *TUIOutput) Info(msg string) {
	if t.progressView != nil {
		t.progressView.AddInfo(msg)
		return
	}
	tui.PrintInfo(msg)
}

func (t *TUIOutput) Success(msg string) {
	if t.progressView != nil {
		t.progressView.AddSuccess(msg)
		return
	}
	tui.PrintSuccess(msg)
}

func (t *TUIOutput) Warning(msg string) {
	if t.progressView != nil {
		t.progressView.AddWarning(msg)
		return
	}
	tui.PrintWarning(msg)
}

func (t *TUIOutput) Error(msg string) {
	if t.progressView != nil {
		t.progressView.AddError(msg)
		return
	}
	tui.PrintError(msg)
}

func (t *TUIOutput) Status(msg string) {
	if t.progressView != nil {
		t.progressView.AddStatus(msg)
		return
	}
	tui.PrintStatus(msg)
}

func (t *TUIOutput) Step(current, total int, msg string) {
	if t.progressView != nil {
		t.progressView.AddInfo(fmt.Sprintf("[%d/%d] %s", current, total, msg))
		return
	}
	tui.PrintStep(current, total, msg)
}

func (t *TUIOutput) Box(title string, lines []string) {
	if t.progressView != nil {
		if title != "" {
			t.progressView.AddText(title)
		}
		for _, line := range lines {
			t.progressView.AddText("  " + line)
		}
		return
	}
	tui.PrintBox(title, lines)
}

func (t *TUIOutput) KV(key, value string) string {
	return tui.KV(key+": ", value)
}

func (t *TUIOutput) Table(headers []string, rows [][]string) {
	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = len(h)
	}
	for _, row := range rows {
		for i, cell := range row {
			if i < len(widths) && len(cell) > widths[i] {
				widths[i] = len(cell)
			}
		}
	}

	var formatParts []string
	for _, w := range widths {
		formatParts = append(formatParts, fmt.Sprintf("%%-%ds", w+2))
	}
	format := strings.Join(formatParts, "")

	if t.progressView != nil {
		headerArgs := make([]interface{}, len(headers))
		for i, h := range headers {
			headerArgs[i] = h
		}
		t.progressView.AddText(fmt.Sprintf(format, headerArgs...))
		for _, row := range rows {
			rowArgs := make([]interface{}, len(row))
			for i, cell := range row {
				rowArgs[i] = cell
			}
			t.progressView.AddText(fmt.Sprintf(format, rowArgs...))
		}
		return
	}

	format += "\n"
	headerArgs := make([]interface{}, len(headers))
	for i, h := range headers {
		headerArgs[i] = h
	}
	fmt.Printf(format, headerArgs...)

	total := 0
	for _, w := range widths {
		total += w + 2
	}
	t.Separator(total)

	for _, row := range rows {
		rowArgs := make([]interface{}, len(row))
		for i, cell := range row {
			rowArgs[i] = cell
		}
		fmt.Printf(format, rowArgs...)
	}
}

func (t *TUIOutput) Separator(length int) {
	if t.progressView != nil {
		t.progressView.AddText(strings.Repeat("-", length))
		return
	}
	fmt.Println(strings.Repeat("-", length))
}

func (t *TUIOutput) ShowInfo(cfg actions.InfoConfig) error {
	tuiCfg := tui.InfoConfig{
		Title:       cfg.Title,
		Description: cfg.Description,
	}
	for _, section := range cfg.Sections {
		tuiSection := tui.InfoSection{Title: section.Title}
		for _, row := range section.Rows {
			tuiSection.Rows = append(tuiSection.Rows, tui.InfoRow{
				Key:     row.Key,
				Value:   row.Value,
				Columns: row.Columns,
			})
		}
		tuiCfg.Sections = append(tuiCfg.Sections, tuiSection)
	}
	return tui.ShowInfo(tuiCfg)
}

func (t *TUIOutput) BeginProgress(title string) {
	t.progressView = tui.NewProgressView(title)
}

func (t *TUIOutput) EndProgress() {
	if t.progressView != nil {
		t.progressView.Done()
		t.progressView = nil
	}
}

func (t *TUIOutput) DismissProgress() {
	if t.progressView != nil {
		t.progressView.Dismiss()
		t.progressView = nil
	}
}

func (t *TUIOutput) IsProgressActive() bool {
	return t.progressView != nil
}

var _ actions.OutputWriter = (*TUIOutput)(nil)

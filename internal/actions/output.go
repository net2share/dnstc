package actions

// OutputWriter defines the interface for action output.
// This abstracts the output mechanism so handlers work in both CLI and TUI.
type OutputWriter interface {
	Print(msg string)
	Printf(format string, args ...interface{})
	Println(args ...interface{})

	Info(msg string)
	Success(msg string)
	Warning(msg string)
	Error(msg string)

	Status(msg string)
	Step(current, total int, msg string)

	Box(title string, lines []string)
	KV(key, value string) string

	Table(headers []string, rows [][]string)
	Separator(length int)

	ShowInfo(cfg InfoConfig) error

	BeginProgress(title string)
	EndProgress()
	DismissProgress()
	IsProgressActive() bool
}

// InfoConfig configures an info display.
type InfoConfig struct {
	Title       string
	Description string
	Sections    []InfoSection
}

// InfoSection represents a section in the info view.
type InfoSection struct {
	Title string
	Rows  []InfoRow
}

// InfoRow represents a single row in an info section.
type InfoRow struct {
	Key     string
	Value   string
	Columns []string
}

// Standard symbols for output.
const (
	SymbolSuccess = "✓"
	SymbolError   = "✗"
	SymbolWarning = "⚠"
	SymbolInfo    = "ℹ"
	SymbolRunning = "●"
	SymbolStopped = "○"
	SymbolArrow   = "→"
)

// GetPickerOptions retrieves picker options from context after PickerFunc is called.
func GetPickerOptions(ctx *Context) []SelectOption {
	optionsVal, ok := ctx.Values["_picker_options"]
	if !ok {
		return nil
	}
	options, ok := optionsVal.([]SelectOption)
	if !ok {
		return nil
	}
	return options
}

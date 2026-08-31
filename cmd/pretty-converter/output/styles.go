package output

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

var (
	primaryColor = lipgloss.Color("#7C3AED")
	successColor = lipgloss.Color("#10B981")
	errorColor   = lipgloss.Color("#EF4444")
	warningColor = lipgloss.Color("#F59E0B")
	infoColor    = lipgloss.Color("#3B82F6")
	mutedColor   = lipgloss.Color("#6B7280")
	dimColor     = lipgloss.Color("#9CA3AF")

	SuccessStyle = lipgloss.NewStyle().Foreground(successColor)
	ErrorStyle   = lipgloss.NewStyle().Foreground(errorColor)
	WarningStyle = lipgloss.NewStyle().Foreground(warningColor)
	InfoStyle    = lipgloss.NewStyle().Foreground(infoColor)
	MutedStyle   = lipgloss.NewStyle().Foreground(mutedColor)
	PrimaryStyle = lipgloss.NewStyle().Foreground(primaryColor)
	BoldStyle    = lipgloss.NewStyle().Bold(true)

	SuccessSymbol = SuccessStyle.Render("✓")
	ErrorSymbol   = ErrorStyle.Render("✗")
	WarningSymbol = WarningStyle.Render("⚠")
	InfoSymbol    = InfoStyle.Render("ℹ")
	BulletSymbol  = MutedStyle.Render("•")

	PanelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(primaryColor).
			Padding(1, 2)

	PanelTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(primaryColor)

	StepPendingStyle = lipgloss.NewStyle().Foreground(mutedColor)
	StepRunningStyle = lipgloss.NewStyle().Foreground(infoColor)
	StepDoneStyle    = lipgloss.NewStyle().Foreground(successColor)
	StepErrorStyle   = lipgloss.NewStyle().Foreground(errorColor)

	HeadingStyle    = lipgloss.NewStyle().Bold(true).Foreground(primaryColor)
	SubheadingStyle = lipgloss.NewStyle().Foreground(mutedColor)

	DividerStyle = lipgloss.NewStyle().Foreground(dimColor)

	CodeStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#E5E7EB")).
			Background(lipgloss.Color("#1F2937")).
			Padding(0, 1)

	FilePathStyle = lipgloss.NewStyle().Foreground(infoColor).Italic(true)
	NumberStyle   = lipgloss.NewStyle().Foreground(successColor).Bold(true)
)

func Success(msg string) string    { return SuccessSymbol + " " + msg }
func Error(msg string) string      { return ErrorSymbol + " " + ErrorStyle.Render(msg) }
func Warn(msg string) string       { return WarningSymbol + " " + WarningStyle.Render(msg) }
func Info(msg string) string       { return InfoSymbol + " " + msg }
func Bullet(msg string) string     { return BulletSymbol + " " + msg }
func Heading(msg string) string    { return HeadingStyle.Render(msg) }
func Subheading(msg string) string { return SubheadingStyle.Render(msg) }

func Panel(title, body string) string {
	content := PanelTitleStyle.Render(title) + "\n\n" + body
	return PanelStyle.Render(content)
}

func KeyValue(key, value string) string {
	return MutedStyle.Render(key+":") + " " + value
}

func NoColor() {
	lipgloss.SetColorProfile(termenv.Ascii)
}

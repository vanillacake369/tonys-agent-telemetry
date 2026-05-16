package tui

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/vanillacake369/tonys-agent-telemetry/internal/platform"
	"github.com/vanillacake369/tonys-agent-telemetry/internal/skill"
)

// AnalyzeStep represents the current step in the analysis wizard.
type AnalyzeStep int

const (
	// StepModel is the first step: choose the model to use.
	StepModel AnalyzeStep = iota
	// StepPrompt is the second step: preview and optionally edit the prompt.
	StepPrompt
)

// AnalyzeExecuteMsg is sent when the wizard is ready to execute the analysis.
type AnalyzeExecuteMsg struct {
	Model  string
	Prompt string
}

// defaultAnalyzeTemplate is the prompt template used when no custom prompt is given.
const defaultAnalyzeTemplate = `Analyze this Claude Code skill for my workflow:

Name: %s
URL: %s
Description: %s

README (excerpt):
%s

Please tell me:
1. Key benefits and trade-offs
2. How to integrate into my development workflow
3. Compatibility with my setup (Nix, multi-provider agents, NixOS k8s)
4. Any security or performance concerns`

// analyzeTempFile is the temp file path used to pass the prompt to the shell command.
const analyzeTempFile = "/tmp/tat-analyze-prompt.txt"

// AnalyzeWizard is a multi-step overlay that lets the user choose a model and
// review/edit a prompt before running an AI analysis of a skill.
type AnalyzeWizard struct {
	visible bool
	step    AnalyzeStep
	skill   skill.Skill

	// Step 1: model selection.
	models      []string
	modelCursor int

	// Step 2: prompt editor.
	prompt        textarea.Model
	defaultPrompt string

	width  int
	height int
	keys   KeyMap
}

// NewAnalyzeWizard creates a new AnalyzeWizard with default settings.
func NewAnalyzeWizard() AnalyzeWizard {
	ta := textarea.New()
	ta.ShowLineNumbers = false
	ta.CharLimit = 0 // unlimited
	ta.SetWidth(60)
	ta.SetHeight(12)

	return AnalyzeWizard{
		models: []string{"claude", "codex", "gemini"},
		prompt: ta,
		keys:   DefaultKeyMap(),
	}
}

// Show opens the wizard for the given skill and pre-populates the prompt.
func (w *AnalyzeWizard) Show(s skill.Skill, readme string) {
	w.visible = true
	w.step = StepModel
	w.modelCursor = 0
	w.skill = s

	excerpt := readme
	const maxReadme = 2000
	if len([]rune(excerpt)) > maxReadme {
		excerpt = string([]rune(excerpt)[:maxReadme]) + "\n..."
	}

	w.defaultPrompt = fmt.Sprintf(defaultAnalyzeTemplate,
		s.Name, s.URL, s.Description, excerpt)

	w.prompt.Reset()
	w.prompt.SetValue(w.defaultPrompt)
}

// Update processes key messages for the wizard.
// Returns the updated wizard and any resulting tea.Cmd.
func (w AnalyzeWizard) Update(msg tea.Msg) (AnalyzeWizard, tea.Cmd) {
	if !w.visible {
		return w, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch w.step {
		case StepModel:
			return w.updateModelStep(msg)
		case StepPrompt:
			return w.updatePromptStep(msg)
		}
	}

	// Forward non-key messages to textarea when on prompt step.
	if w.step == StepPrompt {
		var cmd tea.Cmd
		w.prompt, cmd = w.prompt.Update(msg)
		return w, cmd
	}

	return w, nil
}

// updateModelStep handles key events during model selection (Step 1).
func (w AnalyzeWizard) updateModelStep(msg tea.KeyMsg) (AnalyzeWizard, tea.Cmd) {
	switch {
	case key.Matches(msg, w.keys.Escape):
		w.visible = false
		return w, nil

	case key.Matches(msg, w.keys.Up):
		if w.modelCursor > 0 {
			w.modelCursor--
		}
		return w, nil

	case key.Matches(msg, w.keys.Down):
		if w.modelCursor < len(w.models)-1 {
			w.modelCursor++
		}
		return w, nil

	case msg.Type == tea.KeyTab:
		// Tab: use default model (first = "claude") and advance.
		w.modelCursor = 0
		w.step = StepPrompt
		w.prompt.Focus()
		return w, textarea.Blink

	case key.Matches(msg, w.keys.Enter):
		// Enter: confirm selection and advance.
		w.step = StepPrompt
		w.prompt.Focus()
		return w, textarea.Blink
	}

	return w, nil
}

// updatePromptStep handles key events during prompt preview/edit (Step 2).
func (w AnalyzeWizard) updatePromptStep(msg tea.KeyMsg) (AnalyzeWizard, tea.Cmd) {
	switch {
	case key.Matches(msg, w.keys.Escape):
		// Esc: go back to model selection.
		w.step = StepModel
		w.prompt.Blur()
		return w, nil

	case msg.Type == tea.KeyTab:
		// Tab: use the default prompt and execute immediately.
		w.prompt.SetValue(w.defaultPrompt)
		model := w.models[w.modelCursor]
		prompt := w.defaultPrompt
		w.visible = false
		return w, func() tea.Msg {
			return AnalyzeExecuteMsg{Model: model, Prompt: prompt}
		}

	case msg.Type == tea.KeyCtrlS, key.Matches(msg, w.keys.Enter):
		// Enter: execute with the current prompt text.
		model := w.models[w.modelCursor]
		prompt := w.prompt.Value()
		w.visible = false
		return w, func() tea.Msg {
			return AnalyzeExecuteMsg{Model: model, Prompt: prompt}
		}
	}

	// Forward all other keys to the textarea.
	var cmd tea.Cmd
	w.prompt, cmd = w.prompt.Update(msg)
	return w, cmd
}

// View renders the wizard as an overlay string.
// Returns an empty string when the wizard is not visible.
func (w AnalyzeWizard) View() string {
	if !w.visible {
		return ""
	}

	switch w.step {
	case StepModel:
		return w.viewModelStep()
	case StepPrompt:
		return w.viewPromptStep()
	}
	return ""
}

// viewModelStep renders the model selection step.
func (w AnalyzeWizard) viewModelStep() string {
	titleStr := "Analyze: " + w.skill.Name

	keyStyle := lipgloss.NewStyle().Foreground(colorPrimary).Bold(true)
	dimStyle := lipgloss.NewStyle().Foreground(colorDim)
	textStyle := lipgloss.NewStyle().Foreground(colorText)

	var lines []string
	lines = append(lines, textStyle.Render("Select model:"))
	lines = append(lines, "")

	for i, m := range w.models {
		var row string
		if i == w.modelCursor {
			row = keyStyle.Render(" ▸ " + m)
		} else {
			row = dimStyle.Render("   " + m)
		}
		lines = append(lines, row)
	}

	lines = append(lines, "")
	hints := dimStyle.Render("Tab: use default  │  Enter: confirm  │  Esc: cancel")
	lines = append(lines, hints)

	content := strings.Join(lines, "\n")
	innerContent := lipgloss.NewStyle().Padding(1, 3).Render(content)
	panelWidth := lipgloss.Width(innerContent) + 4
	panelHeight := lipgloss.Height(innerContent) + 4

	return RenderPanel(titleStr, innerContent, panelWidth, panelHeight, true)
}

// viewPromptStep renders the prompt preview/edit step.
func (w AnalyzeWizard) viewPromptStep() string {
	titleStr := "Analyze: " + w.skill.Name

	keyStyle := lipgloss.NewStyle().Foreground(colorPrimary).Bold(true)
	dimStyle := lipgloss.NewStyle().Foreground(colorDim)
	textStyle := lipgloss.NewStyle().Foreground(colorText)

	selectedModel := ""
	if w.modelCursor < len(w.models) {
		selectedModel = w.models[w.modelCursor]
	}

	var lines []string
	lines = append(lines, textStyle.Render("Model: ")+keyStyle.Render(selectedModel))
	lines = append(lines, "")
	lines = append(lines, textStyle.Render("Prompt (edit or Tab to use default):"))
	lines = append(lines, w.prompt.View())
	lines = append(lines, "")
	hints := dimStyle.Render("Tab: use default  │  Enter: execute  │  Esc: back")
	lines = append(lines, hints)

	content := strings.Join(lines, "\n")
	innerContent := lipgloss.NewStyle().Padding(1, 2).Render(content)
	panelWidth := lipgloss.Width(innerContent) + 4
	panelHeight := lipgloss.Height(innerContent) + 4

	return RenderPanel(titleStr, innerContent, panelWidth, panelHeight, true)
}

// buildAnalyzeCmd writes the prompt to a temp file and returns the shell command
// string that should be passed to platform.OpenPane.
func buildAnalyzeCmd(model, prompt string) (string, error) {
	if err := os.WriteFile(analyzeTempFile, []byte(prompt), 0600); err != nil {
		return "", fmt.Errorf("analyze: write prompt file: %w", err)
	}

	switch model {
	case "claude":
		return fmt.Sprintf(
			`claude -p "$(cat %s)" && rm -f %s`,
			analyzeTempFile, analyzeTempFile,
		), nil
	case "codex", "gemini":
		// Route via cli-proxy-api.
		return fmt.Sprintf(
			`curl -s http://127.0.0.1:4001/v1/chat/completions `+
				`-H 'Content-Type: application/json' `+
				`-d "{\"model\":\"%s\",\"messages\":[{\"role\":\"user\",\"content\":\"$(cat %s | sed 's/\"/\\\\\"/g')\"}]}" `+
				`&& rm -f %s`,
			model, analyzeTempFile, analyzeTempFile,
		), nil
	}

	// Fallback: treat unknown models as claude.
	return fmt.Sprintf(
		`claude -p "$(cat %s)" && rm -f %s`,
		analyzeTempFile, analyzeTempFile,
	), nil
}

// executeAnalysis writes the prompt to a temp file and opens a new pane.
func executeAnalysis(model, prompt string) error {
	cmd, err := buildAnalyzeCmd(model, prompt)
	if err != nil {
		return err
	}
	return platform.Detect().OpenPane(cmd)
}

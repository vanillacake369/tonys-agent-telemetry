package tui

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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

// AnalyzeFinishedMsg is sent (via tea.ExecProcess callback) when the analysis
// process exits. Err is non-nil on non-zero exit or launch failure.
type AnalyzeFinishedMsg struct{ Err error }

// ErrCodexRemoved is returned for the "codex" model.
// Removed 2026-05-26: no Codex CLI; old cli-proxy-api (localhost:4001) was broken.
var ErrCodexRemoved = errors.New(
	"codex removed (2026-05-26): use 'claude' or 'gemini' instead",
)

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

// noModelsHint is displayed in the model picker when no model CLI is on PATH.
const noModelsHint = "No analysis models found on PATH.\nInstall `claude` or `gemini` CLI."

// lookPath is injected by tests for determinism; production uses exec.LookPath.
var lookPath = exec.LookPath

// modelCLIs is the SSoT table of supported models. codex removed (see ErrCodexRemoved).
var modelCLIs = []struct {
	model  string
	binary string
}{
	{"claude", "claude"},
	{"gemini", "gemini"},
}

// availableAnalyzeModels returns models whose binary is on PATH.
func availableAnalyzeModels() []string {
	var available []string
	for _, m := range modelCLIs {
		if _, err := lookPath(m.binary); err == nil {
			available = append(available, m.model)
		}
	}
	return available
}

// buildAnalyzeCmd returns an *exec.Cmd for tea.ExecProcess.
// Prompt is passed as an argv argument — no shell, no temp-file cat.
// codex: removed 2026-05-26 (ErrCodexRemoved).
func buildAnalyzeCmd(model, prompt string) (*exec.Cmd, error) {
	switch model {
	case "claude":
		return exec.Command("claude", prompt), nil
	case "gemini":
		return exec.Command("gemini", "--prompt", prompt), nil
	case "codex":
		return nil, ErrCodexRemoved
	}
	return nil, fmt.Errorf("analyze: unknown model %q", model)
}

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

// NewAnalyzeWizard creates a new AnalyzeWizard. Models are filtered to those
// whose binary is on PATH via availableAnalyzeModels().
func NewAnalyzeWizard() AnalyzeWizard {
	ta := textarea.New()
	ta.ShowLineNumbers = false
	ta.CharLimit = 0 // unlimited
	ta.SetWidth(60)
	ta.SetHeight(12)

	return AnalyzeWizard{
		models: availableAnalyzeModels(),
		prompt: ta,
		keys:   DefaultKeyMap(),
	}
}

// Show opens the wizard for the given skill and pre-populates the prompt.
// Refreshes the model list so PATH changes between calls are reflected.
func (w *AnalyzeWizard) Show(s skill.Skill, readme string) {
	w.visible = true
	w.step = StepModel
	w.modelCursor = 0
	w.skill = s
	w.models = availableAnalyzeModels()

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
		// Tab: use default model (first available) and advance.
		w.modelCursor = 0
		w.step = StepPrompt
		w.prompt.Focus()
		return w, textarea.Blink

	case key.Matches(msg, w.keys.Enter):
		// Enter: confirm selection and advance (only if models are available).
		if len(w.models) == 0 {
			return w, nil
		}
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
// When no models are available it shows an install hint instead of an empty list.
func (w AnalyzeWizard) viewModelStep() string {
	titleStr := "Analyze: " + w.skill.Name

	keyStyle := lipgloss.NewStyle().Foreground(colorPrimary).Bold(true)
	dimStyle := lipgloss.NewStyle().Foreground(colorDim)
	textStyle := lipgloss.NewStyle().Foreground(colorText)

	var lines []string

	if len(w.models) == 0 {
		// No CLIs on PATH — show install hint.
		lines = append(lines, textStyle.Render(noModelsHint))
		lines = append(lines, "")
		lines = append(lines, dimStyle.Render("Esc: cancel"))
		content := strings.Join(lines, "\n")
		innerContent := lipgloss.NewStyle().Padding(1, 3).Render(content)
		panelWidth := lipgloss.Width(innerContent) + 4
		panelHeight := lipgloss.Height(innerContent) + 4
		return RenderPanel(titleStr, innerContent, panelWidth, panelHeight, true)
	}

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

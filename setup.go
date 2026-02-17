package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"golang.org/x/term"
)

// needsSetup checks if config.json exists using the same search logic as readConfig.
func needsSetup(configPath string) bool {
	if len(configPath) > 0 {
		_, err := os.Stat(configPath)
		return os.IsNotExist(err)
	}

	// Check current directory
	if _, err := os.Stat("config.json"); err == nil {
		return false
	}

	// Check executable's directory
	exePath, err := os.Executable()
	if err == nil {
		altPath := filepath.Join(filepath.Dir(exePath), "config.json")
		if _, err := os.Stat(altPath); err == nil {
			return false
		}
	}

	return true
}

// runSetup runs the first-run configuration wizard.
func runSetup() (*Config, error) {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		fmt.Fprintln(os.Stderr, "No config.json found and no TTY available.")
		fmt.Fprintln(os.Stderr, "Create a config.json manually (see config.json.default) or run interactively.")
		return nil, fmt.Errorf("no TTY for setup wizard")
	}

	m := initialSetupModel()
	p := tea.NewProgram(m)
	result, err := p.Run()
	if err != nil {
		return nil, fmt.Errorf("setup wizard error: %w", err)
	}

	final := result.(setupModel)
	if final.cancelled {
		return nil, fmt.Errorf("setup cancelled")
	}

	return &final.config, nil
}

// --- service definitions ---

type service struct {
	name      string
	binary    string
	install   string
	needsNode bool
}

var availableServices = []service{
	{name: "GitHub Copilot", binary: "copilot-node-server", install: "npm install -g @anthropic-ai/copilot-node-server", needsNode: true},
	{name: "Intelephense (PHP)", binary: "intelephense", install: "npm install -g intelephense", needsNode: true},
	{name: "Gopls (Go)", binary: "gopls", install: "go install golang.org/x/tools/gopls@latest", needsNode: false},
	{name: "Vue Language Server", binary: "vue-language-server", install: "npm install -g @vue/language-server", needsNode: true},
}

// --- bubbletea model ---

type setupStep int

const (
	stepWelcome setupStep = iota
	stepSelectServices
	stepDetect
	stepInstall
	stepSettings
	stepSummary
	stepDone
)

type setupModel struct {
	step     setupStep
	cursor   int
	selected [4]bool   // matches availableServices indices
	detected [4]string // detected paths
	nodePath string
	tsdkPath string

	// settings - text inputs
	portInput    textinput.Model
	licenseInput textinput.Model
	enableLogging bool
	settingsFocus int // 0=port, 1=license, 2=logging

	// install state
	installing    bool
	installIdx    int
	installResult string

	config    Config
	cancelled bool
	err       error
}

func initialSetupModel() setupModel {
	portInput := textinput.New()
	portInput.Placeholder = "8787"
	portInput.SetValue("8787")
	portInput.CharLimit = 5
	portInput.Width = 20

	licenseInput := textinput.New()
	licenseInput.Placeholder = "optional"
	licenseInput.CharLimit = 100
	licenseInput.Width = 40

	return setupModel{
		step:         stepWelcome,
		portInput:    portInput,
		licenseInput: licenseInput,
	}
}

// --- messages ---

type detectDoneMsg struct {
	detected [4]string
	nodePath string
	tsdkPath string
}

type installDoneMsg struct {
	path string
	err  error
}

// --- commands ---

func detectBinaries(selected [4]bool) tea.Cmd {
	return func() tea.Msg {
		var detected [4]string
		for i, svc := range availableServices {
			if selected[i] {
				if p, err := exec.LookPath(svc.binary); err == nil {
					detected[i] = p
				}
			}
		}
		nodePath, _ := exec.LookPath("node")
		tsdkPath := findTsdk()
		return detectDoneMsg{detected: detected, nodePath: nodePath, tsdkPath: tsdkPath}
	}
}

func findTsdk() string {
	candidates := []string{}

	// npm global
	if out, err := exec.Command("npm", "root", "-g").Output(); err == nil {
		candidates = append(candidates, filepath.Join(strings.TrimSpace(string(out)), "typescript", "lib"))
	}

	// yarn global
	home, _ := os.UserHomeDir()
	candidates = append(candidates,
		filepath.Join(home, ".config", "yarn", "global", "node_modules", "typescript", "lib"),
		filepath.Join(home, ".yarn", "global", "node_modules", "typescript", "lib"),
	)

	for _, c := range candidates {
		if _, err := os.Stat(filepath.Join(c, "tsserver.js")); err == nil {
			return c
		}
	}
	return ""
}

func installBinary(svc service) tea.Cmd {
	return func() tea.Msg {
		parts := strings.Fields(svc.install)
		cmd := exec.Command(parts[0], parts[1:]...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err := cmd.Run()
		if err != nil {
			return installDoneMsg{err: err}
		}
		path, _ := exec.LookPath(svc.binary)
		return installDoneMsg{path: path}
	}
}

// --- styles ---

var (
	titleStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
	subtleStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	checkedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	errorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
)

// --- update ---

func (m setupModel) Init() tea.Cmd {
	return nil
}

func (m setupModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Only intercept ctrl+c globally; let steps handle everything else
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		if keyMsg.String() == "ctrl+c" {
			m.cancelled = true
			return m, tea.Quit
		}
		// q quits except during text input steps
		if keyMsg.String() == "q" && m.step != stepSettings {
			m.cancelled = true
			return m, tea.Quit
		}
	}

	switch m.step {
	case stepWelcome:
		return m.updateWelcome(msg)
	case stepSelectServices:
		return m.updateSelectServices(msg)
	case stepDetect:
		return m.updateDetect(msg)
	case stepInstall:
		return m.updateInstall(msg)
	case stepSettings:
		return m.updateSettings(msg)
	case stepSummary:
		return m.updateSummary(msg)
	}

	return m, nil
}

func (m setupModel) updateWelcome(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		if keyMsg.String() == "enter" {
			m.step = stepSelectServices
		}
	}
	return m, nil
}

func (m setupModel) updateSelectServices(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(availableServices)-1 {
				m.cursor++
			}
		case " ":
			m.selected[m.cursor] = !m.selected[m.cursor]
		case "enter":
			m.step = stepDetect
			return m, detectBinaries(m.selected)
		}
	}
	return m, nil
}

func (m setupModel) updateDetect(msg tea.Msg) (tea.Model, tea.Cmd) {
	if dm, ok := msg.(detectDoneMsg); ok {
		m.detected = dm.detected
		m.nodePath = dm.nodePath
		m.tsdkPath = dm.tsdkPath

		hasMissing := false
		for i := range availableServices {
			if m.selected[i] && m.detected[i] == "" {
				hasMissing = true
				break
			}
		}
		needsNode := false
		for i, svc := range availableServices {
			if m.selected[i] && svc.needsNode {
				needsNode = true
				break
			}
		}
		if needsNode && m.nodePath == "" {
			hasMissing = true
		}
		if m.selected[3] && m.tsdkPath == "" {
			hasMissing = true
		}

		if hasMissing {
			m.step = stepInstall
			m.installIdx = -1
		} else {
			m.step = stepSettings
			m.settingsFocus = 0
			m.portInput.Focus()
		}
		return m, nil
	}
	return m, nil
}

func (m setupModel) updateInstall(msg tea.Msg) (tea.Model, tea.Cmd) {
	if dm, ok := msg.(installDoneMsg); ok {
		if dm.err != nil {
			m.installResult = errorStyle.Render(fmt.Sprintf("Failed: %v", dm.err))
		} else {
			if m.installIdx >= 0 && m.installIdx < len(availableServices) {
				m.detected[m.installIdx] = dm.path
			} else if m.installIdx == 100 {
				m.tsdkPath = findTsdk()
			}
		}
		m.installing = false
		return m, nil
	}

	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		if m.installing {
			return m, nil
		}
		switch keyMsg.String() {
		case "enter":
			for i, svc := range availableServices {
				if m.selected[i] && m.detected[i] == "" {
					m.installing = true
					m.installIdx = i
					m.installResult = ""
					return m, installBinary(svc)
				}
			}
			if m.selected[3] && m.tsdkPath == "" {
				m.installing = true
				m.installIdx = 100
				m.installResult = ""
				return m, installBinary(service{
					name: "TypeScript", binary: "tsc",
					install: "npm install -g typescript",
				})
			}
			m.step = stepSettings
			m.settingsFocus = 0
			m.portInput.Focus()
		case "s":
			m.step = stepSettings
			m.settingsFocus = 0
			m.portInput.Focus()
		}
	}
	return m, nil
}

func (m setupModel) updateSettings(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "tab", "enter":
			switch m.settingsFocus {
			case 0: // port field
				m.portInput.Blur()
				if m.selected[1] { // intelephense selected, go to license
					m.settingsFocus = 1
					m.licenseInput.Focus()
				} else {
					m.settingsFocus = 2 // skip to logging toggle
				}
				return m, nil
			case 1: // license field
				m.licenseInput.Blur()
				m.settingsFocus = 2
				return m, nil
			case 2: // logging toggle - enter confirms
				if keyMsg.String() == "enter" {
					m.buildConfig()
					m.step = stepSummary
					return m, nil
				}
			}
			return m, nil
		case "l":
			if m.settingsFocus == 2 {
				m.enableLogging = !m.enableLogging
				return m, nil
			}
		}
	}

	// Delegate to the focused text input
	var cmd tea.Cmd
	switch m.settingsFocus {
	case 0:
		m.portInput, cmd = m.portInput.Update(msg)
	case 1:
		m.licenseInput, cmd = m.licenseInput.Update(msg)
	}
	return m, cmd
}

func (m setupModel) updateSummary(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "enter", "y":
			if err := writeConfig(m.config); err != nil {
				m.err = err
				return m, tea.Quit
			}
			m.step = stepDone
			return m, tea.Quit
		case "n":
			m.cancelled = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m *setupModel) buildConfig() {
	m.config = Config{
		Port:          m.portInput.Value(),
		EnableLogging: m.enableLogging,
		NodePath:      m.nodePath,
		TsdkPath:      m.tsdkPath,
	}

	if m.selected[0] {
		m.config.CopilotPath = m.detected[0]
	}
	if m.selected[1] {
		m.config.IntelephensePath = m.detected[1]
		m.config.IntelephenseLicense = m.licenseInput.Value()
		m.config.IntelephenseStorage = "/tmp/intelephense"
	}
	if m.selected[2] {
		m.config.GoplsPath = m.detected[2]
	}
	if m.selected[3] {
		m.config.VueLSPath = m.detected[3]
	}
}

func writeConfig(cfg Config) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	configPath := "config.json"
	if exePath, err := os.Executable(); err == nil {
		candidate := filepath.Join(filepath.Dir(exePath), "config.json")
		if f, err := os.Create(candidate); err == nil {
			f.Close()
			os.Remove(candidate)
			configPath = candidate
		}
	}

	return os.WriteFile(configPath, data, 0644)
}

// --- view ---

func (m setupModel) View() string {
	switch m.step {
	case stepWelcome:
		return m.viewWelcome()
	case stepSelectServices:
		return m.viewSelectServices()
	case stepDetect:
		return m.viewDetect()
	case stepInstall:
		return m.viewInstall()
	case stepSettings:
		return m.viewSettings()
	case stepSummary:
		return m.viewSummary()
	case stepDone:
		return m.viewDone()
	}
	return ""
}

func (m setupModel) viewWelcome() string {
	s := titleStyle.Render("lsp-client Setup") + "\n\n"
	s += "No config.json found. Let's set up lsp-client.\n\n"
	s += subtleStyle.Render("Press Enter to continue...")
	return s
}

func (m setupModel) viewSelectServices() string {
	s := titleStyle.Render("Select Language Servers") + "\n\n"
	s += "Use j/k to move, Space to toggle, Enter to confirm:\n\n"

	for i, svc := range availableServices {
		cursor := "  "
		if m.cursor == i {
			cursor = "> "
		}
		checked := "[ ]"
		if m.selected[i] {
			checked = checkedStyle.Render("[x]")
		}
		s += fmt.Sprintf("%s%s %s\n", cursor, checked, svc.name)
	}

	s += "\n" + subtleStyle.Render("Enter: confirm selection")
	return s
}

func (m setupModel) viewDetect() string {
	return titleStyle.Render("Detecting dependencies...") + "\n"
}

func (m setupModel) viewInstall() string {
	s := titleStyle.Render("Dependencies") + "\n\n"

	needsNode := false
	for i, svc := range availableServices {
		if m.selected[i] && svc.needsNode {
			needsNode = true
			break
		}
	}
	if needsNode {
		if m.nodePath != "" {
			s += checkedStyle.Render("  node: "+m.nodePath) + "\n"
		} else {
			s += errorStyle.Render("  node: not found (install Node.js first)") + "\n"
		}
	}

	for i, svc := range availableServices {
		if !m.selected[i] {
			continue
		}
		if m.detected[i] != "" {
			s += checkedStyle.Render(fmt.Sprintf("  %s: %s", svc.name, m.detected[i])) + "\n"
		} else {
			s += errorStyle.Render(fmt.Sprintf("  %s: not found", svc.name)) + "\n"
			s += subtleStyle.Render(fmt.Sprintf("    install: %s", svc.install)) + "\n"
		}
	}

	if m.selected[3] {
		if m.tsdkPath != "" {
			s += checkedStyle.Render("  TypeScript SDK: "+m.tsdkPath) + "\n"
		} else {
			s += errorStyle.Render("  TypeScript SDK: not found") + "\n"
			s += subtleStyle.Render("    install: npm install -g typescript") + "\n"
		}
	}

	if m.installing {
		s += "\n  Installing...\n"
	}
	if m.installResult != "" {
		s += "\n  " + m.installResult + "\n"
	}

	s += "\n" + subtleStyle.Render("Enter: install missing  |  s: skip  |  q: quit")
	return s
}

func (m setupModel) viewSettings() string {
	s := titleStyle.Render("Settings") + "\n\n"

	// Port
	portFocus := ""
	if m.settingsFocus == 0 {
		portFocus = " "
	}
	s += fmt.Sprintf("  HTTP Port:%s %s\n", portFocus, m.portInput.View())

	// License (only if intelephense selected)
	if m.selected[1] {
		licenseFocus := ""
		if m.settingsFocus == 1 {
			licenseFocus = " "
		}
		s += fmt.Sprintf("  Intelephense License:%s %s\n", licenseFocus, m.licenseInput.View())
	}

	// Logging toggle
	loggingVal := "no"
	if m.enableLogging {
		loggingVal = "yes"
	}
	loggingMarker := " "
	if m.settingsFocus == 2 {
		loggingMarker = ">"
	}
	s += fmt.Sprintf(" %s Enable Logging: %s\n", loggingMarker, loggingVal)

	s += "\n"
	if m.settingsFocus < 2 {
		s += subtleStyle.Render("Tab/Enter: next field")
	} else {
		s += subtleStyle.Render("l: toggle logging  |  Enter: continue")
	}
	return s
}

func (m setupModel) viewSummary() string {
	s := titleStyle.Render("Configuration Summary") + "\n\n"

	if m.config.CopilotPath != "" {
		s += fmt.Sprintf("  Copilot: %s\n", m.config.CopilotPath)
	}
	if m.config.IntelephensePath != "" {
		s += fmt.Sprintf("  Intelephense: %s\n", m.config.IntelephensePath)
	}
	if m.config.GoplsPath != "" {
		s += fmt.Sprintf("  Gopls: %s\n", m.config.GoplsPath)
	}
	if m.config.VueLSPath != "" {
		s += fmt.Sprintf("  Vue LS: %s\n", m.config.VueLSPath)
	}
	if m.config.NodePath != "" {
		s += fmt.Sprintf("  Node: %s\n", m.config.NodePath)
	}
	if m.config.TsdkPath != "" {
		s += fmt.Sprintf("  TypeScript SDK: %s\n", m.config.TsdkPath)
	}
	s += fmt.Sprintf("  Port: %s\n", m.config.Port)
	s += fmt.Sprintf("  Logging: %v\n", m.config.EnableLogging)

	s += "\n" + subtleStyle.Render("y/Enter: write config.json  |  n: cancel")
	return s
}

func (m setupModel) viewDone() string {
	s := checkedStyle.Render("config.json written successfully!") + "\n"
	s += "Starting lsp-client...\n"
	return s
}

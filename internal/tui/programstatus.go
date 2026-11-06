package tui

import (
	"regexp"

	tea "charm.land/bubbletea/v2"
	"charm.land/log/v2"
)

// gh-dash can identify itself to terminals and multiplexers that implement the
// Program Status Protocol (OSC 7501), such as Rex, which use the reported app
// name for grouping and icons. It reports an idle root record when the config
// is loaded and clears it on exit. It is off unless programStatusApp is set.

var programStatusAppRe = regexp.MustCompile(`^[A-Za-z0-9_.+-]{1,32}$`)

func programStatusSequence(payload string) string {
	return "\x1b]7501;" + payload + "\x1b\\"
}

// reportProgramStatus returns the command that reports gh-dash's app name.
func (m *Model) reportProgramStatus() tea.Cmd {
	app := m.ctx.Config.ProgramStatusApp
	if app == "" {
		return nil
	}
	if !programStatusAppRe.MatchString(app) {
		log.Warn("Ignoring invalid programStatusApp", "app", app)
		return nil
	}
	m.programStatusReported = true
	return tea.Raw(programStatusSequence("state=idle:app=" + app))
}

// ProgramStatusExitSequence is written after the program ends, to remove the
// record reported at startup. It is empty if nothing was reported.
func (m *Model) ProgramStatusExitSequence() string {
	if !m.programStatusReported {
		return ""
	}
	return programStatusSequence("state=clear")
}

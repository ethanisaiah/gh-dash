package tui

import (
	"fmt"
	"io"
	"os"
	"os/exec"

	tea "charm.land/bubbletea/v2"
	"charm.land/log/v2"
	"github.com/cli/go-gh/v2/pkg/browser"

	"github.com/dlvhdr/gh-dash/v4/internal/data"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/constants"
	"github.com/dlvhdr/gh-dash/v4/internal/utils"
)

// Images are never rendered inline. The i key opens one image of the issue
// body with `kitten icat` (kitty's graphics protocol) with the TUI suspended,
// or in the browser when kitten is missing or fails.

type imageDownloadedMsg struct {
	url  string
	path string
	err  error
}

type imageShownMsg struct {
	url  string
	path string
	err  error
}

func (m *Model) openIssueImages() tea.Cmd {
	issue := m.currentIssue()
	if issue == nil {
		return nil
	}
	images := utils.ExtractImageURLs(issue.Body)
	switch len(images) {
	case 0:
		return m.notifyErr(fmt.Sprintf("#%d has no images in its description", issue.Number))
	case 1:
		return m.openImage(images[0].URL)
	default:
		return m.openSidebarForInput(func(bool) tea.Cmd {
			return m.issueSidebar.PickImage(images)
		})
	}
}

func kittenPath() string {
	p, err := exec.LookPath("kitten")
	if err != nil {
		return ""
	}
	return p
}

func (m *Model) openImage(url string) tea.Cmd {
	if kittenPath() == "" {
		return m.openImageInBrowser(url, "kitten not found")
	}
	return tea.Batch(
		m.notify("Downloading image"),
		func() tea.Msg {
			path, err := data.DownloadImage(url)
			return imageDownloadedMsg{url: url, path: path, err: err}
		},
	)
}

func (m *Model) onImageDownloaded(msg imageDownloadedMsg) tea.Cmd {
	if msg.err != nil {
		return m.openImageInBrowser(msg.url, msg.err.Error())
	}
	// --hold keeps the image up until a key is pressed.
	script := `clear; kitten icat --hold "$1"`
	c := exec.Command("sh", "-c", script, "sh", msg.path)
	return tea.ExecProcess(c, func(err error) tea.Msg {
		return imageShownMsg{url: msg.url, path: msg.path, err: err}
	})
}

func (m *Model) onImageShown(msg imageShownMsg) tea.Cmd {
	if err := os.Remove(msg.path); err != nil {
		log.Warn("Failed removing temporary image", "path", msg.path, "err", err)
	}
	if msg.err != nil {
		return m.openImageInBrowser(msg.url, "kitten icat failed")
	}
	return nil
}

func (m *Model) openImageInBrowser(url, reason string) tea.Cmd {
	log.Info("Opening image in browser", "url", url, "reason", reason)
	return tea.Batch(
		m.notify(fmt.Sprintf("Opening image in browser (%s)", reason)),
		func() tea.Msg {
			// Discard launcher output so it cannot corrupt the TUI.
			b := browser.New("", io.Discard, io.Discard)
			if err := b.Browse(url); err != nil {
				return constants.ErrMsg{Err: fmt.Errorf("failed opening image: %w", err)}
			}
			return nil
		},
	)
}

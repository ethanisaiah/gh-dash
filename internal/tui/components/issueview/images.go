package issueview

import (
	"fmt"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/cmpcontroller"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/fuzzyselect"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/constants"
	"github.com/dlvhdr/gh-dash/v4/internal/utils"
)

// ImageChosenMsg is sent when an image of the issue body was picked.
type ImageChosenMsg struct {
	URL string
}

// PickImage lets the user choose one of several images.
func (m *Model) PickImage(images []utils.ImageRef) tea.Cmd {
	if m.issue == nil {
		return nil
	}
	m.images = images
	options := make([]fuzzyselect.Suggestion, 0, len(images))
	for i, img := range images {
		options = append(options, fuzzyselect.Suggestion{
			Value:  fmt.Sprintf("%d. %s", i+1, img.Label()),
			Detail: img.URL,
		})
	}
	return m.enterProjectPicker(cmpcontroller.ModeImage,
		"Open image (enter to pick)"+constants.Ellipsis, options)
}

func (m *Model) submitImage(value string) tea.Cmd {
	images := m.images
	m.images = nil
	num, _, ok := strings.Cut(value, ".")
	if !ok {
		return nil
	}
	i, err := strconv.Atoi(num)
	if err != nil || i < 1 || i > len(images) {
		return nil
	}
	url := images[i-1].URL
	return func() tea.Msg { return ImageChosenMsg{URL: url} }
}

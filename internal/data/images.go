package data

import (
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
	"time"

	"github.com/cli/go-gh/v2/pkg/auth"
)

// maxImageBytes caps image downloads.
const maxImageBytes = 50 << 20

// isGitHubImageHost reports whether the GitHub token may be sent to host. It
// is only sent to github.com itself; Go drops the header on redirects to
// other hosts (e.g. the signed githubusercontent.com URL).
func isGitHubImageHost(host string) bool {
	return host == "github.com"
}

// DownloadImage saves an image to a temporary file and returns its path. The
// caller removes the file. GitHub-hosted attachments of private repos are
// fetched with the gh token.
func DownloadImage(imageUrl string) (string, error) {
	u, err := url.Parse(imageUrl)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequest(http.MethodGet, imageUrl, nil)
	if err != nil {
		return "", err
	}
	if isGitHubImageHost(u.Hostname()) {
		if token, _ := auth.TokenForHost(u.Hostname()); token != "" {
			req.Header.Set("Authorization", "token "+token)
		}
	}
	req.Header.Set("Accept", "image/*")

	client := &http.Client{Timeout: 30 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("downloading image: %s", res.Status)
	}
	contentType := res.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "image/") {
		return "", fmt.Errorf("not an image (%s)", contentType)
	}

	ext := path.Ext(u.Path)
	if exts, _ := mime.ExtensionsByType(strings.Split(contentType, ";")[0]); len(exts) > 0 {
		ext = exts[0]
	}
	f, err := os.CreateTemp("", "gh-dash-image-*"+ext)
	if err != nil {
		return "", err
	}
	defer f.Close()
	n, err := io.Copy(f, io.LimitReader(res.Body, maxImageBytes+1))
	if err == nil && n > maxImageBytes {
		err = fmt.Errorf("image larger than %d MB", maxImageBytes>>20)
	}
	if err != nil {
		os.Remove(f.Name())
		return "", err
	}
	return f.Name(), nil
}

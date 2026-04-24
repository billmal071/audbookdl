package notify

import (
	"fmt"
	"os/exec"
	"runtime"
)

// Send sends a desktop notification.
func Send(title, message string) error {
	switch runtime.GOOS {
	case "linux":
		return exec.Command("notify-send", title, message).Run()
	case "darwin":
		script := fmt.Sprintf(`display notification "%s" with title "%s"`, message, title)
		return exec.Command("osascript", "-e", script).Run()
	default:
		// Silently ignore on unsupported platforms
		return nil
	}
}

// DownloadComplete sends a notification for a completed download.
func DownloadComplete(title, author string) error {
	return Send("Download Complete", fmt.Sprintf("%s by %s", title, author))
}

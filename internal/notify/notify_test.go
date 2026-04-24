package notify

import "testing"

func TestSend_DoesNotPanic(t *testing.T) {
	// Just verify it doesn't panic — actual notification delivery is platform-dependent
	_ = Send("Test", "This is a test notification")
}

func TestDownloadComplete_DoesNotPanic(t *testing.T) {
	_ = DownloadComplete("Test Book", "Test Author")
}

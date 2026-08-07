package util

import (
	"os"
	"runtime"
)

// Hostname returns the system hostname.
func Hostname() string {
	h, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	return h
}

// IsWindows reports whether we're running on Windows.
func IsWindows() bool {
	return runtime.GOOS == "windows"
}

// IsLinux reports whether we're running on Linux (including Android/Termux).
func IsLinux() bool {
	return runtime.GOOS == "linux"
}

// IsAndroid reports whether we're running on Android.
func IsAndroid() bool {
	// Termux sets this env var
	return os.Getenv("TERMUX_VERSION") != "" || os.Getenv("ANDROID_ROOT") != ""
}

// Platform returns a human-readable platform identifier.
func Platform() string {
	osName := runtime.GOOS
	arch := runtime.GOARCH
	if IsAndroid() {
		osName = "android"
	}
	return osName + "/" + arch
}

// GoOS returns the GOOS value.
func GoOS() string {
	return runtime.GOOS
}

// GoArch returns the GOARCH value.
func GoArch() string {
	return runtime.GOARCH
}

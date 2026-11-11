package utils

import (
	"strconv"
	"strings"

	"github.com/AlexanderSlokov/helvilette-wunjo/internal/kamal/localshell"
)

// JoinCommands strips each command and joins them with spaces, as
// Kamal::Utils.join_commands does.
func JoinCommands(commands []string) string {
	parts := make([]string, len(commands))
	for i, command := range commands {
		parts[i] = strings.TrimSpace(command)
	}
	return strings.Join(parts, " ")
}

// DockerArch returns the architecture of the local Docker as amd64 or arm64, as
// Kamal::Utils.docker_arch does. Any other answer comes back unchanged.
//
//	arch, err := utils.DockerArch(localshell.System{})
func DockerArch(sh localshell.Shell) (string, error) {
	result, err := sh.Run("docker info --format '{{.Architecture}}'")
	if err != nil {
		return "", err
	}
	arch := strings.TrimSpace(result.Stdout)
	switch {
	case strings.Contains(arch, "aarch64"):
		return "arm64", nil
	case strings.Contains(arch, "x86_64"):
		return "amd64", nil
	default:
		return arch, nil
	}
}

// OlderVersion reports whether version sorts before other, as
// Gem::Version.new(version) < Gem::Version.new(other) does after removing a leading
// "v". A segment with letters marks a prerelease, which sorts before the release.
//
//	utils.OlderVersion("v0.8.0", "v0.9.2") // true
func OlderVersion(version, other string) bool {
	return compareVersions(versionSegments(version), versionSegments(other)) < 0
}

// versionSegment is one part of a Gem::Version: a number or a prerelease word.
type versionSegment struct {
	number int
	word   string
	isWord bool
}

func versionSegments(version string) []versionSegment {
	version = strings.TrimPrefix(strings.TrimSpace(version), "v")
	var segments []versionSegment
	for _, part := range splitVersion(version) {
		if n, err := strconv.Atoi(part); err == nil {
			segments = append(segments, versionSegment{number: n})
		} else {
			segments = append(segments, versionSegment{word: part, isWord: true})
		}
	}
	return trimTrailingZeros(segments)
}

// splitVersion splits "1.2.0.rc1" into 1, 2, 0, rc, 1, as Gem::Version does.
func splitVersion(version string) []string {
	var parts []string
	for _, dotted := range strings.Split(version, ".") {
		parts = append(parts, splitDigitRuns(dotted)...)
	}
	return parts
}

func splitDigitRuns(text string) []string {
	var parts []string
	start := 0
	for i := 1; i <= len(text); i++ {
		if i == len(text) || isDigit(text[i]) != isDigit(text[start]) {
			parts = append(parts, text[start:i])
			start = i
		}
	}
	return parts
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

// trimTrailingZeros drops zeros at the end of the release part, so 1.0 equals 1.
func trimTrailingZeros(segments []versionSegment) []versionSegment {
	for len(segments) > 0 {
		last := segments[len(segments)-1]
		if last.isWord || last.number != 0 {
			break
		}
		segments = segments[:len(segments)-1]
	}
	return segments
}

func compareVersions(a, b []versionSegment) int {
	for i := 0; i < len(a) || i < len(b); i++ {
		if c := compareSegments(segmentAt(a, i), segmentAt(b, i)); c != 0 {
			return c
		}
	}
	return 0
}

func segmentAt(segments []versionSegment, i int) versionSegment {
	if i < len(segments) {
		return segments[i]
	}
	return versionSegment{}
}

// compareSegments puts a word before any number, so 1.0.rc1 is older than 1.0.0.
func compareSegments(a, b versionSegment) int {
	switch {
	case a.isWord && b.isWord:
		return strings.Compare(a.word, b.word)
	case a.isWord:
		return -1
	case b.isWord:
		return 1
	default:
		return a.number - b.number
	}
}

// Contains reports whether list holds item.
func Contains(list []string, item string) bool {
	for _, s := range list {
		if s == item {
			return true
		}
	}
	return false
}

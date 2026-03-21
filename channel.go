package main

import (
	"fmt"
	"net/http"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

var (
	versionRe   = regexp.MustCompile(`^(\d+\.\d+)`)
	probeClient = &http.Client{Timeout: 3 * time.Second}
)

func probeChannel(ch string) bool {
	url := fmt.Sprintf("https://search.nixos.org/backend/latest-%s-nixos-%s", esSchema, ch)
	req, err := http.NewRequest("HEAD", url, nil)
	if err != nil {
		return false
	}
	req.SetBasicAuth(esUser, esPass)
	resp, err := probeClient.Do(req)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == 200
}

func probeStableChannel() string {
	out, err := exec.Command("nixos-version").Output()
	if err != nil {
		return ""
	}
	m := versionRe.FindString(strings.TrimSpace(string(out)))
	if m == "" {
		return ""
	}
	// Parse YY.MM
	var year, month int
	fmt.Sscanf(m, "%d.%d", &year, &month)

	// Step backwards through biannual releases (YY.11, YY.05, ...)
	for i := 0; i < 4; i++ {
		if month == 11 {
			month = 5
		} else {
			month = 11
			year--
		}
		ch := fmt.Sprintf("%d.%02d", year, month)
		if probeChannel(ch) {
			return ch
		}
	}
	return ""
}

func detectAltChannel(active string) string {
	if active == "unstable" {
		return probeStableChannel()
	}
	return "unstable"
}

func detectChannel() string {
	out, err := exec.Command("nixos-version").Output()
	if err != nil {
		return "unstable"
	}

	version := strings.TrimSpace(string(out))

	if strings.Contains(version, "pre") || strings.Contains(version, "unstable") {
		return "unstable"
	}

	m := versionRe.FindString(version)
	if m == "" {
		return "unstable"
	}

	if probeChannel(m) {
		return m
	}
	return "unstable"
}

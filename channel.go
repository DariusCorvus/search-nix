package main

import (
	"fmt"
	"net/http"
	"os/exec"
	"regexp"
	"strings"
)

var versionRe = regexp.MustCompile(`^(\d+\.\d+)`)

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

	// Probe whether the ES index exists
	url := fmt.Sprintf("https://search.nixos.org/backend/latest-%s-nixos-%s", esSchema, m)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "unstable"
	}
	req.SetBasicAuth(esUser, esPass)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "unstable"
	}
	resp.Body.Close()

	if resp.StatusCode == 200 {
		return m
	}
	return "unstable"
}

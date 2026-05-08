package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	esUser = "aWVSALXpZv"
	esPass = "X8gPHnzL52wFEekuxsfQ9cSh"
)

// esSchema is the Elasticsearch index schema version. NixOS bumps this
// periodically with no notice; the value here is the latest known at build
// time, but it is overridden at runtime by the on-disk cache and by the
// retry-on-404 probe in searchFrom.
var esSchema = "48"

type ESResponse struct {
	Hits struct {
		Total struct {
			Value int `json:"value"`
		} `json:"total"`
		Hits []ESHit `json:"hits"`
	} `json:"hits"`
	Error *json.RawMessage `json:"error,omitempty"`
}

type ESHit struct {
	Source SearchResult `json:"_source"`
}

type SearchResult struct {
	PackageAttrName    string      `json:"package_attr_name"`
	PackageVersion     string      `json:"package_pversion"`
	PackageDescription string      `json:"package_description"`
	PackagePrograms    []string    `json:"package_programs"`
	PackageHomepage    any         `json:"package_homepage"`
	PackageLicenseSet  []License   `json:"package_license_set"`
	PackagePname           string `json:"package_pname"`
	PackageLongDescription string `json:"package_longDescription"`
}

// License can be either a string or an object with spdxId/fullName fields.
type License struct {
	Raw      string // if it's a plain string
	SpdxID   string
	FullName string
}

func (l *License) UnmarshalJSON(data []byte) error {
	// Try string first
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		l.Raw = s
		return nil
	}
	// Otherwise it's an object
	var obj struct {
		SpdxID   string `json:"spdxId"`
		FullName string `json:"fullName"`
	}
	if err := json.Unmarshal(data, &obj); err != nil {
		return err
	}
	l.SpdxID = obj.SpdxID
	l.FullName = obj.FullName
	return nil
}

func search(query string, channel string, size int) (*ESResponse, time.Duration, error) {
	return searchFrom(query, channel, size, 0)
}

func searchFrom(query string, channel string, size int, from int) (*ESResponse, time.Duration, error) {
	return searchAttempt(query, channel, size, from, false)
}

func searchAttempt(query string, channel string, size int, from int, retried bool) (*ESResponse, time.Duration, error) {
	url := fmt.Sprintf("https://search.nixos.org/backend/latest-%s-nixos-%s/_search", esSchema, channel)

	payload := buildQuery(query, size, from)

	start := time.Now()

	req, err := http.NewRequest("POST", url, strings.NewReader(payload))
	if err != nil {
		return nil, 0, err
	}
	req.SetBasicAuth(esUser, esPass)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	elapsed := time.Since(start)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, 0, err
	}

	var esResp ESResponse
	if err := json.Unmarshal(body, &esResp); err != nil {
		return nil, 0, fmt.Errorf("failed to parse response: %w", err)
	}

	if esResp.Error != nil {
		var errObj struct {
			Reason string `json:"reason"`
		}
		reason := ""
		if json.Unmarshal(*esResp.Error, &errObj) == nil {
			reason = errObj.Reason
		}

		if !retried && strings.Contains(reason, "no such index") {
			if newSchema, ok := resolveSchema(channel); ok {
				esSchema = newSchema
				saveCachedSchema(newSchema)
				return searchAttempt(query, channel, size, from, true)
			}
		}

		if reason != "" {
			return nil, 0, fmt.Errorf("%s", reason)
		}
		return nil, 0, fmt.Errorf("elasticsearch error: %s", string(*esResp.Error))
	}

	return &esResp, elapsed, nil
}

// resolveSchema HEAD-probes nearby schema versions to find a working index for
// the given channel. It scans cur+5 down to cur+1 first (returning the highest
// live schema above the current value), then cur-1 down to cur-5 as a fallback
// for the rare case where the server rolls back. Returns the resolved schema
// string and true on success.
func resolveSchema(channel string) (string, bool) {
	cur, err := strconv.Atoi(esSchema)
	if err != nil {
		return "", false
	}
	for k := 5; k >= 1; k-- {
		cand := strconv.Itoa(cur + k)
		if probeSchema(cand, channel) {
			return cand, true
		}
	}
	for k := 1; k <= 5; k++ {
		n := cur - k
		if n < 1 {
			break
		}
		cand := strconv.Itoa(n)
		if probeSchema(cand, channel) {
			return cand, true
		}
	}
	return "", false
}

func probeSchema(schema, channel string) bool {
	url := fmt.Sprintf("https://search.nixos.org/backend/latest-%s-nixos-%s", schema, channel)
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

func cacheSchemaPath() string {
	base := os.Getenv("XDG_CACHE_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		base = filepath.Join(home, ".cache")
	}
	return filepath.Join(base, "search-nix", "schema")
}

func loadCachedSchema() {
	p := cacheSchemaPath()
	if p == "" {
		return
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return
	}
	s := strings.TrimSpace(string(data))
	if _, err := strconv.Atoi(s); err != nil {
		return
	}
	esSchema = s
}

func saveCachedSchema(schema string) {
	p := cacheSchemaPath()
	if p == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return
	}
	_ = os.WriteFile(p, []byte(schema+"\n"), 0o644)
}

func buildQuery(query string, size int, from int) string {
	q := map[string]interface{}{
		"from": from,
		"size": size,
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"filter": []map[string]interface{}{
					{"term": map[string]interface{}{"type": map[string]interface{}{"value": "package"}}},
				},
				"must": []map[string]interface{}{
					{
						"dis_max": map[string]interface{}{
							"tie_breaker": 0.7,
							"queries": []map[string]interface{}{
								{
									"multi_match": map[string]interface{}{
										"type":     "cross_fields",
										"query":    query,
										"analyzer": "whitespace",
										"operator": "and",
										"fields": []string{
											"package_attr_name^9",
											"package_attr_name.*^5.4",
											"package_pname^6",
											"package_pname.*^3.6",
											"package_description^1.3",
											"package_longDescription^1",
										},
									},
								},
								{
									"multi_match": map[string]interface{}{
										"type":      "best_fields",
										"query":     query,
										"analyzer":  "whitespace",
										"operator":  "and",
										"fields":    []string{"package_programs^7.5"},
										"fuzziness": 1,
									},
								},
							},
						},
					},
				},
			},
		},
	}

	data, _ := json.Marshal(q)
	return string(data)
}

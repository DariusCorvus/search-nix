package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	esUser   = "aWVSALXpZv"
	esPass   = "X8gPHnzL52wFEekuxsfQ9cSh"
	esSchema = "44"
)

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
		// Try to extract reason from error
		var errObj struct {
			Reason string `json:"reason"`
		}
		if json.Unmarshal(*esResp.Error, &errObj) == nil && errObj.Reason != "" {
			return nil, 0, fmt.Errorf("%s", errObj.Reason)
		}
		return nil, 0, fmt.Errorf("elasticsearch error: %s", string(*esResp.Error))
	}

	return &esResp, elapsed, nil
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

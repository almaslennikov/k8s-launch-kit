// Copyright 2025 NVIDIA CORPORATION & AFFILIATES
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// SPDX-License-Identifier: Apache-2.0

package networkoperatorplugin

import (
	"bytes"
	_ "embed"
	"fmt"
	htmltemplate "html/template"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/template"

	"github.com/nvidia/k8s-launch-kit/pkg/config"
	"github.com/nvidia/k8s-launch-kit/pkg/profiles"
)

// templateFuncs provides helper functions for Go templates
var templateFuncs = template.FuncMap{
	"add": func(a, b int) int { return a + b },
	"sub": func(a, b int) int { return a - b },
	"gt":  func(a, b int) bool { return a > b },
	"untilStep": func(start, stop, step int) []int {
		result := []int{}
		for i := start; i < stop; i += step {
			result = append(result, i)
		}
		return result
	},
	"replaceVars": func(template string, nicID, plane, rail int) string {
		// Replace template variables with actual values
		result := template
		result = strings.ReplaceAll(result, "%nic_id%", fmt.Sprintf("%d", nicID))
		result = strings.ReplaceAll(result, "%plane%", fmt.Sprintf("%d", plane))
		result = strings.ReplaceAll(result, "%rail%", fmt.Sprintf("%d", rail))
		return result
	},
}

// ProcessTemplate processes a Go template file with the given config
func ProcessTemplate(templatePath string, config *config.LaunchKubernetesConfig) (string, error) {
	// Read the template file
	templateContent, err := os.ReadFile(templatePath)
	if err != nil {
		return "", fmt.Errorf("failed to read template file %s: %w", templatePath, err)
	}

	// Parse the template with helper functions
	tmpl, err := template.New(filepath.Base(templatePath)).Funcs(templateFuncs).Parse(string(templateContent))
	if err != nil {
		return "", fmt.Errorf("failed to parse template %s: %w", templatePath, err)
	}

	// Execute the template
	var buf bytes.Buffer
	err = tmpl.Execute(&buf, config)
	if err != nil {
		return "", fmt.Errorf("failed to execute template %s: %w", templatePath, err)
	}

	return buf.String(), nil
}

// ProcessProfileTemplates processes all template files in a profile directory
func (p *NetworkOperatorPlugin) GenerateProfileDeploymentFiles(profile *profiles.Profile, cfg *config.LaunchKubernetesConfig) (map[string]string, error) {
	results := make(map[string]string)

	for _, templatePath := range profile.Templates {
		processed, err := ProcessTemplate(templatePath, cfg)
		if err != nil {
			return nil, fmt.Errorf("failed to process template %s: %w", templatePath, err)
		}

		results[filepath.Base(templatePath)] = processed
	}

	// Generate HTML overview
	operatorVersion := ""
	docsBaseURL := ""
	if cfg.NetworkOperator != nil {
		operatorVersion = cfg.NetworkOperator.Version
		docsBaseURL = cfg.NetworkOperator.DocsBaseURL
	}
	htmlContent, err := GenerateOverviewHTML(profile, operatorVersion, docsBaseURL, results)
	if err != nil {
		return nil, fmt.Errorf("failed to generate overview HTML: %w", err)
	}
	results["overview.html"] = htmlContent

	return results, nil
}

// buildDocURL constructs the full documentation URL from the base URL and guide path.
func buildDocURL(baseURL, guidePath string) string {
	if baseURL == "" || guidePath == "" {
		return ""
	}
	return strings.TrimRight(baseURL, "/") + "/" + guidePath
}

// overviewFile represents a generated file with its content.
type overviewFile struct {
	Name    string
	Content string
}

// overviewData holds the data for the HTML overview template.
type overviewData struct {
	ProfileName     string
	Description     string
	Notes           string
	Files           []overviewFile
	DocURL          string
	OperatorVersion string
}

// GenerateOverviewHTML renders an HTML overview page for the generated profile.
func GenerateOverviewHTML(profile *profiles.Profile, operatorVersion, docsBaseURL string, renderedFiles map[string]string) (string, error) {
	filenames := make([]string, 0, len(renderedFiles))
	for filename := range renderedFiles {
		if filename != "overview.html" {
			filenames = append(filenames, filename)
		}
	}
	sort.Strings(filenames)

	files := make([]overviewFile, 0, len(filenames))
	for _, name := range filenames {
		files = append(files, overviewFile{Name: name, Content: renderedFiles[name]})
	}

	data := overviewData{
		ProfileName:     profile.Name,
		Description:     strings.TrimSpace(profile.Description),
		Notes:           strings.TrimSpace(profile.Notes),
		Files:           files,
		DocURL:          buildDocURL(docsBaseURL, profile.DeploymentGuide),
		OperatorVersion: operatorVersion,
	}

	tmpl, err := htmltemplate.New("overview").Parse(overviewHTMLTemplate)
	if err != nil {
		return "", fmt.Errorf("failed to parse overview template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute overview template: %w", err)
	}

	return buf.String(), nil
}

//go:embed overview.html
var overviewHTMLTemplate string

// Package sarif provides SARIF 2.1.0 output formatting for Keel.
// SARIF is a standard JSON format for static analysis results.
package sarif

import (
	"encoding/json"
)

const (
	SchemaURL = "https://json.schemastore.org/sarif-2.1.0.json"
	Version   = "2.1.0"
)

// Report is the top-level SARIF document.
type Report struct {
	Schema  string `json:"$schema"`
	Version string `json:"version"`
	Runs    []Run  `json:"runs"`
}

// Run represents a single invocation of a tool.
type Run struct {
	Tool    Tool     `json:"tool"`
	Results []Result `json:"results"`
}

// Tool describes the analysis tool.
type Tool struct {
	Driver Driver `json:"driver"`
}

// Driver contains tool metadata.
type Driver struct {
	Name            string `json:"name"`
	Version         string `json:"version,omitempty"`
	SemanticVersion string `json:"semanticVersion,omitempty"`
	Rules           []Rule `json:"rules,omitempty"`
}

// Rule defines a static analysis rule.
type Rule struct {
	ID                   string                  `json:"id"`
	ShortDescription     MessageString           `json:"shortDescription"`
	FullDescription      MessageString           `json:"fullDescription,omitempty"`
	DefaultConfiguration *ReportingConfiguration `json:"defaultConfiguration,omitempty"`
}

// ReportingConfiguration controls the default severity level for a rule.
type ReportingConfiguration struct {
	Level string `json:"level"`
}

// MessageString is a message with optional markdown.
type MessageString struct {
	Text string `json:"text"`
}

// Result represents a single analysis result.
type Result struct {
	RuleID    string     `json:"ruleId"`
	Message   Message    `json:"message"`
	Level     string     `json:"level"`
	Locations []Location `json:"locations,omitempty"`
}

// Message contains the result message.
type Message struct {
	Text string `json:"text"`
}

// Location describes where a result was found.
type Location struct {
	PhysicalLocation PhysicalLocation `json:"physicalLocation"`
}

// PhysicalLocation contains file and region information.
type PhysicalLocation struct {
	ArtifactLocation ArtifactLocation `json:"artifactLocation"`
}

// ArtifactLocation identifies a file.
type ArtifactLocation struct {
	URI string `json:"uri"`
}

// NewReport creates a SARIF report with the standard schema and version.
func NewReport() *Report {
	return &Report{
		Schema:  SchemaURL,
		Version: Version,
		Runs:    []Run{},
	}
}

// NewRun creates a new run with the given tool name and version.
func NewRun(toolName, toolVersion string) *Run {
	return &Run{
		Tool: Tool{
			Driver: Driver{
				Name:            toolName,
				SemanticVersion: toolVersion,
				Rules:           []Rule{},
			},
		},
		Results: []Result{},
	}
}

// AddRule adds a rule definition to the run.
func (r *Run) AddRule(id, shortDesc, fullDesc string, level string) {
	rule := Rule{
		ID: id,
		ShortDescription: MessageString{
			Text: shortDesc,
		},
		FullDescription: MessageString{
			Text: fullDesc,
		},
	}
	if level != "" {
		rule.DefaultConfiguration = &ReportingConfiguration{Level: level}
	}
	r.Tool.Driver.Rules = append(r.Tool.Driver.Rules, rule)
}

// AddResult adds a result to the run.
func (r *Run) AddResult(ruleID, message, level, filePath string) {
	result := Result{
		RuleID: ruleID,
		Message: Message{
			Text: message,
		},
		Level: level,
	}

	if filePath != "" {
		result.Locations = []Location{
			{
				PhysicalLocation: PhysicalLocation{
					ArtifactLocation: ArtifactLocation{
						URI: filePath,
					},
				},
			},
		}
	}

	r.Results = append(r.Results, result)
}

// ToJSON serializes the report to indented JSON.
func (r *Report) ToJSON() ([]byte, error) {
	return json.MarshalIndent(r, "", "  ")
}

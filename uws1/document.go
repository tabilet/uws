package uws1

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// ExecutionOptions carries executor-owned behavior that is intentionally not
// serialized into the UWS wire format.
type ExecutionOptions struct {
	// AwaitTimeout bounds await polling inside the core orchestrator. Zero means
	// no internal timeout; await still terminates when the context is canceled.
	AwaitTimeout time.Duration `json:"-" yaml:"-" hcl:"-"`
}

// Document is the root object of a UWS 1.x document.
type Document struct {
	UWS                string               `json:"uws" yaml:"uws" hcl:"uws"`
	Info               *Info                `json:"info" yaml:"info" hcl:"info,block"`
	SourceDescriptions []*SourceDescription `json:"sourceDescriptions,omitempty" yaml:"sourceDescriptions,omitempty" hcl:"sourceDescription,block"`
	// Variables is an intentionally open-shape map; any JSON-compatible value is
	// allowed. The JSON Schema enforces object shape; UWS does not restrict keys
	// or values further.
	Variables  map[string]any      `json:"variables,omitempty" yaml:"variables,omitempty" hcl:"variables,optional"`
	Operations []*Operation        `json:"operations" yaml:"operations" hcl:"operation,block"`
	Workflows  []*Workflow         `json:"workflows,omitempty" yaml:"workflows,omitempty" hcl:"workflow,block"`
	Triggers   []*Trigger          `json:"triggers,omitempty" yaml:"triggers,omitempty" hcl:"trigger,block"`
	Results    []*StructuralResult `json:"results,omitempty" yaml:"results,omitempty" hcl:"result,block"`
	Components *Components         `json:"components,omitempty" yaml:"components,omitempty" hcl:"components,block"`
	Extensions map[string]any      `json:"-" yaml:"-" hcl:"-"`

	// Runtime is the specialized executor bound to this document.
	Runtime Runtime `json:"-" yaml:"-" hcl:"-"`
	// ExecutionOptions are executor-owned knobs and are not part of the UWS wire
	// contract.
	ExecutionOptions ExecutionOptions `json:"-" yaml:"-" hcl:"-"`

	lastExecutionRecords map[string]ExecutionRecord
}

// SetRuntime binds a specialized runtime to the document.
func (d *Document) SetRuntime(r Runtime) {
	d.Runtime = r
}

// Execute executes the document using the bound runtime.
func (d *Document) Execute(ctx context.Context) error {
	if d.Runtime == nil {
		return fmt.Errorf("uws1: document execution requires a bound runtime")
	}
	if err := d.Validate(); err != nil {
		return err
	}
	if err := d.ValidateExecutable(); err != nil {
		return err
	}
	if err := d.ValidateExecutionEntrypoint(); err != nil {
		return err
	}
	orch := NewOrchestrator(d, d.Runtime)
	return orch.Execute(ctx)
}

// DispatchTrigger routes one trigger event into the document's executable
// targets using the bound runtime.
func (d *Document) DispatchTrigger(ctx context.Context, triggerID string, output int, payload any) error {
	if d.Runtime == nil {
		return fmt.Errorf("uws1: trigger dispatch requires a bound runtime")
	}
	if err := d.Validate(); err != nil {
		return err
	}
	if err := d.ValidateExecutable(); err != nil {
		return err
	}
	orch := NewOrchestrator(d, d.Runtime)
	err := orch.ExecuteTrigger(ctx, triggerID, output, payload)
	d.setExecutionRecords(orch.snapshotRecords())
	return err
}

type documentAlias Document

var documentKnownFields = []string{
	"uws", "info", "sourceDescriptions", "variables",
	"operations", "workflows", "triggers", "results",
	"components",
}

func (d *Document) UnmarshalJSON(data []byte) error {
	var alias documentAlias
	if err := json.Unmarshal(data, &alias); err != nil {
		return fmt.Errorf("unmarshaling document: %w", err)
	}
	*d = Document(alias)

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("unmarshaling document extensions: %w", err)
	}
	if err := rejectUnknownFields(raw, documentKnownFields, "document"); err != nil {
		return err
	}
	extensions, err := extractExtensions(raw, documentKnownFields)
	if err != nil {
		return fmt.Errorf("unmarshaling document extensions: %w", err)
	}
	d.Extensions = extensions
	return nil
}

func (d Document) MarshalJSON() ([]byte, error) {
	alias := documentAlias(d)
	return marshalWithExtensions(&alias, d.Extensions)
}

// Info provides metadata about the UWS document.
type Info struct {
	Title       string         `json:"title" yaml:"title" hcl:"title"`
	Summary     string         `json:"summary,omitempty" yaml:"summary,omitempty" hcl:"summary,optional"`
	Description string         `json:"description,omitempty" yaml:"description,omitempty" hcl:"description,optional"`
	Version     string         `json:"version" yaml:"version" hcl:"version"`
	Extensions  map[string]any `json:"-" yaml:"-" hcl:"-"`
}

type infoAlias Info

var infoKnownFields = []string{
	"title", "summary", "description", "version",
}

func (i *Info) UnmarshalJSON(data []byte) error {
	var alias infoAlias
	if err := json.Unmarshal(data, &alias); err != nil {
		return fmt.Errorf("unmarshaling info: %w", err)
	}
	*i = Info(alias)

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("unmarshaling info extensions: %w", err)
	}
	if err := rejectUnknownFields(raw, infoKnownFields, "info"); err != nil {
		return err
	}
	extensions, err := extractExtensions(raw, infoKnownFields)
	if err != nil {
		return fmt.Errorf("unmarshaling info extensions: %w", err)
	}
	i.Extensions = extensions
	return nil
}

func (i Info) MarshalJSON() ([]byte, error) {
	alias := infoAlias(i)
	return marshalWithExtensions(&alias, i.Extensions)
}

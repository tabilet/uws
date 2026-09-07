// Package browserregistration defines the portable UWS browser account-
// registration profile and operation-extension wire types.
//
// The package contains inert metadata only. Account identifiers, passwords,
// verification values, cookies, browser storage, page content, and live
// session handles remain runtime-private and must never be placed in these
// structures.
package browserregistration

import (
	"bytes"
	"encoding/json"
	"fmt"

	"gopkg.in/yaml.v3"
)

const (
	// ProfileName is the portable browser account-registration recipe profile.
	ProfileName = "uws.browser-registration.1.0"
	// CallProfileName identifies extension-owned UWS registration operations.
	CallProfileName = "uws.browser-registration-call.1.0"
	// ProfileNameV11 adds typed private inputs and explicit input checkpoints.
	ProfileNameV11 = "uws.browser-registration.1.1"
	// CallProfileNameV11 selects the private input binding supplement.
	CallProfileNameV11 = "uws.browser-registration-call.1.1"
	// ExtensionRegistration is the operation-level registration-call key.
	ExtensionRegistration = "x-uws-browser-registration"
)

// Profile is one portable, secret-free browser account-registration recipe.
type Profile struct {
	Profile         string                    `json:"profile" yaml:"profile"`
	Info            Info                      `json:"info" yaml:"info"`
	ObservationKind string                    `json:"observationKind" yaml:"observationKind"`
	Evidence        Evidence                  `json:"evidence" yaml:"evidence"`
	Confidence      string                    `json:"confidence" yaml:"confidence"`
	ExpiresAfter    string                    `json:"expiresAfter" yaml:"expiresAfter"`
	Verification    Verification              `json:"verification" yaml:"verification"`
	CredentialSlots map[string]CredentialSlot `json:"credentialSlots" yaml:"credentialSlots"`
	Flows           map[string]Flow           `json:"flows" yaml:"flows"`
	InputSlots      map[string]InputSlot      `json:"inputSlots,omitempty" yaml:"inputSlots,omitempty"`
	Discovery       *Discovery                `json:"discovery,omitempty" yaml:"discovery,omitempty"`
}

// Info identifies the exact application and registration origins covered by
// a profile. Origins are exact origins, not URL-prefix allowlists.
type Info struct {
	Title               string   `json:"title" yaml:"title"`
	Provider            string   `json:"provider,omitempty" yaml:"provider,omitempty"`
	ApplicationOrigins  []string `json:"applicationOrigins" yaml:"applicationOrigins"`
	RegistrationOrigins []string `json:"registrationOrigins" yaml:"registrationOrigins"`
}

// Evidence records when and from what reviewed observation the recipe was
// learned. Source is a non-secret provenance label, not captured content.
type Evidence struct {
	LearnedAt string `json:"learnedAt" yaml:"learnedAt"`
	Source    string `json:"source,omitempty" yaml:"source,omitempty"`
}

// Verification records the latest successful review of the inert recipe. It
// does not assert that an account was created.
type Verification struct {
	LastVerifiedAt   string   `json:"lastVerifiedAt" yaml:"lastVerifiedAt"`
	UIStabilityScore *float64 `json:"uiStabilityScore,omitempty" yaml:"uiStabilityScore,omitempty"`
}

// CredentialSlot declares the class of a symbolic runtime-resolved value. It
// never carries an account identifier, password, or verification value.
type CredentialSlot struct {
	Kind string `json:"kind" yaml:"kind"`
}

// Locator is an accessibility-tree locator. CSS, XPath, coordinates, and
// arbitrary scripts are intentionally absent.
type Locator struct {
	Role  string `json:"role" yaml:"role"`
	Name  string `json:"name,omitempty" yaml:"name,omitempty"`
	Text  string `json:"text,omitempty" yaml:"text,omitempty"`
	Value string `json:"value,omitempty" yaml:"value,omitempty"`
}

// Flow is one explicitly selected account-registration alternative.
type Flow struct {
	Description        string             `json:"description,omitempty" yaml:"description,omitempty"`
	Sequence           []Step             `json:"sequence" yaml:"sequence"`
	Effects            []string           `json:"effects" yaml:"effects"`
	ConfirmationPolicy ConfirmationPolicy `json:"confirmationPolicy" yaml:"confirmationPolicy"`
	Success            SuccessCondition   `json:"success" yaml:"success"`
}

// ConfirmationPolicy marks account creation as an explicitly approved
// mutation. Required is fixed to true by the public schema.
type ConfirmationPolicy struct {
	Required bool   `json:"required" yaml:"required"`
	Prompt   string `json:"prompt,omitempty" yaml:"prompt,omitempty"`
}

// Step is a closed declarative registration macro union. Exactly one field is
// set. Submit is the sole state-changing arm and may occur exactly once.
type Step struct {
	Navigate        string               `json:"navigate,omitempty" yaml:"navigate,omitempty"`
	TypeCredential  *TypeCredentialStep  `json:"type_credential,omitempty" yaml:"type_credential,omitempty"`
	Click           *ClickStep           `json:"click,omitempty" yaml:"click,omitempty"`
	Submit          *SubmitStep          `json:"submit,omitempty" yaml:"submit,omitempty"`
	HumanCheckpoint *HumanCheckpointStep `json:"human_checkpoint,omitempty" yaml:"human_checkpoint,omitempty"`
	WaitFor         *WaitForCondition    `json:"wait_for,omitempty" yaml:"wait_for,omitempty"`
	InputCheckpoint *InputCheckpointStep `json:"input_checkpoint,omitempty" yaml:"input_checkpoint,omitempty"`
	FillInput       *FillInputStep       `json:"fill_input,omitempty" yaml:"fill_input,omitempty"`
}

// TypeCredentialStep fills a reviewed locator from a symbolic slot.
type TypeCredentialStep struct {
	Locator Locator `json:"locator" yaml:"locator"`
	Slot    string  `json:"slot" yaml:"slot"`
}

// ClickStep activates a reviewed non-submission accessibility locator.
type ClickStep struct {
	Locator Locator `json:"locator" yaml:"locator"`
}

// SubmitStep activates the one reviewed account-creation control. A runtime
// must obtain the call's exact approval immediately before this step.
type SubmitStep struct {
	Locator Locator `json:"locator" yaml:"locator"`
}

// HumanCheckpointStep pauses for ordinary human handling of a control. It
// carries no verification response or credential value.
type HumanCheckpointStep struct {
	Kind    string   `json:"kind" yaml:"kind"`
	Locator *Locator `json:"locator,omitempty" yaml:"locator,omitempty"`
}

// WaitForCondition waits for one reviewed locator.
type WaitForCondition struct {
	Locator Locator `json:"locator" yaml:"locator"`
}

// SuccessCondition proves only the reviewed registration outcome.
type SuccessCondition struct {
	Origin  string  `json:"origin" yaml:"origin"`
	Locator Locator `json:"locator" yaml:"locator"`
	Path    string  `json:"path,omitempty" yaml:"path,omitempty"`
}

func (s Step) validateUnion() error {
	count := 0
	for _, present := range []bool{
		s.Navigate != "", s.TypeCredential != nil, s.Click != nil,
		s.Submit != nil, s.HumanCheckpoint != nil, s.WaitFor != nil,
		s.InputCheckpoint != nil, s.FillInput != nil,
	} {
		if present {
			count++
		}
	}
	if count != 1 {
		return fmt.Errorf("browserregistration: step must set exactly one action arm")
	}
	return nil
}

// MarshalJSON rejects invalid in-memory step unions.
func (s Step) MarshalJSON() ([]byte, error) {
	if err := s.validateUnion(); err != nil {
		return nil, err
	}
	type wire Step
	return json.Marshal(wire(s))
}

// UnmarshalJSON rejects invalid step unions before populating the receiver.
func (s *Step) UnmarshalJSON(data []byte) error {
	type wire Step
	var value wire
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	next := Step(value)
	if err := next.validateUnion(); err != nil {
		return err
	}
	*s = next
	return nil
}

// MarshalYAML rejects invalid in-memory step unions.
func (s Step) MarshalYAML() (any, error) {
	if err := s.validateUnion(); err != nil {
		return nil, err
	}
	type wire Step
	return wire(s), nil
}

// UnmarshalYAML rejects invalid step unions before populating the receiver.
func (s *Step) UnmarshalYAML(node *yaml.Node) error {
	type wire Step
	var value wire
	if err := node.Decode(&value); err != nil {
		return err
	}
	next := Step(value)
	if err := next.validateUnion(); err != nil {
		return err
	}
	*s = next
	return nil
}

// OperationRegistration is the typed x-uws-browser-registration payload.
type OperationRegistration struct {
	Profile             string            `json:"profile" hcl:"profile"`
	Flow                string            `json:"flow" hcl:"flow"`
	CredentialBindings  map[string]string `json:"credentialBindings" hcl:"credentialBindings"`
	Approval            string            `json:"approval" hcl:"approval"`
	DuplicatePrevention string            `json:"duplicatePrevention" hcl:"duplicatePrevention"`
	OnDuplicate         string            `json:"onDuplicate" hcl:"onDuplicate"`
	AmbiguousOutcome    string            `json:"ambiguousOutcome" hcl:"ambiguousOutcome"`
	CleanupDisposition  string            `json:"cleanupDisposition" hcl:"cleanupDisposition"`
	InputBinding        string            `json:"inputBinding,omitempty" hcl:"inputBinding"`
}

// ReadRegistrationExtension decodes x-uws-browser-registration.
func ReadRegistrationExtension(extensions map[string]any) (*OperationRegistration, bool, error) {
	var out OperationRegistration
	if len(extensions) == 0 {
		return &out, false, nil
	}
	value, ok := extensions[ExtensionRegistration]
	if !ok {
		return &out, false, nil
	}
	data, err := json.Marshal(value)
	if err != nil {
		return &out, false, fmt.Errorf("marshal %s extension: %w", ExtensionRegistration, err)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&out); err != nil {
		return &out, false, fmt.Errorf("unmarshal %s extension: %w", ExtensionRegistration, err)
	}
	return &out, true, nil
}

// SetRegistrationExtension encodes x-uws-browser-registration.
func SetRegistrationExtension(dst *map[string]any, value *OperationRegistration) error {
	if value == nil {
		return nil
	}
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	var generic any
	if err := json.Unmarshal(data, &generic); err != nil {
		return err
	}
	if *dst == nil {
		*dst = make(map[string]any)
	}
	(*dst)[ExtensionRegistration] = generic
	return nil
}

package schemas

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"unicode/utf8"
)

const registrationInputVersion = "uws.browser-registration-input.1.0"

// PathForBrowserRegistrationInput returns the best local private-envelope
// schema path. The schema is portable; filled instances are always private.
func PathForBrowserRegistrationInput(anchorDir, version string) string {
	return pathForSchemaName(anchorDir, familySchemaName(version, "browser-registration-input", "1.0"))
}

// BrowserRegistrationInputSchema returns independent bytes of the selected
// private-envelope schema. Empty version selects 1.0.
func BrowserRegistrationInputSchema(version string) ([]byte, error) {
	data, err := embeddedSchemaDocument(familySchemaName(version, "browser-registration-input", "1.0"))
	if err != nil {
		return nil, fmt.Errorf("load browser registration input schema: %w", err)
	}
	return append([]byte(nil), data...), nil
}

func validateRegistrationInputsProfile(root map[string]any, origins map[string]struct{}) error {
	fields := root["inputSlots"].(map[string]any)
	credentials := root["credentialSlots"].(map[string]any)
	if len(fields)+len(credentials) > 64 {
		return fmt.Errorf("registration: combined input and credential slot limit is 64")
	}
	for name, raw := range fields {
		if _, exists := credentials[name]; exists {
			return fmt.Errorf("inputSlots.%s: credential slot collision", name)
		}
		field := raw.(map[string]any)
		kind := field["type"].(string)
		for _, key := range []string{"minLength", "maxLength"} {
			if _, present := field[key]; present && kind != "string" {
				return fmt.Errorf("inputSlots.%s: length constraint requires string", name)
			}
		}
		for _, key := range []string{"minimum", "maximum"} {
			if value, present := field[key]; present && (kind != "number" && kind != "integer" || !registrationScalar("number", value)) {
				return fmt.Errorf("inputSlots.%s: invalid numeric constraint", name)
			}
		}
		for _, pair := range [][2]string{{"minLength", "maxLength"}, {"minimum", "maximum"}} {
			lo, lok := field[pair[0]].(float64)
			hi, hik := field[pair[1]].(float64)
			if lok && hik && lo > hi {
				return fmt.Errorf("inputSlots.%s: inverted bounds", name)
			}
		}
		if values, ok := field["enum"].([]any); ok {
			for _, value := range values {
				if !registrationFieldValue(field, value, false) {
					return fmt.Errorf("inputSlots.%s: enum does not satisfy field constraints", name)
				}
			}
		}
		if condition, ok := field["requiredWhen"].(map[string]any); ok {
			parent, exists := fields[condition["slot"].(string)].(map[string]any)
			if !exists || condition["slot"] == name || parent["required"] != true {
				return fmt.Errorf("inputSlots.%s: condition requires an unconditional required input", name)
			}
			choices, _ := parent["enum"].([]any)
			if !registrationContains(choices, condition["equals"]) {
				return fmt.Errorf("inputSlots.%s: condition must select a declared public choice", name)
			}
		}
	}
	if discovery, ok := root["discovery"].(map[string]any); ok {
		for _, entry := range discovery["entryPoints"].([]any) {
			if err := validateAuthenticationTarget(entry.(string), origins); err != nil {
				return fmt.Errorf("discovery.entryPoints: %w", err)
			}
		}
	}
	for name, raw := range root["flows"].(map[string]any) {
		flow := raw.(map[string]any)
		sequence := flow["sequence"].([]any)
		if _, ok := sequence[0].(map[string]any)["input_checkpoint"]; !ok {
			return fmt.Errorf("flows.%s: input readiness must precede browser activity", name)
		}
		loaded := map[string]bool{}
		checkpointIDs := map[string]bool{}
		used := map[string]bool{}
		pending := map[string]bool{}
		submitted := false
		for _, item := range sequence {
			step := item.(map[string]any)
			if cp, ok := step["input_checkpoint"].(map[string]any); ok {
				id := cp["id"].(string)
				if checkpointIDs[id] || submitted {
					return fmt.Errorf("flows.%s: repeated checkpoint or input revision after submit", name)
				}
				if len(pending) != 0 {
					return fmt.Errorf("flows.%s: checkpoint inputs must be applied before advancing", name)
				}
				checkpointIDs[id] = true
				for _, rawSlot := range cp["slots"].([]any) {
					slot := rawSlot.(string)
					if fields[slot] == nil && credentials[slot] == nil {
						return fmt.Errorf("flows.%s: undeclared checkpoint slot", name)
					}
					loaded[slot] = true
					pending[slot] = true
				}
				for slot := range loaded {
					field, _ := fields[slot].(map[string]any)
					if condition, ok := field["requiredWhen"].(map[string]any); ok && pending[condition["slot"].(string)] && !pending[slot] {
						return fmt.Errorf("flows.%s: changing a condition requires reapplying its dependent input", name)
					}
				}
				for _, rawSlot := range cp["slots"].([]any) {
					field, _ := fields[rawSlot.(string)].(map[string]any)
					if condition, ok := field["requiredWhen"].(map[string]any); ok && !loaded[condition["slot"].(string)] {
						return fmt.Errorf("flows.%s: condition input must be loaded before its dependent field", name)
					}
				}
			}
			for _, action := range []string{"fill_input", "type_credential"} {
				fill, ok := step[action].(map[string]any)
				if !ok {
					continue
				}
				slot := fill["slot"].(string)
				if !loaded[slot] || submitted {
					return fmt.Errorf("flows.%s: input must follow its checkpoint and precede submit", name)
				}
				used[slot] = true
				delete(pending, slot)
				if action == "fill_input" {
					field, ok := fields[slot].(map[string]any)
					if !ok || !registrationControl(field, fill["control"].(string)) {
						return fmt.Errorf("flows.%s: undeclared or incompatible form input", name)
					}
				}
			}
			if _, ok := step["submit"]; ok {
				if len(pending) != 0 {
					return fmt.Errorf("flows.%s: checkpoint inputs must be applied before submit", name)
				}
				submitted = true
			}
		}
		if len(checkpointIDs) == 0 {
			return fmt.Errorf("flows.%s: registration 1.1 requires an input checkpoint", name)
		}
		for slot := range loaded {
			if !used[slot] {
				return fmt.Errorf("flows.%s: checkpoint slot has no form control", name)
			}
		}
	}
	return nil
}

func registrationControl(field map[string]any, control string) bool {
	switch control {
	case "fill":
		return field["type"] == "string" || field["type"] == "integer" || field["type"] == "number"
	case "check":
		return field["type"] == "boolean"
	case "select":
		choices, _ := field["enum"].([]any)
		return field["type"] == "string" && len(choices) > 0
	}
	return false
}

func registrationScalar(kind string, value any) bool {
	switch kind {
	case "string":
		text, ok := value.(string)
		return ok && utf8.RuneCountInString(text) <= 4096
	case "boolean":
		_, ok := value.(bool)
		return ok
	case "number", "integer":
		number, ok := value.(float64)
		return ok && !math.IsNaN(number) && !math.IsInf(number, 0) && math.Abs(number) <= 9007199254740991 && (kind == "number" || math.Trunc(number) == number)
	}
	return false
}

func registrationFieldValue(field map[string]any, value any, checkEnum bool) bool {
	if !registrationScalar(field["type"].(string), value) {
		return false
	}
	if checkEnum {
		if choices, ok := field["enum"].([]any); ok && !registrationContains(choices, value) {
			return false
		}
	}
	if text, ok := value.(string); ok {
		length := float64(utf8.RuneCountInString(text))
		if minimum, ok := field["minLength"].(float64); ok && length < minimum {
			return false
		}
		if maximum, ok := field["maxLength"].(float64); ok && length > maximum {
			return false
		}
	}
	if number, ok := value.(float64); ok {
		if minimum, ok := field["minimum"].(float64); ok && number < minimum {
			return false
		}
		if maximum, ok := field["maximum"].(float64); ok && number > maximum {
			return false
		}
	}
	return true
}

func registrationContains(values []any, candidate any) bool {
	for _, value := range values {
		if reflect.DeepEqual(value, candidate) {
			return true
		}
	}
	return false
}

func registrationInputRoot(profile []byte, flow string) (map[string]any, map[string]any, error) {
	if err := ValidateBrowserRegistrationProfile(profile); err != nil {
		return nil, nil, fmt.Errorf("registration input: invalid profile")
	}
	value, _, _ := decodeSchemaDocument(profile, "registration")
	root := value.(map[string]any)
	selected, ok := root["flows"].(map[string]any)[flow].(map[string]any)
	if root["profile"] != "uws.browser-registration.1.1" || !ok {
		return nil, nil, fmt.Errorf("registration input: unsupported profile or flow")
	}
	return root, selected, nil
}

func registrationFlowSlots(flow map[string]any) map[string]bool {
	slots := map[string]bool{}
	for _, raw := range flow["sequence"].([]any) {
		step := raw.(map[string]any)
		if cp, ok := step["input_checkpoint"].(map[string]any); ok {
			for _, slot := range cp["slots"].([]any) {
				slots[slot.(string)] = true
			}
		}
	}
	return slots
}

// BrowserRegistrationInputTemplate creates an unfilled private JSON draft for
// one registration type. Null means unfilled; it never means consent or false.
// Callers own private file permissions and must never package filled instances.
func BrowserRegistrationInputTemplate(profile []byte, flow string) ([]byte, error) {
	_, selected, err := registrationInputRoot(profile, flow)
	if err != nil {
		return nil, err
	}
	values := map[string]any{}
	for slot := range registrationFlowSlots(selected) {
		values[slot] = nil
	}
	return json.MarshalIndent(map[string]any{
		"version": registrationInputVersion, "profileSha256": fmt.Sprintf("%x", sha256.Sum256(profile)),
		"registrationType": flow, "revision": 1, "values": values,
	}, "", "  ")
}

// ValidateBrowserRegistrationCallBinding checks a 1.1 call against the actual
// selected profile, including flow existence and exact credential-slot keys.
// Resolving inputBinding and binding private values remain runtime operations.
func ValidateBrowserRegistrationCallBinding(profile, call []byte) error {
	if err := ValidateBrowserRegistrationCallSupplementForProfile(call, "uws.browser-registration-call.1.1"); err != nil {
		return err
	}
	value, _, _ := decodeSchemaDocument(call, "registration call")
	op := value.(map[string]any)["x-uws-browser-registration"].(map[string]any)
	_, flow, err := registrationInputRoot(profile, op["flow"].(string))
	if err != nil {
		return err
	}
	used := map[string]bool{}
	for _, raw := range flow["sequence"].([]any) {
		if step, ok := raw.(map[string]any)["type_credential"].(map[string]any); ok {
			used[step["slot"].(string)] = true
		}
	}
	bindings := op["credentialBindings"].(map[string]any)
	if len(used) != len(bindings) {
		return fmt.Errorf("registration call: credential bindings do not match selected flow")
	}
	for slot := range used {
		if _, exists := bindings[slot]; !exists {
			return fmt.Errorf("registration call: credential bindings do not match selected flow")
		}
	}
	return nil
}

func decodeRegistrationPrivateInput(profile []byte, root, selected map[string]any, data []byte, flow string) (map[string]any, error) {
	bad := fmt.Errorf("registration input: invalid private document")
	if len(data) > maxBrowserRegistrationProfileBytes || !utf8.Valid(data) || !json.Valid(data) {
		return nil, bad
	}
	value, document, err := decodeSchemaDocument(data, "private input")
	if err != nil {
		return nil, bad // Decoder diagnostics can contain private values or keys.
	}
	registrationInputSchemaOnce.Do(func() {
		registrationInputSchema, registrationInputSchemaErr = compileEmbeddedSchema("browser-registration-input.1.0.json")
	})
	if registrationInputSchemaErr != nil || registrationInputSchema.Validate(document) != nil {
		return nil, bad
	}
	input := value.(map[string]any)
	if input["profileSha256"] != fmt.Sprintf("%x", sha256.Sum256(profile)) || input["registrationType"] != flow {
		return nil, fmt.Errorf("registration input: profile or flow binding mismatch")
	}
	fields := root["inputSlots"].(map[string]any)
	allowed := registrationFlowSlots(selected)
	for slot, value := range input["values"].(map[string]any) {
		if !allowed[slot] {
			return nil, bad
		}
		if value == nil {
			continue
		}
		if field, ok := fields[slot].(map[string]any); ok {
			if !registrationFieldValue(field, value, true) {
				return nil, bad
			}
		} else if text, ok := value.(string); !ok || text == "" || !registrationScalar("string", value) {
			return nil, bad
		}
	}
	return input, nil
}

// ValidateBrowserRegistrationInputUpdate checks a private snapshot before one
// declared checkpoint. Previous is nil only at the first checkpoint. Otherwise
// supply the last accepted snapshot, never another unaccepted draft. Errors
// intentionally omit input values, keys and nested parser/schema diagnostics.
// The runtime separately enforces checkpoint order, activation, expiry, atomic
// reads, identity/attempt history, immutable submission and private storage.
func ValidateBrowserRegistrationInputUpdate(profile, current, previous []byte, flow, checkpoint string) error {
	root, selected, err := registrationInputRoot(profile, flow)
	if err != nil {
		return err
	}
	input, err := decodeRegistrationPrivateInput(profile, root, selected, current, flow)
	if err != nil {
		return err
	}
	var cp map[string]any
	first := true
	for _, raw := range selected["sequence"].([]any) {
		if candidate, ok := raw.(map[string]any)["input_checkpoint"].(map[string]any); ok {
			if candidate["id"] == checkpoint {
				cp = candidate
				break
			}
			first = false
		}
	}
	if cp == nil || previous == nil && !first {
		return fmt.Errorf("registration input: checkpoint requires prior accepted state")
	}
	values := input["values"].(map[string]any)
	fields := root["inputSlots"].(map[string]any)
	credentials := root["credentialSlots"].(map[string]any)
	editable := map[string]bool{}
	for _, raw := range cp["slots"].([]any) {
		slot := raw.(string)
		editable[slot] = true
		required := true
		if field, ok := fields[slot].(map[string]any); ok {
			if condition, ok := field["requiredWhen"].(map[string]any); ok {
				parent := values[condition["slot"].(string)]
				if parent == nil {
					return fmt.Errorf("registration input: condition input is missing")
				}
				required = reflect.DeepEqual(parent, condition["equals"])
			} else {
				required = field["required"].(bool)
			}
		}
		if required && values[slot] == nil {
			return fmt.Errorf("registration input: required checkpoint input is missing")
		}
	}
	if previous != nil {
		old, err := decodeRegistrationPrivateInput(profile, root, selected, previous, flow)
		if err != nil {
			return err
		}
		if input["revision"].(float64) <= old["revision"].(float64) {
			return fmt.Errorf("registration input: revision must increase")
		}
		before := old["values"].(map[string]any)
		for slot := range registrationFlowSlots(selected) {
			if (!editable[slot] || credentials[slot] != nil && before[slot] != nil) && !reflect.DeepEqual(values[slot], before[slot]) {
				return fmt.Errorf("registration input: change outside checkpoint or bound identity")
			}
		}
	}
	return nil
}

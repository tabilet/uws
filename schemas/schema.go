// Package schemas locates and validates the versioned schema documents kept in
// the repository's document-only versions directory.
package schemas

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"golang.org/x/mod/module"
	"gopkg.in/yaml.v3"
)

const uwsModulePath = "github.com/OpenUdon/uws"

//go:generate go run ../internal/generateversionarchive -source ../versions -output version-documents.zip

// embeddedVersionDocuments preserves schema lookup for installed binaries
// without putting Go source in the document-only versions directory.
//
//go:embed version-documents.zip
var embeddedVersionDocuments []byte

var (
	browser15SchemaOnce          sync.Once
	browser15Schema              *jsonschema.Schema
	browser15SchemaErr           error
	browser16SchemaOnce          sync.Once
	browser16Schema              *jsonschema.Schema
	browser16SchemaErr           error
	browser17SchemaOnce          sync.Once
	browser17Schema              *jsonschema.Schema
	browser17SchemaErr           error
	auth10SchemaOnce             sync.Once
	auth10Schema                 *jsonschema.Schema
	auth10SchemaErr              error
	auth11SchemaOnce             sync.Once
	auth11Schema                 *jsonschema.Schema
	auth11SchemaErr              error
	authCall10SchemaOnce         sync.Once
	authCall10Schema             *jsonschema.Schema
	authCall10SchemaErr          error
	authCall11SchemaOnce         sync.Once
	authCall11Schema             *jsonschema.Schema
	authCall11SchemaErr          error
	registration10SchemaOnce     sync.Once
	registration10Schema         *jsonschema.Schema
	registration10SchemaErr      error
	registrationCall10SchemaOnce sync.Once
	registrationCall10Schema     *jsonschema.Schema
	registrationCall10SchemaErr  error
	registration11SchemaOnce     sync.Once
	registration11Schema         *jsonschema.Schema
	registration11SchemaErr      error
	registrationCall11SchemaOnce sync.Once
	registrationCall11Schema     *jsonschema.Schema
	registrationCall11SchemaErr  error
	registrationInputSchemaOnce  sync.Once
	registrationInputSchema      *jsonschema.Schema
	registrationInputSchemaErr   error
)

const maxBrowserAuthenticationProfileBytes = 1 << 20
const maxBrowserRegistrationProfileBytes = 1 << 20

// PathForVersion returns the best local schema path for a UWS document version.
func PathForVersion(anchorDir, version string) string {
	version = strings.TrimSpace(version)
	if version == "" {
		version = "1.0.0"
	}
	return pathForSchemaName(anchorDir, version+".json")
}

// PathForRuntimeSupplement returns the best local schema path for a runtime supplement profile.
func PathForRuntimeSupplement(anchorDir, profile string) string {
	return pathForSchemaName(anchorDir, runtimeSupplementSchemaName(profile))
}

// PathForBrowserSourceProfile returns the best local schema path for a browser
// source profile.
func PathForBrowserSourceProfile(anchorDir, profile string) string {
	return pathForSchemaName(anchorDir, familySchemaName(profile, "browser", "1.7"))
}

// BrowserSourceProfileSchema returns an independent copy of the embedded
// browser-profile JSON Schema selected by profile. An empty profile selects
// the current uws.browser.1.7 contract.
func BrowserSourceProfileSchema(profile string) ([]byte, error) {
	name := familySchemaName(profile, "browser", "1.7")
	data, err := embeddedSchemaDocument(name)
	if err != nil {
		return nil, fmt.Errorf("load browser source profile schema %q: %w", profile, err)
	}
	return append([]byte(nil), data...), nil
}

// PathForBrowserAuthenticationProfile returns the best local schema path for
// a portable browser-authentication recipe.
func PathForBrowserAuthenticationProfile(anchorDir, profile string) string {
	return pathForSchemaName(anchorDir, familySchemaName(profile, "browser-authentication", "1.1"))
}

// BrowserAuthenticationProfileSchema returns an independent copy of the
// embedded browser-authentication JSON Schema selected by profile. An empty
// profile selects uws.browser-authentication.1.1.
func BrowserAuthenticationProfileSchema(profile string) ([]byte, error) {
	name := familySchemaName(profile, "browser-authentication", "1.1")
	data, err := embeddedSchemaDocument(name)
	if err != nil {
		return nil, fmt.Errorf("load browser authentication profile schema %q: %w", profile, err)
	}
	return append([]byte(nil), data...), nil
}

// PathForBrowserAuthenticationCallSupplement returns the best local schema
// path for the browser-authentication-call operation supplement.
func PathForBrowserAuthenticationCallSupplement(anchorDir, profile string) string {
	return pathForSchemaName(anchorDir, familySchemaName(profile, "browser-authentication-call", "1.1"))
}

// BrowserAuthenticationCallSupplementSchema returns an independent copy of
// the embedded browser-authentication-call JSON Schema selected by profile.
// An empty profile selects uws.browser-authentication-call.1.1.
func BrowserAuthenticationCallSupplementSchema(profile string) ([]byte, error) {
	name := familySchemaName(profile, "browser-authentication-call", "1.1")
	data, err := embeddedSchemaDocument(name)
	if err != nil {
		return nil, fmt.Errorf("load browser authentication call schema %q: %w", profile, err)
	}
	return append([]byte(nil), data...), nil
}

// PathForBrowserRegistrationProfile returns the best local schema path for a
// portable browser account-registration recipe.
func PathForBrowserRegistrationProfile(anchorDir, profile string) string {
	return pathForSchemaName(anchorDir, familySchemaName(profile, "browser-registration", "1.0"))
}

// BrowserRegistrationProfileSchema returns an independent copy of the
// embedded browser-registration JSON Schema. An empty profile selects
// uws.browser-registration.1.0.
func BrowserRegistrationProfileSchema(profile string) ([]byte, error) {
	name := familySchemaName(profile, "browser-registration", "1.0")
	data, err := embeddedSchemaDocument(name)
	if err != nil {
		return nil, fmt.Errorf("load browser registration profile schema %q: %w", profile, err)
	}
	return append([]byte(nil), data...), nil
}

// PathForBrowserRegistrationCallSupplement returns the best local schema path
// for the browser-registration-call operation supplement.
func PathForBrowserRegistrationCallSupplement(anchorDir, profile string) string {
	return pathForSchemaName(anchorDir, familySchemaName(profile, "browser-registration-call", "1.0"))
}

// BrowserRegistrationCallSupplementSchema returns an independent copy of the
// embedded browser-registration-call JSON Schema. An empty profile selects
// uws.browser-registration-call.1.0.
func BrowserRegistrationCallSupplementSchema(profile string) ([]byte, error) {
	name := familySchemaName(profile, "browser-registration-call", "1.0")
	data, err := embeddedSchemaDocument(name)
	if err != nil {
		return nil, fmt.Errorf("load browser registration call schema %q: %w", profile, err)
	}
	return append([]byte(nil), data...), nil
}

// ValidateBrowserSourceProfile validates one JSON or YAML browser-profile
// document against the embedded schema selected by its profile discriminator.
// Freshness, review evidence, registry lifecycle, sessions, and execution
// policy remain downstream responsibilities.
func ValidateBrowserSourceProfile(data []byte) error {
	value, document, err := decodeSchemaDocument(data, "browser source profile")
	if err != nil {
		return err
	}
	profile, err := profileDiscriminator(value, "browser source profile")
	if err != nil {
		return err
	}
	schema, err := compiledBrowserSourceProfileSchema(profile)
	if err != nil {
		return err
	}
	if err := schema.Validate(document); err != nil {
		return fmt.Errorf("validate browser source profile: %w", err)
	}
	if profile == "uws.browser.1.6" || profile == "uws.browser.1.7" {
		root, _ := value.(map[string]any)
		if err := validateBrowserContexts(root, browserProfileOrigins(root)); err != nil {
			return err
		}
	}
	return nil
}

// ValidateBrowserAuthenticationProfile validates one portable, secret-free
// browser authentication recipe. In addition to JSON Schema validation it
// enforces exact safe origins and the 1 MiB document bound.
func ValidateBrowserAuthenticationProfile(data []byte) error {
	if len(data) > maxBrowserAuthenticationProfileBytes {
		return fmt.Errorf("browser authentication profile exceeds %d bytes", maxBrowserAuthenticationProfileBytes)
	}
	value, document, err := decodeSchemaDocument(data, "browser authentication profile")
	if err != nil {
		return err
	}
	profile, err := profileDiscriminator(value, "browser authentication profile")
	if err != nil {
		return err
	}
	schema, err := compiledBrowserAuthenticationProfileSchema(profile)
	if err != nil {
		return err
	}
	if err := schema.Validate(document); err != nil {
		return fmt.Errorf("validate browser authentication profile: %w", err)
	}
	root, ok := value.(map[string]any)
	if !ok {
		return fmt.Errorf("browser authentication profile must be an object")
	}
	info, _ := root["info"].(map[string]any)
	declaredOrigins := make(map[string]struct{})
	originCount := 0
	for _, field := range []string{"applicationOrigins", "authenticationOrigins"} {
		origins, _ := info[field].([]any)
		originCount += len(origins)
		for i, raw := range origins {
			origin, _ := raw.(string)
			if err := validateAuthenticationOrigin(origin); err != nil {
				return fmt.Errorf("info.%s[%d]: %w", field, i, err)
			}
			declaredOrigins[canonicalAuthenticationOrigin(origin)] = struct{}{}
		}
	}
	if originCount > 32 {
		return fmt.Errorf("info origins: combined applicationOrigins and authenticationOrigins exceed 32")
	}
	if profile == "uws.browser-authentication.1.1" {
		if err := validateBrowserContexts(root, declaredOrigins); err != nil {
			return err
		}
	}
	credentialSlots, _ := root["credentialSlots"].(map[string]any)
	flows, _ := root["flows"].(map[string]any)
	for name, raw := range flows {
		flow, _ := raw.(map[string]any)
		effects, _ := flow["effects"].([]any)
		hasMFAEffect := containsString(effects, "sends_mfa_challenge")
		hasChallenge := false
		sequence, _ := flow["sequence"].([]any)
		for i, rawStep := range sequence {
			step, _ := rawStep.(map[string]any)
			if rawNavigate, ok := step["navigate"]; ok {
				navigate, contextID := browserNavigate(rawNavigate)
				if err := validateAuthenticationTarget(navigate, declaredOrigins); err != nil {
					return fmt.Errorf("flows.%s.sequence[%d].navigate: %w", name, i, err)
				}
				if err := validateContextTarget(root, contextID, navigate); err != nil {
					return fmt.Errorf("flows.%s.sequence[%d].navigate: %w", name, i, err)
				}
			}
			if rawType, ok := step["type_credential"]; ok {
				typeStep, _ := rawType.(map[string]any)
				slot, _ := typeStep["slot"].(string)
				rawSlot, ok := credentialSlots[slot]
				if !ok {
					return fmt.Errorf("flows.%s.sequence[%d].type_credential.slot: undeclared credential slot %q", name, i, slot)
				}
				slotDef, _ := rawSlot.(map[string]any)
				if kind, _ := slotDef["kind"].(string); kind == "totp_seed" {
					return fmt.Errorf("flows.%s.sequence[%d].type_credential.slot: totp_seed slots may be used only by a totp challenge", name, i)
				}
			}
			if rawChallenge, ok := step["challenge"]; ok {
				hasChallenge = true
				challenge, _ := rawChallenge.(map[string]any)
				if slot, _ := challenge["slot"].(string); slot != "" {
					rawSlot, ok := credentialSlots[slot]
					if !ok {
						return fmt.Errorf("flows.%s.sequence[%d].challenge.slot: undeclared credential slot %q", name, i, slot)
					}
					slotDef, _ := rawSlot.(map[string]any)
					if kind, _ := slotDef["kind"].(string); kind != "totp_seed" {
						return fmt.Errorf("flows.%s.sequence[%d].challenge.slot: TOTP requires a totp_seed slot", name, i)
					}
				}
			}
		}
		if hasChallenge != hasMFAEffect {
			return fmt.Errorf("flows.%s.effects: sends_mfa_challenge must be present exactly when the flow has a challenge step", name)
		}
		success, _ := flow["success"].(map[string]any)
		origin, _ := success["origin"].(string)
		if err := validateAuthenticationOrigin(origin); err != nil {
			return fmt.Errorf("flows.%s.success.origin: %w", name, err)
		}
		if _, ok := declaredOrigins[canonicalAuthenticationOrigin(origin)]; !ok {
			return fmt.Errorf("flows.%s.success.origin: origin is not declared by info", name)
		}
		if rawPath, _ := success["path"].(string); rawPath != "" && !isCleanBrowserPath(rawPath) {
			return fmt.Errorf("flows.%s.success.path: must be an exact clean path", name)
		}
		if err := validateContextOrigin(root, stringField(success, "context"), origin); err != nil {
			return fmt.Errorf("flows.%s.success.context: %w", name, err)
		}
	}
	return nil
}

// ValidateBrowserAuthenticationCallSupplement validates the extension payload
// envelope used by an explicit authentication operation.
func ValidateBrowserAuthenticationCallSupplement(data []byte) error {
	return ValidateBrowserAuthenticationCallSupplementForProfile(data, "uws.browser-authentication-call.1.0")
}

// ValidateBrowserAuthenticationCallSupplementForProfile validates the call
// envelope against the explicitly selected supplement version. Call envelopes
// have no document discriminator, so version selection cannot be inferred.
func ValidateBrowserAuthenticationCallSupplementForProfile(data []byte, profile string) error {
	value, document, err := decodeSchemaDocument(data, "browser authentication call")
	if err != nil {
		return err
	}
	schema, err := compiledBrowserAuthenticationCallSupplementSchema(profile)
	if err != nil {
		return err
	}
	if err := schema.Validate(document); err != nil {
		return fmt.Errorf("validate browser authentication call: %w", err)
	}
	root, _ := value.(map[string]any)
	call, _ := root["x-uws-browser-authentication"].(map[string]any)
	profilePath, _ := call["profile"].(string)
	clean := path.Clean(profilePath)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") || path.IsAbs(clean) {
		return fmt.Errorf("x-uws-browser-authentication.profile must be a safe relative path")
	}
	return nil
}

// ValidateBrowserRegistrationProfile validates one portable, secret-free
// account-registration recipe. It enforces exact safe origins, symbolic slot
// references, exactly one account-creation submit, explicit confirmation, and
// consistency between human checkpoints and declared effects.
func ValidateBrowserRegistrationProfile(data []byte) error {
	if len(data) > maxBrowserRegistrationProfileBytes {
		return fmt.Errorf("browser registration profile exceeds %d bytes", maxBrowserRegistrationProfileBytes)
	}
	value, document, err := decodeSchemaDocument(data, "browser registration profile")
	if err != nil {
		return err
	}
	profile, err := profileDiscriminator(value, "browser registration profile")
	if err != nil {
		return err
	}
	schema, err := compiledBrowserRegistrationProfileSchema(profile)
	if err != nil {
		return err
	}
	if err := schema.Validate(document); err != nil {
		return fmt.Errorf("validate browser registration profile: %w", err)
	}
	root, ok := value.(map[string]any)
	if !ok {
		return fmt.Errorf("browser registration profile must be an object")
	}
	info, _ := root["info"].(map[string]any)
	declaredOrigins := make(map[string]struct{})
	originCount := 0
	for _, field := range []string{"applicationOrigins", "registrationOrigins"} {
		origins, _ := info[field].([]any)
		originCount += len(origins)
		for i, raw := range origins {
			origin, _ := raw.(string)
			if err := validateAuthenticationOrigin(origin); err != nil {
				return fmt.Errorf("info.%s[%d]: %w", field, i, err)
			}
			declaredOrigins[canonicalAuthenticationOrigin(origin)] = struct{}{}
		}
	}
	if originCount > 32 {
		return fmt.Errorf("info origins: combined applicationOrigins and registrationOrigins exceed 32")
	}
	credentialSlots, _ := root["credentialSlots"].(map[string]any)
	flows, _ := root["flows"].(map[string]any)
	for name, raw := range flows {
		flow, _ := raw.(map[string]any)
		effects, _ := flow["effects"].([]any)
		hasHumanEffect := containsString(effects, "requires_human_verification")
		hasHumanCheckpoint := false
		submitCount := 0
		sequence, _ := flow["sequence"].([]any)
		for i, rawStep := range sequence {
			step, _ := rawStep.(map[string]any)
			if navigate, ok := step["navigate"].(string); ok {
				if err := validateAuthenticationTarget(navigate, declaredOrigins); err != nil {
					return fmt.Errorf("flows.%s.sequence[%d].navigate: %w", name, i, err)
				}
			}
			if rawType, ok := step["type_credential"]; ok {
				typeStep, _ := rawType.(map[string]any)
				slot, _ := typeStep["slot"].(string)
				if _, ok := credentialSlots[slot]; !ok {
					return fmt.Errorf("flows.%s.sequence[%d].type_credential.slot: undeclared credential slot %q", name, i, slot)
				}
			}
			if _, ok := step["submit"]; ok {
				submitCount++
			}
			if _, ok := step["human_checkpoint"]; ok {
				hasHumanCheckpoint = true
			}
			if _, ok := step["input_checkpoint"]; ok {
				hasHumanCheckpoint = true
			}
		}
		if submitCount != 1 {
			return fmt.Errorf("flows.%s.sequence: registration flow must contain exactly one submit step", name)
		}
		if hasHumanCheckpoint != hasHumanEffect {
			return fmt.Errorf("flows.%s.effects: requires_human_verification must be present exactly when the flow has a human_checkpoint step", name)
		}
		success, _ := flow["success"].(map[string]any)
		origin, _ := success["origin"].(string)
		if err := validateAuthenticationOrigin(origin); err != nil {
			return fmt.Errorf("flows.%s.success.origin: %w", name, err)
		}
		if _, ok := declaredOrigins[canonicalAuthenticationOrigin(origin)]; !ok {
			return fmt.Errorf("flows.%s.success.origin: origin is not declared by info", name)
		}
		if rawPath, _ := success["path"].(string); rawPath != "" && !isCleanBrowserPath(rawPath) {
			return fmt.Errorf("flows.%s.success.path: must be an exact clean path", name)
		}
	}
	if profile == "uws.browser-registration.1.1" {
		return validateRegistrationInputsProfile(root, declaredOrigins)
	}
	return nil
}

// ValidateBrowserRegistrationCallSupplement validates the extension envelope
// used by one explicitly approved registration mutation.
func ValidateBrowserRegistrationCallSupplement(data []byte) error {
	return ValidateBrowserRegistrationCallSupplementForProfile(data, "uws.browser-registration-call.1.0")
}

// ValidateBrowserRegistrationCallSupplementForProfile validates an explicitly
// selected supplement. The unversioned API retains its original 1.0 meaning.
func ValidateBrowserRegistrationCallSupplementForProfile(data []byte, profile string) error {
	if len(data) > maxBrowserRegistrationProfileBytes {
		return fmt.Errorf("browser registration call is too large")
	}
	value, document, err := decodeSchemaDocument(data, "browser registration call")
	if err != nil {
		return err
	}
	schema, err := compiledBrowserRegistrationCallSupplementSchema(profile)
	if err != nil {
		return err
	}
	if err := schema.Validate(document); err != nil {
		return fmt.Errorf("validate browser registration call: %w", err)
	}
	root, _ := value.(map[string]any)
	call, _ := root["x-uws-browser-registration"].(map[string]any)
	profilePath, _ := call["profile"].(string)
	clean := path.Clean(profilePath)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") || path.IsAbs(clean) || clean != profilePath {
		return fmt.Errorf("x-uws-browser-registration.profile must be a canonical safe relative path")
	}
	return nil
}

func profileDiscriminator(value any, label string) (string, error) {
	root, ok := value.(map[string]any)
	if !ok {
		return "", fmt.Errorf("%s must be an object", label)
	}
	profile, ok := root["profile"].(string)
	if !ok || strings.TrimSpace(profile) == "" {
		return "", fmt.Errorf("%s profile discriminator is required", label)
	}
	return profile, nil
}

func decodeSchemaDocument(data []byte, label string) (any, any, error) {
	if len(bytes.TrimSpace(data)) == 0 {
		return nil, nil, fmt.Errorf("%s document is empty", label)
	}
	encoded, err := decodeSingleJSONOrYAMLDocument(data)
	if err != nil {
		return nil, nil, fmt.Errorf("decode %s: %w", label, err)
	}
	var value any
	if err := json.Unmarshal(encoded, &value); err != nil {
		return nil, nil, fmt.Errorf("decode %s value: %w", label, err)
	}
	document, err := jsonschema.UnmarshalJSON(bytes.NewReader(encoded))
	if err != nil {
		return nil, nil, fmt.Errorf("decode %s as JSON: %w", label, err)
	}
	return value, document, nil
}

func validateAuthenticationOrigin(raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("invalid origin: %w", err)
	}
	if parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || (parsed.Path != "" && parsed.Path != "/") {
		return fmt.Errorf("must be an exact origin without credentials, path, query, or fragment")
	}
	host := parsed.Hostname()
	loopback := host == "localhost"
	if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
		loopback = true
	}
	if parsed.Scheme != "https" && !(parsed.Scheme == "http" && loopback) {
		return fmt.Errorf("must use https (http is allowed only for loopback)")
	}
	return nil
}

func canonicalAuthenticationOrigin(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil || parsed == nil || parsed.Host == "" {
		return ""
	}
	host := strings.ToLower(parsed.Hostname())
	port := parsed.Port()
	if (parsed.Scheme == "https" && port == "443") || (parsed.Scheme == "http" && port == "80") {
		port = ""
	}
	if port != "" {
		host = net.JoinHostPort(host, port)
	} else if strings.Contains(host, ":") {
		host = "[" + host + "]"
	}
	return strings.ToLower(parsed.Scheme) + "://" + host
}

func validateAuthenticationTarget(raw string, origins map[string]struct{}) error {
	parsed, err := url.Parse(raw)
	if err != nil || !parsed.IsAbs() || parsed.Host == "" {
		return fmt.Errorf("must be an absolute URL")
	}
	if parsed.User != nil || (parsed.Scheme != "https" && !(parsed.Scheme == "http" && isLoopbackHost(parsed.Hostname()))) {
		return fmt.Errorf("must use a declared safe origin")
	}
	if _, ok := origins[canonicalAuthenticationOrigin(raw)]; !ok {
		return fmt.Errorf("target origin is not declared by info")
	}
	return nil
}

func browserProfileOrigins(root map[string]any) map[string]struct{} {
	origins := make(map[string]struct{})
	info, _ := root["info"].(map[string]any)
	switch value := info["origin"].(type) {
	case string:
		origins[canonicalAuthenticationOrigin(value)] = struct{}{}
	case []any:
		for _, raw := range value {
			if origin, ok := raw.(string); ok {
				origins[canonicalAuthenticationOrigin(origin)] = struct{}{}
			}
		}
	}
	return origins
}

func validateBrowserContexts(root map[string]any, origins map[string]struct{}) error {
	contexts, _ := root["contexts"].(map[string]any)
	if info, ok := root["info"].(map[string]any); ok {
		switch value := info["origin"].(type) {
		case string:
			if err := validateAuthenticationOrigin(value); err != nil {
				return fmt.Errorf("info.origin: %w", err)
			}
		case []any:
			for i, raw := range value {
				origin, _ := raw.(string)
				if err := validateAuthenticationOrigin(origin); err != nil {
					return fmt.Errorf("info.origin[%d]: %w", i, err)
				}
			}
		}
	}
	for id, raw := range contexts {
		context, _ := raw.(map[string]any)
		origin, _ := context["origin"].(string)
		if err := validateAuthenticationOrigin(origin); err != nil {
			return fmt.Errorf("contexts.%s.origin: %w", id, err)
		}
		if _, ok := origins[canonicalAuthenticationOrigin(origin)]; !ok {
			return fmt.Errorf("contexts.%s.origin: origin is not declared by profile info", id)
		}
		parent, _ := context["parent"].(string)
		if parent == id {
			return fmt.Errorf("contexts.%s.parent: context cannot parent itself", id)
		}
		if parent != "main" {
			if _, ok := contexts[parent]; !ok {
				return fmt.Errorf("contexts.%s.parent: unknown context %q", id, parent)
			}
		}
		if rawPath, _ := context["path"].(string); rawPath != "" && !isCleanBrowserPath(rawPath) {
			return fmt.Errorf("contexts.%s.path: must be an exact clean path", id)
		}
	}

	depths := make(map[string]int, len(contexts))
	visiting := make(map[string]bool, len(contexts))
	var depth func(string) (int, error)
	depth = func(id string) (int, error) {
		if id == "main" {
			return 0, nil
		}
		if got, ok := depths[id]; ok {
			return got, nil
		}
		if visiting[id] {
			return 0, fmt.Errorf("contexts.%s.parent: context graph contains a cycle", id)
		}
		visiting[id] = true
		context, _ := contexts[id].(map[string]any)
		parent, _ := context["parent"].(string)
		parentDepth, err := depth(parent)
		if err != nil {
			return 0, err
		}
		visiting[id] = false
		got := parentDepth + 1
		if got > 4 {
			return 0, fmt.Errorf("contexts.%s: context depth exceeds 4", id)
		}
		depths[id] = got
		return got, nil
	}
	for id := range contexts {
		if _, err := depth(id); err != nil {
			return err
		}
	}

	opened := make(map[string]int)
	var visit func(any, string) error
	visit = func(value any, valuePath string) error {
		switch typed := value.(type) {
		case map[string]any:
			if contextID, ok := typed["context"].(string); ok {
				if contextID != "main" {
					if _, exists := contexts[contextID]; !exists {
						return fmt.Errorf("%s.context: unknown context %q", valuePath, contextID)
					}
				}
			}
			if openedID, ok := typed["opensContext"].(string); ok {
				context, exists := contexts[openedID].(map[string]any)
				if !exists || context["kind"] != "popup" {
					return fmt.Errorf("%s.opensContext: %q is not a declared popup context", valuePath, openedID)
				}
				parent := stringField(typed, "context")
				if parent == "" {
					parent = "main"
				}
				if context["parent"] != parent {
					return fmt.Errorf("%s.opensContext: popup parent does not match click context", valuePath)
				}
				opened[openedID]++
			}
			for key, child := range typed {
				if valuePath == "document" && key == "contexts" {
					continue
				}
				if err := visit(child, valuePath+"."+key); err != nil {
					return err
				}
			}
		case []any:
			for i, child := range typed {
				if err := visit(child, fmt.Sprintf("%s[%d]", valuePath, i)); err != nil {
					return err
				}
			}
		}
		return nil
	}
	if err := visit(root, "document"); err != nil {
		return err
	}
	for id, raw := range contexts {
		context, _ := raw.(map[string]any)
		if context["kind"] == "popup" && opened[id] != 1 {
			return fmt.Errorf("contexts.%s: popup must be opened by exactly one approved click", id)
		}
	}

	if actions, ok := root["actions"].(map[string]any); ok {
		for actionID, rawAction := range actions {
			action, _ := rawAction.(map[string]any)
			sequence, _ := action["sequence"].([]any)
			for i, rawStep := range sequence {
				step, _ := rawStep.(map[string]any)
				if rawNavigate, ok := step["navigate"]; ok {
					target, contextID := browserNavigate(rawNavigate)
					if err := validateBrowserTarget(root, target, contextID, origins); err != nil {
						return fmt.Errorf("actions.%s.sequence[%d].navigate: %w", actionID, i, err)
					}
				}
			}
		}
	}
	return nil
}

func browserNavigate(value any) (string, string) {
	if target, ok := value.(string); ok {
		return target, "main"
	}
	object, _ := value.(map[string]any)
	contextID, _ := object["context"].(string)
	if contextID == "" {
		contextID = "main"
	}
	return stringField(object, "url"), contextID
}

func validateBrowserTarget(root map[string]any, raw, contextID string, origins map[string]struct{}) error {
	parsed, err := url.Parse(raw)
	if err != nil || raw == "" || parsed.User != nil || parsed.Fragment != "" {
		return fmt.Errorf("must be a safe URL")
	}
	if parsed.IsAbs() {
		if parsed.Scheme != "https" && !(parsed.Scheme == "http" && isLoopbackHost(parsed.Hostname())) {
			return fmt.Errorf("must use a declared safe origin")
		}
		if _, ok := origins[canonicalAuthenticationOrigin(raw)]; !ok {
			return fmt.Errorf("target origin is not declared by info")
		}
	} else {
		if !strings.HasPrefix(raw, "/") {
			return fmt.Errorf("relative target must be root-relative")
		}
		if len(origins) != 1 {
			return fmt.Errorf("relative target requires exactly one info origin")
		}
	}
	return validateContextTarget(root, contextID, raw)
}

func validateContextTarget(root map[string]any, contextID, raw string) error {
	if contextID == "" || contextID == "main" {
		return nil
	}
	contexts, _ := root["contexts"].(map[string]any)
	context, ok := contexts[contextID].(map[string]any)
	if !ok {
		return fmt.Errorf("unknown context %q", contextID)
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("invalid target URL")
	}
	if parsed.IsAbs() && canonicalAuthenticationOrigin(raw) != canonicalAuthenticationOrigin(stringField(context, "origin")) {
		return fmt.Errorf("target origin does not match context %q", contextID)
	}
	return nil
}

func validateContextOrigin(root map[string]any, contextID, origin string) error {
	if contextID == "" || contextID == "main" {
		return nil
	}
	contexts, _ := root["contexts"].(map[string]any)
	context, ok := contexts[contextID].(map[string]any)
	if !ok {
		return fmt.Errorf("unknown context %q", contextID)
	}
	if canonicalAuthenticationOrigin(stringField(context, "origin")) != canonicalAuthenticationOrigin(origin) {
		return fmt.Errorf("origin does not match context %q", contextID)
	}
	return nil
}

func isCleanBrowserPath(raw string) bool {
	if raw == "" || !strings.HasPrefix(raw, "/") || strings.ContainsAny(raw, "?#\\") {
		return false
	}
	segments := strings.Split(raw, "/")
	for i, segment := range segments[1:] {
		if segment == "" && i != len(segments)-2 && raw != "/" {
			return false
		}
		decoded, err := url.PathUnescape(segment)
		if err != nil || decoded == "." || decoded == ".." || strings.Contains(decoded, "/") {
			return false
		}
	}
	return true
}

func stringField(value map[string]any, key string) string {
	got, _ := value[key].(string)
	return got
}

func isLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func containsString(values []any, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func compiledBrowserAuthenticationProfileSchema(profile string) (*jsonschema.Schema, error) {
	switch profile {
	case "uws.browser-authentication.1.0":
		auth10SchemaOnce.Do(func() {
			auth10Schema, auth10SchemaErr = compileEmbeddedSchema("browser-authentication.1.0.json")
		})
		return auth10Schema, auth10SchemaErr
	case "uws.browser-authentication.1.1":
		auth11SchemaOnce.Do(func() {
			auth11Schema, auth11SchemaErr = compileEmbeddedSchema("browser-authentication.1.1.json")
		})
		return auth11Schema, auth11SchemaErr
	default:
		return nil, fmt.Errorf("unsupported browser authentication profile discriminator %q", profile)
	}
}

func compiledBrowserAuthenticationCallSupplementSchema(profile string) (*jsonschema.Schema, error) {
	name := familySchemaName(profile, "browser-authentication-call", "1.1")
	switch name {
	case "browser-authentication-call.1.0.json":
		authCall10SchemaOnce.Do(func() {
			authCall10Schema, authCall10SchemaErr = compileEmbeddedSchema(name)
		})
		return authCall10Schema, authCall10SchemaErr
	case "browser-authentication-call.1.1.json":
		authCall11SchemaOnce.Do(func() {
			authCall11Schema, authCall11SchemaErr = compileEmbeddedSchema(name)
		})
		return authCall11Schema, authCall11SchemaErr
	default:
		return nil, fmt.Errorf("unsupported browser authentication call profile %q", profile)
	}
}

func compiledBrowserRegistrationProfileSchema(profile string) (*jsonschema.Schema, error) {
	switch profile {
	case "uws.browser-registration.1.0":
		registration10SchemaOnce.Do(func() {
			registration10Schema, registration10SchemaErr = compileEmbeddedSchema("browser-registration.1.0.json")
		})
		return registration10Schema, registration10SchemaErr
	case "uws.browser-registration.1.1":
		registration11SchemaOnce.Do(func() {
			registration11Schema, registration11SchemaErr = compileEmbeddedSchema("browser-registration.1.1.json")
		})
		return registration11Schema, registration11SchemaErr
	default:
		return nil, fmt.Errorf("unsupported browser registration profile discriminator %q", profile)
	}
}

func compiledBrowserRegistrationCallSupplementSchema(profile string) (*jsonschema.Schema, error) {
	name := familySchemaName(profile, "browser-registration-call", "1.0")
	switch name {
	case "browser-registration-call.1.0.json":
		registrationCall10SchemaOnce.Do(func() {
			registrationCall10Schema, registrationCall10SchemaErr = compileEmbeddedSchema(name)
		})
		return registrationCall10Schema, registrationCall10SchemaErr
	case "browser-registration-call.1.1.json":
		registrationCall11SchemaOnce.Do(func() {
			registrationCall11Schema, registrationCall11SchemaErr = compileEmbeddedSchema(name)
		})
		return registrationCall11Schema, registrationCall11SchemaErr
	default:
		return nil, fmt.Errorf("unsupported browser registration call profile %q", profile)
	}
}

func compileEmbeddedSchema(name string) (*jsonschema.Schema, error) {
	data, err := embeddedSchemaDocument(name)
	if err != nil {
		return nil, err
	}
	document, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("decode embedded %s schema: %w", name, err)
	}
	compiler := jsonschema.NewCompiler()
	compiler.AssertFormat()
	if err := compiler.AddResource(name, document); err != nil {
		return nil, fmt.Errorf("register embedded %s schema: %w", name, err)
	}
	schema, err := compiler.Compile(name)
	if err != nil {
		return nil, fmt.Errorf("compile embedded %s schema: %w", name, err)
	}
	return schema, nil
}

func decodeSingleJSONOrYAMLDocument(data []byte) ([]byte, error) {
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return nil, fmt.Errorf("multiple YAML documents are not supported")
		}
		return nil, err
	}
	if value == nil {
		return nil, fmt.Errorf("document is empty")
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return encoded, nil
}

func compiledBrowserSourceProfileSchema(profile string) (*jsonschema.Schema, error) {
	switch profile {
	case "uws.browser.1.5":
		browser15SchemaOnce.Do(func() {
			browser15Schema, browser15SchemaErr = compileEmbeddedSchema("browser.1.5.json")
		})
		return browser15Schema, browser15SchemaErr
	case "uws.browser.1.6":
		browser16SchemaOnce.Do(func() {
			browser16Schema, browser16SchemaErr = compileEmbeddedSchema("browser.1.6.json")
		})
		return browser16Schema, browser16SchemaErr
	case "uws.browser.1.7":
		browser17SchemaOnce.Do(func() {
			browser17Schema, browser17SchemaErr = compileEmbeddedSchema("browser.1.7.json")
		})
		return browser17Schema, browser17SchemaErr
	default:
		return nil, fmt.Errorf("unsupported browser source profile discriminator %q", profile)
	}
}

func runtimeSupplementSchemaName(profile string) string {
	profile = strings.TrimSpace(profile)
	if profile == "" {
		return "runtime.1.0.json"
	}
	profile = strings.TrimSuffix(profile, ".json")
	profile = strings.TrimPrefix(profile, "uws.")
	if !strings.HasPrefix(profile, "runtime.") {
		profile = "runtime." + profile
	}
	return profile + ".json"
}

func familySchemaName(profile, name, defaultVersion string) string {
	profile = strings.TrimSpace(profile)
	if profile == "" {
		profile = defaultVersion
	}
	profile = strings.TrimSuffix(profile, ".json")
	profile = strings.TrimPrefix(profile, "uws.")
	if !strings.HasPrefix(profile, name+".") {
		profile = name + "." + profile
	}
	return profile + ".json"
}

func pathForSchemaName(anchorDir, name string) string {
	name = filepath.Base(strings.TrimSpace(name))
	if name == "" || name == "." {
		name = "1.0.0.json"
	}
	if dir := strings.TrimSpace(os.Getenv("UWS_SCHEMA_DIR")); dir != "" {
		return filepath.Join(dir, name)
	}
	if dir := strings.TrimSpace(os.Getenv("OPENUDON_UWS_SCHEMA_DIR")); dir != "" {
		return filepath.Join(dir, name)
	}
	if path, ok := packageSchemaPath(name); ok {
		return path
	}
	if path, ok := moduleCacheSchemaPath(name); ok {
		return path
	}
	if path, ok := embeddedSchemaPath(name); ok {
		return path
	}
	return filepath.Join(anchorDir, "..", "uws", "versions", name)
}

func packageSchemaPath(name string) (string, bool) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "", false
	}
	path := filepath.Join(filepath.Dir(file), "..", "versions", name)
	if _, err := os.Stat(path); err == nil {
		return path, true
	}
	return "", false
}

func moduleCacheSchemaPath(name string) (string, bool) {
	version, ok := uwsModuleVersion()
	if !ok {
		return "", false
	}
	path, err := escapedModuleCachePath(uwsModulePath, version)
	if err != nil {
		return "", false
	}
	schema := filepath.Join(path, "versions", name)
	if _, err := os.Stat(schema); err == nil {
		return schema, true
	}
	return "", false
}

func uwsModuleVersion() (string, bool) {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "", false
	}
	for _, dep := range info.Deps {
		if dep.Path != uwsModulePath {
			continue
		}
		if dep.Version != "" {
			return dep.Version, true
		}
		if dep.Replace != nil && dep.Replace.Version != "" {
			return dep.Replace.Version, true
		}
	}
	return "", false
}

func escapedModuleCachePath(path, version string) (string, error) {
	escapedPath, err := module.EscapePath(path)
	if err != nil {
		return "", err
	}
	escapedVersion, err := module.EscapeVersion(version)
	if err != nil {
		return "", err
	}
	return filepath.Join(moduleCacheDir(), escapedPath+"@"+escapedVersion), nil
}

func moduleCacheDir() string {
	if dir := strings.TrimSpace(os.Getenv("GOMODCACHE")); dir != "" {
		return dir
	}
	gopath := strings.TrimSpace(os.Getenv("GOPATH"))
	if gopath == "" {
		if home, err := os.UserHomeDir(); err == nil {
			gopath = filepath.Join(home, "go")
		}
	}
	if gopath == "" {
		return ""
	}
	first := filepath.SplitList(gopath)[0]
	if first == "" {
		return ""
	}
	return filepath.Join(first, "pkg", "mod")
}

func embeddedSchemaPath(name string) (string, bool) {
	data, err := embeddedSchemaDocument(name)
	if err != nil {
		return "", false
	}
	sum := sha256.Sum256(data)
	dir := filepath.Join(os.TempDir(), "uws-schema", fmt.Sprintf("%x", sum[:8]))
	path := filepath.Join(dir, name)
	if existing, err := os.ReadFile(path); err == nil && bytes.Equal(existing, data) {
		return path, true
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", false
	}
	tmp, err := os.CreateTemp(dir, name+".*.tmp")
	if err != nil {
		return "", false
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return "", false
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return "", false
	}
	if err := os.Chmod(tmpName, 0o644); err != nil {
		os.Remove(tmpName)
		return "", false
	}
	if err := os.Rename(tmpName, path); err != nil {
		os.Remove(tmpName)
		return "", false
	}
	return path, true
}

func embeddedSchemaDocument(name string) ([]byte, error) {
	name = filepath.ToSlash(filepath.Base(strings.TrimSpace(name)))
	archive, err := zip.NewReader(bytes.NewReader(embeddedVersionDocuments), int64(len(embeddedVersionDocuments)))
	if err != nil {
		return nil, fmt.Errorf("open embedded version documents: %w", err)
	}
	for _, file := range archive.File {
		if file.Name != name {
			continue
		}
		reader, err := file.Open()
		if err != nil {
			return nil, err
		}
		data, readErr := io.ReadAll(reader)
		closeErr := reader.Close()
		if readErr != nil {
			return nil, readErr
		}
		if closeErr != nil {
			return nil, closeErr
		}
		return data, nil
	}
	return nil, fmt.Errorf("version document %q is not embedded", name)
}

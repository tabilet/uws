package schemas_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OpenUdon/uws/browserregistration"
	"github.com/OpenUdon/uws/schemas"
	"gopkg.in/yaml.v3"
)

func registrationForm(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "testdata", "browser-registration", "private-form.json"))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func registrationObject(t *testing.T, data []byte) map[string]any {
	t.Helper()
	var value map[string]any
	if err := json.Unmarshal(data, &value); err != nil {
		t.Fatal(err)
	}
	return value
}

func registrationJSON(t *testing.T, value any) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestRegistrationRecipesDoNotRequireDiscoveryAndPreserveLegacyMetadata(t *testing.T) {
	for _, coverage := range []string{"", "partial", "owner_reviewed"} {
		include := coverage != ""
		object := registrationObject(t, registrationForm(t))
		if !include {
			delete(object, "discovery")
		} else {
			object["discovery"].(map[string]any)["coverage"] = coverage
		}
		data := registrationJSON(t, object)
		if err := schemas.ValidateBrowserRegistrationProfile(data); err != nil {
			t.Fatal(err)
		}
		if err := schemas.ValidateBrowserRegistrationCallBinding(data, readBrowserRegistrationFixture(t, "private-form-call.json")); err != nil {
			t.Fatal(err)
		}
		if _, err := schemas.BrowserRegistrationInputTemplate(data, "advertiser"); err != nil {
			t.Fatal(err)
		}
		var profile browserregistration.Profile
		if err := json.Unmarshal(data, &profile); err != nil {
			t.Fatal(err)
		}
		roundtrip := registrationObject(t, registrationJSON(t, profile))
		_, retained := roundtrip["discovery"]
		if retained != include {
			t.Fatal("round trip added or stripped optional discovery")
		}
		if include && string(registrationJSON(t, roundtrip["discovery"])) != string(registrationJSON(t, object["discovery"])) {
			t.Fatal("legacy discovery changed")
		}
	}
}

func TestRegistrationPrivateFormVersionAndWireRoundtrip(t *testing.T) {
	data := registrationForm(t)
	if err := schemas.ValidateBrowserRegistrationProfile(data); err != nil {
		t.Fatal(err)
	}
	var profile browserregistration.Profile
	if err := json.Unmarshal(data, &profile); err != nil {
		t.Fatal(err)
	}
	for _, marshal := range []func(any) ([]byte, error){json.Marshal, yaml.Marshal} {
		encoded, err := marshal(profile)
		if err != nil {
			t.Fatal(err)
		}
		if err := schemas.ValidateBrowserRegistrationProfile(encoded); err != nil {
			t.Fatal(err)
		}
	}
	old := strings.Replace(string(data), "uws.browser-registration.1.1", "uws.browser-registration.1.0", 1)
	if err := schemas.ValidateBrowserRegistrationProfile([]byte(old)); err == nil {
		t.Fatal("1.0 accepted new fields")
	}
	unknown := strings.Replace(string(data), "uws.browser-registration.1.1", "uws.browser-registration.9.9", 1)
	if err := schemas.ValidateBrowserRegistrationProfile([]byte(unknown)); err == nil {
		t.Fatal("unknown version accepted")
	}
	call := readBrowserRegistrationFixture(t, "private-form-call.json")
	if err := schemas.ValidateBrowserRegistrationCallSupplement(call); err == nil {
		t.Fatal("legacy call accepted input binding")
	}
	if err := schemas.ValidateBrowserRegistrationCallBinding(data, call); err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(call, &payload); err != nil {
		t.Fatal(err)
	}
	op, ok, err := browserregistration.ReadRegistrationExtension(payload)
	if err != nil || !ok || op.InputBinding != "dedicated_registration_input" {
		t.Fatalf("call wire: %v", err)
	}
	var roundtrip map[string]any
	if err := browserregistration.SetRegistrationExtension(&roundtrip, op); err != nil {
		t.Fatal(err)
	}
	if err := schemas.ValidateBrowserRegistrationCallBinding(data, registrationJSON(t, roundtrip)); err != nil {
		t.Fatal(err)
	}
	for _, version := range []string{"1.0", "1.1"} {
		for _, load := range []func(string) ([]byte, error){schemas.BrowserRegistrationProfileSchema, schemas.BrowserRegistrationCallSupplementSchema} {
			one, err := load(version)
			if err != nil {
				t.Fatal(err)
			}
			two, err := load(version)
			if err != nil {
				t.Fatal(err)
			}
			one[0] = 'x'
			if two[0] == 'x' {
				t.Fatal("shared schema bytes")
			}
		}
	}
	one, err := schemas.BrowserRegistrationInputSchema("")
	if err != nil {
		t.Fatal(err)
	}
	two, err := schemas.BrowserRegistrationInputSchema("uws.browser-registration-input.1.0")
	if err != nil {
		t.Fatal(err)
	}
	one[0] = 'x'
	if two[0] == 'x' {
		t.Fatal("shared private-envelope schema bytes")
	}
	if _, err := schemas.BrowserRegistrationInputSchema("9.9"); err == nil {
		t.Fatal("unknown input schema accepted")
	}
	if !strings.HasSuffix(schemas.PathForBrowserRegistrationInput(".", ""), "browser-registration-input.1.0.json") {
		t.Fatal("input schema path")
	}
}

func TestRegistrationInputProfileRejectsUnsafeDefinitions(t *testing.T) {
	for name, mutate := range map[string]func(map[string]any, map[string]any, []any){
		"inline value":         func(r, f map[string]any, s []any) { f["first_name"].(map[string]any)["value"] = "private" },
		"inline default":       func(r, f map[string]any, s []any) { f["phone"].(map[string]any)["default"] = "private" },
		"credential collision": func(r, f map[string]any, s []any) { f["password"] = f["phone"] },
		"unknown condition": func(r, f map[string]any, s []any) {
			f["company"].(map[string]any)["requiredWhen"].(map[string]any)["slot"] = "missing"
		},
		"credential comparison": func(r, f map[string]any, s []any) {
			f["company"].(map[string]any)["requiredWhen"].(map[string]any)["slot"] = "password"
		},
		"condition literal outside choices": func(r, f map[string]any, s []any) {
			f["company"].(map[string]any)["requiredWhen"].(map[string]any)["equals"] = "private"
		},
		"conditional cycle": func(r, f map[string]any, s []any) {
			p := f["account_kind"].(map[string]any)
			delete(p, "required")
			p["requiredWhen"] = map[string]any{"slot": "company", "equals": "business"}
		},
		"wrong enum type":     func(r, f map[string]any, s []any) { f["account_kind"].(map[string]any)["enum"] = []any{true} },
		"bad constraint type": func(r, f map[string]any, s []any) { f["newsletter"].(map[string]any)["minLength"] = 1 },
		"inverted bounds":     func(r, f map[string]any, s []any) { f["first_name"].(map[string]any)["minLength"] = 101 },
		"false completeness":  func(r, f map[string]any, s []any) { r["discovery"].(map[string]any)["coverage"] = "complete" },
		"discovery origin escape": func(r, f map[string]any, s []any) {
			r["discovery"].(map[string]any)["entryPoints"] = []any{"https://elsewhere.example.test/register"}
		},
		"undeclared slot": func(r, f map[string]any, s []any) {
			s[4].(map[string]any)["fill_input"].(map[string]any)["slot"] = "missing"
		},
		"wrong control": func(r, f map[string]any, s []any) {
			s[4].(map[string]any)["fill_input"].(map[string]any)["control"] = "check"
		},
		"select without choices": func(r, f map[string]any, s []any) { delete(f["account_kind"].(map[string]any), "enum") },
		"no checkpoint before fill": func(r, f map[string]any, s []any) {
			s[0] = map[string]any{"wait_for": map[string]any{"locator": map[string]any{"role": "form"}}}
		},
		"duplicate checkpoint": func(r, f map[string]any, s []any) {
			s[7].(map[string]any)["input_checkpoint"].(map[string]any)["id"] = "identity"
		},
		"unapplied revision": func(r, f map[string]any, s []any) {
			s[4] = map[string]any{"wait_for": map[string]any{"locator": map[string]any{"role": "form"}}}
		},
		"checkpoint after submit": func(r, f map[string]any, s []any) { s[12], s[7] = s[7], s[12] },
		"missing human effect": func(r, f map[string]any, s []any) {
			r["flows"].(map[string]any)["advertiser"].(map[string]any)["effects"] = []any{"creates_account"}
		},
	} {
		t.Run(name, func(t *testing.T) {
			r := registrationObject(t, registrationForm(t))
			f := r["inputSlots"].(map[string]any)
			s := r["flows"].(map[string]any)["advertiser"].(map[string]any)["sequence"].([]any)
			mutate(r, f, s)
			if err := schemas.ValidateBrowserRegistrationProfile(registrationJSON(t, r)); err == nil {
				t.Fatal("unsafe profile accepted")
			}
		})
	}
}

func registrationDraft(t *testing.T, profile []byte, kind string) map[string]any {
	t.Helper()
	data, err := schemas.BrowserRegistrationInputTemplate(profile, kind)
	if err != nil {
		t.Fatal(err)
	}
	draft := registrationObject(t, data)
	v := draft["values"].(map[string]any)
	v["identifier"] = "fixture@example.invalid"
	v["password"] = "synthetic-password"
	v["first_name"] = "Fixture"
	v["account_kind"] = "business"
	return draft
}

func TestRegistrationPrivateInputStages(t *testing.T) {
	p := registrationForm(t)
	first := registrationDraft(t, p, "advertiser")
	if _, exists := first["values"].(map[string]any)["website"]; exists {
		t.Fatal("template includes another flow's field")
	}
	a := registrationJSON(t, first)
	if err := schemas.ValidateBrowserRegistrationInputUpdate(p, a, nil, "advertiser", "identity"); err != nil {
		t.Fatal(err)
	}
	second := registrationObject(t, a)
	second["revision"] = 2
	v := second["values"].(map[string]any)
	v["company"] = "Synthetic Company"
	v["newsletter"] = false
	b := registrationJSON(t, second)
	if err := schemas.ValidateBrowserRegistrationInputUpdate(p, b, a, "advertiser", "contact"); err != nil {
		t.Fatal(err)
	}
	// Optional null and false remain distinct; the publisher's website is required.
	publisher := registrationDraft(t, p, "publisher")
	pubA := registrationJSON(t, publisher)
	publisher["revision"] = 2
	publisher["values"].(map[string]any)["company"] = "Fixture"
	if err := schemas.ValidateBrowserRegistrationInputUpdate(p, registrationJSON(t, publisher), pubA, "publisher", "contact"); err == nil {
		t.Fatal("missing publisher website accepted")
	}
	publisher["values"].(map[string]any)["website"] = "https://publisher.example.invalid"
	if err := schemas.ValidateBrowserRegistrationInputUpdate(p, registrationJSON(t, publisher), pubA, "publisher", "contact"); err != nil {
		t.Fatal(err)
	}
	individual := registrationDraft(t, p, "advertiser")
	individual["values"].(map[string]any)["account_kind"] = "individual"
	indA := registrationJSON(t, individual)
	individual["revision"] = 2
	if err := schemas.ValidateBrowserRegistrationInputUpdate(p, registrationJSON(t, individual), indA, "advertiser", "contact"); err != nil {
		t.Fatal(err)
	}
}

func TestRegistrationPrivateInputRejectsChangesAndDoesNotLeak(t *testing.T) {
	p := registrationForm(t)
	first := registrationDraft(t, p, "advertiser")
	a := registrationJSON(t, first)
	second := registrationObject(t, a)
	second["revision"] = 2
	second["values"].(map[string]any)["company"] = "Synthetic Company"
	b := registrationJSON(t, second)
	for name, mutate := range map[string]func(map[string]any){
		"stale revision":              func(d map[string]any) { d["revision"] = 1 },
		"fractional revision":         func(d map[string]any) { d["revision"] = 1.5 },
		"profile changed":             func(d map[string]any) { d["profileSha256"] = strings.Repeat("a", 64) },
		"wrong registration type":     func(d map[string]any) { d["registrationType"] = "publisher" },
		"credential edit":             func(d map[string]any) { d["values"].(map[string]any)["password"] = "PRIVATE_SENTINEL" },
		"outside checkpoint":          func(d map[string]any) { d["values"].(map[string]any)["first_name"] = "PRIVATE_SENTINEL" },
		"unknown field":               func(d map[string]any) { d["values"].(map[string]any)["PRIVATE_SENTINEL"] = "PRIVATE_SENTINEL" },
		"required conditional absent": func(d map[string]any) { d["values"].(map[string]any)["company"] = nil },
		"wrong scalar":                func(d map[string]any) { d["values"].(map[string]any)["newsletter"] = "PRIVATE_SENTINEL" },
		"long string":                 func(d map[string]any) { d["values"].(map[string]any)["phone"] = strings.Repeat("PRIVATE_SENTINEL", 10) },
		"unknown envelope":            func(d map[string]any) { d["PRIVATE_SENTINEL"] = "PRIVATE_SENTINEL" },
		"composite value": func(d map[string]any) {
			d["values"].(map[string]any)["phone"] = map[string]any{"PRIVATE_SENTINEL": true}
		},
	} {
		t.Run(name, func(t *testing.T) {
			d := registrationObject(t, b)
			mutate(d)
			err := schemas.ValidateBrowserRegistrationInputUpdate(p, registrationJSON(t, d), a, "advertiser", "contact")
			if err == nil || strings.Contains(err.Error(), "PRIVATE_SENTINEL") {
				t.Fatalf("unsafe error: %v", err)
			}
		})
	}
	for _, raw := range []string{`{"PRIVATE_SENTINEL":`, strings.Replace(string(b), `"revision":2`, `"revision":2,"revision":3`, 1), string(b) + "\n{}", `PRIVATE_SENTINEL: private`} {
		err := schemas.ValidateBrowserRegistrationInputUpdate(p, []byte(raw), a, "advertiser", "contact")
		if err == nil || strings.Contains(err.Error(), "PRIVATE_SENTINEL") {
			t.Fatalf("unsafe parsing error: %v", err)
		}
	}
	if err := schemas.ValidateBrowserRegistrationInputUpdate(p, b, nil, "advertiser", "contact"); err == nil {
		t.Fatal("later checkpoint without predecessor accepted")
	}
	if err := schemas.ValidateBrowserRegistrationInputUpdate(p, b, a, "advertiser", "unknown"); err == nil {
		t.Fatal("unknown checkpoint accepted")
	}
	if err := schemas.ValidateBrowserRegistrationInputUpdate(p, a, nil, "advertiser", "identity"); err != nil {
		t.Fatal(err)
	}
}

func TestRegistrationCallBindingRejectsMismatch(t *testing.T) {
	p := registrationForm(t)
	original := readBrowserRegistrationFixture(t, "private-form-call.json")
	for name, mutate := range map[string]func(map[string]any){
		"missing binding":       func(c map[string]any) { delete(c, "inputBinding") },
		"file path binding":     func(c map[string]any) { c["inputBinding"] = "/private/data.json" },
		"unknown flow":          func(c map[string]any) { c["flow"] = "unknown" },
		"missing credential":    func(c map[string]any) { delete(c["credentialBindings"].(map[string]any), "password") },
		"additional credential": func(c map[string]any) { c["credentialBindings"].(map[string]any)["extra"] = "extra" },
		"inline input":          func(c map[string]any) { c["inputValues"] = map[string]any{} },
		"retry":                 func(c map[string]any) { c["ambiguousOutcome"] = "retry" },
	} {
		t.Run(name, func(t *testing.T) {
			d := registrationObject(t, original)
			mutate(d["x-uws-browser-registration"].(map[string]any))
			if err := schemas.ValidateBrowserRegistrationCallBinding(p, registrationJSON(t, d)); err == nil {
				t.Fatal("bad binding accepted")
			}
		})
	}
}

func TestRegistrationNumericInputsAndMissingRequired(t *testing.T) {
	for _, kind := range []string{"integer", "number"} {
		t.Run(kind, func(t *testing.T) {
			r := registrationObject(t, registrationForm(t))
			r["inputSlots"].(map[string]any)["phone"] = map[string]any{
				"type": kind, "label": "Synthetic quantity", "required": true,
				"minimum": 0, "maximum": 20,
			}
			p := registrationJSON(t, r)
			first := registrationDraft(t, p, "advertiser")
			a := registrationJSON(t, first)
			first["revision"] = 2
			values := first["values"].(map[string]any)
			values["company"] = "Synthetic"
			for _, number := range []any{nil, "0", -1, 21, 9007199254740992.0} {
				values["phone"] = number
				if err := schemas.ValidateBrowserRegistrationInputUpdate(p, registrationJSON(t, first), a, "advertiser", "contact"); err == nil {
					t.Fatal("invalid numeric input accepted")
				}
			}
			values["phone"] = 0
			if err := schemas.ValidateBrowserRegistrationInputUpdate(p, registrationJSON(t, first), a, "advertiser", "contact"); err != nil {
				t.Fatal(err)
			}
			values["phone"] = 1.5
			err := schemas.ValidateBrowserRegistrationInputUpdate(p, registrationJSON(t, first), a, "advertiser", "contact")
			if (err == nil) != (kind == "number") {
				t.Fatalf("fractional %s validation: %v", kind, err)
			}
		})
	}
	p := registrationForm(t)
	draft, err := schemas.BrowserRegistrationInputTemplate(p, "advertiser")
	if err != nil {
		t.Fatal(err)
	}
	if err := schemas.ValidateBrowserRegistrationInputUpdate(p, draft, nil, "advertiser", "identity"); err == nil {
		t.Fatal("unfilled draft accepted")
	}
	if _, err := schemas.BrowserRegistrationInputTemplate(p, "unknown"); err == nil {
		t.Fatal("unknown flow template accepted")
	}
}

func TestRegistrationReadinessBeforeBrowserAndReapplyChanges(t *testing.T) {
	for _, mode := range []string{"browser_before_ready", "unapplied_update", "changed_condition"} {
		t.Run(mode, func(t *testing.T) {
			r := registrationObject(t, registrationForm(t))
			flow := r["flows"].(map[string]any)["advertiser"].(map[string]any)
			steps := flow["sequence"].([]any)
			switch mode {
			case "browser_before_ready":
				steps[0], steps[1] = steps[1], steps[0]
			case "unapplied_update":
				cp := map[string]any{"input_checkpoint": map[string]any{"id": "update_name", "slots": []any{"first_name"}}}
				steps = append(append(append([]any{}, steps[:12]...), cp), steps[12:]...)
			case "changed_condition":
				cp := map[string]any{"input_checkpoint": map[string]any{"id": "update_kind", "slots": []any{"account_kind"}}}
				fill := steps[5]
				steps = append(append(append([]any{}, steps[:12]...), cp, fill), steps[12:]...)
			}
			flow["sequence"] = steps
			if err := schemas.ValidateBrowserRegistrationProfile(registrationJSON(t, r)); err == nil {
				t.Fatal("unsafe readiness/update accepted")
			}
		})
	}
}

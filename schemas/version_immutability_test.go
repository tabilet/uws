package schemas

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

// publishedVersionSHA256 freezes every published JSON contract. Adding a new
// document requires adding its digest; changing or removing an existing
// document fails this test.
var publishedVersionSHA256 = map[string]string{
	"browser-registration.1.1.json":        "ee14fbb9ddde9bdd63ce31016cbc6f210a435694936e2d01403ba514fcd137ee",
	"browser-registration-call.1.1.json":   "a8ca1819faed5689caccac42fd35b2113c9a305e9fc93ea916e36827d4315a75",
	"browser-registration-input.1.0.json":  "9512ea3430997add675ac9c675143ad177d2d2d74fbe385f682cf29ce0e4e2c9",
	"1.0.0.json":                           "31e4b67462763807571bdd496e7d11e1e614eea43299297c7ca5301c7fa01076",
	"1.1.0.json":                           "f1652bbf473adea57bc3cc01ec2a5caf5389d39c2a4950d51857063adbd40e64",
	"1.1.1.json":                           "ffbabb57b7c6334140c18b209017a826cef5b9e607d18d263e1603e43ee323b2",
	"1.2.0.json":                           "cfd49c4d357097eb0413fac3b07d28617cfe95d7e728b75db9446e6c2b575e15",
	"1.3.0.json":                           "9099b67b0ed149f95b5413c72643dfd27734d0b7f30907023d1a4a8834a23f0e",
	"1.4.0.json":                           "7c41cf2e65c9fbecc00ca2a19d76a92d9ada8a0b2f724038aecb0497de717b1e",
	"1.5.0.json":                           "9bc90ef03383250888ec55040180a8d7e2aeb8f5d59084ad238a502f2c3a7b61",
	"1.6.0.json":                           "a447ce60caf4b12d7e142c36e125b09b9fbcc4c8610ff454207d7910df10776e",
	"1.7.0.json":                           "ca032c02f1ad51e9386a1d55d32f11d66137bd0f9c2af459e8c2a1910e4ddff8",
	"1.8.0.json":                           "ea842defb84380c6f9f6d79c3c54575c703902dd89d75322968cb80bc073e7d6",
	"1.9.0.json":                           "87b72744ea71785613cd1775968503e81312b38d2cee3a00ec5b8bf04bc976e1",
	"1.9.1.json":                           "a055a67d393dacf3c9beac30732adb59e7a81427eb8701e56e5662359c5e624d",
	"ansible.1.0.json":                     "c67738d98732a177863421f3edd062f98aadba325a672e9be344478dfb41c6d6",
	"browser-authentication-call.1.0.json": "586af5315001334ba7ceb69048f31b278ef16b0984ab05da6b7b361a2e035672",
	"browser-authentication-call.1.1.json": "361aa798cefb4a172ecfda8c794a970faeb05919557a73bacb143af57332d35a",
	"browser-registration-call.1.0.json":   "b311aeefb1b6c2b8d675a0839c654140105223f53994eb14260b9387aa293a38",
	"browser-registration.1.0.json":        "6613ba3eecf0d073554cc6e6a4a56ebf2e9e3264714cd49fd740e6e784044605",
	"browser-authentication.1.0.json":      "8ccf16281a83783d0b342312edb0b737a53129fca0944d8f00f31acc91f461ad",
	"browser-authentication.1.1.json":      "61ba61569e062959213533cf95363cfe07b226241e75e5c4198d690d35debbd2",
	"browser.1.5.json":                     "6dda6191ca4d9899183f29c170e80e7c295ecb33f3e32c8994898e62eb8fa4c8",
	"browser.1.6.json":                     "396d36fff165b2bf4fd6ada45cacad7365f330ab3ae16fc95d9a244e34f819bc",
	"browser.1.7.json":                     "feed4f71655b232fe6a87db285ac686615f9e2076c64b751e5c945c9463f446b",
	"runtime.1.0.json":                     "c8ed61ae855c828767a30d94e667bd7f0b3bed75ee8e36407f815d789fe6cd31",
}

func TestPublishedVersionDocumentsAreImmutable(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join("..", "versions", "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	actualNames := make([]string, 0, len(paths))
	for _, path := range paths {
		name := filepath.Base(path)
		actualNames = append(actualNames, name)
		want, ok := publishedVersionSHA256[name]
		if !ok {
			t.Errorf("published JSON document %s is missing from the SHA-256 manifest", name)
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Error(err)
			continue
		}
		sum := sha256.Sum256(data)
		if got := hex.EncodeToString(sum[:]); got != want {
			t.Errorf("published JSON document %s changed: SHA-256 = %s, want %s", name, got, want)
		}
	}
	sort.Strings(actualNames)
	wantNames := make([]string, 0, len(publishedVersionSHA256))
	for name := range publishedVersionSHA256 {
		wantNames = append(wantNames, name)
	}
	sort.Strings(wantNames)
	if len(actualNames) != len(wantNames) {
		t.Fatalf("versions/*.json membership = %v, manifest membership = %v", actualNames, wantNames)
	}
	for i := range actualNames {
		if actualNames[i] != wantNames[i] {
			t.Fatalf("versions/*.json membership = %v, manifest membership = %v", actualNames, wantNames)
		}
	}
}

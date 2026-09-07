# UWS Browser Registration Call Supplement 1.1

`uws.browser-registration-call.1.1` selects a registration 1.1 recipe with one
runtime-owned private input binding. Its normative JSON Schema is
[`browser-registration-call.1.1.json`](browser-registration-call.1.1.json).
The 1.0 supplement and its existing validation API retain their original meaning.

```yaml
x-uws-operation-profile: uws.browser-registration-call.1.1
x-uws-browser-registration:
  profile: browser-registration/private-form.json
  flow: advertiser
  credentialBindings:
    identifier: dedicated_test_identifier
    password: dedicated_test_password
  inputBinding: dedicated_registration_input
  approval: register_dedicated_test_user
  duplicatePrevention: operator_attestation
  onDuplicate: fail
  ambiguousOutcome: stop_without_retry
  cleanupDisposition: delete_separately
```

`profile` is a canonical safe package-relative path. `flow` must exist in that
exact profile. The credential-binding keys must exactly match the selected
flow's used credential slots. `inputBinding` is a symbolic name privately
resolved by the runtime; paths, inline values, environment contents and actual
account identifiers are forbidden in this envelope.

The accepted private document selects that same flow through `registrationType`
and binds the exact loaded profile bytes through `profileSha256`. The runtime
must source credential bindings and ordinary input fields from that one accepted
snapshot. An arbitrary caller string or JSON revision is not proof of approval.

All [profile 1.1](browser-registration.1.1.md) privacy, readiness, revision,
mutation, success and human-checkpoint requirements apply. Immediate submission
approval binds the actual accepted private snapshot as well as the exact
operation and profile; private digests must not enter portable artifacts or
reduced receipts. Input editing never changes the fixed duplicate, ambiguity or
cleanup policy, and never authorizes retrying a consumed attempt.

## Go validation

Use `schemas.ValidateBrowserRegistrationCallSupplementForProfile(data,
"uws.browser-registration-call.1.1")` for the envelope shape and safe path.
Use `schemas.ValidateBrowserRegistrationCallBinding(profileBytes, callBytes)`
for the profile/flow and exact credential-slot linkage. The caller must first
load `profileBytes` from the selected package's bound path; this helper performs
no filesystem access and cannot attest which file the caller read.

`ValidateBrowserRegistrationCallSupplement(data)` continues selecting 1.0;
the call envelope has no discriminator, so its version cannot be guessed from
the payload. Existing schema lookup defaults and `browserregistration.ProfileName`
and `CallProfileName` also remain 1.0. New tooling selects the explicit 1.1
names or `ProfileNameV11` / `CallProfileNameV11`.

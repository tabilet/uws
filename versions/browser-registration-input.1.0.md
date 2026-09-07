# Private UWS Registration Input Document 1.0

This is an owner-private data envelope used by registration profile 1.1.
Its [schema](browser-registration-input.1.0.json) is portable; filled instances
MUST remain outside packages, source control, model prompts, command arguments,
logs and reduced reports. UWS does not read files, resolve secrets or execute
registration through this document.

The envelope contains exactly:

| Field | Meaning |
| --- | --- |
| `version` | `uws.browser-registration-input.1.0` |
| `profileSha256` | Lowercase SHA-256 of the exact public profile bytes. |
| `registrationType` | One selected flow key, for example `advertiser` or `publisher`. |
| `revision` | Positive integer, at most 9007199254740991; strictly increases after each accepted snapshot. |
| `values` | Private scalar values keyed by the selected flow's credential/input slots. |

Use one envelope per registration attempt/type. A local UI may display multiple
types together, but it must extract and bind exactly one envelope for execution.
Each field's label, type, required/optional/conditional status and validation
rules are in the public profile. `null` or absence means unfilled; a checkbox's
`false` and a number's zero are actual values. Null placeholders make templates
editable but do not satisfy required inputs at their checkpoint.

Future-step values may be prepared in advance. Every supplied non-null value
must already satisfy its field's type and constraints, and no undeclared or
other-flow fields may be included. Required values are enforced for the current
checkpoint, allowing later required fields to remain null until that step.
Credential strings must be nonempty; ordinary strings obey their declared
length limits. Conditions use only reviewed public enum choices. When inactive,
a conditional field may retain a draft value but is not applied to the browser.

## Acceptance and revision protocol

1. Generate a draft from the exact reviewed profile and chosen registration type.
2. Let the owner edit it privately. Initial editing has no operation timeout.
3. At explicit activation, atomically read a complete snapshot and validate it
   for the current checkpoint. Reject malformed JSON, duplicate keys, unknown
   fields, mismatched bindings and incomplete required input.
4. Retain the accepted immutable snapshot privately. Later edits are drafts
   until explicitly applied at a named input checkpoint with a higher revision.
5. Only slots named in that checkpoint may change. Once a credential has a
   bound non-null value, no subsequent checkpoint may change or remove it.
   Changing registration identity requires separate account/attempt handling;
   a new filename or revision does not grant that authority.
6. Keep the accepted snapshot fixed throughout each fill and submission. Before
   submission, bind the exact decision to the accepted private revision again.
   File edits do not silently affect executing actions.

The runtime owns checkpoint sequencing, prior accepted state, operator
activation, exclusive attempt ownership, deadlines and the persistent identity
ledger. A validator success is not evidence that those runtime steps occurred.
Server-side session expiry is not paused by draft editing. A timeout, manual
submission or uncertain outcome requires state reconciliation before further
action; this input format supplies no retry mechanism.

## Privacy and storage

Use owner-only storage outside Git and package stores: directories `0700`, files
`0600` on Unix, or equivalent access controls elsewhere. The runtime must reject
unsafe ownership/permissions and symlink redirection, read complete snapshots
without racing partial writes, and preserve necessary attempt history privately.
Do not silently edit a credential file already bound to historical attempts.

Actual values and private snapshot hashes must not appear in portable artifacts
or ordinary errors. Validation errors intentionally omit supplied keys, values
and nested parser/schema diagnostics. These checks do not encrypt the file or
control the caller's filesystem, logging, synchronization or backup behavior.
CAPTCHA responses, email links, OTPs, cookies and session state are not form
input fields. Handle verification through the existing private human channel.

## Go helpers

```go
draft, err := schemas.BrowserRegistrationInputTemplate(profileBytes, "advertiser")
// The caller writes draft privately and later accepts actual owner input.
err = schemas.ValidateBrowserRegistrationInputUpdate(
    profileBytes, currentSnapshot, previousAcceptedSnapshot,
    "advertiser", "contact",
)
```

The template contains null placeholders for only the selected flow's fields.
Pass `nil` for the previous snapshot only at the first checkpoint. Otherwise
pass the last accepted snapshot, never another unaccepted draft. Keep that
snapshot even when the owner replaces the editable file. The helper validates
bindings, types, required conditions, increasing revisions and edit boundaries;
the trusted runtime separately enforces checkpoint order and activation.

All helpers are browser-free. They do not mint operation packets, start clocks,
modify a ledger, submit a form or manufacture execution evidence.

# UWS Browser Registration Profile 1.1

## Purpose and compatibility

`uws.browser-registration.1.1` adds typed private form inputs, bounded discovery
metadata and explicit input-revision checkpoints to account-registration
recipes. The normative shape is
[`browser-registration.1.1.json`](browser-registration.1.1.json); the semantic
requirements below also apply. Registration profile/call 1.0 and every earlier
numbered document remain unchanged. This extension does not change UWS core.

An executor MUST explicitly support this version before running it. A 1.0
executor MUST reject it, without executing a partial recipe. UWS supplies
contracts, inert wire types, template generation and validation; browser
discovery, input storage, UI and execution require implementing runtimes.

## Registration types and discovery

Each key in `flows` names a registration type, such as `advertiser` or
`publisher`. The operation selects exactly one flow. Each flow has its own
reviewed sequence and success condition; sharing a profile does not authorize
executing every flow or registering an identity more than once.

The optional `discovery` object records:

- `entryPoints`: bounded, reviewed URLs on declared origins;
- `coverage`: `partial` or `owner_reviewed`;
- `limitations`: any applicable `authentication_required`,
  `invitation_required`, `conditional_flow`, `unreachable_page`, `unknown_routes`.

This is an observation inventory, not a crawler instruction or a guarantee of
completeness. Even a human or semantic browser agent cannot infer an unlinked
invitation route from pages that never reveal it. `owner_reviewed` records actual
owner review of the inventory for the stated environment; it does not mean all
possible accounts, invitations, locales, feature flags or future routes were
enumerated. Empty limitations does not establish completeness. Discovery can
be strengthened by authorized review of application routes and configuration.
Provenance and timestamps continue to live in `evidence` and `verification`.

## Public definitions and private values

A profile may contain definitions, public form labels and option choices,
symbolic slot names, exact origins, reviewed locators, fixed macros and success
predicates. Account identifiers, passwords, contact details, user selections,
verification values, session state and raw observations MUST NOT appear in the
profile, operation envelope or package. Do not place user data in descriptions,
labels, locators, enum choices or discovery metadata.

`credentialSlots` retains its existing `identifier` and `password` kinds.
`inputSlots` declares other private form fields. The two namespaces MUST be
disjoint and together contain at most 64 slots. At least one input slot is
required in 1.1. Each input slot has:

- `type`: `string`, `boolean`, `integer` or `number`;
- `label`: a public, non-personal field label;
- exactly one of `required: true`, `required: false`, or `requiredWhen`;
- optionally, `enum` containing public scalar choices;
- optionally, `minLength`/`maxLength` for strings or `minimum`/`maximum` for numbers.

Strings are bounded to 4096 Unicode characters. Numbers are finite and within
the interoperable range ±9007199254740991; integers have no fractional part.
Enum members must have the declared type and satisfy the field constraints.
Constraints must be well typed and their lower bounds must not exceed their
upper bounds. Defaults, supplied values, arbitrary expressions, arbitrary
regular expressions and executable validation hooks are absent.

For example, these are definitions, not filled data:

```yaml
inputSlots:
  account_kind:
    type: string
    label: Account kind
    required: true
    enum: [individual, business]
  company:
    type: string
    label: Company name
    requiredWhen: {slot: account_kind, equals: business}
    minLength: 1
  phone:
    type: string
    label: Phone
    required: false
```

`requiredWhen` may reference only an unconditional, required input with an
enumerated public choice. Its `equals` must be one of those choices. Credential
comparisons, self references, conditional dependency chains and cycles are
forbidden. When the condition matches, the dependent input is active and
required. Otherwise it is inactive and its `fill_input` MUST be skipped, even
if an earlier draft retains a value. A runtime MUST clear a previously applied
value if a condition changes to inactive or an optional value is removed; if
the control cannot be cleared or its state established, the run stops. Missing
optional inputs that have never been applied are skipped; `false` and zero are
supplied values, not missing values.

## Input checkpoints and browser controls

1.1 adds two closed step macros:

```yaml
- input_checkpoint:
    id: contact
    slots: [company, phone]
- fill_input:
    slot: company
    control: fill
    locator: {role: textbox, name: Company name}
```

`input_checkpoint` names a unique checkpoint within the selected flow and the
slots that may be accepted or revised there. The first step MUST be an input
checkpoint, before browser activity. Every credential or form fill MUST follow
a checkpoint that loaded its slot. Every checkpoint slot must have a fill in
that flow, and its values must be applied before advancing to another input
checkpoint or submitting. A conditional dependency must already be loaded or
be loaded in the same checkpoint. Revising a loaded condition input requires
reloading and reapplying its loaded dependent fields in that checkpoint.

The runtime presents readiness explicitly and waits for actual activation. No
live operation expiry or browser-session timeout begins while the initial draft
is being edited. Only after activation and validation does the runtime prepare
the exact approved operation and begin browser work. A ready file, file change
or elapsed time is not activation, consent or a human verification response.

Later input checkpoints accept a new revision only on explicit application by
the operator. They do not suspend a website's session expiry or override runtime
deadlines. If the current session cannot safely continue, stop and reconcile
its state; opening another file or changing a revision never authorizes replay.

`fill_input` resolves its symbolic slot from the accepted private snapshot:

| Control | Slot type | Meaning |
| --- | --- | --- |
| `fill` | string, integer, number | Fill one exact editable control; serialize numbers as locale-independent JSON numbers. |
| `check` | boolean | Set the checkbox or switch to the supplied state; never blindly toggle it. |
| `select` | string with enum | Select the exact declared option value in a supported select control. |

Credential values continue through `type_credential`, including password
confirmation by repeating the same slot. Unknown, ambiguous or unsupported
controls stop execution. File uploads and arbitrary widgets are not modeled by
these macros. CAPTCHA, OTP/email verification, MFA, consent and challenges stay
in ordinary `human_checkpoint` handling; do not disguise them as input slots.
Both checkpoint kinds require the existing `requires_human_verification` effect.

## Multi-step registration and mutation boundaries

Several input checkpoints and read-only wizard transitions may precede the one
account-creation `submit`. A `click` is still a non-submission control; it cannot
hide a POST, server-side draft update, email send or intermediate account creation.
This version does not authorize arbitrary multi-POST registration protocols.
Those flows need separately modeled mutation semantics; unsupported flows stop.

Exactly one `submit`, immediate exact approval, fail-on-duplicate handling,
stop-without-retry on ambiguity, the preselected cleanup disposition and ordinary
human verification remain mandatory. No input checkpoint or fill is allowed
after submit. Post-submit verification may proceed through the existing human
checkpoints. Revising inputs does not reopen a consumed or uncertain attempt.

Unexpected new fields require a newly reviewed profile and a new private draft
bound to that profile. Do not silently add a field to the executing package,
relax validation or replay prior steps. A runtime may carry forward appropriate
private draft values only after explicit reconciliation of progress and identity.

## Private document and runtime bindings

The separate private input envelope is defined in
[`browser-registration-input.1.0.md`](browser-registration-input.1.0.md).
It contains a `registrationType`, exact `profileSha256`, increasing `revision`
and private `values`. Labels and optional/conditional definitions remain in the
profile, so a local UI can generate a form or an editable JSON template.

The [1.1 call supplement](browser-registration-call.1.1.md) adds a symbolic
`inputBinding`. Its value is neither a path nor an inline document. A trusted
runtime privately maps that name to owner-supplied input. The same accepted
snapshot must supply both the credential bindings and ordinary field values.
An executor must not mix credentials from another file revision or provider.

Before submission, the runtime binds its exact approval to the profile, flow,
operation, accepted input revision and private snapshot identity, credential
bindings, origins and existing duplicate/cleanup controls. Any accepted input
change invalidates an earlier submission decision; the exact decision must be
rebound under existing authority immediately before submit. Keep private input
digests private as well: they can reveal low-entropy personal information.

Success remains presence of the reviewed outcome. Input values, tokens, cookies
and browser sessions are never workflow outputs or reduced execution evidence.

# Registration authoring boundaries

UWS describes reviewed registration recipes: selected flows, symbolic credential
slots, typed input definitions, steps and success conditions. Discovery is an
authoring activity. OpenUdon owns its private candidate inventory, coverage
limitations, owner corrections and review history; Browsertools supplies
observations. An observed page is a candidate, not proof of a working recipe.

New authoring tools should export registration profiles without `discovery`.
Keep coverage (`unknown` or `partial`) separate from owner review (`pending` or
`reviewed`) in private authoring state. Neither review nor an empty limitation
list guarantees discovery of every route: authentication, invitations,
conditional content and unknown routes can hide additional registration types.

The published registration 1.1 schema and Go wire model remain unchanged.
Existing profiles containing the optional `discovery` member remain valid;
its historical `coverage: owner_reviewed` value never promises completeness.
Preserve such profiles through ordinary parsing and round trips. Do not strip
metadata from an already reviewed or digest-bound profile; author a new profile
and repeat its applicable reviews if changing those bytes is intended.

Registration types are named flows. Private input envelopes bind the selected
flow and exact profile digest. Filled values, including optional fields and
checkpoint updates, remain outside packages, Git, prompts and logs. Discovery
review does not authorize target contact, submission, retries or account cleanup.
This guidance does not introduce a crawler, alter version defaults, or add typed
input execution to consumers that have not adopted registration 1.1.

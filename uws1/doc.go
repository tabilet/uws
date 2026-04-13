// Package uws1 provides Go types for parsing and generating
// Udon Workflow Specification (UWS) 1.x documents.
//
// The types in this package are standalone data-interchange types
// with JSON, YAML, and HCL serialization. They support
// multi-service operations (HTTP, SSH, Cmd, Fnct), execution
// control (depends_on, when, for_each, wait, workflow,
// parallel_group), structural constructs (parallel, switch,
// merge, loop), triggers, security, and specification
// extensions (x-* fields).
package uws1

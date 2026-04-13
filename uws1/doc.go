// Package uws1 provides Go types for parsing and generating
// Udon Workflow Specification (UWS) 1.x documents.
//
// The types in this package are standalone data-interchange types
// with JSON serialization and struct tags for YAML/HCL conversion helpers. They support
// multi-service operations (HTTP, SSH, Cmd, Fnct), execution
// control (depends_on, when, for_each, wait, workflow,
// parallel_group), structural constructs (parallel, switch,
// merge, loop), triggers, security, and JSON specification
// extensions (x-* fields). Use the convert package for extension-preserving YAML
// conversion and HCL helper APIs.
package uws1

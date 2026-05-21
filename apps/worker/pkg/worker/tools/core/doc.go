// Package core defines the single tool abstraction used by the worker
// runtime ("道A"). A builtin tool (knowledge_tree etc., which calls process.X)
// and a dynamic tool (a promoted transform run via transform.Engine) are the
// SAME type. The orchestrator and the registry never see the builtin/dynamic
// distinction; ADK only sees adapter-wrapped versions of these Tools.
//
// Design rationale (kept here because the eval-multitool design doc was
// deleted — "the contract is the Go type"):
//
//   - Every tool is unified as "input JSON -> output JSON + io_schema". A
//     builtin tool's Go-typed output is JSON-marshalled so all tool wiring runs
//     on JSON uniformly. This removes per-tool Args/Result unions and lets the
//     adapter layer be schema-agnostic.
//   - io_schema is mandatory for every tool. builtin schemas are derived from
//     their Go types (jsonschema.For); dynamic tools carry io_schema in the DB.
//   - builtin vs dynamic differ only in registration path (static registry
//     vs runtime DB resolution); from the orchestrator's view the type is
//     identical.
//   - No Go interface. Every tool's run logic is a single function with the
//     same signature, so polymorphism is unneeded; a data record + a value
//     function (RunFn) is the whole abstraction. An interface with one
//     implementer was deliberately rejected as over-abstraction.
package core

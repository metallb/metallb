// Generator of Go structs out of the VPP binary API definitions in JSON format.
//
// The JSON input can be specified as a single file (using the `input-file`
// CLI flag), or as a directory that will be scanned for all `.json` files
// (using the `input-dir` CLI flag). The generated Go bindings will  be
// placed into `output-dir` (by default the current working directory),
// where each Go package will be placed into its own separate directory,
// for example:
//
//    binapi-generator --input-file=examples/bin_api/acl.api.json --output-dir=examples/bin_api
//    binapi-generator --input-dir=examples/bin_api --output-dir=examples/bin_api
//
package main

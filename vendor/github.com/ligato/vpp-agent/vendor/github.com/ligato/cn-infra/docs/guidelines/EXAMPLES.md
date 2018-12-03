# Examples

Put all examples under [/examples/*](../../examples) folder.
This folder should contain minimalistic examples, each ideally focused
on a single feature only.

## Layout

Each example is stored in its own directory under
the [examples](../../examples) folder. From the name of the directory
it should be clear what CN-Infra feature is being presented.
Additionally, suffix the directory name with "-plugin" if the example
runs agent with plugins, or with "-lib" if the demonstrated
functionality is packaged as a (lower-level) library and the example
itself is a flat procedural code (not leveraging the plugin-based
infrastructure).

Complete example source code together with all files needed to run
the example should be present in the directory. Even optional
configuration files should be included with a default or a very common
content. Furthermore, add the "doc.go" file with at least one
single-line comment describing the package for godoc. Instruction to run
the example should be available as Readme.md.

### Plugin-based examples

It is preferred to create each example as a new instance of the agent
with a custom or a re-used flavor extended with a new plugin called
`ExamplePlugin`. The presented code snippets should be put into `Init()`
and/or `AfterInit()` methods of the `ExamplePlugin`. Background tasks
should be demonstrated in the form of Go routines started during the
initialization of `ExamplePlugin`.

Split the example source code into two go files:
 - `main.go`: contains (precisely in this order):
   1. main function
   2. ExamplePlugin struct declaration
   3. ExamplePlugin.Init()
   4. ExamplePlugin.AfterInit()
   5. go routines
   6. Close()
   7. Methods called from within Close (if there are any)
   8. any helper methods which are non-crucial from the demonstration
      point of view

 - `deps.go`: contains (precisely in this order):
   1. Dep struct declaration
   2. Flavor struct declaration
   3. Flavor.Inject()
   4. Flavor.Plugins()
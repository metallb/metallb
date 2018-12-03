# Coding style

# General Rules
- Use `gofmt` to format all source files.
- Address any issues that were discovered by the `golint` & `govet` tools.
- Follow recommendations in [effective go][1] and [Go Code Review Comments][2].
- Please make sure that each dependency in the `Gopkg.toml` has a specific
  `version` or `revision` defined. For more info read [documentation for dep][3].

# Go Channels & go routines
See [Plugin Lifecycle](PLUGIN_LIFECYCLE.md)

# Func signature
## Arguments
If there is more than one return argument (in a structure or in an interface),
always use a name for each primitive type.

Correct:
```
func Method() (found bool, data interface{}) {
    // method body ...
}
```
 
Wrong:
```
func Method() (bool, interface{}) {
    // method body ...
}
```

# Errors

Avoid using `errors.New()` and `fmt.Errorf()` to create error instances.
For errors created through these methods it is impractical to write
error handling code that threats each error differently. The handler
needs to compare error by the error message (returned by `error.Error()`),
which may be long and change over time. The same problem arises when
the error needs to be referenced in the documentation.
The parameters of errors created with `fmt.Errorf()` can be dissected
only by parsing the error message, which is slow and also needs
an update whenever the error message changes.


We recommend to declare each new error as a struct type implementing
the error interface, e.g.:

```go
// UnavailableMicroserviceErr is error returned when a given microservice is not deployed.
type UnavailableMicroserviceErr struct {
    Label string
}

func (e *UnavailableMicroserviceErr) Error() string {
    return fmt.Sprintf("Microservice '%s' is not available", e.label)
}


func GetMicroservice(label string) (Microservice, error) {
  ...
  return nil, UnavailableMicroserviceErr{label}
}
```

This allows you to easily check for a specific error by its type:
```go
if unavailMs, ok := err.(UnavailableMicroserviceErr); ok {
  // Handle UnavailableMicroserviceErr
  // Can easily access the error parameter (microservice label in this case)
  missingMsLabel := unavailMs.Label
} else {
  // Handle all other error types
}
```

Please document all important/frequent error scenarios for at least
public API. Reference errors by their type names.

[1]: https://golang.org/doc/effective_go.html
[2]: https://github.com/golang/go/wiki/CodeReviewComments
[3]: https://golang.github.io/dep/docs/introduction.html

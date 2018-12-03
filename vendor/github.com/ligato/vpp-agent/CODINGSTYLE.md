# Coding style

- Use `gofmt` to format all source files.
- Address any issues that were discovered by the `golint` & `govet` tool.
- Follow recommendations in [effective go][1] and [Go Code Review Comments][2].
- Please make sure that each dependency in the `Gopkg.toml` has a specific
  `version` or `revision` defined. For more info read [documentation for dep][3].

[1]: https://golang.org/doc/effective_go.html
[2]: https://github.com/golang/go/wiki/CodeReviewComments
[3]: https://golang.github.io/dep/docs/introduction.html
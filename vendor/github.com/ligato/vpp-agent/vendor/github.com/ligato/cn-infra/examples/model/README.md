### Protobuf Messages and Go Data Structures
The Agent build tools support easy integration of protobuf data format
definitions into your program.
 
1. Define your data formats as a set of messages in a google protobuf
file. For a simple example, see `examples/model/example.proto`.
1. In the plugin's main file (for example, `etcd/main.go`), add 
the following line at the top of the file:
```apple js
  // go:generate protoc --proto_path=examples/model --gogo_out=examples/model examples/model/example.proto
```
3. You can have multiple files with protobuf message definitions. Add a
similar line for each file.
3. The above line will instruct the go tool to use the `protoc` code 
generator to generate go structures from protobuf messages defined in
the [`example.proto`](example.proto) file. The line also specifies
the path to the file and the location where to put the generated
structures (in our example, all are the same, `examples/model`).
1. Do `make generate` to generate go structures from protobuf message 
definitions. The structures will be put into
[`example.pb.go`](example.pb.go) in the same folder.
The structures will be annotated with protobuf annotations to support
marshalling/un-marshalling at run-time.

You can now use the generated go structures in your code.
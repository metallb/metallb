# Process manager example

The example is divided into three scenarios:
* **basic-scenario:** simple process managing like creating a process, start, stop, restart or status watching
* **advanced-scenario:** status handling as attaching to running processes, reading of process status file 
or automatic restarts
* **templates:** example how to create and use the process template

All the examples use [test-application](test-process/test-process.go) called test-process, which is managed during
the example.

Note: in order to run the example which handles templates, process manager config file needs to be provided
to the example, otherwise it will be skipped.

```
./process-manager-plugin -process-manager-config=<path-to-file>
```

All about process manager config file can be found in process manager [readme](../../process/README.md#Templates)


# Extensibility
The following code snippet illustrates how to start a new flavor of plugins.
The complete code can be found [here](https://github.com/ligato/cn-infra/blob/master/examples/simple-agent/agent.go).

```
func main() {
	logroot.StandardLogger().SetLevel(logging.DebugLevel)

	connectors := connectors.AllConnectorsFlavor{}
	rpcs := rpc.FlavorRPC{}
	agent := core.NewAgent(logroot.StandardLogger(), 15*time.Second, append(
		connectors.Plugins(), rpcs.Plugins()...)...)

	err := core.EventLoopWithInterrupt(agent, nil)
	if err != nil {
		os.Exit(1)
}
```

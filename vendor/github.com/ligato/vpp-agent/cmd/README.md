# Executables

This package groups executables that can be built from sources in the VPP 
Agent repository:

- [vpp-agent](vpp-agent/main.go) - the default off-the-shelf VPP Agent 
  executable (i.e. no app or extension plugins) that can be bundled with
  an off-the-shelf VPP to form a simple cloud-native VNF,
  such as a vswitch.
- [agentctl](agentctl/agentctl.go) - CLI tool that allows to show
  the state and to configure VPP Agents connected to etcd
- [vpp-agent-ctl](vpp-agent-ctl) - a utility for testing VPP
  Agent configuration. It contains a set of hard-wired configurations
  that can be invoked using command line flags and sent to the VPP Agent.
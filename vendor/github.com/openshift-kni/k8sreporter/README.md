# Kubernetes Reporter

This is a small library to produce a readeable dump of the required information contained in a k8s cluster.

The idea is to use it after an end to end test failure, to collect information useful for the failure.

## Usage

Init the reporter with the information about namespaces and the resources that requires to be dumped:

```go
 // When using custom crds, we need to add them to the scheme
 addToScheme := func(s *runtime.Scheme) error {
  err := sriovv1.AddToScheme(s)
  if err != nil {
   return err
  }
  err = metallbv1beta1.AddToScheme(s)
  if err != nil {
   return err
  }
  return nil
 }

  // The namespaces we want to dump resources for (including pods and pod logs)
  dumpNamespace := func(ns string) bool {
      if strings.HasPrefix(ns, "test") {
          return true
      }
      return false
  }

 // The list of CRDs we want to dump
 crds := []k8sreporter.CRData{
  {Cr: &sriovv1.SriovNetworkNodePolicyList{}},
  {Cr: &sriovv1.SriovNetworkList{}},
  {Cr: &sriovv1.SriovNetworkNodePolicyList{}},
  {Cr: &sriovv1.SriovOperatorConfigList{}},
  {Cr: &metallbv1beta1.MetalLBList{}},
 }
```

Create the reporter and invoke dump (note reportbase must exists):

```go
 reporter, err := k8sreporter.New(*kubeconfig, addToScheme, dumpNamespace, "/reportbase", crds...)
 if err != nil {
  log.Fatalf("Failed to initialize the reporter %s", err)
 }
 reporter.Dump(10*time.Minute, "nameofthetest")
```

The output will look like

```bash
├── reportbase
│   └── test
│       ├── crs.log
│       ├── metallb-system-pods_logs.log
│       ├── metallb-system-pods_specs.log
│       └── nodes.log
```

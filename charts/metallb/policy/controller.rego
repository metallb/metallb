package main

# validate serviceAccountName
deny[msg] {
  input.kind == "Deployment"
  serviceAccountName := input.spec.template.spec.serviceAccountName
  not serviceAccountName == "RELEASE-NAME-metallb-controller"
  msg = sprintf("controller serviceAccountName '%s' does not match expected value", [serviceAccountName])
}

# validate config map name in container args
deny[msg] {
  input.kind == "Deployment"
  configArg := input.spec.template.spec.containers[0].args[1]
  not configArg == "--config=RELEASE-NAME-metallb"
  msg = sprintf("controller ConfigMap arg '%s' does not match expected value", [configArg])
}

# validate node selector includes builtin when custom ones are provided
deny[msg] {
  input.kind == "Deployment"
  not input.spec.template.spec.nodeSelector["kubernetes.io/os"] == "linux"
  msg = "controller nodeSelector does not include '\"kubernetes.io/os\": linux'"
}

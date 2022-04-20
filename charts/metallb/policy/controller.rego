package main

# validate serviceAccountName
deny[msg] {
  input.kind == "Deployment"
  serviceAccountName := input.spec.template.spec.serviceAccountName
  not serviceAccountName == "RELEASE-NAME-metallb-controller"
  msg = sprintf("controller serviceAccountName '%s' does not match expected value", [serviceAccountName])
}

# validate node selector includes builtin when custom ones are provided
deny[msg] {
  input.kind == "Deployment"
  not input.spec.template.spec.nodeSelector["kubernetes.io/os"] == "linux"
  msg = "controller nodeSelector does not include '\"kubernetes.io/os\": linux'"
}

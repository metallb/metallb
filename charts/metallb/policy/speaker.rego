package main

# validate serviceAccountName
deny[msg] {
  input.kind == "DaemonSet"
  serviceAccountName := input.spec.template.spec.serviceAccountName
  not serviceAccountName == "RELEASE-NAME-metallb-speaker"
  msg = sprintf("speaker serviceAccountName '%s' does not match expected value", [serviceAccountName])
}

# validate METALLB_ML_SECRET_KEY (memberlist)
deny[msg] {
	input.kind == "DaemonSet"
	not input.spec.template.spec.containers[0].env[5].name == "METALLB_ML_SECRET_KEY"
	msg = "speaker env does not contain METALLB_ML_SECRET_KEY at env[5]"
}

deny[msg] {
	input.kind == "DaemonSet"
	not input.spec.template.spec.containers[0].env[5].valueFrom.secretKeyRef.name == "RELEASE-NAME-metallb-memberlist"
	not input.spec.template.spec.containers[0].env[5].valueFrom.secretKeyRef.key == "secretkey"
	msg = "speaker env METALLB_ML_SECRET_KEY secretKeyRef does not equal expected value"
}

# validate node selector includes builtin when custom ones are provided
deny[msg] {
  input.kind == "DaemonSet"
  not input.spec.template.spec.nodeSelector["kubernetes.io/os"] == "linux"
  msg = "controller nodeSelector does not include '\"kubernetes.io/os\": linux'"
}

# validate tolerations include the builtins when custom ones are provided
deny[msg] {
  input.kind == "DaemonSet"
  not input.spec.template.spec.tolerations[0] == { "key": "node-role.kubernetes.io/master", "effect": "NoSchedule", "operator": "Exists" }
  msg = "controller tolerations does not include node-role.kubernetes.io/master:NoSchedule"
}

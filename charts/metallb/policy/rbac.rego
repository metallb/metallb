package main

# Validate PSP exists in ClusterRole :controller
deny[msg] {
  input.kind == "ClusterRole"
  input.metadata.name == "metallb:controller"
  input.rules[3] == {
	"apiGroups": ["policy"],
	"resources": ["podsecuritypolicies"],
	"resourceNames": ["metallb-controller"],
	"verbs": ["use"]
  }
  msg = "ClusterRole metallb:controller does not include PSP rule"
}

# Validate PSP exists in ClusterRole :speaker
deny[msg] {
  input.kind == "ClusterRole"
  input.metadata.name == "metallb:speaker"
  input.rules[3] == {
	"apiGroups": ["policy"],
	"resources": ["podsecuritypolicies"],
	"resourceNames": ["metallb-controller"],
	"verbs": ["use"]
  }
  msg = "ClusterRole metallb:speaker does not include PSP rule"
}

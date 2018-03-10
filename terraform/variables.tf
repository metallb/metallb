variable "gcp_project" {
  type = "string"
  default = "metallb-e2e-testing"
}

variable "gcp_zone" {
  type = "string"
  default = "us-central1-b"
}

variable "gcp_machine_type" {
  type = "string"
  default = "n1-standard-4"
}

variable "root_ssh_key_file" {
  type = "string"
  default = "~/.ssh/google_compute_engine.pub"
}

variable "ipv4_machine_cidr" {
  type = "string"
  default = "192.168.210.0/24"
}

variable "ipv6_machine_cidr" {
  type = "string"
  default = "fc00:236::/120"
}

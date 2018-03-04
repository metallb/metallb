variable "gcp_project" {
  type = "string"
  default = "metallb-e2e-testing"
}

variable "gcp_zone" {
  type = "string"
  default = "us-west1-a"
}

variable "gcp_machine_type" {
  type = "string"
  default = "n1-standard-4"
}

variable "root_ssh_key_file" {
  type = "string"
  default = "~/.ssh/google_compute_engine.pub"
}

variable "kubeadm_token" {
  type = "string"
  default = "8fa246.5abb934bdd5cce9d"
}

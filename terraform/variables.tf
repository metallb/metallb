# GCP configs
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
  default = "n1-standard-1"
}

# VM stuff
variable "cluster_name" {
  type = "string"
  default = "k8s"
}

variable "ssh_key_file" {
  type = "string"
  default = "~/.ssh/id_rsa.pub"
}

variable "machine_cidr" {
  type = "string"
  default = "192.168.0.0/24"
}

# k8s cluster data
variable "pod_cidr" {
  type = "string"
  default = "192.168.128.0/17"
}

variable "service_cidr" {
  type = "string"
  default = "192.168.1.0/24"
}

variable "kubeadm_token" {
  type = "string"
  default = "8fa246.5abb934bdd5cce9d"
}

locals {
  ssh_key = "${file(pathexpand(var.ssh_key_file))}"
}

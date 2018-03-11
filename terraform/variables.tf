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

# k8s cluster data
variable "kubeadm_token" {
  type = "string"
  default = "8fa246.5abb934bdd5cce9d"
}

variable "protocol" {
  type = "string"
  # IPv6 cluster creation is broken until we have a kubeadm that
  # includes https://github.com/kubernetes/kubernetes/pull/58769
  default = "ipv4"
}

locals {
  ssh_key = "${file(pathexpand(var.ssh_key_file))}"

  machine_cidr = "${local.machine_cidrs[var.protocol]}"
  pod_cidr = "${local.pod_cidrs[var.protocol]}"
  service_cidr = "${local.service_cidrs[var.protocol]}"

  machine_cidrs = {
    "ipv4" = "192.168.0.0/24"
    "ipv6" = "fd00:1::/120"
  }
  pod_cidrs = {
    "ipv4" = "192.168.128.0/17"
    "ipv6" = "fd00:2::/80"
  }
  service_cidrs = {
    "ipv4" = "192.168.1.0/24"
    "ipv6" = "fd00:3::/112"
  }
}

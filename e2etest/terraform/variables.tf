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

variable "network_addon" {
  type = "string"
  default = "flannel"
}

locals {
  ssh_key = "${file(pathexpand(var.ssh_key_file))}"

  machine_cidrs = "${local.machine_cidrs_by_proto[var.protocol]}"
  pod_cidr = "${local.pod_cidrs[var.protocol]}"
  service_cidr = "${local.service_cidrs[var.protocol]}"
  network_addon_file = "network-addons/${local.network_addons[var.network_addon]}"

  machine_cidrs_by_proto = {
    "ipv4" = ["192.168.0.0/24", "192.168.1.0/24"]
    "ipv6" = ["fd00:1::/120", "fd00:2::/120"]
  }
  pod_cidrs = {
    "ipv4" = "192.168.128.0/17"
    "ipv6" = "fd00:2::/80"
  }
  service_cidrs = {
    # This might overlap with GCP's VPCs, but in practice it happens
    # not to. And several network addons assume that the service range
    # is this in their manifests, so it's simpler to just conform for
    # now.
    "ipv4" = "192.168.1.0/24"
    "ipv6" = "fd00:3::/112"
  }
  network_addons = {
    "flannel" = "flannel-0.9.1.yaml"
    "calico" = "calico-3.0.yaml"
    "romana" = "romana-2.0.2.yaml"
    "weave" = "weave-2.2.0.yaml"
  }
}

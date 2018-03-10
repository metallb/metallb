variable "cluster_name" {}
variable "machine_cidr" {}
variable "pod_cidr" {}
variable "ssh_key" {}
variable "bastion_ip" {}
variable "kubeadm_token" {
  type = "string"
  default = "8fa246.5abb934bdd5cce9d"
}

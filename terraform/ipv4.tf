module "k8s_cluster_ipv4" {
  source = "./k8s_cluster"
  cluster_name = "ipv4"
  machine_cidr = "${var.ipv4_machine_cidr}"
  pod_cidr = "10.0.0.0/8"
  ssh_key = "${file(pathexpand(var.root_ssh_key_file))}"
  bastion_ip = "${google_compute_instance.virt_host.network_interface.0.access_config.0.assigned_nat_ip}"
}

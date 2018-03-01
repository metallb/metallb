provider "google" {
  project = "${var.gcp_project}"
}

provider "libvirt" {
  #uri = "qemu+ssh://root@35.224.116.105/system"
  uri = "qemu+ssh://root@${google_compute_instance.virt_host.network_interface.0.access_config.0.assigned_nat_ip}/system"
}

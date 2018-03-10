resource "libvirt_volume" "node2" {
  name = "${var.cluster_name}_node2.qcow2"
  base_volume_name = "fedora.qcow2"
  base_volume_pool = "default"
}

resource "libvirt_cloudinit" "node2" {
  name = "${var.cluster_name}_node2_cloudinit"
  ssh_authorized_key = "${var.ssh_key}"
  local_hostname = "node2"
}

resource "libvirt_domain" "node2" {
  depends_on = ["libvirt_domain.node1"]
  name = "${var.cluster_name}_node2"
  vcpu = 4
  memory = "2048"
  autostart = true
  cloudinit = "${libvirt_cloudinit.node2.id}"
  disk {
    volume_id = "${libvirt_volume.node2.id}"
  }
  network_interface {
    network_id = "${libvirt_network.net.id}"
    addresses = ["${cidrhost(var.machine_cidr, 4)}"]
  }

  connection {
    user = "fedora"
    bastion_host = "${var.bastion_ip}"
    bastion_user = "root"
  }

  provisioner "remote-exec" {
    script = "../install_k8s.sh"
  }

  provisioner "remote-exec" {
    inline = "sudo kubeadm join --discovery-token-unsafe-skip-ca-verification --token ${var.kubeadm_token} ${cidrhost(var.machine_cidr, 2)}:6443"
  }
}

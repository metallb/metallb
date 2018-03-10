resource "libvirt_volume" "node1" {
  name = "${var.cluster_name}_node1.qcow2"
  base_volume_name = "fedora.qcow2"
  base_volume_pool = "default"
}

resource "libvirt_cloudinit" "node1" {
  name = "${var.cluster_name}_node1_cloudinit"
  ssh_authorized_key = "${var.ssh_key}"
  local_hostname = "node1"
}

resource "libvirt_domain" "node1" {
  depends_on = ["libvirt_domain.controller"]
  name = "${var.cluster_name}_node1"
  vcpu = 4
  memory = "2048"
  autostart = true
  cloudinit = "${libvirt_cloudinit.node1.id}"
  disk {
    volume_id = "${libvirt_volume.node1.id}"
  }
  network_interface {
    network_id = "${libvirt_network.net.id}"
    addresses = ["${cidrhost(var.machine_cidr, 3)}"]
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

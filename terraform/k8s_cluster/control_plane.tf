resource "libvirt_volume" "controller" {
  name = "${var.cluster_name}_controller.qcow2"
  base_volume_name = "fedora.qcow2"
  base_volume_pool = "default"
}

resource "libvirt_cloudinit" "controller" {
  name = "${var.cluster_name}_controller_cloudinit"
  ssh_authorized_key = "${var.ssh_key}"
  local_hostname = "controller"
}

resource "libvirt_domain" "controller" {
  name = "${var.cluster_name}_controller"
  vcpu = 4
  memory = "2048"
  autostart = true
  cloudinit = "${libvirt_cloudinit.controller.id}"
  disk {
    volume_id = "${libvirt_volume.controller.id}"
  }
  network_interface {
    network_id = "${libvirt_network.net.id}"
    addresses = ["${cidrhost(var.machine_cidr, 2)}"]
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
    inline = "sudo kubeadm init --pod-network-cidr=${var.pod_cidr} --token ${var.kubeadm_token}"
  }
}

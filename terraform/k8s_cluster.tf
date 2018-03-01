resource "libvirt_network" "net" {
  name = "net"
  mode = "nat"
  domain = "k8s.local"
  addresses = ["192.168.236.0/24", "fc00:236::/120"]
  autostart = true
}

##
## Test runner
##
resource "libvirt_volume" "runner" {
  name = "runner.qcow2"
  base_volume_name = "debian.qcow2"
  base_volume_pool = "default"
}

resource "libvirt_cloudinit" "runner" {
  name = "cloudinit_runner"
  ssh_authorized_key = "${file(pathexpand(var.root_ssh_key_file))}"
  local_hostname = "runner"
}

resource "libvirt_domain" "runner" {
  name = "runner"
  depends_on = ["libvirt_domain.k8s_controller", "libvirt_domain.k8s_node1", "libvirt_domain.k8s_node2"]
  memory = "2048"
  autostart = true
  cloudinit = "${libvirt_cloudinit.runner.id}"
  disk {
    volume_id = "${libvirt_volume.runner.id}"
  }
  network_interface {
    network_id = "${libvirt_network.net.id}"
    addresses = ["192.168.236.2"]
    wait_for_lease = 1
  }

  connection {
    user = "debian"
    bastion_host = "${google_compute_instance.virt_host.network_interface.0.access_config.0.assigned_nat_ip}"
    bastion_user = "root"
  }

  provisioner "remote-exec" {
    script = "install_k8s.sh"
  }
}

##
## Kubernetes controller
##
resource "libvirt_volume" "k8s_controller" {
  name = "k8s_controller.qcow2"
  base_volume_name = "debian.qcow2"
  base_volume_pool = "default"
}

resource "libvirt_cloudinit" "controller" {
  name = "cloudinit_controller"
  ssh_authorized_key = "${file(pathexpand(var.root_ssh_key_file))}"
  local_hostname = "controller"
}

resource "libvirt_domain" "k8s_controller" {
  name = "k8s_controller"
  memory = "2048"
  autostart = true
  cloudinit = "${libvirt_cloudinit.controller.id}"
  disk {
    volume_id = "${libvirt_volume.k8s_controller.id}"
  }
  network_interface {
    network_id = "${libvirt_network.net.id}"
    addresses = ["192.168.236.3"]
    wait_for_lease = 1
  }

  connection {
    user = "debian"
    bastion_host = "${google_compute_instance.virt_host.network_interface.0.access_config.0.assigned_nat_ip}"
    bastion_user = "root"
  }

  provisioner "remote-exec" {
    script = "install_k8s.sh"
  }

  provisioner "remote-exec" {
    inline = "sudo kubeadm init --pod-network-cidr=10.250.0.0/16 --token ${var.kubeadm_token}"
  }
}

##
## Kubernetes worker 1
##
resource "libvirt_volume" "k8s_node1" {
  name = "k8s_node1.qcow2"
  base_volume_name = "debian.qcow2"
  base_volume_pool = "default"
}

resource "libvirt_cloudinit" "k8s_node1" {
  name = "cloudinit_node1"
  ssh_authorized_key = "${file(pathexpand(var.root_ssh_key_file))}"
  local_hostname = "node1"
}

resource "libvirt_domain" "k8s_node1" {
  name = "k8s_node1"
  depends_on = ["libvirt_domain.k8s_controller"]
  memory = "2048"
  autostart = true
  cloudinit = "${libvirt_cloudinit.k8s_node1.id}"
  disk {
    volume_id = "${libvirt_volume.k8s_node1.id}"
  }
  network_interface {
    network_id = "${libvirt_network.net.id}"
    addresses = ["192.168.236.4"]
    wait_for_lease = 1
  }

  connection {
    user = "debian"
    bastion_host = "${google_compute_instance.virt_host.network_interface.0.access_config.0.assigned_nat_ip}"
    bastion_user = "root"
  }

  provisioner "remote-exec" {
    script = "install_k8s.sh"
  }

  provisioner "remote-exec" {
    inline = "sudo kubeadm join --discovery-token-unsafe-skip-ca-verification --token ${var.kubeadm_token} 192.168.236.3:6443"
  }
}

##
## Kubernetes worker 2
##
resource "libvirt_volume" "k8s_node2" {
  name = "k8s_node2.qcow2"
  base_volume_name = "debian.qcow2"
  base_volume_pool = "default"
}

resource "libvirt_cloudinit" "k8s_node2" {
  name = "cloudinit_node2"
  ssh_authorized_key = "${file(pathexpand(var.root_ssh_key_file))}"
  local_hostname = "node2"
}

resource "libvirt_domain" "k8s_node2" {
  name = "k8s_node2"
  depends_on = ["libvirt_domain.k8s_controller", "libvirt_domain.k8s_node1"]
  memory = "2048"
  autostart = true
  cloudinit = "${libvirt_cloudinit.k8s_node2.id}"
  disk {
    volume_id = "${libvirt_volume.k8s_node2.id}"
  }
  network_interface {
    network_id = "${libvirt_network.net.id}"
    addresses = ["192.168.236.5"]
    wait_for_lease = 1
  }

  connection {
    user = "debian"
    bastion_host = "${google_compute_instance.virt_host.network_interface.0.access_config.0.assigned_nat_ip}"
    bastion_user = "root"
  }

  provisioner "remote-exec" {
    script = "install_k8s.sh"
  }

  provisioner "remote-exec" {
    inline = "sudo kubeadm join --discovery-token-unsafe-skip-ca-verification --token ${var.kubeadm_token} 192.168.236.3:6443"
  }
}

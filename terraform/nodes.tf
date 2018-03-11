resource "google_compute_instance" "nodes" {
  count = 2
  name = "${var.cluster_name}-node-${count.index}"
  machine_type = "${var.gcp_machine_type}"
  zone = "${var.gcp_zone}"

  boot_disk {
    initialize_params {
      image = "debian-cloud/debian-9"
      size = 10
      type = "pd-ssd"
    }
  }

  metadata {
    sshKeys = "root:${local.ssh_key}"
    startup-script = <<EOF
#!/bin/bash
perl -pi -e 's/PermitRootLogin no/PermitRootLogin prohibit-password/g' /etc/ssh/sshd_config
systemctl restart ssh.service
EOF
  }

  network_interface {
    network = "default"
    access_config {}
  }

  provisioner "remote-exec" {
    script = "install_k8s.sh"
  }

  provisioner "file" {
    source = "configure_vpn.sh"
    destination = "/tmp/configure_vpn.sh"
  }

  provisioner "remote-exec" {
    inline = [
      "bash /tmp/configure_vpn.sh access ${cidrhost(var.machine_cidr, 3+count.index)} ${element(split("/", var.machine_cidr), 1)} ${google_compute_instance.switch.network_interface.0.address}",
      "kubeadm join --discovery-token-unsafe-skip-ca-verification --token ${var.kubeadm_token} ${cidrhost(var.machine_cidr, 1)}:6443",
    ]
  }
}

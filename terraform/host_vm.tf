data "google_compute_image" "debian_vmx" {
  name = "debian-vmx"
}

resource "google_compute_instance" "virt_host" {
  name = "virt-host"
  machine_type = "${var.gcp_machine_type}"
  zone = "${var.gcp_zone}"

  boot_disk {
    initialize_params {
      image = "${data.google_compute_image.debian_vmx.self_link}"
      size = 30
      type = "pd-ssd"
    }
  }

  metadata {
    sshKeys = "root:${file(pathexpand(var.root_ssh_key_file))}"
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
    script = "install_libvirt.sh"
  }

  # This hackery teaches SSH about the correct host key for this new
  # VM, so that the libvirt provider can just SSH in with no prompting
  # to connect to libvirtd.
  
  provisioner "local-exec" {
    command = "ssh-keygen -R ${google_compute_instance.virt_host.network_interface.0.access_config.0.assigned_nat_ip}"
  }

  provisioner "local-exec" {
    command = "ssh -o StrictHostKeyChecking=no root@${google_compute_instance.virt_host.network_interface.0.access_config.0.assigned_nat_ip} true"
  }
}

output "ip" {
  value = "${google_compute_instance.virt_host.network_interface.0.access_config.0.assigned_nat_ip}"
}

resource "libvirt_network" "net" {
  name = "${var.cluster_name}-net"
  mode = "nat"
  domain = "${var.cluster_name}.local"
  addresses = ["${var.machine_cidr}"]
  autostart = true
}

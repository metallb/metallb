.PHONY: build-debos
build-debos:
	docker build -t debos .

.PHONY: debos
debos:
	docker run -it --privileged -v `pwd`:/home/debos debos --debug-shell os.yaml

.PHONY: boot
boot:
	qemu-system-x86_64 -vga none -nographic -m 1024 -display none -device virtio-net,netdev=net0 -netdev user,id=net0 -device virtio-serial -kernel vmlinuz -initrd initrd.img -no-reboot -append "console=ttyS0,115200 panic=-1 acpi=off nosmp ip=dhcp root=/dev/sda1" debian.qcow2

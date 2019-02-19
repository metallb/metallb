.PHONY: update-addons build-img build-img-niced

update-addons:
# Calico
	curl https://docs.projectcalico.org/v3.3/getting-started/kubernetes/installation/hosted/rbac-kdd.yaml >internal/assets/net/calico.yaml
	echo "---" >>internal/assets/net/calico.yaml
	curl https://docs.projectcalico.org/v3.3/getting-started/kubernetes/installation/hosted/kubernetes-datastore/calico-networking/1.7/calico.yaml >>internal/assets/net/calico.yaml
	perl -pi -e 's#192.168.0.0/16#10.32.0.0/12#g' internal/assets/net/calico.yaml
# Weave
	curl -L 'https://cloud.weave.works/k8s/net?k8s-version=1.13' >internal/assets/net/weave.yaml
# Flannel
	curl -L https://raw.githubusercontent.com/coreos/flannel/bc79dd1505b0c8681ece4de4c0d86c5cd2643275/Documentation/kube-flannel.yml >internal/assets/net/flannel.yaml
	perl -pi -e 's#10.244.0.0/16#10.32.0.0/12#g' internal/assets/net/flannel.yaml
# TODO: cilium, need to figure out the etcd operator nonsense
# TODO: romana, it's currently broken on k8s 1.12

	grep -h "image:" internal/assets/*.yaml internal/assets/net/*.yaml | cut -f2- -d: | tr -d "'\" " | tr '\n' ' ' >internal/assets/addon-images
	+make update-assets

update-assets:
	(cd internal/assets && go generate)

build-img:
	nice -n 19 ionice -c3 make build-img-niced

build-img-niced:
	docker build -t virtuakube-fs vmimg
	docker run --mount=type=bind,source=$$(pwd)/vmimg,destination=/tmp/ctx virtuakube-fs cp /vmlinuz /initrd.img /tmp/ctx
	docker export $$(docker run -d virtuakube-fs /bin/true) >vmimg/virtuakube.tar
	virt-make-fs --partition --format=qcow2 --type=ext4 --size=10G vmimg/virtuakube.tar vmimg/virtuakube-large.qcow2
	rm -f vmimg/virtuakube.tar
	qemu-system-x86_64 \
		-enable-kvm \
		-m 1024 \
		-device virtio-net,netdev=net0 \
		-device virtio-rng-pci,rng=rng0 \
		-object rng-random,filename=/dev/urandom,id=rng0 \
		-netdev user,id=net0 \
		-drive if=virtio,file=vmimg/virtuakube-large.qcow2,media=disk \
		-virtfs local,path=$$(pwd)/vmimg,mount_tag=host0,security_model=none,id=host0 \
		-kernel ./vmimg/vmlinuz -initrd ./vmimg/initrd.img \
		-append "root=/dev/vda1 systemd.journald.forward_to_console"
	qemu-img convert -O qcow2 -c vmimg/virtuakube-large.qcow2 vmimg/virtuakube.qcow2
	rm -f vmimg/virtuakube-large.qcow2 vmimg/vmlinuz vmimg/initrd.img vmimg/boot-done

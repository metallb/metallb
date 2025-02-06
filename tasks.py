import ipaddress
import os
import re
import shutil
import sys
import tempfile
import time
from functools import cache
from typing import Optional

import semver
import yaml
import jinja2

try:
    from urllib.request import urlopen
except ImportError:
    from urllib2 import urlopen

from invoke import run, task
from invoke.exceptions import Exit, UnexpectedExit

LICENSE_PATH = os.path.join("dev-env", "deepcopy_header.txt")

all_binaries = set(["controller", "speaker", "configmaptocrs"])
all_architectures = set(["amd64", "arm", "arm64", "ppc64le", "s390x"])
default_network = "kind"
extra_network = "network2"
controller_gen_version = "v0.17.2"
build_path = os.path.join(os.path.dirname(os.path.abspath(__file__)), "build")
kubectl_path = os.path.join(build_path, "kubectl")
kind_path = os.path.join(build_path, "kind")
ginkgo_path = os.path.join(build_path, "bin", "ginkgo")
controller_gen_path = os.path.join(build_path, "bin", "controller-gen")
kubectl_version = "v1.31.0"
kind_version = "v0.24.0"


def _check_architectures(architectures):
    out = set()
    for arch in architectures:
        if arch == "all":
            out |= all_architectures
        elif arch not in all_architectures:
            print("unknown architecture {}".format(arch))
            print(
                "Supported architectures: {}".format(
                    ", ".join(sorted(all_architectures))
                )
            )
            sys.exit(1)
        else:
            out.add(arch)
    if not out:
        out.add("amd64")
    return list(sorted(out))


def _check_binaries(binaries):
    out = set()
    for binary in binaries:
        if binary == "all":
            out |= all_binaries
        elif binary not in all_binaries:
            print("Unknown binary {}".format(binary))
            print("Known binaries: {}".format(", ".join(sorted(all_binaries))))
            sys.exit(1)
        else:
            out.add(binary)
    if not out:
        out.add("controller")
        out.add("speaker")
        out.add("configmaptocrs")
    return list(sorted(out))


def _docker_build_cmd():
    cmd = os.getenv("DOCKER_BUILD_CMD")
    if cmd:
        out = cmd
    else:
        out = run(
            "docker buildx version >/dev/null"
            "&& echo 'docker buildx build --load' "
            "|| echo 'docker build'",
            hide=True,
        ).stdout.strip()
    return out


def run_with_retry(cmd, tries=6, delay=2):
    mtries, mdelay = tries, delay
    while mtries > 1:
        rv = run(cmd, warn="True").exited
        if rv == 0:
            return
        print("Sleeping for {}s".format(mdelay))
        time.sleep(mdelay)
        mtries -= 1
        mdelay *= 2  # exponential backoff
    run(cmd)


# Returns true if docker is a symbolic link to podman.
def _is_podman():
    return "podman" in os.path.realpath(shutil.which("docker"))


def _is_network_exist(network):
    try:
        run("docker network inspect {network}".format(network=network))
    except:
        print("docker bridge {} doesn't exist".format(network))
        return False
    return True


# Get the list of subnets for the nework.
def _get_network_subnets(network):
    if _is_podman():
        cmd = (
            "podman network inspect {network} ".format(network=network)
            + '-f "{{ range .Subnets }}{{.Subnet}} {{end}}"'
        )
    else:
        cmd = (
            "docker network inspect {network} ".format(network=network)
            + '-f "{{ range .IPAM.Config}}{{.Subnet}} {{end}}"'
        )
    return run(cmd, echo=True).stdout.strip().split(" ")


# Get the list of allocated IPv4 and IPv6 addresses for the kind network.
def _get_subnets_allocated_ips():
    v4_ips = []
    v6_ips = []

    if _is_podman():
        cmd = 'podman ps -f network=kind --format "{{.ID}}"'
        containers = run(cmd, echo=True).stdout.strip().split("\n")
        # for each container, get the IP address and add it to the list of
        # allocated IPs
        for c in containers:
            cmd = (
                "podman inspect {container} --format '"
                "{{{{.NetworkSettings.Networks.kind.IPAddress}}}} "
                "{{{{.NetworkSettings.Networks.kind.GlobalIPv6Address}}}}'"
            ).format(container=c)
            v4, v6 = run(cmd, echo=True).stdout.strip().split(" ")
            v4_ips.append(v4)
            v6_ips.append(v6)
    else:
        v4_ips = (
            run(
                "docker network inspect kind -f "
                '"{{range .Containers}}{{.IPv4Address}} {{end}}"',
                echo=True,
            )
            .stdout.strip()
            .split(" ")
        )
        v6_ips = (
            run(
                "docker network inspect kind -f "
                '"{{range .Containers}}{{.IPv6Address}} {{end}}"',
                echo=True,
            )
            .stdout.strip()
            .split(" ")
        )

    return sorted(v4_ips), sorted(v6_ips)


def _add_nic_to_nodes(cluster_name):
    nodes = (
        run("kind get nodes --name {name}".format(name=cluster_name))
        .stdout.strip()
        .split("\n")
    )
    if not _is_network_exist(extra_network):
        run(
            "docker network create --ipv6 --subnet {ipv6_subnet} -d bridge {bridge_name}".format(
                bridge_name=extra_network, ipv6_subnet="fc00:f853:ccd:e791::/64"
            )
        )
    for node in nodes:
        run(
            "docker network connect {bridge_name} {node}".format(
                bridge_name=extra_network, node=node
            )
        )


# Get the nics of kind cluster node
def _get_node_nics(node):
    default_nic = run(
        'docker exec -i {container} ip r | grep default | cut -d " " -f 5'.format(
            container=node
        )
    ).stdout.strip()
    if not _is_network_exist(extra_network):
        return default_nic
    extra_subnets = _get_network_subnets(extra_network)
    if is_ipv4(extra_subnets[0]):
        extra_nic = run(
            'docker exec -i {container} ip r | grep {dst} | cut -d " " -f 3'.format(
                container=node, dst=extra_subnets[0]
            )
        ).stdout.strip()
    else:
        extra_nic = run(
            'docker exec -i {container} ip -6 r | grep {dst} | cut -d " " -f 3'.format(
                container=node, dst=extra_subnets[0]
            )
        ).stdout.strip()
    return default_nic + "," + extra_nic


def _get_local_nics():
    nics = []
    for net in [default_network, extra_network]:
        if not _is_network_exist(net):
            continue
        subnets = _get_network_subnets(net)
        if is_ipv4(subnets[0]):
            nic = run(
                'ip r | grep {dst} | cut -d " " -f 3'.format(dst=subnets[0])
            ).stdout.strip()
        else:
            nic = run(
                'ip -6 r | grep {dst} | cut -d " " -f 3'.format(dst=subnets[0])
            ).stdout.strip()
        nics.append(nic)
    return ",".join(nics)


@task(
    iterable=["binaries", "architectures"],
    help={
        "binaries": "binaries to build. One or more of {}, or 'all'".format(
            ", ".join(sorted(all_binaries))
        ),
        "architectures": "architectures to build. One or more of {}, or 'all'".format(
            ", ".join(sorted(all_architectures))
        ),
        "registry": "Docker registry under which to tag the images. Default 'quay.io'.",
        "repo": "Docker repository under which to tag the images. Default 'metallb'.",
        "tag": "Docker image tag prefix to use. Actual tag will be <tag>-<arch>. Default 'dev'.",
    },
)
def build(ctx, binaries, architectures, registry="quay.io", repo="metallb", tag="dev"):
    """Build MetalLB docker images."""
    binaries = _check_binaries(binaries)
    architectures = _check_architectures(architectures)
    docker_build_cmd = _docker_build_cmd()

    commit = run("git describe --dirty --always", hide=True).stdout.strip()
    branch = run("git rev-parse --abbrev-ref HEAD", hide=True).stdout.strip()

    for arch in architectures:

        for bin in binaries:
            try:
                if _is_podman():
                    command = "podman"
                else:
                    command = "docker"
                run(
                    "{command} image rm {registry}/{repo}/{bin}:{tag}-{arch}".format(
                        command=command,
                        registry=registry,
                        repo=repo,
                        bin=bin,
                        tag=tag,
                        arch=arch,
                    )
                )
            except:
                pass
            run(
                "{docker_build_cmd} "
                "--platform linux/{arch} "
                "-t {registry}/{repo}/{bin}:{tag}-{arch} "
                "-f {bin}/Dockerfile "
                '--build-arg GIT_BRANCH="{branch}" '
                '--build-arg GIT_COMMIT="{commit}" '
                ".".format(
                    docker_build_cmd=docker_build_cmd,
                    registry=registry,
                    repo=repo,
                    bin=bin,
                    tag=tag,
                    arch=arch,
                    commit=commit,
                    branch=branch,
                ),
                echo=True,
            )


@task(
    iterable=["binaries", "architectures"],
    help={
        "binaries": "binaries to build. One or more of {}, or 'all'".format(
            ", ".join(sorted(all_binaries))
        ),
        "architectures": "architectures to build. One or more of {}, or 'all'".format(
            ", ".join(sorted(all_architectures))
        ),
        "registry": "Docker registry under which to tag the images. Default 'quay.io'.",
        "repo": "Docker repository under which to tag the images. Default 'metallb'.",
        "tag": "Docker image tag prefix to use. Actual tag will be <tag>-<arch>. Default 'dev'.",
    },
)
def push(ctx, binaries, architectures, registry="quay.io", repo="metallb", tag="dev"):
    """Build and push docker images to registry."""
    binaries = _check_binaries(binaries)
    architectures = _check_architectures(architectures)

    for arch in architectures:
        for bin in binaries:
            build(
                ctx,
                binaries=[bin],
                architectures=[arch],
                registry=registry,
                repo=repo,
                tag=tag,
            )
            run(
                "docker push {registry}/{repo}/{bin}:{tag}-{arch}".format(
                    registry=registry, repo=repo, bin=bin, arch=arch, tag=tag
                ),
                echo=True,
            )


@task(
    iterable=["binaries"],
    help={
        "binaries": "binaries to build. One or more of {}, or 'all'".format(
            ", ".join(sorted(all_binaries))
        ),
        "registry": "Docker registry under which to tag the images. Default 'quay.io'.",
        "repo": "Docker repository under which to tag the images. Default 'metallb'.",
        "tag": "Docker image tag prefix to use. Actual tag will be <tag>-<arch>. Default 'dev'.",
    },
)
def push_multiarch(ctx, binaries, registry="quay.io", repo="metallb", tag="dev"):
    """Build and push multi-architecture docker images to registry."""
    binaries = _check_binaries(binaries)
    architectures = _check_architectures(["all"])
    push(
        ctx,
        binaries=binaries,
        architectures=architectures,
        registry=registry,
        repo=repo,
        tag=tag,
    )

    platforms = ",".join("linux/{}".format(arch) for arch in architectures)
    for bin in binaries:
        run(
            "manifest-tool push from-args "
            "--platforms {platforms} "
            "--template {registry}/{repo}/{bin}:{tag}-ARCH "
            "--target {registry}/{repo}/{bin}:{tag}".format(
                platforms=platforms, registry=registry, repo=repo, bin=bin, tag=tag
            ),
            echo=True,
        )


def validate_kind_version():
    """Validate minimum required version of kind."""
    # If kind is not installed, this first command will raise an UnexpectedExit
    # exception, and inv will exit at this point making it clear running "kind"
    # failed.
    min_version = "0.9.0"

    try:
        raw = run("kind version", echo=True)
    except Exception as e:
        raise Exit(message="Could not determine kind version (is kind installed?)")

    actual_version = re.search(r"v(\d*\.\d*\.\d*)", raw.stdout).group(1)
    delta = semver.compare(actual_version, min_version)

    if delta < 0:
        raise Exit(message="kind version >= {} required".format(min_version))


def generate_manifest(
    ctx,
    crd_options="crd:crdVersions=v1",
    bgp_type="native",
    output=None,
    with_prometheus=False,
):
    fetch_kubectl()
    fetch_controller_gen()
    res = run(
        '{}/bin/controller-gen {} rbac:roleName=manager-role webhook paths="./api/..." output:crd:artifacts:config=config/crd/bases'.format(
            build_path, crd_options
        ),
        echo=True,
    )
    if not res.ok:
        raise Exit(message="Failed to generate manifests")

    if output:
        layer = bgp_type
        if with_prometheus:
            layer = "prometheus-" + layer
        res = run("{} kustomize config/{} > {}".format(kubectl_path, layer, output))
        if not res.ok:
            raise Exit(message="Failed to kustomize manifests")


@task(
    help={
        "architecture": "CPU architecture of the local machine. Default 'amd64'.",
        "name": "name of the kind cluster to use.",
        "protocol": "Pre-configure MetalLB with the specified protocol. "
        "Unconfigured by default. Supported: 'bgp','layer2'",
        "node_img": "Optional node image to use for the kind cluster (e.g. kindest/node:v1.18.19)."
        "The node image drives the kubernetes version used in kind.",
        "ip_family": "Optional ipfamily of the cluster."
        "Default: ipv4, supported families are 'ipv6' and 'dual'.",
        "bgp_type": "Type of BGP implementation to use."
        "Supported: 'frr' (default), 'native', 'frr-k8s', 'frr-k8s-external'",
        "frr_volume_dir": "FRR router config directory to be mounted inside frr container. "
        "Default: ./dev-env/bgp/frr-volume",
        "log_level": "Log level for the controller and the speaker."
        "Default: info, Supported: 'all', 'debug', 'info', 'warn', 'error' or 'none'",
        "helm_install": "Optional install MetalLB via helm chart instead of manifests."
        "Default: False.",
        "build_images": "Optional build the images." "Default: True.",
        "with_prometheus": "Deploys the prometheus kubernetes stack" "Default: False.",
        "with_api_audit": "Enables audit on the apiserver" "Default: False.",
    }
)
def dev_env(
    ctx,
    architecture="amd64",
    name="kind",
    protocol=None,
    frr_volume_dir="",
    node_img=None,
    ip_family="ipv4",
    bgp_type="frr",
    log_level="info",
    helm_install=False,
    build_images=True,
    with_prometheus=False,
    with_api_audit=False,
):
    """Build and run MetalLB in a local Kind cluster.

    If the cluster specified by --name (default "kind") doesn't exist,
    it is created. Then, build MetalLB docker images from the
    checkout, push them into kind, and deploy MetalLB through manifests
    or helm to run those images.
    The optional node_img parameter will be used to determine the version of the cluster.
    """

    fetch_kubectl()
    fetch_kind()
    validate_kind_version()

    clusters = run("kind get clusters", hide=True).stdout.strip().splitlines()
    mk_cluster = name not in clusters
    if mk_cluster:
        config = {
            "apiVersion": "kind.x-k8s.io/v1alpha4",
            "kind": "Cluster",
            "nodes": [
                {"role": "control-plane"},
                {"role": "worker"},
                {"role": "worker"},
            ],
        }

        if with_api_audit:
            config["nodes"][0]["kubeadmConfigPatches"] = [
                r"""kind: ClusterConfiguration
apiServer:
  # enable auditing flags on the API server
  extraArgs:
    audit-log-path: /var/log/kubernetes/kube-apiserver-audit.log
    audit-policy-file: /etc/kubernetes/policies/audit-policy.yaml
    # mount new files / directories on the control plane
  extraVolumes:
    - name: audit-policies
      hostPath: /etc/kubernetes/policies
      mountPath: /etc/kubernetes/policies
      readOnly: true
      pathType: "DirectoryOrCreate"
    - name: "audit-logs"
      hostPath: "/var/log/kubernetes"
      mountPath: "/var/log/kubernetes"
      readOnly: false
      pathType: DirectoryOrCreate"""
            ]
            config["nodes"][0]["extraMounts"] = [
                {
                    "hostPath": "./dev-env/audit-policy.yaml",
                    "containerPath": "/etc/kubernetes/policies/audit-policy.yaml",
                    "readOnly": True,
                }
            ]

        networking_config = {}
        if ip_family != "ipv4":
            networking_config["ipFamily"] = ip_family

        if len(networking_config) > 0:
            config["networking"] = networking_config

        extra_options = ""
        if node_img != None:
            extra_options = "--image={}".format(node_img)
        config = yaml.dump(config).encode("utf-8")

        with tempfile.NamedTemporaryFile() as tmp:
            tmp.write(config)
            tmp.flush()
            run(
                "{} create cluster --name={} --config={} {}".format(
                    kind_path, name, tmp.name, extra_options
                ),
                pty=True,
                echo=True,
            )
        _add_nic_to_nodes(name)

    binaries = ["controller", "speaker"]
    if build_images:
        build(ctx, binaries, architectures=[architecture])
    run(
        "{} load docker-image --name={} quay.io/metallb/controller:dev-{}".format(
            kind_path, name, architecture
        ),
        echo=True,
    )
    run(
        "{} load docker-image --name={} quay.io/metallb/speaker:dev-{}".format(
            kind_path, name, architecture
        ),
        echo=True,
    )

    if with_prometheus:
        print("Deploying prometheus")
        deployprometheus(ctx)

    frr_k8s_ns = "frr-k8s-system"
    if bgp_type == "frr-k8s-external":
        run(
            "{} apply -f https://raw.githubusercontent.com/metallb/frr-k8s/v0.0.17/config/all-in-one/frr-k8s.yaml".format(
                kubectl_path
            ),
            echo=True,
        )
        time.sleep(2)
        run(
            "{} -n {} wait --for=condition=Ready --all pods --timeout 300s".format(
                kubectl_path, frr_k8s_ns
            ),
            echo=True,
        )

    if helm_install:
        run("{} apply -f config/native/ns.yaml".format(kubectl_path), echo=True)
        prometheus_values = ""
        if with_prometheus:
            prometheus_values = (
                "--set prometheus.serviceMonitor.enabled=true "
                "--set prometheus.secureMetricsPort=9120 "
                "--set speaker.frr.secureMetricsPort=9121 "
                "--set prometheus.serviceAccount=prometheus-k8s "
                "--set prometheus.namespace=monitoring "
            )
        frr_values = ""

        if bgp_type == "frr":
            frr_values = "--set speaker.frr.enabled=true "
        if bgp_type == "frr-k8s":
            frr_values = "--set frrk8s.enabled=true --set speaker.frr.enabled=false --set frr-k8s.prometheus.serviceMonitor.enabled=false "
            if with_prometheus:
                frr_values = (
                    "--set frrk8s.enabled=true --set speaker.frr.enabled=false --set frr-k8s.prometheus.serviceMonitor.enabled=true "
                    "--set frr-k8s.prometheus.serviceMonitor.metricRelabelings[0].sourceLabels=\\{__name__\\} "
                    '--set frr-k8s.prometheus.serviceMonitor.metricRelabelings[0].regex="frrk8s_bgp_(.*)" '
                    '--set frr-k8s.prometheus.serviceMonitor.metricRelabelings[0].targetLabel="__name__" '
                    '--set frr-k8s.prometheus.serviceMonitor.metricRelabelings[0].replacement="metallb_bgp_\\$1" '
                    "--set frr-k8s.prometheus.serviceMonitor.metricRelabelings[1].sourceLabels=\\{__name__\\} "
                    '--set frr-k8s.prometheus.serviceMonitor.metricRelabelings[1].regex="frrk8s_bfd_(.*)" '
                    '--set frr-k8s.prometheus.serviceMonitor.metricRelabelings[1].targetLabel="__name__" '
                    '--set frr-k8s.prometheus.serviceMonitor.metricRelabelings[1].replacement="metallb_bfd_\\$1" '
                )

        if bgp_type == "frr-k8s-external":
            frr_values = "--set frrk8s.external=true --set frrk8s.namespace={} --set speaker.frr.enabled=false --set frr-k8s.prometheus.serviceMonitor.enabled=false ".format(
                frr_k8s_ns
            )

        run(
            "helm install metallb charts/metallb/ --set controller.image.tag=dev-{} "
            "--set speaker.image.tag=dev-{} --set speaker.logLevel=debug "
            "--set controller.logLevel=debug {} {}  --namespace metallb-system".format(
                architecture, architecture, prometheus_values, frr_values
            ),
            echo=True,
        )
    else:
        run("{} delete po -n metallb-system --all".format(kubectl_path), echo=True)

        with tempfile.TemporaryDirectory() as tmpdir:
            manifest_file = tmpdir + "/metallb.yaml"

            generate_manifest(
                ctx,
                bgp_type=bgp_type,
                output=manifest_file,
                with_prometheus=with_prometheus,
            )

            # open file and replace the images with the newely built MetalLB docker images
            with open(manifest_file) as f:
                manifest = f.read()
            for image in binaries:
                manifest = re.sub(
                    "image: quay.io/metallb/{}:.*".format(image),
                    "image: quay.io/metallb/{}:dev-{}".format(image, architecture),
                    manifest,
                )
                manifest = re.sub(
                    "--log-level=info", "--log-level={}".format(log_level), manifest
                )
            manifest.replace("--log-level=info", "--log-level=debug")

            with open(manifest_file, "w") as f:
                f.write(manifest)
                f.flush()

            run("{} apply -f {}".format(kubectl_path, manifest_file), echo=True)

    # Kind puts the remove exclusions annotation on the master node while
    # the e2e tests expect master to be serviceable, so we remove the annotations
    remove_lb_exclusion_from_nodes(ctx)

    if protocol == "bgp":
        print("Configuring MetalLB with a BGP test environment")
        bgp_dev_env(ip_family, frr_volume_dir)
    elif protocol == "layer2":
        print("Configuring MetalLB with a layer 2 test environment")
        layer2_dev_env()
    else:
        print("Leaving MetalLB unconfigured")


# Configure MetalLB in the dev-env for layer2 testing.
# Identify the unused network address range from kind network and used it in configmap.
def layer2_dev_env():
    fetch_kubectl()
    dev_env_dir = os.getcwd() + "/dev-env/layer2"
    with open("%s/config.yaml.tmpl" % dev_env_dir, "r") as f:
        layer2_config = "# THIS FILE IS AUTOGENERATED\n" + f.read()
    layer2_config = layer2_config.replace("SERVICE_V4_RANGE", get_available_ips(4))
    layer2_config = layer2_config.replace("SERVICE_V6_RANGE", get_available_ips(6))
    with open("%s/config.yaml" % dev_env_dir, "w") as f:
        f.write(layer2_config)
    run(
        "{} wait --for=condition=Available deploy/controller -n metallb-system".format(
            kubectl_path
        )
    )
    # Apply the MetalLB ConfigMap
    run("{} apply -f {}/config.yaml".format(kubectl_path, dev_env_dir))


# Configure MetalLB in the dev-env for BGP testing. Start an frr based BGP
# router in a container and configure MetalLB to peer with it.
# See dev-env/bgp/README.md for some more information.
def bgp_dev_env(ip_family, frr_volume_dir):
    fetch_kubectl()
    dev_env_dir = os.getcwd() + "/dev-env/bgp"
    if frr_volume_dir == "":
        frr_volume_dir = run("mktemp -d").stdout.strip() + "/frr-volume"

    nodes = run("{} get nodes -o name".format(kubectl_path), echo=True)
    nodes = nodes.stdout.strip().split()
    if len(nodes) != 3:
        raise Exit(message="Expected 3 nodes, got %d" % len(node_ips))

    # Create a new directory that will be used as the config volume for frr.
    try:
        os.mkdir(frr_volume_dir)
    except FileExistsError:
        pass
    except Exception as e:
        raise Exit(message="Failed to create frr-volume directory: %s" % str(e))

    # These config files are static, so we copy them straight in.
    copy_files = ("zebra.conf", "daemons", "vtysh.conf")
    for f in copy_files:
        shutil.copyfile("%s/frr/%s" % (dev_env_dir, f), "%s/%s" % (frr_volume_dir, f))

    # TODO -- The IP address handling will need updates to add support for IPv6
    # We need the IPs for each Node in the cluster to place them in the BGP
    # router configuration file (bgpd.conf). Each Node will peer with this
    # router.
    node_ips = run(
        "{} get nodes -o jsonpath='{{.items[*].status.addresses"
        '[?(@.type=="InternalIP")].address}}{{"\\n"}}\''.format(kubectl_path),
        echo=True,
    )
    node_ips = node_ips.stdout.strip().split()

    node_ips_v4 = [ip for ip in node_ips if is_ipv4(ip)]
    node_ips_v6 = [ip for ip in node_ips if not is_ipv4(ip)]
    if ip_family == "dual":
        ip_families = ["ipv4", "ipv6"]
    else:
        ip_families = [ip_family]

    # bgpd.conf is created from a template so that we can include the current
    # Node IPs.
    with open("%s/frr/bgpd.conf.j2" % dev_env_dir, "r") as f:
        bgpd_config = (
            jinja2.Environment()
            .from_string(f.read())
            .render(
                ip_families=ip_families,
                node_ips=node_ips,
                node_ip_by_family={"ipv4": node_ips_v4, "ipv6": node_ips_v6},
            )
        )
    with open("%s/bgpd.conf" % frr_volume_dir, "w") as f:
        f.write(bgpd_config)

    # Run a BGP router in a container for all of the speakers to peer with.
    run(
        "for frr in $(docker ps -a -f name=frr --format {{.Names}}) ; do "
        "    docker rm -f $frr ; "
        "done",
        echo=True,
    )
    run(
        "docker run -d --privileged --network kind --rm --ulimit core=-1 --name frr --volume %s:/etc/frr "
        "quay.io/frrouting/frr:9.1.0" % frr_volume_dir,
        echo=True,
    )

    if ip_family == "ipv4":
        peer_addresses = [
            run(
                'docker inspect -f "{{ '
                'range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}" frr',
                echo=True,
            ).stdout.strip()
        ]

    elif ip_family == "ipv6":
        peer_addresses = [
            run(
                'docker inspect -f "{{ '
                'range .NetworkSettings.Networks}}{{.GlobalIPv6Address}}{{end}}" frr',
                echo=True,
            ).stdout.strip()
        ]

    elif ip_family == "dual":
        peer_addresses = (
            run(
                'docker inspect -f "{{ '
                'range .NetworkSettings.Networks}}{{.IPAddress}},{{.GlobalIPv6Address}}{{end}}" frr',
                echo=True,
            )
            .stdout.strip()
            .split(",")
        )

    else:
        raise Exit(message="Unsupported ip address family %s" % ip_family)

    with open("%s/config.yaml.j2" % dev_env_dir, "r") as f:
        mlb_config = (
            jinja2.Environment()
            .from_string(f.read())
            .render(peer_addresses=peer_addresses)
        )
    with open("%s/config.yaml" % dev_env_dir, "w") as f:
        f.write(mlb_config)
    # Apply the MetalLB ConfigMap
    run_with_retry("{} apply -f {}/config.yaml".format(kubectl_path, dev_env_dir))


def get_available_ips(ip_family=None):
    if ip_family is None or (ip_family != 4 and ip_family != 6):
        raise Exit(message="Please provide network version: 4 or 6.")

    v4, v6 = _get_subnets_allocated_ips()
    for i in _get_network_subnets(default_network):
        network = ipaddress.ip_network(i)
        if network.version == ip_family:
            used_list = v4 if ip_family == 4 else v6
            last_used = ipaddress.ip_interface(used_list[-1])

            # try to get 10 IP addresses after the last assigned node address in the kind network subnet,
            # plus we give room to thr frr single hop containers.
            # if failed, just quit (recreate kind cluster might solve the situation)
            service_ip_range_start = last_used + 5
            service_ip_range_end = last_used + 15
            if service_ip_range_start not in network:
                raise Exit(
                    message="network range %s is not in %s"
                    % (service_ip_range_start, network)
                )
            if service_ip_range_end not in network:
                raise Exit(
                    message="network range %s is not in %s"
                    % (service_ip_range_end, network)
                )
            return "%s-%s" % (service_ip_range_start.ip, service_ip_range_end.ip)


@task(
    help={
        "name": "name of the kind cluster to delete.",
        "frr_volume_dir": "FRR router config directory to be cleaned up. "
        "Default: ./dev-env/bgp/frr-volume",
    }
)
def dev_env_cleanup(ctx, name="kind", frr_volume_dir=""):
    """Remove traces of the dev env."""
    validate_kind_version()
    fetch_kind()

    clusters = (
        run("{} get clusters".format(kind_path), hide=True).stdout.strip().splitlines()
    )
    if name in clusters:
        run("{} delete cluster --name={}".format(kind_path, name), hide=True)

    run(
        "for frr in $(docker ps -a -f name=frr --format {{.Names}}) ; do "
        "    docker rm -f $frr ; "
        "done",
        hide=True,
    )

    run(
        "for frr in $(docker ps -a -f name=vrf --format {{.Names}}) ; do "
        "    docker rm -f $frr ; "
        "done",
        hide=True,
    )

    # cleanup bgp configs
    dev_env_dir = os.getcwd() + "/dev-env/bgp"
    if frr_volume_dir == "":
        frr_volume_dir = dev_env_dir + "/frr-volume"

    # sudo because past docker runs will have changed ownership of this dir
    run('sudo rm -rf "%s"' % frr_volume_dir)
    run('rm -f "%s"/config.yaml' % dev_env_dir)

    # cleanup layer2 configs
    dev_env_dir = os.getcwd() + "/dev-env/layer2"
    run('rm -f "%s"/config.yaml' % dev_env_dir)

    # cleanup extra bridge
    run("docker network rm {bridge_name}".format(bridge_name=extra_network), warn=True)
    run("docker network rm vrf-net", warn=True)


@task(
    help={
        "version": "version of MetalLB to release.",
        "skip-release-notes": "make the release even if there are no release notes.",
    }
)
def release(ctx, version, skip_release_notes=False):
    """Tag a new release."""
    status = run("git status --porcelain", hide=True).stdout.strip()
    if status != "":
        raise Exit(message="git checkout not clean, cannot release")

    sem_version = semver.parse_version_info(version)
    is_patch_release = sem_version.patch != 0

    # Check that we have release notes for the desired version.
    run("git checkout main", echo=True)
    if not skip_release_notes:
        with open("website/content/release-notes/_index.md") as release_notes:
            if "## Version {}".format(sem_version) not in release_notes.read():
                raise Exit(message="no release notes for v{}".format(sem_version))

    # Move HEAD to the correct release branch - either a new one, or
    # an existing one.
    if is_patch_release:
        run(
            "git checkout v{}.{}".format(sem_version.major, sem_version.minor),
            echo=True,
        )
    else:
        run(
            "git checkout -b v{}.{}".format(sem_version.major, sem_version.minor),
            echo=True,
        )

    # Copy over release notes from main.
    if not skip_release_notes:
        run("git checkout main -- website/content/release-notes/_index.md", echo=True)

    # Update links on the website to point to files at the version
    # we're creating.
    if is_patch_release:
        previous_version = "v{}.{}.{}".format(
            sem_version.major, sem_version.minor, sem_version.patch - 1
        )
    else:
        previous_version = "main"
    bumprelease(ctx, version, previous_version)

    run(
        "git commit -a -m 'Automated update for release v{}'".format(sem_version),
        echo=True,
    )
    run(
        "git tag v{} -m 'See the release notes for details:\n\nhttps://metallb.universe.tf/release-notes/#version-{}-{}-{}'".format(
            sem_version, sem_version.major, sem_version.minor, sem_version.patch
        ),
        echo=True,
    )
    run("git checkout main", echo=True)


@task(
    help={
        "version": "version of MetalLB to release.",
        "previous_version": "version of the previous release.",
    }
)
def bumprelease(ctx, version, previous_version):
    version = semver.parse_version_info(version)

    def _replace(pattern):
        oldpat = pattern.format(previous_version)
        newpat = pattern.format("v{}").format(version)
        run(
            "perl -pi -e 's#{}#{}#g' website/content/*.md website/content/*/*.md".format(
                oldpat, newpat
            ),
            echo=True,
        )

    _replace("/metallb/metallb/{}")
    _replace("/metallb/metallb/tree/{}")
    _replace("/metallb/metallb/blob/{}")

    # Update the version listed on the website sidebar
    run(
        "perl -pi -e 's/MetalLB .*/MetalLB v{}/g' website/content/_header.md".format(
            version
        ),
        echo=True,
    )

    # Update the manifests with the new version
    run(
        "perl -pi -e 's,image: quay.io/metallb/speaker:.*,image: quay.io/metallb/speaker:v{},g' config/controllers/speaker.yaml".format(
            version
        ),
        echo=True,
    )
    run(
        "perl -pi -e 's,image: quay.io/metallb/speaker:.*,image: quay.io/metallb/speaker:v{},g' config/frr/speaker-patch.yaml".format(
            version
        ),
        echo=True,
    )
    run(
        "perl -pi -e 's,image: quay.io/metallb/controller:.*,image: quay.io/metallb/controller:v{},g' config/controllers/controller.yaml".format(
            version
        ),
        echo=True,
    )

    # Update the versions in the helm chart (version and appVersion are always the same)
    # helm chart versions follow Semantic Versioning, and thus exclude the leading 'v'
    # we change the version of the crd dependency only, ignoring the frr-k8s version that comes from main
    run(
        r"sed -i '/condition: crds.enabled/{{N;s/version:.*/version: {}/;}}' charts/metallb/Chart.yaml".format(
            version
        ),
        echo=True,
    )
    run(
        r"sed -i '/MetalLB chart version/{{N;s/version:.*/version: {}/;}}' charts/metallb/Chart.yaml".format(
            version
        ),
        echo=True,
    )
    run(
        r"sed -i 's/^appVersion: .*/appVersion: v{}/g' charts/metallb/Chart.yaml".format(
            version
        ),
        echo=True,
    )
    run(
        r"sed -i 's/^version: .*/version: {}/g' charts/metallb/charts/crds/Chart.yaml".format(
            version
        ),
        echo=True,
    )
    run(
        r"sed -i 's/^appVersion: .*/appVersion: v{}/g' charts/metallb/charts/crds/Chart.yaml".format(
            version
        ),
        echo=True,
    )
    run(
        r"sed -i 's/^Current chart version is: .*/Current chart version is: `{}`/g' charts/metallb/README.md".format(
            version
        ),
        echo=True,
    )
    run("helm dependency update charts/metallb", echo=True)

    # Generate the manifests with the new version of the images
    generatemanifests(ctx)

    # Update the version in kustomize instructions
    #
    # TODO: Check if kustomize instructions really need the version in the
    # website or if there is a simpler way. For now, though, we just replace the
    # only page that mentions the version on release.
    run(
        r"sed -i 's/github.com\/metallb\/metallb\/config\/native?ref=.*$/github.com\/metallb\/metallb\/config\/native?ref=v{}/g' website/content/installation/_index.md".format(
            version
        )
    )
    run(
        r"sed -i 's/github.com\/metallb\/metallb\/config\/frr?ref=.*$/github.com\/metallb\/metallb\/config\/frr?ref=v{}/g' website/content/installation/_index.md".format(
            version
        )
    )

    # Update the version embedded in the binary
    run(
        r"perl -pi -e 's/version\s+=.*/version = \"{}\"/g' internal/version/version.go".format(
            version
        ),
        echo=True,
    )
    run("gofmt -w internal/version/version.go", echo=True)

    res = run('grep ":main" config/manifests/*.yaml', warn=True).stdout
    if res:
        raise Exit(message="ERROR: Found image still referring to the main tag")


@task
def test(ctx):
    """Run unit tests."""
    envtest_asset_dir = os.getcwd() + "/dev-env/unittest"
    k8s_version = "1.27.1"
    run(
        "{}/setup-envtest.sh {}".format(envtest_asset_dir, envtest_asset_dir), echo=True
    )
    kubebuilder_assets = run(
        "{}/bin/setup-envtest use {} --bin-dir {}/bin -p path".format(
            envtest_asset_dir, k8s_version, envtest_asset_dir
        )
    ).stdout.strip()
    run(
        "KUBEBUILDER_ASSETS={} go test -short ./...".format(kubebuilder_assets),
        echo=True,
    )
    run(
        "KUBEBUILDER_ASSETS={} go test -short -race ./...".format(kubebuilder_assets),
        echo=True,
    )


@task
def linttasks(ctx):
    """Formats tasks.py with "black" linter."""
    run("black tasks.py")


@task
def checkpatch(ctx):
    # Generate a diff of all changes on this branch from origin/main
    # and look for any added lines with 2 spaces after a period.
    try:
        lines = run(
            "git diff $(diff -u <(git rev-list --first-parent HEAD) "
            " <(git rev-list --first-parent origin/main) "
            " | sed -ne 's/^ //p' | head -1)..HEAD | "
            r" grep '+.*\.\  '"
        )

        if len(lines.stdout.strip()) > 0:
            raise Exit(
                message="ERROR: Found changed lines with 2 spaces " "after a period."
            )
    except UnexpectedExit:
        # Will exit non-zero if no double-space-after-period lines are found.
        pass


@task(
    help={
        "env": "Specify in which environment to run the linter . Default 'container'. Supported: 'container','host'"
    }
)
def lint(ctx, env="container"):
    """Run linter.

    By default, this will run a golangci-lint docker image against the code.
    However, in some environments (such as the MetalLB CI), it may be more
    convenient to install the golangci-lint binaries on the host. This can be
    achieved by running `inv lint --env host`.
    """
    version = "1.63.4"
    golangci_cmd = "golangci-lint run --timeout 10m0s ./..."

    if env == "container":
        run(
            "docker run --rm -v $(git rev-parse --show-toplevel):/app -w /app golangci/golangci-lint:v{} {}".format(
                version, golangci_cmd
            ),
            echo=True,
        )
    elif env == "host":
        run(
            "curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin v{}".format(
                version
            )
        )
        run(golangci_cmd)
    else:
        raise Exit(message="Unsupported linter environment: {}".format(env))


@task(
    help={
        "env": "Specify in which environment to run helmdocs . Default 'container'. Supported: 'container','host'"
    }
)
def helmdocs(ctx, env="container"):
    """Run helm-docs.

    By default, this will run a helm-docs docker image against the code.
    However, in some environments (such as the MetalLB CI), it may be more
    convenient to install the helm-docs binaries on the host. This can be
    achieved by running `inv helmdocs --env host`.
    """
    version = "1.10.0"
    cmd = "helm-docs"

    if env == "container":
        run(
            "docker run --rm -v $(git rev-parse --show-toplevel):/app -w /app jnorwood/helm-docs:v{} {}".format(
                version, cmd
            ),
            echo=True,
        )
    elif env == "host":
        run(cmd)
    else:
        raise Exit(message="Unsupported helm-docs environment: {}".format(env))


@task(
    help={
        "name": "name of the kind cluster to test (only kind uses).",
        "export": "where to export kind logs.",
        "kubeconfig": "kubeconfig location. By default, use the kubeconfig from kind.",
        "system_namespaces": "comma separated list of Kubernetes system namespaces",
        "service_pod_port": "port number that service pods open.",
        "skip_docker": "don't use docker command in BGP testing.",
        "focus": "the list of arguments to pass into as -focus",
        "skip": "the list of arguments to pass into as -skip",
        "ipv4_service_range": "a range of IPv4 addresses for MetalLB to use when running in layer2 mode.",
        "ipv6_service_range": "a range of IPv6 addresses for MetalLB to use when running in layer2 mode.",
        "prometheus_namespace": "the namespace prometheus is deployed to, to validate metrics against prometheus.",
        "node_nics": "a list of node's interfaces separated by comma, default is kind",
        "local_nics": "a list of bridges related node's interfaces separated by comma, default is kind",
        "external_containers": "a comma separated list of external containers names to use for the test. (valid parameters are: ibgp-single-hop / ibgp-multi-hop / ebgp-single-hop / ebgp-multi-hop)",
        "with_vrf": "tells if we want to run the tests against containers reacheable via linux VRFs",
        "bgp_mode": "tells what bgp mode the cluster is using. valid values are native, frr, frr-k8s.",
        "external_frr_image": "overrides the image used for the external frr containers used in tests",
        "ginkgo_params": "additional ginkgo params to run the e2e tests with",
        "junit_report": "export JUnit reports xml to file, default junit-report.xml",
        "host_bgp_mode": "tells whether to run the host container in ebgp or ibgp mode",
    }
)
def e2etest(
    ctx,
    name="kind",
    export=None,
    kubeconfig=None,
    system_namespaces="kube-system,metallb-system",
    service_pod_port=80,
    skip_docker=False,
    focus="",
    skip="",
    ipv4_service_range=None,
    ipv6_service_range=None,
    prometheus_namespace="",
    node_nics="kind",
    local_nics="kind",
    external_containers="",
    bgp_mode="",
    with_vrf=False,
    external_frr_image="",
    ginkgo_params="",
    junit_report="junit-report.xml",
    host_bgp_mode="ibgp",
    frr_k8s_namespace="",
):
    """Run E2E tests against development cluster."""
    fetch_kubectl()
    fetch_kind()
    fetch_ginkgo()

    if skip_docker:
        opt_skip_docker = "--skip-docker"
    else:
        opt_skip_docker = ""

    ginkgo_skip = ""
    if skip:
        ginkgo_skip = '--skip="' + skip + '"'

    ginkgo_focus = ""
    if focus:
        ginkgo_focus = '--focus="' + focus + '"'

    if kubeconfig is None:
        validate_kind_version()
        clusters = (
            run("{} get clusters".format(kind_path), hide=True)
            .stdout.strip()
            .splitlines()
        )
        if name in clusters:
            kubeconfig_file = tempfile.NamedTemporaryFile()
            kubeconfig = kubeconfig_file.name
            run(
                "{} export kubeconfig --name={} --kubeconfig={}".format(
                    kind_path, name, kubeconfig
                ),
                pty=True,
                echo=True,
            )
        else:
            raise Exit(message="Unable to find cluster named: {}".format(name))
    else:
        os.environ["KUBECONFIG"] = kubeconfig

    namespaces = system_namespaces.replace(" ", "").split(",")
    for ns in namespaces:
        run(
            "{} -n {} wait --for=condition=Ready --all pods --timeout 300s".format(
                kubectl_path, ns
            ),
            hide=True,
        )

    if node_nics == "kind":
        nodes = (
            run(
                "{} get nodes -o jsonpath={{.items[*].metadata.name}}".format(
                    kubectl_path
                ),
                hide=True,
            )
            .stdout.strip()
            .split()
        )
        node_nics = _get_node_nics(nodes[0])

    if local_nics == "kind":
        local_nics = _get_local_nics()

    if ipv4_service_range is None:
        ipv4_service_range = get_available_ips(4)

    if ipv6_service_range is None:
        ipv6_service_range = get_available_ips(6)

    if export != None:
        report_path = export
    else:
        report_path = "/tmp/metallbreport{}".format(time.time())

    if prometheus_namespace != "":
        prometheus_namespace = "--prometheus-namespace=" + prometheus_namespace

    print("Writing reports to {}".format(report_path))
    os.makedirs(report_path, exist_ok=True)

    if external_containers != "":
        external_containers = "--external-containers=" + (external_containers)

    if external_frr_image != "":
        external_frr_image = "--frr-image=" + (external_frr_image)
    testrun = run(
        "cd `git rev-parse --show-toplevel`/e2etest &&"
        "KUBECONFIG={} {} {} --junit-report={} --timeout=3h {} {} -- --kubeconfig={} --service-pod-port={} -ipv4-service-range={} -ipv6-service-range={} {} --report-path {} {} -node-nics {} -local-nics {} {} -bgp-mode={} -with-vrf={} {} --host-bgp-mode={} --kubectl={} --frr-k8s-namespace={}".format(
            kubeconfig,
            ginkgo_path,
            ginkgo_params,
            junit_report,
            ginkgo_focus,
            ginkgo_skip,
            kubeconfig,
            service_pod_port,
            ipv4_service_range,
            ipv6_service_range,
            opt_skip_docker,
            report_path,
            prometheus_namespace,
            node_nics,
            local_nics,
            external_containers,
            bgp_mode,
            with_vrf,
            external_frr_image,
            host_bgp_mode,
            kubectl_path,
            frr_k8s_namespace,
        ),
        warn=True,
        echo=True,
    )

    if export != None:
        run("{} export logs {}".format(kind_path, export), warn=True)

    if testrun.failed:
        raise Exit(message="E2E tests failed", code=testrun.return_code)


@task
def remove_lb_exclusion_from_nodes(ctx):
    fetch_kubectl()
    nodes = (
        run(
            "{} get nodes -o jsonpath={{.items[*].metadata.name}}".format(kubectl_path),
            hide=True,
        )
        .stdout.strip()
        .split()
    )
    for node in nodes:
        run(
            "{} label nodes {} node.kubernetes.io/exclude-from-external-load-balancers-".format(
                kubectl_path, node
            ),
            hide=True,
        )

    for iter in range(1, 11):
        res = run(
            "{} get nodes -l node.kubernetes.io/exclude-from-external-load-balancers -o jsonpath={{.items}}".format(
                kubectl_path
            ),
            hide=True,
        ).stdout
        if res == "[]":
            return
        time.sleep(1)
    raise Exception("not able to remove lb exclusions", res)


@task
def bumplicense(ctx):
    """Bumps the license header on all go files that have it missing"""

    res = run("find . -name '*.go' | grep -v dev-env")
    for file in res.stdout.splitlines():
        res = run("grep -q License {}".format(file), warn=True)
        if not res.ok:
            run(r"sed -i '1s/^/\/\/ SPDX-License-Identifier:Apache-2.0\n\n/' " + file)


@task
def verifylicense(ctx):
    """Verifies all files have the corresponding license"""
    res = run("find . -name '*.go' | grep -v dev-env", hide="out")
    no_license = False
    for file in res.stdout.splitlines():
        res = run("grep -q License {}".format(file), warn=True)
        if not res.ok:
            no_license = True
            print("{} is missing license".format(file))
    if no_license:
        raise Exit(
            message="#### Files with no license found.\n#### Please run "
            "inv bumplicense"
            " to add the license header"
        )


@task
def gomodtidy(ctx):
    """Runs go mod tidy"""
    res = run("go mod tidy", hide="out")
    if not res.ok:
        raise Exit(message="go mod tidy failed")


@task
def generatemanifests(ctx):
    """Re-generates the all-in-one manifests under config/manifests"""
    generate_manifest(ctx, bgp_type="frr", output="config/manifests/metallb-frr.yaml")
    generate_manifest(
        ctx, bgp_type="native", output="config/manifests/metallb-native.yaml"
    )
    generate_manifest(
        ctx,
        bgp_type="frr",
        with_prometheus=True,
        output="config/manifests/metallb-frr-prometheus.yaml",
    )
    generate_manifest(
        ctx,
        bgp_type="native",
        with_prometheus=True,
        output="config/manifests/metallb-native-prometheus.yaml",
    )
    generate_manifest(
        ctx, bgp_type="frr-k8s", output="config/manifests/metallb-frr-k8s.yaml"
    )
    generate_manifest(
        ctx,
        bgp_type="frr-k8s",
        with_prometheus=True,
        output="config/manifests/metallb-frr-k8s-prometheus.yaml",
    )

    generate_deepcopy()

    _align_helm_crds(
        source="config/manifests/metallb-frr.yaml",
        output="charts/metallb/charts/crds/templates/crds.yaml",
    )


def generate_deepcopy():
    fetch_controller_gen()
    res = run(
        f'{build_path}/bin/controller-gen object:headerFile="{LICENSE_PATH}" paths="./api/..."'
    )
    if not res.ok:
        raise Exit(message="Failed to generate deepcopy functions")


def _align_helm_crds(source, output):
    run(
        """yq eval-all 'select(.kind == "CustomResourceDefinition")' {} > {}""".format(
            source, output
        )
    )
    run("sed -i 's/metallb-system/{{{{ .Release.Namespace }}}}/g' {}".format(output))


@task
def generateapidocs(ctx):
    """Generates the docs for the CRDs"""
    run("go install github.com/elastic/crd-ref-docs@v0.0.12")
    run(
        "crd-ref-docs --source-path=./api --config=website/generatecrddoc/crdgen.yaml --templates-dir=website/generatecrddoc/template --renderer markdown --output-path=/tmp/generated_apidoc.md"
    )
    run(
        "cat website/generatecrddoc/prefix.html /tmp/generated_apidoc.md > website/content/apis/_index.md"
    )


@task(
    help={
        "action": "The action to take to fix the uncommitted changes",
    }
)
def checkchanges(ctx, action="check uncommitted files"):
    """Verifies no uncommitted files are available"""
    res = run("git status --porcelain", hide="out")
    if res.stdout != "":
        print("{} must be committed".format(res))
        raise Exit(
            message="#### Uncommitted files found, you may need to {} ####\n".format(
                action
            )
        )


@task
def deployprometheus(ctx):
    """Deploys the prometheus operator under the namespace monitoring"""
    fetch_kubectl()
    run(
        "{} apply --server-side -f dev-env/kube-prometheus/manifests/setup".format(
            kubectl_path
        )
    )
    run(
        "until {} get servicemonitors --all-namespaces ; do date; sleep 1; echo "
        "; done".format(kubectl_path)
    )
    run("{} apply -f dev-env/kube-prometheus/manifests/".format(kubectl_path))
    print("Waiting for prometheus pods to be running")
    run(
        "{} -n monitoring wait --for=condition=Ready --all pods --timeout 300s".format(
            kubectl_path
        )
    )


@cache
def fetch_kubectl():
    curl_command = "curl -o {} -LO https://dl.k8s.io/release/{}/bin/$(go env GOOS)/$(go env GOARCH)/kubectl".format(
        kubectl_path, kubectl_version
    )
    get_version_command = f"{kubectl_path} version"
    fetch_dependency(
        kubectl_path,
        kubectl_version,
        curl_command,
        get_version_command,
        "Client Version:",
    )


@cache
def fetch_kind():
    curl_command = "curl -o {} -LO https://github.com/kubernetes-sigs/kind/releases/download/{}/kind-$(go env GOOS)-$(go env GOARCH)".format(
        kind_path, kind_version
    )
    get_version_command = f"{kind_path} version"
    fetch_dependency(kind_path, kind_version, curl_command, get_version_command, "kind")


@cache
def fetch_controller_gen():
    fetch_command = (
        f"GOBIN={build_path}/bin/ GOPATH={build_path} go install sigs.k8s.io/controller-tools/cmd"
        f"/controller-gen@{controller_gen_version}"
    )
    get_version_command = f"{controller_gen_path} --version"
    fetch_dependency(
        controller_gen_path,
        controller_gen_version,
        fetch_command,
        get_version_command,
        "Version:",
    )


@cache
def fetch_ginkgo():
    ginkgo_version = run(
        "go list -m -f '{{.Version}}' github.com/onsi/ginkgo/v2",
        warn=True,
        hide="both",
    ).stdout

    print(ginkgo_version)
    fetch_command = (
        f"GOBIN={build_path}/bin/ GOPATH={build_path} go install github.com/onsi/ginkgo/v2/ginkgo@"
        + ginkgo_version
    )
    print(fetch_command)
    print(ginkgo_path)
    get_version_command = f"{ginkgo_path} version"
    fetch_dependency(
        ginkgo_path,
        ginkgo_version,
        fetch_command,
        get_version_command,
        "Ginkgo Version ",
    )


def fetch_dependency(
    path: str,
    version: str,
    fetch_command: str,
    get_version_command: str,
    version_prefix_in_output: str,
):
    if not os.path.exists(build_path):
        os.makedirs(build_path, mode=0o750)
    if not os.path.exists(path):
        run(fetch_command)
        run("chmod +x {}".format(path))
        return
    current_version = get_command_version(get_version_command, version_prefix_in_output)
    if current_version == version:
        return
    run(fetch_command)
    run("chmod +x {}".format(path))


def get_command_version(get_version_command: str, version_prefix: str) -> Optional[str]:
    version = run(get_version_command, warn=True, hide="both").stdout
    for line in version.splitlines():
        if line.startswith(version_prefix):
            if ":" in line:
                return line.split(":")[1].strip()
            return line.split(" ")[1].strip()


def is_ipv4(addr):
    ip = ipaddress.ip_network(addr)
    return ip.version == 4

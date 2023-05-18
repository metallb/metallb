# pylint: disable=missing-function-docstring
# pylint: disable=too-many-lines
# pylint: disable=fixme
# pylint: disable=missing-module-docstring
# pylint: disable=too-many-arguments
import ipaddress
import os
import re
import shutil
import sys
import tempfile
import time

import semver
import yaml
from invoke import run, task
from invoke.exceptions import Exit, UnexpectedExit

ALL_BINARIES = set(["controller", "speaker", "configmaptocrs"])
ALL_ARCHITECTURES = set(["amd64", "arm", "arm64", "ppc64le", "s390x"])
DEFAULT_NETWORK = "kind"
EXTRA_NETWORK = "network2"
CONTROLLER_GEN_VERSION = "v0.11.1"
BUILD_PATH = os.path.join(os.path.dirname(os.path.abspath(__file__)), "build")
KUBECTL_PATH = os.path.join(BUILD_PATH, "kubectl")
KUBECTL_VERSION = "v1.27.0"


def _check_architectures(architectures):
    out = set()
    for arch in architectures:
        if arch == "all":
            out |= ALL_ARCHITECTURES
        elif arch not in ALL_ARCHITECTURES:
            print(f"unknown architecture {arch}")
            all_archs = ", ".join(sorted(ALL_ARCHITECTURES))
            print(f"Supported architectures: {all_archs}")
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
            out |= ALL_BINARIES
        elif binary not in ALL_BINARIES:
            print(f"Unknown binary {binary}")
            known_bins = ", ".join(sorted(ALL_BINARIES))
            print(f"Known binaries: {known_bins}")
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
            "docker buildx ls >/dev/null"
            "&& echo 'docker buildx build --load' "
            "|| echo 'docker build'",
            hide=True,
        ).stdout.strip()
    return out


def run_with_retry(cmd, tries=6, delay=2):
    """run a shell command with retry"""
    mtries, mdelay = tries, delay
    while mtries > 1:
        return_value = run(cmd, warn="True").exited
        if return_value == 0:
            return
        print(f"Sleeping for {mdelay}s")
        time.sleep(mdelay)
        mtries -= 1
        mdelay *= 2  # exponential backoff
    run(cmd)


# Returns true if docker is a symbolic link to podman.
def _is_podman():
    return "podman" in os.path.realpath(shutil.which("docker"))


def _is_network_exist(network):
    try:
        run(f"docker network inspect {network}")
    except BaseException:  # pylint: disable=broad-exception-caught
        print(f"docker bridge {network} doesn't exist")
        return False
    return True


# Get the list of subnets for the nework.
def _get_network_subnets(network):
    if _is_podman():
        cmd = (
            f"podman network inspect {network} "
            + '-f "{{ range (index .plugins 0).ipam.ranges}}{{ (index . 0).subnet }} {{end}}"'
        )
    else:
        cmd = (
            f"docker network inspect {network} "
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
        for container in containers:
            cmd = (
                f"podman inspect {container} --format '"
                "{{{{.NetworkSettings.Networks.kind.IPAddress}}}} "
                "{{{{.NetworkSettings.Networks.kind.GlobalIPv6Address}}}}'"
            )
            ip_v4, ip_v6 = run(cmd, echo=True).stdout.strip().split(" ")
            v4_ips.append(ip_v4)
            v6_ips.append(ip_v6)
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
    nodes = run(f"kind get nodes --name {cluster_name}").stdout.strip().split("\n")
    ipv6_subnet = "fc00:f853:ccd:e791::/64"
    run(
        f"docker network create --ipv6 --subnet {ipv6_subnet} -d bridge {EXTRA_NETWORK}",
    )
    for node in nodes:
        run(f"docker network connect {EXTRA_NETWORK} {node}")


# Get the nics of kind cluster node
def _get_node_nics(node):
    default_nic = run(
        f'docker exec -i {node} ip r | grep default | cut -d " " -f 5'
    ).stdout.strip()
    if not _is_network_exist(EXTRA_NETWORK):
        return default_nic
    extra_subnets = _get_network_subnets(EXTRA_NETWORK)
    ip = ipaddress.ip_network(extra_subnets[0])  # pylint: disable=invalid-name
    if ip.version == 4:
        extra_nic = run(
            f'docker exec -i {node} ip r | grep {extra_subnets[0]} | cut -d " " -f 3'
        ).stdout.strip()
    else:
        extra_nic = run(
            f'docker exec -i {node} ip -6 r | grep {extra_subnets[0]} | cut -d " " -f 3'
        ).stdout.strip()
    return default_nic + "," + extra_nic


def _get_local_nics():
    nics = []
    for net in [DEFAULT_NETWORK, EXTRA_NETWORK]:
        if not _is_network_exist(net):
            continue
        subnets = _get_network_subnets(net)
        ip = ipaddress.ip_network(subnets[0])  # pylint: disable=invalid-name
        if ip.version == 4:
            nic = run(f'ip r | grep {subnets[0]} | cut -d " " -f 3').stdout.strip()
        else:
            nic = run(f'ip -6 r | grep {subnets[0]} | cut -d " " -f 3').stdout.strip()
        nics.append(nic)
    return ",".join(nics)


@task(
    iterable=["binaries", "architectures"],
    help={
        "binaries": "binaries to build. One or more of "
        f"{', '.join(sorted(ALL_BINARIES))}, or 'all'",
        "architectures": "architectures to build. One or more of "
        f"{', '.join(sorted(ALL_ARCHITECTURES))}, or 'all'",
        "registry": "Docker registry under which to tag the images. Default 'quay.io'.",
        "repo": "Docker repository under which to tag the images. Default 'metallb'.",
        "tag": "Docker image tag prefix to use. Actual tag will be <tag>-<arch>. Default 'dev'.",
    },
)
def build(_ctx, binaries, architectures, registry="quay.io", repo="metallb", tag="dev"):
    """Build MetalLB docker images."""
    binaries = _check_binaries(binaries)
    architectures = _check_architectures(architectures)
    docker_build_cmd = _docker_build_cmd()

    commit = run("git describe --dirty --always", hide=True).stdout.strip()
    branch = run("git rev-parse --abbrev-ref HEAD", hide=True).stdout.strip()

    for arch in architectures:
        for binary in binaries:
            try:
                if _is_podman():
                    command = "podman"
                else:
                    command = "docker"
                run(f"{command} image rm {registry}/{repo}/{binary }:{tag}-{arch}")
            except BaseException:  # pylint: disable=broad-exception-caught
                pass
            run(
                f"{docker_build_cmd} "
                f"--platform linux/{arch} "
                f"-t {registry}/{repo}/{binary}:{tag}-{arch} "
                f"-f {bin}/Dockerfile "
                f'--build-arg GIT_BRANCH="{branch}" '
                f'--build-arg GIT_COMMIT="{commit}" '
                ".",
                echo=True,
            )


@task(
    iterable=["binaries", "architectures"],
    help={
        "binaries": "binaries to build. One or more of "
        f"{', '.join(sorted(ALL_BINARIES))}, or 'all'",
        "architectures": f"architectures to build. One or more of "
        f"{', '.join(sorted(ALL_ARCHITECTURES))}, or 'all'",
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
        for binary in binaries:
            build(
                ctx,
                binaries=[binary],
                architectures=[arch],
                registry=registry,
                repo=repo,
                tag=tag,
            )
            run(
                f"docker push {registry}/{repo}/{binary }:{tag}-{arch}",
                echo=True,
            )


@task(
    iterable=["binaries"],
    help={
        "binaries": "binaries to build. One or more of "
        f"{', '.join(sorted(ALL_BINARIES))}, or 'all'",
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

    platforms = ",".join(f"linux/{arch}" for arch in architectures)
    for binary in binaries:
        run(
            f"manifest-tool push from-args "
            f"--platforms {platforms} "
            f"--template {registry}/{repo}/{binary}:{tag}-ARCH "
            f"--target {registry}/{repo}/{binary}:{tag}",
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
    except Exception as exc:
        raise Exit(
            message="Could not determine kind version (is kind installed?)"
        ) from exc

    actual_version = re.search("v(\\d*\\.\\d*\\.\\d*)", raw.stdout).group(1)
    delta = semver.compare(actual_version, min_version)

    if delta < 0:
        raise Exit(message=f"kind version >= {min_version} required")


def generate_manifest(
    _ctx,
    crd_options="crd:crdVersions=v1",
    bgp_type="native",
    output=None,
    with_prometheus=False,
):
    _fetch_kubectl()
    run(
        f"GOPATH={BUILD_PATH} go install "
        "sigs.k8s.io/controller-tools/cmd/controller-gen@{CONTROLLER_GEN_VERSION}",
    )
    res = run(
        f'{BUILD_PATH}/bin/controller-gen {crd_options} rbac:roleName=manager-role webhook paths="./api/..." output:crd'
        ":art"
        "ifacts:config=config/crd/bases"
    )
    if not res.ok:
        raise Exit(message="Failed to generate manifests")

    if output:
        layer = bgp_type
        if with_prometheus:
            layer = "prometheus-" + layer
        res = run(f"{KUBECTL_PATH} kustomize config/{layer} > {output}")
        if not res.ok:
            raise Exit(message="Failed to kustomize manifests")


def dev_env_handle_helm(*, with_prometheus, architecture, bgp_type):
    run(f"{KUBECTL_PATH} apply -f config/native/ns.yaml", echo=True)
    prometheus_values = ""
    if with_prometheus:
        prometheus_values = (
            "--set prometheus.serviceMonitor.enabled=true "
            "--set prometheus.secureMetricsPort=9120 "
            "--set speaker.frr.secureMetricsPort=9121 "
            "--set prometheus.serviceAccount=prometheus-k8s "
            "--set prometheus.namespace=monitoring "
        )
    run(
        f"helm install metallb charts/metallb/ --set controller.image.tag=dev-{architecture} "
        f"--set speaker.image.tag=dev-{architecture} "
        f"--set speaker.frr.enabled={'true' if bgp_type == 'frr' else 'false'} "
        f"--set speaker.logLevel=debug "
        f"--set controller.logLevel=debug {prometheus_values} --namespace metallb-system",
        echo=True,
    )


def dev_env_handle_kubectl(
    ctx, *, bgp_type, with_prometheus, binaries, architecture, log_level
):
    run(f"{KUBECTL_PATH} delete po -n metallb-system --all", echo=True)

    with tempfile.TemporaryDirectory() as tmpdir:
        manifest_file = tmpdir + "/metallb.yaml"

        generate_manifest(
            ctx,
            bgp_type=bgp_type,
            output=manifest_file,
            with_prometheus=with_prometheus,
        )

        # open file and replace the images with the newely built MetalLB
        # docker images
        with open(manifest_file, encoding="utf-8") as f:  # pylint: disable=invalid-name
            manifest = f.read()
        for image in binaries:
            manifest = re.sub(
                f"image: quay.io/metallb/{image}:.*",
                f"image: quay.io/metallb/{image}:dev-{architecture}",
                manifest,
            )
            manifest = re.sub("--log-level=info", f"--log-level={log_level}", manifest)
        manifest.replace("--log-level=info", "--log-level=debug")

        with open(
            manifest_file, "w", encoding="utf-8"
        ) as f:  # pylint: disable=invalid-name
            f.write(manifest)
            f.flush()

        run(f"{KUBECTL_PATH} apply -f {manifest_file}", echo=True)


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
        "Supported: 'native' (default), 'frr'",
        "frr_volume_dir": "FRR router config directory to be mounted inside frr container. "
        "Default: ./dev-env/bgp/frr-volume",
        "log_level": "Log level for the controller and the speaker."
        "Default: info, Supported: 'all', 'debug', 'info', 'warn', 'error' or 'none'",
        "helm_install": "Optional install MetalLB via helm chart instead of manifests."
        "Default: False.",
        "build_images": "Optional build the images." "Default: True.",
        "with_prometheus": "Deploys the prometheus kubernetes stack" "Default: False.",
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
    bgp_type="native",
    log_level="info",
    helm_install=False,
    build_images=True,
    with_prometheus=False,
):
    """Build and run MetalLB in a local Kind cluster.

    If the cluster specified by --name (default "kind") doesn't exist,
    it is created. Then, build MetalLB docker images from the
    checkout, push them into kind, and deploy MetalLB through manifests
    or helm to run those images.
    The optional node_img parameter will be used to determine the version of the cluster.
    """

    _fetch_kubectl()
    validate_kind_version()

    clusters = run("kind get clusters", hide=True).stdout.strip().splitlines()

    def make_cluster(name):
        config = {
            "apiVersion": "kind.x-k8s.io/v1alpha4",
            "kind": "Cluster",
            "nodes": [
                {"role": "control-plane"},
                {"role": "worker"},
                {"role": "worker"},
            ],
        }

        networking_config = {}
        if ip_family != "ipv4":
            networking_config["ipFamily"] = ip_family

        if len(networking_config) > 0:
            config["networking"] = networking_config

        extra_options = ""
        if node_img is not None:
            extra_options = f"--image={node_img}"
        config = yaml.dump(config).encode("utf-8")
        with tempfile.NamedTemporaryFile() as tmp:
            tmp.write(config)
            tmp.flush()
            run(
                f"kind create cluster --name={name} --config={tmp.name} {extra_options}",
                pty=True,
                echo=True,
            )
        _add_nic_to_nodes(name)

    # if name not in clusters then make cluster
    if name not in clusters:
        make_cluster(name)

    binaries = ["controller", "speaker"]
    if build_images:
        build(ctx, binaries, architectures=[architecture])
    run(
        f"kind load docker-image --name={name} quay.io/metallb/controller:dev-{architecture}",
        echo=True,
    )
    run(
        f"kind load docker-image --name={name} quay.io/metallb/speaker:dev-{architecture}",
        echo=True,
    )

    if with_prometheus:
        print("Deploying prometheus")
        deploy_prometheus(ctx)

    if helm_install:
        dev_env_handle_helm(
            with_prometheus=with_prometheus,
            architecture=architecture,
            bgp_type=bgp_type,
        )
    else:
        dev_env_handle_kubectl(
            ctx,
            bgp_type=bgp_type,
            with_prometheus=with_prometheus,
            binaries=binaries,
            architecture=architecture,
            log_level=log_level,
        )

    if protocol == "bgp":
        print("Configuring MetalLB with a BGP test environment")
        bgp_dev_env(ip_family, frr_volume_dir)
    elif protocol == "layer2":
        print("Configuring MetalLB with a layer 2 test environment")
        layer2_dev_env()
    else:
        print("Leaving MetalLB unconfigured")


# Configure MetalLB in the dev-env for layer2 testing.
# Identify the unused network address range from kind network and used it
# in configmap.
def layer2_dev_env():
    _fetch_kubectl()
    dev_env_dir = os.getcwd() + "/dev-env/layer2"
    with open(
        f"{dev_env_dir}/config.yaml.tmpl", "r", encoding="utf-8"
    ) as f:  # pylint: disable=invalid-name
        layer2_config = "# THIS FILE IS AUTOGENERATED\n" + f.read()
    layer2_config = layer2_config.replace("SERVICE_V4_RANGE", get_available_ips(4)[0])
    layer2_config = layer2_config.replace("SERVICE_V6_RANGE", get_available_ips(6)[0])
    with open(
        f"{dev_env_dir}/config.yaml", "w", encoding="utf-8"
    ) as f:  # pylint: disable=invalid-name
        f.write(layer2_config)
    # Apply the MetalLB ConfigMap
    run(f"{KUBECTL_PATH} apply -f {dev_env_dir}/config.yaml")


def bgp_dev_env(ip_family, frr_volume_dir):
    """
    Configure MetalLB in the dev-env for BGP testing. Start an frr based BGP
    router in a container and configure MetalLB to peer with it.
    See dev-env/bgp/README.md for some more information.
    """
    _fetch_kubectl()
    dev_env_dir = os.getcwd() + "/dev-env/bgp"
    if frr_volume_dir == "":
        frr_volume_dir = dev_env_dir + "/frr-volume"

    # TODO -- The IP address handling will need updates to add support for IPv6

    # We need the IPs for each Node in the cluster to place them in the BGP
    # router configuration file (bgpd.conf). Each Node will peer with this
    # router.
    node_ips = run(
        f"{KUBECTL_PATH} get nodes -o jsonpath='{{.items[*].status.addresses"
        '[?(@.type=="InternalIP")].address}}{{"\\n"}}\'',
        echo=True,
    )
    node_ips = node_ips.stdout.strip().split()
    if len(node_ips) != 3:
        raise Exit(message=f"Expected 3 nodes, got {node_ips:d}")

    # Create a new directory that will be used as the config volume for frr.
    try:
        # sudo because past docker runs will have changed ownership of this dir
        run(f'sudo rm -rf "{frr_volume_dir}"')
        os.mkdir(frr_volume_dir)
    except FileExistsError:
        pass
    except Exception as exception:
        raise Exit(
            message=f"Failed to create frr-volume directory: {str(exception)}"
        ) from exception

    # These config files are static, so we copy them straight in.
    copy_files = ("zebra.conf", "daemons", "vtysh.conf")
    for f in copy_files:  # pylint: disable=invalid-name
        shutil.copyfile(f"{dev_env_dir}/frr/{f}", f"{frr_volume_dir}/{f}")

    # bgpd.conf is created from a template so that we can include the current
    # Node IPs.
    with open(
        f"{dev_env_dir}/frr/bgpd.conf.tmpl", "r", encoding="utf-8"
    ) as f:  # pylint: disable=invalid-name
        bgpd_config = "! THIS FILE IS AUTOGENERATED\n" + f.read()
        bgpd_config = bgpd_config.replace("PROTOCOL", ip_family)
    for n, node_ip in enumerate(node_ips):  # pylint: disable=invalid-name
        bgpd_config = bgpd_config.replace(f"NODE{n}_IP", node_ip)
    with open(
        f"{frr_volume_dir}/bgpd.conf", "w", encoding="utf-8"
    ) as f:  # pylint: disable=invalid-name
        f.write(bgpd_config)

    # Run a BGP router in a container for all of the speakers to peer with.
    run(
        "for frr in $(docker ps -a -f name=frr --format {{.Names}}) ; do "
        "    docker rm -f $frr ; "
        "done",
        echo=True,
    )
    run(
        "docker run -d --privileged --network kind --rm --ulimit core=-1 "
        f"--name frr --volume {frr_volume_dir}:/etc/frr "
        "quay.io/frrouting/frr:8.4.2",
        echo=True,
    )

    if ip_family == "ipv4":
        peer_address = run(
            'docker inspect -f "{{ '
            'range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}" frr',
            echo=True,
        )
    elif ip_family == "ipv6":
        peer_address = run(
            'docker inspect -f "{{ '
            'range .NetworkSettings.Networks}}{{.GlobalIPv6Address}}{{end}}" frr',
            echo=True,
        )
    else:
        raise Exit(message=f"Unsupported ip address family {ip_family}")

    with open(
        f"{dev_env_dir}/config.yaml.tmpl", "r", encoding="utf-8"
    ) as f:  # pylint: disable=invalid-name
        mlb_config = "# THIS FILE IS AUTOGENERATED\n" + f.read()
    mlb_config = mlb_config.replace("PEER_IP_ADDRESS", peer_address.stdout.strip())
    with open(
        f"{dev_env_dir}/config.yaml", "w", encoding="utf-8"
    ) as f:  # pylint: disable=invalid-name
        f.write(mlb_config)
    # Apply the MetalLB ConfigMap
    run_with_retry(f"{KUBECTL_PATH} apply -f {dev_env_dir}/config.yaml")


def get_available_ips(ip_family=None):
    if ip_family is None or (ip_family not in [4, 6]):
        raise Exit(message="Please provide network version: 4 or 6.")

    v4, v6 = _get_subnets_allocated_ips()  # pylint: disable=invalid-name
    for i in _get_network_subnets(DEFAULT_NETWORK):
        network = ipaddress.ip_network(i)
        if network.version == ip_family:
            used_list = v4 if ip_family == 4 else v6
            last_used = ipaddress.ip_interface(used_list[-1])

            # try to get 10 IP addresses after the last assigned node address in the kind network subnet,
            # plus we give room to thr frr single hop containers.
            # if failed, just quit (recreate kind cluster might solve the
            # situation)
            service_ip_range_start = last_used + 5
            service_ip_range_end = last_used + 15
            if service_ip_range_start not in network:
                raise Exit(
                    message=f"network range {service_ip_range_start} is not in {network}"
                )
            if service_ip_range_end not in network:
                raise Exit(
                    message=f"network range {service_ip_range_end} is not in { network}",
                )
            return f"{service_ip_range_start.ip}-{service_ip_range_end.ip}"
    return None


@task(
    help={
        "name": "name of the kind cluster to delete.",
        "frr_volume_dir": "FRR router config directory to be cleaned up. "
        "Default: ./dev-env/bgp/frr-volume",
    }
)
def dev_env_cleanup(_ctx, name="kind", frr_volume_dir=""):
    """Remove traces of the dev env."""
    validate_kind_version()
    clusters = run("kind get clusters", hide=True).stdout.strip().splitlines()
    if name in clusters:
        run(f"kind delete cluster --name={name}", hide=True)

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
    run(f'sudo rm -rf "{frr_volume_dir}"')
    run(f'rm -f "{dev_env_dir}"/config.yaml')

    # cleanup layer2 configs
    dev_env_dir = os.getcwd() + "/dev-env/layer2"
    run(f'rm -f "{dev_env_dir}"/config.yaml')

    # cleanup extra bridge
    run(f"docker network rm {EXTRA_NETWORK}", warn=True)
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
        with open(
            "website/content/release-notes/_index.md", encoding="utf-8"
        ) as release_notes:
            if f"## Version {sem_version}" not in release_notes.read():
                raise Exit(message=f"no release notes for v{sem_version}")

    # Move HEAD to the correct release branch - either a new one, or
    # an existing one.
    if is_patch_release:
        run(
            f"git checkout v{sem_version.major}.{sem_version.minor}",
            echo=True,
        )
    else:
        run(
            f"git checkout -b v{sem_version.major}.{sem_version.minor}",
            echo=True,
        )

    # Copy over release notes from main.
    if not skip_release_notes:
        run("git checkout main -- website/content/release-notes/_index.md", echo=True)

    # Update links on the website to point to files at the version
    # we're creating.
    if is_patch_release:
        previous_version = (
            f"v{sem_version.major}.{sem_version.minor}.{sem_version.patch - 1}"
        )
    else:
        previous_version = "main"
    bumprelease(ctx, version, previous_version)

    run(
        f"git commit -a -m 'Automated update for release v{sem_version}'",
        echo=True,
    )
    detail_url = (
        "https://metallb.universe.tf/release-notes/"
        f"#version-{sem_version.major}-{sem_version.minor}-{sem_version.patch}"
    )
    run(
        f"git tag v{sem_version} -m 'See the release notes for details:\n\n{detail_url}'",
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
            f"perl -pi -e 's#{oldpat}#{newpat}#g' website/content/*.md website/content/*/*.md",
            echo=True,
        )

    _replace("/metallb/metallb/{}")
    _replace("/metallb/metallb/tree/{}")
    _replace("/metallb/metallb/blob/{}")

    # Update the version listed on the website sidebar
    run(
        f"perl -pi -e 's/MetalLB .*/MetalLB v{version}/g' website/content/_header.md",
        echo=True,
    )

    # Update the manifests with the new version
    run(
        f"perl -pi -e 's,image: quay.io/metallb/speaker:.*,image: quay.io/metallb/speaker:v{version},g' "
        "config/controllers/speaker.yaml",
        echo=True,
    )
    run(
        f"perl -pi -e 's,image: quay.io/metallb/controller:.*,image: quay.io/metallb/controller:v{version},g' "
        "config/controllers/controller.yaml",
        echo=True,
    )

    # Update the versions in the helm chart (version and appVersion are always the same)
    # helm chart versions follow Semantic Versioning, and thus exclude the
    # leading 'v'
    run(
        f"perl -pi -e 's,version: .*,version: {version},g' charts/metallb/Chart.yaml",
        echo=True,
    )
    run(
        f"perl -pi -e 's,^appVersion: .*,appVersion: v{version},g' charts/metallb/Chart.yaml",
        echo=True,
    )
    run(
        f"perl -pi -e 's,^version: .*,version: {version},g' charts/metallb/charts/crds/Chart.yaml",
        echo=True,
    )
    run(
        f"perl -pi -e 's,^appVersion: .*,appVersion: v{version},g' charts/metallb/charts/crds/Chart.yaml",
        echo=True,
    )
    run(
        "perl -pi -e 's,^Current chart version is: .*,"
        f"Current chart version is: `{version}`,g' charts/metallb/README.md",
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
        "sed -i "
        "'s/github.com\\/metallb\\/metallb\\/config\\/"
        f"native?ref=.*$/github.com\\/metallb\\/metallb\\/config\\/native?ref=v{version}/g' "
        "website/content/installation/_index.md"
    )
    run(
        "sed -i "
        "'s/github.com\\/metallb\\/metallb\\/config\\/"
        f"frr?ref=.*$/github.com\\/metallb\\/metallb\\/config\\/frr?ref=v{version}/g' "
        "website/content/installation/_index.md"
    )

    # Update the version embedded in the binary
    run(
        f"perl -pi -e 's/version\\s+=.*/version = \"{version}\"/g' internal/version/version.go",
        echo=True,
    )
    run("gofmt -w internal/version/version.go", echo=True)


@task
def test(_ctx):
    """Run unit tests."""
    envtest_asset_dir = os.getcwd() + "/dev-env/unittest"
    run(
        f"source {envtest_asset_dir}/setup-envtest.sh; fetch_envtest_tools {envtest_asset_dir}",
        echo=True,
    )
    run(
        f"source {envtest_asset_dir}/setup-envtest.sh; setup_envtest_env {envtest_asset_dir}; go test -short ./...",
        echo=True,
    )
    run(
        f"source {envtest_asset_dir}/setup-envtest.sh; "
        f"setup_envtest_env {envtest_asset_dir}; go test -short -race ./...",
        echo=True,
    )


@task
def checkpatch(_ctx):
    # Generate a diff of all changes on this branch from origin/main
    # and look for any added lines with 2 spaces after a period.
    try:
        lines = run(
            "git diff $(diff -u <(git rev-list --first-parent HEAD) "
            " <(git rev-list --first-parent origin/main) "
            " | sed -ne 's/^ //p' | head -1)..HEAD | "
            " grep '+.*\\.\\  '"
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
def lint(_ctx, env="container"):
    """Run linter.

    By default, this will run a golangci-lint docker image against the code.
    However, in some environments (such as the MetalLB CI), it may be more
    convenient to install the golangci-lint binaries on the host. This can be
    achieved by running `inv lint --env host`.
    """
    version = "1.52.2"
    golangci_cmd = "golangci-lint run --timeout 10m0s ./..."

    if env == "container":
        run(
            f"docker run --rm -v $(git rev-parse --show-toplevel):/app "
            f"-w /app golangci/golangci-lint:v{version} {golangci_cmd}",
            echo=True,
        )
    elif env == "host":
        url = (
            "https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh"
        )
        run(
            f"curl -sSfL {url}| sh -s -- -b $(go env GOPATH)/bin v{version}",
        )
        run(golangci_cmd)
    else:
        raise Exit(message=f"Unsupported linter environment: {env}")


@task(
    help={
        "env": "Specify in which environment to run helmdocs . Default 'container'. Supported: 'container','host'"
    }
)
def helmdocs(_ctx, env="container"):
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
            f"docker run --rm -v $(git rev-parse --show-toplevel):/app -w /app jnorwood/helm-docs:v{version} {cmd}",
            echo=True,
        )
    elif env == "host":
        run(cmd)
    else:
        raise Exit(message=f"Unsupported helm-docs environment: {env}")


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
        "external_containers": "a comma separated list of external containers names to use for the test. "
        "(valid parameters are: ibgp-single-hop / ibgp-multi-hop / ebgp-single-hop / ebgp-multi-hop)",
        "native_bgp": "tells if the given cluster is deployed using native bgp mode ",
        "external_frr_image": "overrides the image used for the "
        "external frr containers used in tests",
    }
)
def e2etest(
    _ctx,
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
    native_bgp=False,
    external_frr_image="",
):  # pylint: disable=too-many-locals
    """Run E2E tests against development cluster."""
    _fetch_kubectl()

    opt_skip_docker = "--skip-docker" if skip_docker else ""
    ginkgo_skip = '--skip="' + skip + '"' if len(skip) > 0 else ""
    ginkgo_focus = '--focus="' + focus + '"' if len(focus) > 0 else ""

    if kubeconfig is None:
        validate_kind_version()
        clusters = run("kind get clusters", hide=True).stdout.strip().splitlines()
        if name in clusters:
            with tempfile.NamedTemporaryFile() as kubeconfig_file:
                kubeconfig = kubeconfig_file.name
                run(
                    f"kind export kubeconfig --name={name} --kubeconfig={kubeconfig}",
                    pty=True,
                    echo=True,
                )
        else:
            raise Exit(message=f"Unable to find cluster named: {name}")
    else:
        os.environ["KUBECONFIG"] = kubeconfig

    namespaces = system_namespaces.replace(" ", "").split(",")
    for namespace in namespaces:
        run(
            f"{KUBECTL_PATH} -n {namespace} wait --for=condition=Ready --all pods --timeout 300s",
            hide=True,
        )

    if node_nics == "kind":
        nodes = run(f"kind get nodes --name {name}").stdout.strip().split("\n")
        node_nics = _get_node_nics(nodes[0])

    if local_nics == "kind":
        local_nics = _get_local_nics()

    if ipv4_service_range is None:
        ipv4_service_range = get_available_ips(4)

    if ipv6_service_range is None:
        ipv6_service_range = get_available_ips(6)

    report_path = export if export is not None else f"/tmp/metallbreport{time.time()}"
    prometheus_namespace = (
        "--prometheus-namespace=" + prometheus_namespace
        if prometheus_namespace != ""
        else ""
    )

    print(f"Writing reports to {report_path}")
    os.makedirs(report_path, exist_ok=True)

    external_containers = (
        "--external-containers=" + (external_containers)
        if external_containers != ""
        else ""
    )

    external_frr_image = (
        "--frr-image=" + (external_frr_image) if external_frr_image != "" else ""
    )

    testrun = run(
        "cd `git rev-parse --show-toplevel`/e2etest &&"
        f"KUBECONFIG={kubeconfig} ginkgo --timeout=3h {ginkgo_focus} {ginkgo_skip} "
        f"-- --provider=local --kubeconfig={kubeconfig} "
        f"--service-pod-port={service_pod_port} "
        f"-ipv4-service-range={ipv4_service_range} -ipv6-service-range={ipv6_service_range} "
        f"{opt_skip_docker} --report-path {report_path} "
        f"{prometheus_namespace} -node-nics {node_nics} "
        f"-local-nics {local_nics} {external_containers} "
        f"-bgp-native-mode={native_bgp} {external_frr_image}",
        warn="True",
    )

    if export is not None:
        run(f"kind export logs {export}")

    if testrun.failed:
        raise Exit(message="E2E tests failed", code=testrun.return_code)


@task
def bumplicense(_ctx):
    """Bumps the license header on all go files that have it missing"""

    res = run("find . -name '*.go' | grep -v dev-env")
    for file in res.stdout.splitlines():
        res = run(f"grep -q License {file}", warn=True)
        if not res.ok:
            run(r"sed -i '1s/^/\/\/ SPDX-License-Identifier:Apache-2.0\n\n/' " + file)


@task
def verifylicense(_ctx):
    """Verifies all files have the corresponding license"""
    res = run("find . -name '*.go' | grep -v dev-env", hide="out")
    no_license = False
    for file in res.stdout.splitlines():
        res = run(f"grep -q License {file}", warn=True)
        if not res.ok:
            no_license = True
            print(f"{file} is missing license")
    if no_license:
        raise Exit(
            message="#### Files with no license found.\n#### Please run "
            "inv bumplicense"
            " to add the license header"
        )


@task
def gomodtidy(_ctx):
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


@task
def generateapidocs(_ctx):
    """Generates the docs for the CRDs"""
    run(
        "go install "
        "github.com/ahmetb/gen-crd-api-reference-docs@3f29e6853552dcf08a8e846b1225f275ed0f3e3b"
    )
    run(
        "gen-crd-api-reference-docs -config website/generatecrddoc/crdgen.json "
        "-template-dir website/generatecrddoc/template "
        '-api-dir "go.universe.tf/metallb/api" -out-file /tmp/generated_apidoc.html'
    )
    run(
        "cat website/generatecrddoc/prefix.html "
        "/tmp/generated_apidoc.html > website/content/apis/_index.md"
    )


@task(
    help={
        "action": "The action to take to fix the uncommitted changes",
    }
)
def checkchanges(_ctx, action="check uncommitted files"):
    """Verifies no uncommitted files are available"""
    res = run("git status --porcelain", hide="out")
    if res.stdout != "":
        print(f"{res} must be committed")
        raise Exit(
            message=f"#### Uncommitted files found, you may need to {action} ####\n"
        )


@task
def deploy_prometheus(_ctx):
    """Deploys the prometheus operator under the namespace monitoring"""
    _fetch_kubectl()
    run(
        f"{KUBECTL_PATH} apply --server-side -f dev-env/kube-prometheus/manifests/setup"
    )
    run(
        f"until {KUBECTL_PATH} get servicemonitors --all-namespaces ; do date; sleep 1; echo "
        "; done"
    )
    run(f"{KUBECTL_PATH} apply -f dev-env/kube-prometheus/manifests/")
    print("Waiting for prometheus pods to be running")
    run(
        f"{KUBECTL_PATH} -n monitoring wait --for=condition=Ready --all pods --timeout 300s"
    )


def _fetch_kubectl():
    if not os.path.exists(BUILD_PATH):
        os.makedirs(BUILD_PATH, mode=0o750)
    url = f"https://dl.k8s.io/release/{KUBECTL_VERSION}/bin/$(go env GOOS)/$(go env GOARCH)/kubectl"
    curl_command = f"curl -o {KUBECTL_PATH} -LO {url}"
    if not os.path.exists(KUBECTL_PATH):
        run(curl_command)
        run(f"chmod +x {KUBECTL_PATH}")
        return
    version = run(f"{KUBECTL_PATH} version --short", warn=True, hide="both").stdout
    for line in version.splitlines():
        if line.startswith("Client Version:"):
            version = line.split(":")[1].strip()
            if version == KUBECTL_VERSION:
                return
    run(curl_command)
    run(f"chmod +x {KUBECTL_PATH}")

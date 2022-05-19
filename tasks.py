import ipaddress
import os
import semver
import re
import shutil
import sys
import yaml
import tempfile
import time
try:
    from urllib.request import urlopen
except ImportError:
    from urllib2 import urlopen

from invoke import run, task
from invoke.exceptions import Exit, UnexpectedExit

all_binaries = set(["controller",
                    "speaker",
                    "mirror-server"])
all_architectures = set(["amd64",
                         "arm",
                         "arm64",
                         "ppc64le",
                         "s390x"])

def _check_architectures(architectures):
    out = set()
    for arch in architectures:
        if arch == "all":
            out |= all_architectures
        elif arch not in all_architectures:
            print("unknown architecture {}".format(arch))
            print("Supported architectures: {}".format(", ".join(sorted(all_architectures))))
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
    return list(sorted(out))

def _docker_build_cmd():
    cmd = os.getenv('DOCKER_BUILD_CMD')
    if cmd:
        out = cmd
    else:
        out = run("docker buildx ls >/dev/null"
                  "&& echo 'docker buildx build --load' "
                  "|| echo 'docker build'", hide=True).stdout.strip()
    return out

def _make_build_dirs():
    for arch in all_architectures:
        for binary in all_binaries:
            dir = os.path.join("build", arch, binary)
            if not os.path.exists(dir):
                os.makedirs(dir, mode=0o750)


# Returns true if docker is a symbolic link to podman.
def _is_podman():
    return 'podman' in os.path.realpath(shutil.which('docker'))


# Get the list of subnets for the kind nework.
def _get_network_subnets():
    if _is_podman():
        cmd = ('podman network inspect kind -f "'
               '{{ range (index .plugins 0).ipam.ranges}}'
               '{{ (index . 0).subnet }} {{end}}"')
    else:
        cmd = ('docker network inspect kind -f "'
               '{{ range .IPAM.Config}}{{.Subnet}} {{end}}"'
               )
    return run(cmd, echo=True).stdout.strip().split(' ')


# Get the list of allocated IPv4 and IPv6 addresses for the kind network.
def _get_subnets_allocated_ips():
    v4_ips = []
    v6_ips = []

    if _is_podman():
        cmd = 'podman ps -f network=kind --format "{{.ID}}"'
        containers = run(cmd, echo=True).stdout.strip().split('\n')
        # for each container, get the IP address and add it to the list of
        # allocated IPs
        for c in containers:
            cmd = ("podman inspect {container} --format '"
                   "{{{{.NetworkSettings.Networks.kind.IPAddress}}}} "
                   "{{{{.NetworkSettings.Networks.kind.GlobalIPv6Address}}}}'"
                   ).format(container=c)
            v4, v6 = run(cmd, echo=True).stdout.strip().split(' ')
            v4_ips.append(v4)
            v6_ips.append(v6)
    else:
        v4_ips = run('docker network inspect kind -f '
                     '"{{range .Containers}}{{.IPv4Address}} {{end}}"',
                     echo=True).stdout.strip().split(' ')
        v6_ips = run('docker network inspect kind -f '
                     '"{{range .Containers}}{{.IPv6Address}} {{end}}"',
                     echo=True).stdout.strip().split(' ')

    return sorted(v4_ips), sorted(v6_ips)


@task(iterable=["binaries", "architectures"],
      help={
          "binaries": "binaries to build. One or more of {}, or 'all'".format(", ".join(sorted(all_binaries))),
          "architectures": "architectures to build. One or more of {}, or 'all'".format(", ".join(sorted(all_architectures))),
          "registry": "Docker registry under which to tag the images. Default 'quay.io'.",
          "repo": "Docker repository under which to tag the images. Default 'metallb'.",
          "tag": "Docker image tag prefix to use. Actual tag will be <tag>-<arch>. Default 'dev'.",
      })
def build(ctx, binaries, architectures, registry="quay.io", repo="metallb", tag="dev"):
    """Build MetalLB docker images."""
    binaries = _check_binaries(binaries)
    architectures = _check_architectures(architectures)
    docker_build_cmd = _docker_build_cmd()
    _make_build_dirs()

    commit = run("git describe --dirty --always", hide=True).stdout.strip()
    branch = run("git rev-parse --abbrev-ref HEAD", hide=True).stdout.strip()

    for arch in architectures:
        env = {
            "CGO_ENABLED": "0",
            "GOOS": "linux",
            "GOARCH": arch,
            "GOARM": "6",
            "GO111MODULE": "on",
        }
        if "speaker" in binaries:
            shutil.copy("frr-reloader/frr-reloader.sh","build/{arch}/speaker/".format(arch=arch))
            run("go build -v -o build/{arch}/speaker/frr-metrics -ldflags "
                "'-X go.universe.tf/metallb/internal/version.gitCommit={commit} "
                "-X go.universe.tf/metallb/internal/version.gitBranch={branch}' "
                "frr-metrics/exporter.go".format(
                    arch=arch,
                    commit=commit,
                    branch=branch),
                    env=env,
                    echo=True,
                )

        for bin in binaries:
            run("go build -v -o build/{arch}/{bin}/{bin} -ldflags "
                "'-X go.universe.tf/metallb/internal/version.gitCommit={commit} "
                "-X go.universe.tf/metallb/internal/version.gitBranch={branch}' "
                "go.universe.tf/metallb/{bin}".format(
                    arch=arch,
                    bin=bin,
                    commit=commit,
                    branch=branch),
                env=env,
                echo=True)
            run("{docker_build_cmd} "
                "--platform linux/{arch} "
                "-t {registry}/{repo}/{bin}:{tag}-{arch} "
                "-f {bin}/Dockerfile build/{arch}/{bin}".format(
                    docker_build_cmd=docker_build_cmd,
                    registry=registry,
                    repo=repo,
                    bin=bin,
                    tag=tag,
                    arch=arch),
                echo=True)


@task(iterable=["binaries", "architectures"],
      help={
          "binaries": "binaries to build. One or more of {}, or 'all'".format(", ".join(sorted(all_binaries))),
          "architectures": "architectures to build. One or more of {}, or 'all'".format(", ".join(sorted(all_architectures))),
          "registry": "Docker registry under which to tag the images. Default 'quay.io'.",
          "repo": "Docker repository under which to tag the images. Default 'metallb'.",
          "tag": "Docker image tag prefix to use. Actual tag will be <tag>-<arch>. Default 'dev'.",
      })
def push(ctx, binaries, architectures, registry="quay.io", repo="metallb", tag="dev"):
    """Build and push docker images to registry."""
    binaries = _check_binaries(binaries)
    architectures = _check_architectures(architectures)

    for arch in architectures:
        for bin in binaries:
            build(ctx, binaries=[bin], architectures=[arch], registry=registry, repo=repo, tag=tag)
            run("docker push {registry}/{repo}/{bin}:{tag}-{arch}".format(
                registry=registry,
                repo=repo,
                bin=bin,
                arch=arch,
                tag=tag),
                echo=True)


@task(iterable=["binaries"],
      help={
          "binaries": "binaries to build. One or more of {}, or 'all'".format(", ".join(sorted(all_binaries))),
          "registry": "Docker registry under which to tag the images. Default 'quay.io'.",
          "repo": "Docker repository under which to tag the images. Default 'metallb'.",
          "tag": "Docker image tag prefix to use. Actual tag will be <tag>-<arch>. Default 'dev'.",
      })
def push_multiarch(ctx, binaries, registry="quay.io", repo="metallb", tag="dev"):
    """Build and push multi-architecture docker images to registry."""
    binaries = _check_binaries(binaries)
    architectures = _check_architectures(["all"])
    push(ctx, binaries=binaries, architectures=architectures, registry=registry, repo=repo, tag=tag)

    platforms = ",".join("linux/{}".format(arch) for arch in architectures)
    for bin in binaries:
        run("manifest-tool push from-args "
            "--platforms {platforms} "
            "--template {registry}/{repo}/{bin}:{tag}-ARCH "
            "--target {registry}/{repo}/{bin}:{tag}".format(
                platforms=platforms,
                registry=registry,
                repo=repo,
                bin=bin,
                tag=tag),
            echo=True)


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

    actual_version = re.search("v(\d*\.\d*\.\d*)", raw.stdout).group(1)
    delta = semver.compare(actual_version, min_version)

    if delta < 0:
        raise Exit(message="kind version >= {} required".format(min_version))

@task(help={
    "version": "version of cert-manager to install."
                "Default: v1.5.4",
})
def install_cert_manager(ctx, version="v1.5.4"):
    res = run("kubectl apply -f https://github.com/jetstack/cert-manager/releases/download/{}/cert-manager.yaml".format(version))
    if not res.ok:
        raise Exit(message="Failed to install cert-manager")
    
    # wait for cert-manager to be ready
    attempts = 0
    max_attempts = 60
    cert_manager_ready = False
    while not cert_manager_ready and attempts != max_attempts:
        print("Waiting for cert-manager to be ready attempt: {}".format(attempts))
        try:
            res = run("kubectl apply -f config/certmanager/self-signed-cert.yaml")
        except Exception as e:
            print("Failed, retrying")
            time.sleep(5)
        else:
            print("cert-manager is ready")
            cert_manager_ready = True
            res = run("kubectl delete -f config/certmanager/self-signed-cert.yaml")
        attempts += 1
    if not cert_manager_ready:
        raise Exit(message="Timed out waiting for cert-manage to be ready")

def generate_manifest(ctx, controller_gen="controller-gen", crd_options="crd:crdVersions=v1",
        kustomize_cli="kustomize", bgp_type="native", output=None, enable_webhooks=False):
    res = run("{} {} rbac:roleName=manager-role webhook paths=\"./api/...\" output:crd:artifacts:config=config/crd/bases".format(controller_gen, crd_options))
    if not res.ok:
        raise Exit(message="Failed to generate manifests")

    webhook_flag = "-with-webhook" if enable_webhooks else ""

    if output:
        res = run("kubectl kustomize config/{}{} > {}".format(bgp_type, webhook_flag, output))
        if not res.ok:
            raise Exit(message="Failed to kustomize manifests")

@task(help={
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
    "enable_webhooks": "Optional enable MetalLB webhooks."
                "Default: False.",
})
def dev_env(ctx, architecture="amd64", name="kind", protocol=None, frr_volume_dir="",
        node_img=None, ip_family="ipv4", bgp_type="native", log_level="info",
        helm_install=False, enable_webhooks=False):
    """Build and run MetalLB in a local Kind cluster.

    If the cluster specified by --name (default "kind") doesn't exist,
    it is created. Then, build MetalLB docker images from the
    checkout, push them into kind, and deploy MetalLB through manifests
    or helm to run those images.
    The optional node_img parameter will be used to determine the version of the cluster.
    """

    validate_kind_version()

    clusters = run("kind get clusters", hide=True).stdout.strip().splitlines()
    mk_cluster = name not in clusters
    if mk_cluster:
        config = {
            "apiVersion": "kind.x-k8s.io/v1alpha4",
            "kind": "Cluster",
            "nodes": [{"role": "control-plane"},
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
        if node_img != None:
            extra_options = "--image={}".format(node_img)
        config = yaml.dump(config).encode("utf-8")
        with tempfile.NamedTemporaryFile() as tmp:
            tmp.write(config)
            tmp.flush()
            run("kind create cluster --name={} --config={} {}".format(name, tmp.name, extra_options), pty=True, echo=True)

    binaries = ["controller", "speaker", "mirror-server"]
    build(ctx, binaries, architectures=[architecture])
    run("kind load docker-image --name={} quay.io/metallb/controller:dev-{}".format(name, architecture), echo=True)
    run("kind load docker-image --name={} quay.io/metallb/speaker:dev-{}".format(name, architecture), echo=True)
    run("kind load docker-image --name={} quay.io/metallb/mirror-server:dev-{}".format(name, architecture), echo=True)

    if enable_webhooks:
        install_cert_manager(ctx)

    if helm_install:
        run("helm install metallb charts/metallb/ --set controller.image.tag=dev-{} "
                "--set speaker.image.tag=dev-{} --set speaker.frr.enabled={} --set speaker.logLevel=debug "
                "--set controller.logLevel=debug --set webhooks.enable={}".format(architecture,
                architecture, "true" if bgp_type == "frr" else "false", "true" if enable_webhooks else "false"), echo=True)
    else:
        run("kubectl delete po -nmetallb-system --all", echo=True)

        with tempfile.TemporaryDirectory() as tmpdir:
            manifest_file = tmpdir + "/metallb.yaml"

            generate_manifest(ctx, bgp_type=bgp_type, output=manifest_file, enable_webhooks=enable_webhooks)

            # open file and replace the images with the newely built MetalLB docker images
            with open(manifest_file) as f:
                manifest = f.read()
            for image in binaries:
                manifest = re.sub("image: quay.io/metallb/{}:.*".format(image),
                            "image: quay.io/metallb/{}:dev-{}".format(image, architecture), manifest)
                manifest = re.sub("--log-level=info", "--log-level={}".format(log_level), manifest)
            manifest.replace("--log-level=info", "--log-level=debug")

            with open(manifest_file, "w") as f:
                f.write(manifest)
                f.flush()

            run("kubectl apply -f {}".format(manifest_file), echo=True)

    with open("e2etest/manifests/mirror-server.yaml") as f:
        manifest = f.read()
    manifest = manifest.replace(":main", ":dev-{}".format(architecture))
    with tempfile.NamedTemporaryFile() as tmp:
        tmp.write(manifest.encode("utf-8"))
        tmp.flush()
        run("kubectl apply -f {}".format(tmp.name), echo=True)

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
    dev_env_dir = os.getcwd() + "/dev-env/layer2"
    with open("%s/config.yaml.tmpl" % dev_env_dir, 'r') as f:
        layer2_config = "# THIS FILE IS AUTOGENERATED\n" + f.read()
    layer2_config = layer2_config.replace(
        "SERVICE_V4_RANGE", get_available_ips(4)[0])
    layer2_config = layer2_config.replace(
        "SERVICE_V6_RANGE", get_available_ips(6)[0])
    with open("%s/config.yaml" % dev_env_dir, 'w') as f:
        f.write(layer2_config)
    # Apply the MetalLB ConfigMap
    run("kubectl apply -f %s/config.yaml" % dev_env_dir)

# Configure MetalLB in the dev-env for BGP testing. Start an frr based BGP
# router in a container and configure MetalLB to peer with it.
# See dev-env/bgp/README.md for some more information.
def bgp_dev_env(ip_family, frr_volume_dir):
    dev_env_dir = os.getcwd() + "/dev-env/bgp"
    if frr_volume_dir == "":
        frr_volume_dir = dev_env_dir + "/frr-volume"

    # TODO -- The IP address handling will need updates to add support for IPv6

    # We need the IPs for each Node in the cluster to place them in the BGP
    # router configuration file (bgpd.conf). Each Node will peer with this
    # router.
    node_ips = run("kubectl get nodes -o jsonpath='{.items[*].status.addresses"
            "[?(@.type==\"InternalIP\")].address}{\"\\n\"}'", echo=True)
    node_ips = node_ips.stdout.strip().split()
    if len(node_ips) != 3:
        raise Exit(message='Expected 3 nodes, got %d' % len(node_ips))

    # Create a new directory that will be used as the config volume for frr.
    try:
        # sudo because past docker runs will have changed ownership of this dir
        run('sudo rm -rf "%s"' % frr_volume_dir)
        os.mkdir(frr_volume_dir)
    except FileExistsError:
        pass
    except Exception as e:
        raise Exit(message='Failed to create frr-volume directory: %s'
                   % str(e))

    # These config files are static, so we copy them straight in.
    copy_files = ('zebra.conf', 'daemons', 'vtysh.conf')
    for f in copy_files:
        shutil.copyfile("%s/frr/%s" % (dev_env_dir, f),
                        "%s/%s" % (frr_volume_dir, f))

    # bgpd.conf is created from a template so that we can include the current
    # Node IPs.
    with open("%s/frr/bgpd.conf.tmpl" % dev_env_dir, 'r') as f:
        bgpd_config = "! THIS FILE IS AUTOGENERATED\n" + f.read()
        bgpd_config = bgpd_config.replace("PROTOCOL", ip_family)
    for n in range(0, len(node_ips)):
        bgpd_config = bgpd_config.replace("NODE%d_IP" % n, node_ips[n])
    with open("%s/bgpd.conf" % frr_volume_dir, 'w') as f:
        f.write(bgpd_config)

    # Run a BGP router in a container for all of the speakers to peer with.
    run('for frr in $(docker ps -a -f name=frr --format {{.Names}}) ; do '
        '    docker rm -f $frr ; '
        'done', echo=True)
    run("docker run -d --privileged --network kind --rm --ulimit core=-1 --name frr --volume %s:/etc/frr "
            "frrouting/frr:v7.5.1" % frr_volume_dir, echo=True)

    if ip_family == "ipv4":
        peer_address = run('docker inspect -f "{{ '
            'range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}" frr', echo=True)
    elif ip_family == "ipv6":
        peer_address = run('docker inspect -f "{{ '
            'range .NetworkSettings.Networks}}{{.GlobalIPv6Address}}{{end}}" frr', echo=True)
    else:
        raise Exit(message='Unsupported ip address family %s' % ip_family)

    with open("%s/config.yaml.tmpl" % dev_env_dir, 'r') as f:
        mlb_config = "# THIS FILE IS AUTOGENERATED\n" + f.read()
    mlb_config = mlb_config.replace("PEER_IP_ADDRESS", peer_address.stdout.strip())
    with open("%s/config.yaml" % dev_env_dir, 'w') as f:
        f.write(mlb_config)
    # Apply the MetalLB ConfigMap
    run("kubectl apply -f %s/config.yaml" % dev_env_dir)


def get_available_ips(ip_family=None):
    if ip_family is None or (ip_family != 4 and ip_family != 6):
        raise Exit(message="Please provide network version: 4 or 6.")

    v4, v6 = _get_subnets_allocated_ips()

    for i in _get_network_subnets():
        network = ipaddress.ip_network(i)
        if network.version == ip_family:
            used_list = v4 if ip_family == 4 else v6
            last_used = ipaddress.ip_interface(used_list[-1])

            for_containers = []
            for i in range(2):
                last_used = last_used + 1
                for_containers.append(str(last_used))

            # try to get 10 IP addresses after the last assigned node address in the kind network subnet
            # if failed, just quit (recreate kind cluster might solve the situation)
            service_ip_range_start = last_used + 1
            service_ip_range_end = last_used + 11
            if service_ip_range_start not in network:
                raise Exit(message='network range %s is not in %s' % (service_ip_range_start, network))
            if service_ip_range_end not in network:
                raise Exit(message='network range %s is not in %s' % (service_ip_range_end, network))
            return '%s-%s' % (service_ip_range_start.ip, service_ip_range_end.ip), ','.join(for_containers)

@task(help={
    "name": "name of the kind cluster to delete.",
    "frr_volume_dir": "FRR router config directory to be cleaned up. "
                      "Default: ./dev-env/bgp/frr-volume"
})
def dev_env_cleanup(ctx, name="kind", frr_volume_dir=""):
    """Remove traces of the dev env."""
    validate_kind_version()
    clusters = run("kind get clusters", hide=True).stdout.strip().splitlines()
    if name in clusters:
        run("kind delete cluster --name={}".format(name), hide=True)
    else:
        raise Exit(message="Unable to find cluster named: {}".format(name))

    run('for frr in $(docker ps -a -f name=frr --format {{.Names}}) ; do '
        '    docker rm -f $frr ; '
        'done', hide=True)

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


@task(help={
    "version": "version of MetalLB to release.",
    "skip-release-notes": "make the release even if there are no release notes.",
})
def release(ctx, version, skip_release_notes=False):
    """Tag a new release."""
    status = run("git status --porcelain", hide=True).stdout.strip()
    if status != "":
        raise Exit(message="git checkout not clean, cannot release")

    version = semver.parse_version_info(version)
    is_patch_release = version.patch != 0

    # Check that we have release notes for the desired version.
    run("git checkout main", echo=True)
    if not skip_release_notes:
        with open("website/content/release-notes/_index.md") as release_notes:
            if "## Version {}".format(version) not in release_notes.read():
                raise Exit(message="no release notes for v{}".format(version))

    # Move HEAD to the correct release branch - either a new one, or
    # an existing one.
    if is_patch_release:
        run("git checkout v{}.{}".format(version.major, version.minor), echo=True)
    else:
        run("git checkout -b v{}.{}".format(version.major, version.minor), echo=True)

    # Copy over release notes from main.
    if not skip_release_notes:
        run("git checkout main -- website/content/release-notes/_index.md", echo=True)

    # Update links on the website to point to files at the version
    # we're creating.
    if is_patch_release:
        previous_version = "v{}.{}.{}".format(version.major, version.minor, version.patch-1)
    else:
        previous_version = "main"
    bumprelease(ctx, version, previous_version)
    
    run("git commit -a -m 'Automated update for release v{}'".format(version), echo=True)
    run("git tag v{} -m 'See the release notes for details:\n\nhttps://metallb.universe.tf/release-notes/#version-{}-{}-{}'".format(version, version.major, version.minor, version.patch), echo=True)
    run("git checkout main", echo=True)

@task(help={
    "version": "version of MetalLB to release.",
    "previous_version": "version of the previous release.",
})
def bumprelease(ctx, version, previous_version):
    version = semver.parse_version_info(version)

    def _replace(pattern):
        oldpat = pattern.format(previous_version)
        newpat = pattern.format("v{}").format(version)
        run("perl -pi -e 's#{}#{}#g' website/content/*.md website/content/*/*.md".format(oldpat, newpat),
            echo=True)
    _replace("/metallb/metallb/{}")
    _replace("/metallb/metallb/tree/{}")
    _replace("/metallb/metallb/blob/{}")

    # Update the version listed on the website sidebar
    run("perl -pi -e 's/MetalLB .*/MetalLB v{}/g' website/content/_header.md".format(version), echo=True)

    # Update the manifests with the new version
    run("perl -pi -e 's,image: quay.io/metallb/speaker:.*,image: quay.io/metallb/speaker:v{},g' config/controllers/speaker.yaml".format(version), echo=True)
    run("perl -pi -e 's,image: quay.io/metallb/controller:.*,image: quay.io/metallb/controller:v{},g' config/controllers/controller.yaml".format(version), echo=True)

    # Update the versions in the helm chart (version and appVersion are always the same)
    # helm chart versions follow Semantic Versioning, and thus exclude the leading 'v'
    run("perl -pi -e 's,^version: .*,version: {},g' charts/metallb/Chart.yaml".format(version), echo=True)
    run("perl -pi -e 's,^appVersion: .*,appVersion: v{},g' charts/metallb/Chart.yaml".format(version), echo=True)
    run("perl -pi -e 's,^Current chart version is: .*,Current chart version is: `{}`,g' charts/metallb/README.md".format(version), echo=True)

    # Generate the manifests with the new version of the images
    generatemanifests(ctx)

    # Update the version in kustomize instructions
    #
    # TODO: Check if kustomize instructions really need the version in the
    # website or if there is a simpler way. For now, though, we just replace the
    # only page that mentions the version on release.
    run("sed -i 's/github.com\/metallb\/metallb\/config\/native?ref=main/github.com\/metallb\/metallb\/config\/native?ref={}/g' website/content/installation/_index.md".format(version))
    run("sed -i 's/github.com\/metallb\/metallb\/config\/native?ref=main/github.com\/metallb\/metallb\/config\/frr?ref={}/g' website/content/installation/_index.md".format(version))

    # Update the version embedded in the binary
    run("perl -pi -e 's/version\s+=.*/version = \"{}\"/g' internal/version/version.go".format(version), echo=True)
    run("gofmt -w internal/version/version.go", echo=True)

@task
def test(ctx):
    """Run unit tests."""
    run("go test -short ./...")
    run("go test -short -race ./...")


@task
def checkpatch(ctx):
    # Generate a diff of all changes on this branch from origin/main
    # and look for any added lines with 2 spaces after a period.
    try:
        lines = run("git diff $(diff -u <(git rev-list --first-parent HEAD) "
                                " <(git rev-list --first-parent origin/main) "
                                " | sed -ne 's/^ //p' | head -1)..HEAD | "
                                " grep '+.*\.\  '")

        if len(lines.stdout.strip()) > 0:
            raise Exit(message="ERROR: Found changed lines with 2 spaces "
                               "after a period.")
    except UnexpectedExit:
        # Will exit non-zero if no double-space-after-period lines are found.
        pass


@task(help={
    "env": "Specify in which environment to run the linter . Default 'container'. Supported: 'container','host'"
})
def lint(ctx, env="container"):
    """Run linter.

    By default, this will run a golangci-lint docker image against the code.
    However, in some environments (such as the MetalLB CI), it may be more
    convenient to install the golangci-lint binaries on the host. This can be
    achieved by running `inv lint --env host`.
    """
    version = "1.45.2"
    golangci_cmd = "golangci-lint run --timeout 10m0s ./..."

    if env == "container":
        run("docker run --rm -v $(git rev-parse --show-toplevel):/app -w /app golangci/golangci-lint:v{} {}".format(version, golangci_cmd), echo=True)
    elif env == "host":
        run("curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin v{}".format(version))
        run(golangci_cmd)
    else:
        raise Exit(message="Unsupported linter environment: {}". format(env))


@task(help={
    "name": "name of the kind cluster to test (only kind uses).",
    "export": "where to export kind logs.",
    "kubeconfig": "kubeconfig location. By default, use the kubeconfig from kind.",
    "system_namespaces": "comma separated list of Kubernetes system namespaces",
    "service_pod_port": "port number that service pods open.",
    "skip_docker": "don't use docker command in BGP testing.",
    "focus": "the list of arguments to pass into as -ginkgo.focus",
    "skip": "the list of arguments to pass into as -ginkgo.skip",
    "ipv4_service_range": "a range of IPv4 addresses for MetalLB to use when running in layer2 mode.",
    "ipv6_service_range": "a range of IPv6 addresses for MetalLB to use when running in layer2 mode.",
})
def e2etest(ctx, name="kind", export=None, kubeconfig=None, system_namespaces="kube-system,metallb-system", service_pod_port=80, skip_docker=False, focus="", skip="", ipv4_service_range=None, ipv6_service_range=None):
    """Run E2E tests against development cluster."""
    if skip_docker:
        opt_skip_docker = "--skip-docker"
    else:
        opt_skip_docker = ""

    ginkgo_skip = ""
    if skip:
        ginkgo_skip = "--ginkgo.skip=\"" + skip + "\""

    ginkgo_focus = ""
    if focus:
        ginkgo_focus = "--ginkgo.focus=\"" + focus + "\""
    
    if kubeconfig is None:
        validate_kind_version()
        clusters = run("kind get clusters", hide=True).stdout.strip().splitlines()
        if name in clusters:
            kubeconfig_file = tempfile.NamedTemporaryFile()
            kubeconfig = kubeconfig_file.name
            run("kind export kubeconfig --name={} --kubeconfig={}".format(name, kubeconfig), pty=True, echo=True)
        else:
            raise Exit(message="Unable to find cluster named: {}".format(name))
    else:
        os.environ['KUBECONFIG'] = kubeconfig

    namespaces = system_namespaces.replace(' ', '').split(',')
    for ns in namespaces:
        run("kubectl -n {} wait --for=condition=Ready --all pods --timeout 300s".format(ns), hide=True)

    ips_for_containers_v4 = ""
    if ipv4_service_range is None:
        ipv4_service_range, ips = get_available_ips(4)
        ips_for_containers_v4 = "--ips-for-containers-v4=" + ips

    ips_for_containers_v6 = ""
    if ipv6_service_range is None:
        ipv6_service_range, ips = get_available_ips(6)
        ips_for_containers_v6 = "--ips-for-containers-v6=" + ips

    if export != None:
        report_path = export
    else:
        report_path = "/tmp/metallbreport{}".format(time.time())

    print("Writing reports to {}".format(report_path))
    os.makedirs(report_path, exist_ok=True)

    testrun = run("cd `git rev-parse --show-toplevel`/e2etest &&"
            "KUBECONFIG={} go test -timeout 2h {} {} --provider=local --kubeconfig={} --service-pod-port={} {} {} -ipv4-service-range={} -ipv6-service-range={} {} --report-path {}".format(kubeconfig, ginkgo_focus, ginkgo_skip, kubeconfig, service_pod_port, ips_for_containers_v4, ips_for_containers_v6, ipv4_service_range, ipv6_service_range, opt_skip_docker, report_path), warn="True")

    if export != None:
        run("kind export logs {}".format(export))

    if testrun.failed:
        raise Exit(message="E2E tests failed", code=testrun.return_code)

@task
def bumplicense(ctx):
    """Bumps the license header on all go files that have it missing"""

    res = run("find . -name '*.go'")
    for file in res.stdout.splitlines():
        res = run("grep -q License {}".format(file), warn=True)
        if not res.ok:
            run(r"sed -i '1s/^/\/\/ SPDX-License-Identifier:Apache-2.0\n\n/' " + file)
 
@task
def verifylicense(ctx):
    """Verifies all files have the corresponding license"""
    res = run("find . -name '*.go'", hide="out")
    no_license = False
    for file in res.stdout.splitlines():
        res = run("grep -q License {}".format(file), warn=True)
        if not res.ok:
            no_license = True
            print("{} is missing license".format(file))
    if no_license:
        raise Exit(message="#### Files with no license found.\n#### Please run ""inv bumplicense"" to add the license header")

@task
def gomodtidy(ctx):
    """Runs go mod tidy"""
    res = run("go mod tidy", hide="out")
    if not res.ok:
        raise Exit(message="go mod tidy failed")

@task(help={
    "controller_gen": "KubeBuilder CLI."
                    "Default: controller_gen (version v0.7.0).",
    "kustomize_cli": "YAML files customization CLI."
                    "Default: kustomize (version v4.4.0).",
})  
def generatemanifests(ctx, controller_gen="controller-gen", kustomize_cli="kustomize"):
    """ Re-generates the all-in-one manifests under config/manifests"""
    generate_manifest(ctx, controller_gen=controller_gen, kustomize_cli=kustomize_cli, bgp_type="frr", output="config/manifests/metallb-frr.yaml")
    generate_manifest(ctx, controller_gen=controller_gen, kustomize_cli=kustomize_cli, bgp_type="native", output="config/manifests/metallb-native.yaml")
    generate_manifest(ctx, controller_gen=controller_gen, kustomize_cli=kustomize_cli, bgp_type="frr", enable_webhooks=True, output="config/manifests/metallb-frr-with-webhooks.yaml")
    generate_manifest(ctx, controller_gen=controller_gen, kustomize_cli=kustomize_cli, bgp_type="native", enable_webhooks=True, output="config/manifests/metallb-native-with-webhooks.yaml")

@task
def generateapidocs(ctx):
    """Generates the docs for the CRDs"""
    run("go install github.com/ahmetb/gen-crd-api-reference-docs@3f29e6853552dcf08a8e846b1225f275ed0f3e3b")
    run('gen-crd-api-reference-docs -config website/generatecrddoc/crdgen.json -template-dir website/generatecrddoc/template -api-dir "go.universe.tf/metallb/api" -out-file /tmp/generated_apidoc.html')
    run("cat website/generatecrddoc/prefix.html /tmp/generated_apidoc.html > website/content/apis/_index.md")

@task(help={
    "action": "The action to take to fix the uncommitted changes",
    })
def checkchanges(ctx, action="check uncommitted files"):
    """Verifies no uncommitted files are available"""
    res = run("git status --porcelain", hide="out")
    if res.stdout != "":
        print("{} must be committed".format(res))
        raise Exit(message="#### Uncommitted files found, you may need to {} ####\n".format(action))


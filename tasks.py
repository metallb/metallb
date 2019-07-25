import glob
import os.path
import re
import semver
import sys
import yaml
import tempfile
try:
    from io import StringIO
except ImportError:
    from StringIO import StringIO
try:
    from urllib.request import urlopen
except ImportError:
    from urllib2 import urlopen

from invoke import run, task
from invoke.exceptions import Exit

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
            print("Known binaries: {}".format(", ",join(sorted(all_binaries))))
            sys.exit(1)
        else:
            out.add(binary)
    if not out:
        out.add("controller")
        out.add("speaker")
    return list(sorted(out))

def _make_build_dirs():
    for arch in all_architectures:
        for binary in all_binaries:
            dir = os.path.join("build", arch, binary)
            if not os.path.exists(dir):
                os.makedirs(dir, mode=0o750)

@task(iterable=["binaries", "architectures"],
      help={
          "binaries": "binaries to build. One or more of {}, or 'all'".format(", ".join(sorted(all_binaries))),
          "architectures": "architectures to build. One or more of {}, or 'all'".format(", ".join(sorted(all_architectures))),
          "tag": "docker image tag prefix to use. Actual tag will be <tag>-<arch>. Default 'dev'.",
          "docker-user": "docker user under which to tag the images. Default 'metallb'.",
      })
def build(ctx, binaries, architectures, tag="dev", docker_user="metallb"):
    """Build MetalLB docker images."""
    binaries = _check_binaries(binaries)
    architectures = _check_architectures(architectures)
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
            run("docker build "
                "-t {user}/{bin}:{tag}-{arch} "
                "-f {bin}/Dockerfile build/{arch}/{bin}".format(
                    user=docker_user,
                    bin=bin,
                    tag=tag,
                    arch=arch),
                echo=True)

@task(iterable=["binaries", "architectures"],
      help={
          "binaries": "binaries to build. One or more of {}, or 'all'".format(", ".join(sorted(all_binaries))),
          "architectures": "architectures to build. One or more of {}, or 'all'".format(", ".join(sorted(all_architectures))),
          "tag": "docker image tag prefix to use. Actual tag will be <tag>-<arch>. Default 'dev'.",
          "docker-user": "docker user under which to tag the images. Default 'metallb'.",
      })
def push(ctx, binaries, architectures, tag="dev", docker_user="metallb"):
    """Build and push docker images to registry."""
    binaries = _check_binaries(binaries)
    architectures = _check_architectures(architectures)

    for arch in architectures:
        for bin in binaries:
            build(ctx, binaries=[bin], architectures=[arch], tag=tag, docker_user=docker_user)
            run("docker push {user}/{bin}:{tag}-{arch}".format(
                user=docker_user,
                bin=bin,
                arch=arch,
                tag=tag),
                echo=True)

@task(iterable=["binaries"],
      help={
          "binaries": "binaries to build. One or more of {}, or 'all'".format(", ".join(sorted(all_binaries))),
          "tag": "docker image tag prefix to use. Actual tag will be <tag>-<arch>. Default 'dev'.",
          "docker-user": "docker user under which to tag the images. Default 'metallb'.",
      })
def push_multiarch(ctx, binaries, tag="dev", docker_user="metallb"):
    """Build and push multi-architecture docker images to registry."""
    binaries = _check_binaries(binaries)
    architectures = _check_architectures(["all"])
    push(ctx, binaries=binaries, architectures=architectures, tag=tag, docker_user=docker_user)
    
    platforms = ",".join("linux/{}".format(arch) for arch in architectures)
    for bin in binaries:
        run("manifest-tool push from-args "
            "--platforms {platforms} "
            "--template {user}/{bin}:{tag}-ARCH "
            "--target {user}/{bin}:{tag}".format(
                platforms=platforms,
                user=docker_user,
                bin=bin,
                tag=tag),
            echo=True)

@task(help={
    "architecture": "CPU architecture of the local machine. Default 'amd64'.",
    "name": "name of the kind cluster to use.",
})
def dev_env(ctx, architecture="amd64", name="kind", cni=None):
    """Build and run MetalLB in a local Kind cluster.

    If the cluster specified by --name (default "kind") doesn't exist,
    it is created. Then, build MetalLB docker images from the
    checkout, push them into kind, and deploy manifests/metallb.yaml
    to run those images.
    """
    clusters = run("kind get clusters", hide=True).stdout.strip().splitlines()
    mk_cluster = name not in clusters
    if mk_cluster:
        config = {
            "apiVersion": "kind.sigs.k8s.io/v1alpha3",
            "kind": "Cluster",
            "nodes": [{"role": "control-plane"},
                      {"role": "worker"},
                      {"role": "worker"},
            ],
        }
        if cni:
            config["networking"] = {
                "disableDefaultCNI": True,
            }
        config = yaml.dump(config).encode("utf-8")
        with tempfile.NamedTemporaryFile() as tmp:
            tmp.write(config)
            tmp.flush()
            run("kind create cluster --name={} --config={}".format(name, tmp.name), pty=True, echo=True)

    config = run("kind get kubeconfig-path --name={}".format(name), hide=True).stdout.strip()
    env = {"KUBECONFIG": config}
    if mk_cluster and cni:
        run("kubectl apply -f e2etest/manifests/{}.yaml".format(cni), echo=True, env=env)

    build(ctx, binaries=["controller", "speaker", "mirror-server"], architectures=[architecture])
    run("kind load docker-image --name={} metallb/controller:dev-{}".format(name, architecture), echo=True)
    run("kind load docker-image --name={} metallb/speaker:dev-{}".format(name, architecture), echo=True)
    run("kind load docker-image --name={} metallb/mirror-server:dev-{}".format(name, architecture), echo=True)

    with open("manifests/metallb.yaml") as f:
        manifest = f.read()
    manifest = manifest.replace(":master", ":dev-{}".format(architecture))
    manifest = manifest.replace("imagePullPolicy: Always", "imagePullPolicy: Never")
    with tempfile.NamedTemporaryFile() as tmp:
        tmp.write(manifest.encode("utf-8"))
        tmp.flush()
        run("kubectl apply -f {}".format(tmp.name), echo=True, env=env)

    with open("e2etest/manifests/mirror-server.yaml") as f:
        manifest = f.read()
    manifest = manifest.replace(":master", ":dev-{}".format(architecture))
    with tempfile.NamedTemporaryFile() as tmp:
        tmp.write(manifest.encode("utf-8"))
        tmp.flush()
        run("kubectl apply -f {}".format(tmp.name), echo=True, env=env)

    print("""

To access the cluster:

export KUBECONFIG={}
""".format(config))

# @task
# def client_machine(ctx, architecture="amd64", name="kind"):
#     """Create a virtual client 'machine' in a container."""
#     run("docker rm -f {}-client".format(name), warn=True, echo=True)
#     run("docker run -d --name={}-client --rm metallb/traffic-generator:dev-{}".format())

# @task
# def e2e_create_world(ctx, name="kind", cni=None, architecture="amd64"):
#     e2e_delete_world(ctx, name=name)
#     kind(ctx, name=name, cni=cni, architecture=architecture)
#     run("docker run -d --name={}-client --rm debian:stable /bin/bash -c 'sleep 1200'".format(name), echo=True)

# @task
# def e2e_delete_world(ctx, name="kind"):
#     run("kind delete cluster --name={}".format(name), warn=True, echo=True)
#     run("docker rm -f {}".format(client), warn=True, echo=True)

@task
def helm(ctx):
    """Generate manifests/metallb.yaml from the Helm chart."""
    helm_options = {
        "controller.resources.limits.cpu": "100m",
        "controller.resources.limits.memory": "100Mi",
        "speaker.resources.limits.cpu": "100m",
        "speaker.resources.limits.memory": "100Mi",
        "prometheus.scrapeAnnotations": "true",
        "existingConfigMap": "config",
    }
    helm_options = ",".join(k+"="+v for k,v in helm_options.items())
    manifests = run("helm template --namespace metallb-system --set {} helm-chart".format(helm_options), hide=True).stdout.strip()
    manifests = list(m for m in yaml.safe_load_all(manifests) if m)

    # Add in a namespace definition, which the helm chart doesn't
    # have.
    manifests.insert(0, {
        "apiVersion": "v1",
        "kind": "Namespace",
        "metadata": {
            "name": "metallb-system",
            "labels": {
                "app": "metallb",
            },
        },
    })

    def clean_name(name):
        name = name.replace("release-name-metallb-", "")
        return name.replace("release-name-metallb:", "metallb-system:")

    def remove_helm_labels(d):
        labels = d.get("metadata", {}).get("labels", {})
        labels.pop("heritage", None)
        labels.pop("chart", None)
        labels.pop("release", None)

    def add_namespace(d):
        d.get("metadata", {})["namespace"] = "metallb-system"

    def clean_role_resourcenames(role):
        for rule in role["rules"]:
            names = rule.get("resourceNames", [])
            for i in range(len(names)):
                names[i] = clean_name(names[i])

    def clean_binding(binding):
        binding["roleRef"]["name"] = clean_name(binding["roleRef"]["name"])
        for subject in binding.get("subjects", []):
            subject["name"] = clean_name(subject["name"])

    def clean_deployment_or_daemonset(obj):
        obj["spec"]["selector"]["matchLabels"].pop("release", None)
        remove_helm_labels(obj["spec"]["template"])
        obj["spec"]["template"]["spec"]["serviceAccountName"] = clean_name(obj["spec"]["template"]["spec"]["serviceAccountName"])

    for m in manifests:
        kind = m["kind"]
        m["metadata"]["name"] = clean_name(m["metadata"]["name"])
        remove_helm_labels(m)

        if kind not in ("ClusterRole", "ClusterRoleBinding", "Namespace"):
            add_namespace(m)
        if kind in ("Role", "ClusterRole"):
            clean_role_resourcenames(m)
        if kind in ("RoleBinding", "ClusterRoleBinding"):
            clean_binding(m)
        if kind in ("Deployment", "DaemonSet"):
            clean_deployment_or_daemonset(m)

    with open("manifests/metallb.yaml", "w") as f:
        yaml.dump_all([m for m in manifests if m], f)

@task
def test_cni_manifests(ctx):
    """Update CNI manifests for e2e tests."""
    def _fetch(url):
        bs = urlopen(url).read()
        return list(m for m in yaml.safe_load_all(bs) if m)
    def _write(file, manifest):
        with open(file, "w") as f:
            f.write(yaml.dump_all(manifest))

    calico = _fetch("https://docs.projectcalico.org/v3.6/getting-started/kubernetes/installation/hosted/kubernetes-datastore/calico-networking/1.7/calico.yaml")
    for manifest in calico:
        if manifest["kind"] != "DaemonSet":
            continue
        manifest["spec"]["template"]["spec"]["containers"][0]["env"].append({
            "name": "FELIX_IGNORELOOSERPF",
            "value": "true",
        })
    _write("e2etest/manifests/calico.yaml", calico)

    weave = _fetch("https://cloud.weave.works/k8s/net?k8s-version=1.15&env.NO_MASQ_LOCAL=1")
    _write("e2etest/manifests/weave.yaml", weave)

    flannel = _fetch("https://raw.githubusercontent.com/coreos/flannel/master/Documentation/kube-flannel.yml")
    _write("e2etest/manifests/flannel.yaml", flannel)

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
    run("git checkout master", echo=True)
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

    # Copy over release notes from master.
    if not skip_release_notes:
        run("git checkout master -- website/content/release-notes/_index.md", echo=True)

    # Update links on the website to point to files at the version
    # we're creating.
    if is_patch_release:
        previous_version = "v{}.{}.{}".format(version.major, version.minor, version.patch-1)
    else:
        previous_version = "master"
    def _replace(pattern):
        oldpat = pattern.format(previous_version)
        newpat = pattern.format("v{}").format(version)
        run("perl -pi -e 's#{}#{}#g' website/content/*.md website/content/*/*.md".format(oldpat, newpat),
            echo=True)
    _replace("/google/metallb/{}")
    _replace("/google/metallb/tree/{}")
    _replace("/google/metallb/blob/{}")

    # Pin the manifests and Helm charts to the version we're creating.
    run("perl -pi -e 's/appVersion: .*/appVersion: {}/g' helm-chart/Chart.yaml".format(version), echo=True)
    run("perl -pi -e 's/tag: .*/tag: v{}/g' helm-chart/values.yaml".format(version), echo=True)
    run("perl -pi -e 's/pullPolicy: .*/pullPolicy: IfNotPresent/g' helm-chart/values.yaml", echo=True)
    helm(ctx)

    # Update the version listed on the website sidebar
    run("perl -pi -e 's/MetalLB .*/MetalLB v{}/g' website/content/_header.md".format(version), echo=True)

    # Update the version embedded in the binary
    run("perl -pi -e 's/version\s+=.*/version = \"{}\"/g' internal/version/version.go".format(version), echo=True)
    run("gofmt -w internal/version/version.go", echo=True)

    run("git commit -a -m 'Automated update for release v{}'".format(version), echo=True)
    run("git tag v{} -m 'See the release notes for details:\n\nhttps://metallb.universe.tf/release-notes/#version-{}-{}-{}'".format(version, version.major, version.minor, version.patch), echo=True)
    run("git checkout master", echo=True)


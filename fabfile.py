import contextlib
import subprocess
import tempfile
import time
import shutil
import sys

from fabric.api import *

def _silent_nofail(*args, **kwargs):
    with settings(warn_only=True):
        return _silent(*args, **kwargs)

def _silent(*args, **kwargs):
    with hide('warnings', 'running'):
        return local(*args, capture=True, **kwargs)

@contextlib.contextmanager
def _tempdir():
    name = tempfile.mkdtemp()
    try:
        yield name
    finally:
        shutil.rmtree(name)

## Dockerfile generation

def gen_docker():
    """Generate ./dockerfiles/{dev,prod}"""
    local("mkdir -p ./dockerfiles/prod ./dockerfiles/dev")
    for env in ('dev', 'prod'):
        for binary in ('controller', 'bgp-speaker'):
            local('sed -e "s/%%BINARY%%/{0}/g" ./dockerfiles/{1}.tmpl >./dockerfiles/{1}/{0}'.format(binary, env))

## Debugging

def wireshark():
    node = _silent('kubectl get nodes -o go-template="{{ (index (index .items 0).status.addresses 0).address }}"')
    nodePort = _silent('kubectl get svc -n metallb-system test-bgp-router-ui -o go-template="{{ (index .spec.ports 0).nodePort }}"')
    with _tempdir() as tmp:
        local('curl -o {0}/pcap http://{1}:{2}/pcap'.format(tmp, node, nodePort))
        local('wireshark-gtk {0}/pcap'.format(tmp))

## Releases

def _error(msg):
    print(msg)
    sys.exit(1)

def _versions_md(vi):
    import semver
    versions = []
    tags = _silent("git tag").split("\n")
    for tag in tags:
        if not tag.startswith('v'):
            continue
        versions.append(semver.parse_version_info(tag[1:]))

    seen = {}
    version_lines = []
    for v in sorted(versions, reverse=True):
        maj_min = (v.major, v.minor)
        if maj_min in seen:
            continue
        if vi is not None and maj_min == (vi.major, vi.minor):
            continue
        site = "https://v{0}.{1}--metallb.netlify.com/".format(v.major, v.minor)
        if maj_min == (0, 1):
            site = "https://github.com/google/metallb/tree/v0.1"
        version_lines.append("- [{0}.{1}.x]({2})".format(v.major, v.minor, site))
    version_lines.append("- [latest development build](https://master--metallb.netlify.com/)")

    out = """---
title: Versions
weight: 70
---
"""
    if vi is None:
        out += """This site is for the **development** version of MetalLB.

Here are all versions of the website:

"""
    else:
        out += """This site is for the {0}.{1}.x releases of MetalLB.

Here are the websites for other versions:

""".format(vi.major, vi.minor)
    out +='\n'.join(version_lines)
    return out

def release(version):
    # Import here so that people who aren't making releases don't
    # need to pip install.
    import semver

    _versions_md(None)
    if _silent("git status --porcelain"):
        _error("git working directory not clean, cannot prepare release")
    vi = semver.parse_version_info(version)
    branch_name = 'v{0}.{1}'.format(vi.major, vi.minor)
    if vi.patch != 0 and _silent_nofail("git rev-parse --verify {0}".format(branch_name)).failed:
        _error("Cannot release {0}, branch {1} does not exist".format(version, branch_name))
    if vi.patch == 0:
        local("git checkout master")
        local("git checkout -b {0}".format(branch_name))

        with lcd("website/content"):
            local("perl -pi -e 's#/google/metallb/master#/google/metallb/{0}#g' *".format(branch_name))
        with lcd("manifests"):
            local("perl -pi -e 's/:latest/:v{0}/g' *".format(version))
        with open("website/content/versions.md", "wb") as f:
            f.write(_versions_md(vi))
    else:
        local("git checkout {0}".format(branch_name))
        with lcd("manifests"):
            local("perl -pi -e 's/:v{0}.{1}.{2}/:v{0}.{1}.{3}/g' *".format(vi.major, vi.minor, vi.patch-1, vi.patch))
    with lcd("website"):
        local("perl -pi -e 's/version = .*/version = \"v{0}\"/g' config.toml".format(version))
    local('git commit -a -m "Update documentation for release {0}"'.format(version))
    local('git tag v{0} -m "Release version {0}"'.format(version))
    local('git checkout master')
    if vi.patch == 0:
        with open("website/content/versions.md", "wb") as f:
            f.write(_versions_md(None))
        local('git commit -a -m "Update website versions for release {0}"'.format(version))

## Minikube bringup/teardown

def _minikube_running():
    return _silent_nofail("minikube ip").succeeded

def start():
    """Start minikube and configure it for MetalLB"""
    if not _minikube_running():
        local("minikube start")
    local("minikube addons enable registry")
    print("waiting for registry to start")
    while _silent_nofail("kubectl get svc -n kube-system registry").failed:
        time.sleep(1)
    regSvcType = _silent("kubectl get svc -n kube-system registry -o go-template=\"{{.spec.type}}\"")

    if _silent_nofail("kubectl get ns metallb-system").failed:
        push_manifests()

def stop():
    """Delete running minikube cluster"""
    if _minikube_running():
        local("minikube delete")

## Platform-agnostic stuff - just uses kubectl and assumes the kube-system registry is running

def _registry_clusterip():
    return "%s:80" % _silent("kubectl get svc -n kube-system registry -o go-template='{{.spec.clusterIP}}'")

def _kube_obj_exists(n):
    return _silent_nofail("kubectl get %s").succeeded

@contextlib.contextmanager
def _proxy_to_registry():
    registry_pod = _silent("kubectl get pod -n kube-system -l kubernetes.io/minikube-addons=registry -o go-template=\"{{(index .items 0).metadata.name}}\"")
    p = subprocess.Popen("kubectl port-forward -n kube-system %s 5000:5000" % registry_pod, shell=True)
    try:
        print("Waiting for kube port-forward to come up...")
        while _silent_nofail("curl http://localhost:5000/").failed:
            time.sleep(0.1)
        yield
    finally:
        p.kill()

def push_config():
    """Push a basic MetalLB config that connects to test-bgp-router."""
    # As it happens, the tutorial config is exactly what we need here.
    local("kubectl apply -f manifests/tutorial-1.yaml")

def push_manifests():
    """Push the metallb binary manifests"""
    local("kubectl apply -f manifests/metallb.yaml,manifests/test-bgp-router.yaml")
    if _silent_nofail("kubectl get configmap -n metallb-system config").failed:
        push_config()

def _build(ts, name, registry):
    with _tempdir() as tmp:
        local("env GOOS=linux GOARCH=amd64 go install ./%s" % name)
        local("env GOOS=linux GOARCH=amd64 go build -o %s/%s ./%s" % (tmp, name, name))
        local("cp ./dockerfiles/dev/%s %s/Dockerfile" % (name, tmp))
        local("sudo docker build -t %s/%s:latest %s" % (registry, name, tmp))
        local("sudo docker tag %s/%s:latest %s/%s:%s" % (registry, name, registry, name, ts))
        with _proxy_to_registry():
            local("sudo docker push %s/%s:%s" % (registry, name, ts))
        local("sudo docker rmi %s/%s:%s" % (registry, name, ts))

def _set_image(ts, name, job):
    local("kubectl set image -n metallb-system {2} {1}={3}/{1}:{0}".format(ts, name, job, _registry_clusterip()))

def _wait_for_rollout(typ, name):
    local("kubectl rollout status -n metallb-system {0} {1}".format(typ, name))

def push(registry="localhost:5000"):
    """Build and repush metallb binaries"""
    if _silent_nofail("kubectl get ns metallb-system").failed:
        push_manifests()

    ts = "%f" % time.time()
    _build(ts, "controller", registry)
    _build(ts, "bgp-speaker", registry)
    _build(ts, "test-bgp-router", registry)

    _set_image(ts, "controller", "deploy/controller")
    _set_image(ts, "bgp-speaker", "ds/bgp-speaker")
    _set_image(ts, "test-bgp-router", "deploy/test-bgp-router")

    _wait_for_rollout("deployment", "controller")
    _wait_for_rollout("daemonset", "bgp-speaker")
    _wait_for_rollout("deployment", "test-bgp-router")

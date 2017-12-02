import contextlib
import subprocess
import tempfile
import time
import shutil

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

def _build(ts, name):
    with _tempdir() as tmp:
        local("go install ./%s" % name)
        local("go build -o %s/%s ./%s" % (tmp, name, name))
        local("cp ./dockerfiles/dev/%s %s/Dockerfile" % (name, tmp))
        local("sudo docker build -t localhost:5000/%s:latest %s" % (name, tmp))
        local("sudo docker tag localhost:5000/%s:latest localhost:5000/%s:%s" % (name, name, ts))
        with _proxy_to_registry():
            local("sudo docker push localhost:5000/%s:%s" % (name, ts))
        local("sudo docker rmi localhost:5000/%s:%s" % (name, ts))

def _set_image(ts, name, job):
    local("kubectl set image -n metallb-system {2} {1}={3}/{1}:{0}".format(ts, name, job, _registry_clusterip()))

def _wait_for_rollout(typ, name):
    local("kubectl rollout status -n metallb-system {0} {1}".format(typ, name))
    
def push():
    """Build and repush metallb binaries"""
    if _silent_nofail("kubectl get ns metallb-system").failed:
        push_manifests()

    ts = "%f" % time.time()
    _build(ts, "controller")
    _build(ts, "bgp-speaker")
    _build(ts, "test-bgp-router")

    _set_image(ts, "controller", "deploy/controller")
    _set_image(ts, "bgp-speaker", "ds/bgp-speaker")
    _set_image(ts, "test-bgp-router", "deploy/test-bgp-router")

    _wait_for_rollout("deployment", "controller")
    _wait_for_rollout("daemonset", "bgp-speaker")
    _wait_for_rollout("deployment", "test-bgp-router")

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
    local("rm -rf ./dockerfiles/prod ./dockerfiles/dev")
    local("mkdir -p ./dockerfiles/prod ./dockerfiles/dev")
    for env in ('dev', 'prod'):
        for binary in ('controller', 'bgp-speaker', 'bgp-spy'):
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
    yield
    p.kill()

def push_manifests():
    """Push the metallb binary manifests"""
    local("kubectl apply -f manifests/metallb.yaml")

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

def push():
    """Build and repush metallb binaries"""
    if _silent_nofail("kubectl get ns metallb-system").failed:
        push_manifests()

    ts = "%f" % time.time()
    _build(ts, "controller")
    _build(ts, "bgp-speaker")
    local("kubectl set image -n metallb-system deploy/controller controller=%s/controller:%s" % (_registry_clusterip(), ts))
    local("kubectl set image -n metallb-system ds/bgp-speaker bgp-speaker=%s/bgp-speaker:%s" % (_registry_clusterip(), ts))
    local("kubectl rollout status -n metallb-system deployment controller")
    local("kubectl rollout status -n metallb-system daemonset bgp-speaker")

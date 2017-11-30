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

def _minikube_running():
    return _silent_nofail("minikube ip").succeeded

def _registry_nodeport():
    return _silent("minikube service --url -n kube-system registry")[7:]

def _registry_clusterip():
    return "%s:80" % _silent("kubectl get svc -n kube-system registry -o go-template='{{.spec.clusterIP}}'")

def _kube_obj_exists(n):
    return _silent_nofail("kubectl get %s").succeeded

@contextlib.contextmanager
def _tempdir():
    name = tempfile.mkdtemp()
    try:
        yield name
    finally:
        shutil.rmtree(name)

@contextlib.contextmanager
def _proxy_to_registry():
    p = subprocess.Popen("socat TCP-LISTEN:5000,fork,reuseaddr,retry=5 TCP:%s" % _registry_nodeport(), shell=True)
    yield
    p.kill()

def start():
    if not _minikube_running():
        local("minikube start")

    local("minikube addons enable registry")
    print("waiting for registry to start")
    while _silent_nofail("kubectl get svc -n kube-system registry").failed:
        time.sleep(1)
    regSvcType = _silent("kubectl get svc -n kube-system registry -o go-template=\"{{.spec.type}}\"")
    if regSvcType != "NodePort":
        local('kubectl patch svc -n kube-system registry -p \'{"spec":{"type":"NodePort"}}\'')

    if not _kube_obj_exists("ns metallb-system"):
        push_manifests()

def delete():
    if _minikube_running():
        local("minikube delete")

def push_manifests():
    local("kubectl apply -f manifests/metallb.yaml")

def _build(ts, name):
    with _tempdir() as tmp:
        local("go build -o %s/%s ./%s" % (tmp, name, name))
        local("cp ./dockerfiles/dev/%s %s/Dockerfile" % (name, tmp))
        with _proxy_to_registry():
            local("sudo docker build -t localhost:5000/%s:latest %s" % (name, tmp))
            local("sudo docker tag localhost:5000/%s:latest localhost:5000/%s:%s" % (name, name, ts))
            local("sudo docker push localhost:5000/%s:%s" % (name, ts))
            local("sudo docker rmi localhost:5000/%s:%s" % (name, ts))

        local("kubectl set image -n metallb-system deploy/controller controller=%s/controller:%s" % (_registry_clusterip(), ts))
        local("kubectl set image -n metallb-system ds/bgp-speaker bgp-speaker=%s/bgp-speaker:%s" % (_registry_clusterip(), ts))

def push():
    ts = "%f" % time.time()
    _build(ts, "controller")
    _build(ts, "bgp-speaker")

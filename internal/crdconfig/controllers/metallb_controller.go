/*


Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"bytes"
	"context"
	"fmt"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"io/ioutil"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	uns "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/yaml"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"text/template"

	loadbalancerv1 "go.universe.tf/metallb/api/v1"
)

// MetalLBReconciler reconciles a MetalLB object
type MetalLBReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

const (
	configMapFile string = "./template/config.yaml"
)

// +kubebuilder:rbac:groups=loadbalancer.loadbalancer.operator.io,resources=metallbs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=loadbalancer.loadbalancer.operator.io,resources=metallbs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=batch,resources=jobs/status,verbs=get
func (r *MetalLBReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("metallb", req.NamespacedName)
	log.Info("Reconciling MetalLB")

	instance := &loadbalancerv1.MetalLB{}
	if err := r.Get(ctx, req.NamespacedName, instance); err != nil {
		log.Error(err, "unable to find MetalLB")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Create config map based on CR
	if err := r.applyConfigMap(ctx, instance); err != nil {
		errors.Wrap(err, "Failed to create config map")
		return ctrl.Result{}, err
	}

	log.Info("Reconcile complete")
	return ctrl.Result{}, nil
}

func (r *MetalLBReconciler) applyConfigMap(ctx context.Context, instance *loadbalancerv1.MetalLB) error {
	data := make(map[string]interface{})

	data["Peers"] = instance.Spec.Peers
	data["AddressPools"] = instance.Spec.AddressPools

	source, err := ioutil.ReadFile(configMapFile)
	if err != nil {
		return errors.Wrapf(err, "failed to read manifest %s", configMapFile)
	}

	tmpl := template.New(configMapFile).Option("missingkey=error")
	if _, err := tmpl.Parse(string(source)); err != nil {
		return errors.Wrapf(err, "failed to parse manifest %s as template", configMapFile)
	}

	rendered := bytes.Buffer{}
	if err := tmpl.Execute(&rendered, data); err != nil {
		return errors.Wrapf(err, "failed to render manifest %s", configMapFile)
	}

	decoder := yaml.NewYAMLOrJSONDecoder(&rendered, 4096)
	obj := uns.Unstructured{}
	if err := decoder.Decode(&obj); err != nil {
		return errors.Wrapf(err, "failed to unmarshal manifest %s", configMapFile)
	}

	name := obj.GetName()
	namespace := obj.GetNamespace()
	if name == "" {
		return errors.Errorf("Object %s has no name", obj.GroupVersionKind().String())
	}
	gvk := obj.GroupVersionKind()
	// used for logging and errors
	objDesc := fmt.Sprintf("(%s) %s/%s", gvk.String(), namespace, name)

	// Get existing
	existing := &uns.Unstructured{}
	existing.SetGroupVersionKind(gvk)
	err = r.Client.Get(ctx, types.NamespacedName{Name: obj.GetName(), Namespace: obj.GetNamespace()}, existing)

	if err != nil && apierrors.IsNotFound(err) {
		err = r.Client.Create(ctx, &obj)
		if err != nil {
			return errors.Wrapf(err, "could not create %s", objDesc)
		}
		return nil
	}
	if err != nil {
		return errors.Wrapf(err, "could not retrieve existing %s", objDesc)
	}

	if !equality.Semantic.DeepEqual(existing, &obj) {
		if err := r.Client.Update(ctx, &obj); err != nil {
			return errors.Wrapf(err, "could not update object %s", objDesc)
		}
	}
	return nil
}

func (r *MetalLBReconciler) SetupWithManager(mgr ctrl.Manager) error {

	return ctrl.NewControllerManagedBy(mgr).
		For(&loadbalancerv1.MetalLB{}).
		Complete(r)
}

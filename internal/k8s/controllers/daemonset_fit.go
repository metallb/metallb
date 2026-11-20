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
	"errors"
	"fmt"
	"reflect"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// nodeContinuesRunningDaemonPod reports whether the DaemonSet controller would
// keep an already scheduled pod on node. It follows NodeShouldRunDaemonPod:
// selector and affinity mismatches evict the pod, and so does an untolerated
// NoExecute taint. An untolerated NoSchedule taint blocks new pods but does
// not evict one that is already running.
func nodeContinuesRunningDaemonPod(node *corev1.Node, ds *appsv1.DaemonSet) bool {
	if node == nil || ds == nil {
		return false
	}
	if ds.Spec.Template.Spec.NodeName != "" && ds.Spec.Template.Spec.NodeName != node.Name {
		return false
	}

	spec := ds.Spec.Template.Spec.DeepCopy()
	addDaemonSetTolerations(spec)
	matches, _ := podSchedulingMatches(spec, node)
	if !matches {
		return false
	}

	schedulable := func(t *corev1.Taint) bool {
		return t.Effect == corev1.TaintEffectNoSchedule || t.Effect == corev1.TaintEffectNoExecute
	}
	if !hasUntoleratedTaint(node.Spec.Taints, spec.Tolerations, schedulable) {
		return true
	}
	noExecute := func(t *corev1.Taint) bool {
		return t.Effect == corev1.TaintEffectNoExecute
	}
	return !hasUntoleratedTaint(node.Spec.Taints, spec.Tolerations, noExecute)
}

// addDaemonSetTolerations applies the tolerations the DaemonSet controller
// adds to every pod before deciding where it should run.
func addDaemonSetTolerations(spec *corev1.PodSpec) {
	add := func(key string, effect corev1.TaintEffect) {
		toleration := corev1.Toleration{
			Key:      key,
			Operator: corev1.TolerationOpExists,
			Effect:   effect,
		}
		for i := range spec.Tolerations {
			if sameToleration(spec.Tolerations[i], toleration) {
				return
			}
		}
		spec.Tolerations = append(spec.Tolerations, toleration)
	}
	add(corev1.TaintNodeNotReady, corev1.TaintEffectNoExecute)
	add(corev1.TaintNodeUnreachable, corev1.TaintEffectNoExecute)
	add(corev1.TaintNodeDiskPressure, corev1.TaintEffectNoSchedule)
	add(corev1.TaintNodeMemoryPressure, corev1.TaintEffectNoSchedule)
	add(corev1.TaintNodePIDPressure, corev1.TaintEffectNoSchedule)
	add(corev1.TaintNodeUnschedulable, corev1.TaintEffectNoSchedule)
	if spec.HostNetwork {
		add(corev1.TaintNodeNetworkUnavailable, corev1.TaintEffectNoSchedule)
	}
}

func sameToleration(a, b corev1.Toleration) bool {
	if a.Key != b.Key || a.Operator != b.Operator || a.Value != b.Value || a.Effect != b.Effect {
		return false
	}
	if a.TolerationSeconds == nil || b.TolerationSeconds == nil {
		return a.TolerationSeconds == b.TolerationSeconds
	}
	return *a.TolerationSeconds == *b.TolerationSeconds
}

func hasUntoleratedTaint(taints []corev1.Taint, tolerations []corev1.Toleration, include func(*corev1.Taint) bool) bool {
	for i := range taints {
		taint := taints[i]
		if include != nil && !include(&taint) {
			continue
		}
		tolerated := false
		for j := range tolerations {
			if tolerations[j].ToleratesTaint(&taint) {
				tolerated = true
				break
			}
		}
		if !tolerated {
			return true
		}
	}
	return false
}

// podSchedulingMatches reports whether spec's nodeSelector and required node
// affinity accept node. Parse errors follow the DaemonSet controller, which
// ignores them and treats the node as a non-match when nothing else matched.
func podSchedulingMatches(spec *corev1.PodSpec, node *corev1.Node) (bool, error) {
	if len(spec.NodeSelector) > 0 && !labels.SelectorFromSet(spec.NodeSelector).Matches(labels.Set(node.Labels)) {
		return false, nil
	}
	affinity := spec.Affinity
	if affinity == nil || affinity.NodeAffinity == nil || affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution == nil {
		return true, nil
	}
	return nodeAffinityMatches(affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution, node)
}

func nodeAffinityMatches(ns *corev1.NodeSelector, node *corev1.Node) (bool, error) {
	var errs []error
	sawTerm := false
	for i := range ns.NodeSelectorTerms {
		term := &ns.NodeSelectorTerms[i]
		if len(term.MatchExpressions) == 0 && len(term.MatchFields) == 0 {
			continue
		}
		sawTerm = true
		ok, err := nodeSelectorTermMatches(term, node)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if ok {
			return true, nil
		}
	}
	if !sawTerm {
		return false, nil
	}
	if len(errs) > 0 {
		return false, fmt.Errorf("parsing node affinity: %w", errors.Join(errs...))
	}
	return false, nil
}

func nodeSelectorTermMatches(term *corev1.NodeSelectorTerm, node *corev1.Node) (bool, error) {
	if len(term.MatchExpressions) > 0 {
		selector, err := nodeSelectorRequirementsAsSelector(term.MatchExpressions)
		if err != nil {
			return false, err
		}
		if !selector.Matches(labels.Set(node.Labels)) {
			return false, nil
		}
	}
	if len(term.MatchFields) > 0 && node.Name != "" {
		selector, err := nodeFieldRequirementsAsSelector(term.MatchFields)
		if err != nil {
			return false, err
		}
		if !selector.Matches(fields.Set{"metadata.name": node.Name}) {
			return false, nil
		}
	}
	return true, nil
}

func nodeSelectorRequirementsAsSelector(reqs []corev1.NodeSelectorRequirement) (labels.Selector, error) {
	selector := labels.NewSelector()
	for _, expr := range reqs {
		op, err := nodeSelectorOperator(expr.Operator)
		if err != nil {
			return nil, err
		}
		requirement, err := labels.NewRequirement(expr.Key, op, expr.Values)
		if err != nil {
			return nil, err
		}
		selector = selector.Add(*requirement)
	}
	return selector, nil
}

func nodeSelectorOperator(op corev1.NodeSelectorOperator) (selection.Operator, error) {
	switch op {
	case corev1.NodeSelectorOpIn:
		return selection.In, nil
	case corev1.NodeSelectorOpNotIn:
		return selection.NotIn, nil
	case corev1.NodeSelectorOpExists:
		return selection.Exists, nil
	case corev1.NodeSelectorOpDoesNotExist:
		return selection.DoesNotExist, nil
	case corev1.NodeSelectorOpGt:
		return selection.GreaterThan, nil
	case corev1.NodeSelectorOpLt:
		return selection.LessThan, nil
	default:
		return "", fmt.Errorf("unsupported node selector operator %q", op)
	}
}

func nodeFieldRequirementsAsSelector(reqs []corev1.NodeSelectorRequirement) (fields.Selector, error) {
	sels := make([]fields.Selector, 0, len(reqs))
	for _, expr := range reqs {
		if len(expr.Values) != 1 {
			return nil, fmt.Errorf("field selector %s must have one value", expr.Key)
		}
		switch expr.Operator {
		case corev1.NodeSelectorOpIn:
			sels = append(sels, fields.OneTermEqualSelector(expr.Key, expr.Values[0]))
		case corev1.NodeSelectorOpNotIn:
			sels = append(sels, fields.OneTermNotEqualSelector(expr.Key, expr.Values[0]))
		default:
			return nil, fmt.Errorf("unsupported field selector operator %q", expr.Operator)
		}
	}
	return fields.AndSelectors(sels...), nil
}

// nodeSchedulingChangedPredicate accepts label and taint changes for nodeName.
// An empty nodeName matches every node. Creates are ignored; the configuration
// informer already reconciles objects that exist at startup.
func nodeSchedulingChangedPredicate(nodeName string) predicate.Predicate {
	matches := func(obj client.Object) bool {
		return obj != nil && (nodeName == "" || obj.GetName() == nodeName)
	}
	return predicate.Funcs{
		CreateFunc: func(event.CreateEvent) bool { return false },
		DeleteFunc: func(e event.DeleteEvent) bool { return matches(e.Object) },
		UpdateFunc: func(e event.UpdateEvent) bool {
			if e.ObjectOld == nil || e.ObjectNew == nil || !matches(e.ObjectNew) {
				return false
			}
			if !reflect.DeepEqual(e.ObjectOld.GetLabels(), e.ObjectNew.GetLabels()) {
				return true
			}
			oldNode, ok1 := e.ObjectOld.(*corev1.Node)
			newNode, ok2 := e.ObjectNew.(*corev1.Node)
			if !ok1 || !ok2 {
				return false
			}
			return !reflect.DeepEqual(oldNode.Spec.Taints, newNode.Spec.Taints)
		},
		GenericFunc: func(event.GenericEvent) bool { return false },
	}
}

func daemonSetSchedulingPredicate(namespace, name string) predicate.Predicate {
	matches := func(obj client.Object) bool {
		return obj != nil && obj.GetNamespace() == namespace && obj.GetName() == name
	}
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool { return matches(e.Object) },
		DeleteFunc: func(e event.DeleteEvent) bool { return matches(e.Object) },
		UpdateFunc: func(e event.UpdateEvent) bool {
			if !matches(e.ObjectNew) {
				return false
			}
			oldDS, ok1 := e.ObjectOld.(*appsv1.DaemonSet)
			newDS, ok2 := e.ObjectNew.(*appsv1.DaemonSet)
			if !ok1 || !ok2 {
				return true
			}
			return daemonSetSchedulingChanged(oldDS, newDS)
		},
		GenericFunc: func(event.GenericEvent) bool { return false },
	}
}

func daemonSetSchedulingChanged(oldDS, newDS *appsv1.DaemonSet) bool {
	oldSpec := &oldDS.Spec.Template.Spec
	newSpec := &newDS.Spec.Template.Spec
	return !reflect.DeepEqual(oldSpec.NodeSelector, newSpec.NodeSelector) ||
		!reflect.DeepEqual(oldSpec.Affinity, newSpec.Affinity) ||
		!reflect.DeepEqual(oldSpec.Tolerations, newSpec.Tolerations) ||
		oldSpec.NodeName != newSpec.NodeName ||
		oldSpec.HostNetwork != newSpec.HostNetwork
}

// SPDX-License-Identifier:Apache-2.0

package v1beta1

import (
	"fmt"

	"go.universe.tf/metallb/api/v1beta2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/conversion"
)

// ConvertTo converts this BGPPeer to the Hub version (v1beta2).
func (src *BGPPeer) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1beta2.BGPPeer)
	dst.Spec.ASN = src.Spec.ASN
	dst.Spec.MyASN = src.Spec.MyASN
	dst.Spec.Address = src.Spec.Address
	dst.Spec.SrcAddress = src.Spec.SrcAddress
	dst.Spec.Port = src.Spec.Port
	dst.Spec.HoldTime = src.Spec.HoldTime
	dst.Spec.KeepaliveTime = src.Spec.KeepaliveTime
	dst.Spec.RouterID = src.Spec.RouterID
	dst.Spec.Password = src.Spec.Password
	dst.Spec.BFDProfile = src.Spec.BFDProfile
	dst.Spec.EBGPMultiHop = src.Spec.EBGPMultiHop
	var err error
	dst.Spec.NodeSelectors, err = parseNodeSelectors(src.Spec.NodeSelectors)
	if err != nil {
		return err
	}
	dst.ObjectMeta = src.ObjectMeta
	return nil
}

// ConvertFrom converts from the Hub version (v1beta2) to this version.
func (dst *BGPPeer) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1beta2.BGPPeer)
	dst.ObjectMeta = src.ObjectMeta
	dst.Spec.MyASN = src.Spec.MyASN
	dst.Spec.ASN = src.Spec.ASN
	dst.Spec.Address = src.Spec.Address
	dst.Spec.SrcAddress = src.Spec.SrcAddress
	dst.Spec.Port = src.Spec.Port
	dst.Spec.HoldTime = src.Spec.HoldTime
	dst.Spec.KeepaliveTime = src.Spec.KeepaliveTime
	dst.Spec.RouterID = src.Spec.RouterID
	dst.Spec.Password = src.Spec.Password
	dst.Spec.BFDProfile = src.Spec.BFDProfile
	dst.Spec.EBGPMultiHop = src.Spec.EBGPMultiHop
	var err error
	dst.Spec.NodeSelectors, err = labelsToLegacySelector(src.Spec.NodeSelectors)
	if err != nil {
		return err
	}
	return nil
}

func parseNodeSelectors(selectors []NodeSelector) ([]metav1.LabelSelector, error) {
	var nodeSels []metav1.LabelSelector
	for _, sel := range selectors {
		nodeSel, err := parseNodeSelector(sel)
		if err != nil {
			return nil, fmt.Errorf("parsing node selector: %s", err)
		}
		nodeSels = append(nodeSels, nodeSel)
	}
	return nodeSels, nil
}

func parseNodeSelector(ns NodeSelector) (metav1.LabelSelector, error) {
	if len(ns.MatchLabels)+len(ns.MatchExpressions) == 0 {
		return metav1.LabelSelector{}, nil
	}

	sel := metav1.LabelSelector{
		MatchLabels: make(map[string]string),
	}
	for k, v := range ns.MatchLabels {
		sel.MatchLabels[k] = v
	}
	for _, req := range ns.MatchExpressions {
		sel.MatchExpressions = append(sel.MatchExpressions, metav1.LabelSelectorRequirement{
			Key:      req.Key,
			Operator: metav1.LabelSelectorOperator(req.Operator),
			Values:   req.Values,
		})
	}
	return sel, nil
}

func labelsToLegacySelector(selectors []metav1.LabelSelector) ([]NodeSelector, error) {
	res := []NodeSelector{}
	for _, sel := range selectors {
		toAdd := NodeSelector{
			MatchLabels:      make(map[string]string),
			MatchExpressions: make([]MatchExpression, 0),
		}
		for k, v := range sel.MatchLabels {
			toAdd.MatchLabels[k] = v
		}
		for _, e := range sel.MatchExpressions {
			m := MatchExpression{
				Key:      e.Key,
				Operator: string(e.Operator),
				Values:   make([]string, len(e.Values)),
			}
			copy(m.Values, e.Values)
			toAdd.MatchExpressions = append(toAdd.MatchExpressions, m)
		}
		res = append(res, toAdd)
	}
	return res, nil
}

/*
Copyright 2018 The Kubernetes Authors.

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

package backendprotocol

import (
	"regexp"
	"strings"

	karmadanetworking "github.com/karmada-io/karmada/pkg/apis/networking/v1alpha1"
	networking "k8s.io/api/networking/v1"
	"k8s.io/klog/v2"

	"k8s.io/ingress-nginx/internal/ingress/annotations/parser"
	"k8s.io/ingress-nginx/internal/ingress/resolver"
)

// HTTP protocol
const HTTP = "HTTP"

var (
	validProtocols = regexp.MustCompile(`^(AUTO_HTTP|HTTP|HTTPS|AJP|GRPC|GRPCS|FCGI)$`)
)

type backendProtocol struct {
	r resolver.Resolver
}

// NewParser creates a new backend protocol annotation parser
func NewParser(r resolver.Resolver) parser.IngressAnnotation {
	return backendProtocol{r}
}

// Parse parses the annotations contained in the ingress
// rule used to indicate the backend protocol.
func (a backendProtocol) Parse(ing *networking.Ingress) (interface{}, error) {
	if ing.GetAnnotations() == nil {
		return HTTP, nil
	}

	proto, err := parser.GetStringAnnotation("backend-protocol", ing)
	if err != nil {
		return HTTP, nil
	}

	proto = strings.TrimSpace(strings.ToUpper(proto))
	if !validProtocols.MatchString(proto) {
		klog.Warningf("Protocol %v is not a valid value for the backend-protocol annotation. Using HTTP as protocol", proto)
		return HTTP, nil
	}

	return proto, nil
}

// ParseByMCI parses the annotations contained in the multiclusteringress
// rule used to indicate the backend protocol.
func (a backendProtocol) ParseByMCI(mci *karmadanetworking.MultiClusterIngress) (interface{}, error) {
	if mci.GetAnnotations() == nil {
		return HTTP, nil
	}

	proto, err := parser.GetStringAnnotationFromMCI("backend-protocol", mci)
	if err != nil {
		return HTTP, nil
	}

	proto = strings.TrimSpace(strings.ToUpper(proto))
	if !validProtocols.MatchString(proto) {
		klog.Warningf("Protocol %v is not a valid value for the backend-protocol annotation. Using HTTP as protocol", proto)
		return HTTP, nil
	}

	return proto, nil
}

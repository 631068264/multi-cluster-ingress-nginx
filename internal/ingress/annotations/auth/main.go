/*
Copyright 2015 The Kubernetes Authors.

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

package auth

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	karmadanetworking "github.com/karmada-io/karmada/pkg/apis/networking/v1alpha1"
	api "k8s.io/api/core/v1"
	networking "k8s.io/api/networking/v1"
	"k8s.io/client-go/tools/cache"

	"k8s.io/ingress-nginx/internal/file"
	"k8s.io/ingress-nginx/internal/ingress/annotations/parser"
	ing_errors "k8s.io/ingress-nginx/internal/ingress/errors"
	"k8s.io/ingress-nginx/internal/ingress/resolver"
)

var (
	authTypeRegex = regexp.MustCompile(`basic|digest`)
	// AuthDirectory default directory used to store files
	// to authenticate request
	AuthDirectory = "/etc/ingress-controller/auth"
)

const (
	fileAuth = "auth-file"
	mapAuth  = "auth-map"
)

// Config returns authentication configuration for an Ingress rule
type Config struct {
	Type       string `json:"type"`
	Realm      string `json:"realm"`
	File       string `json:"file"`
	Secured    bool   `json:"secured"`
	FileSHA    string `json:"fileSha"`
	Secret     string `json:"secret"`
	SecretType string `json:"secretType"`
}

// Equal tests for equality between two Config types
func (bd1 *Config) Equal(bd2 *Config) bool {
	if bd1 == bd2 {
		return true
	}
	if bd1 == nil || bd2 == nil {
		return false
	}
	if bd1.Type != bd2.Type {
		return false
	}
	if bd1.Realm != bd2.Realm {
		return false
	}
	if bd1.File != bd2.File {
		return false
	}
	if bd1.Secured != bd2.Secured {
		return false
	}
	if bd1.FileSHA != bd2.FileSHA {
		return false
	}
	if bd1.Secret != bd2.Secret {
		return false
	}
	return true
}

type auth struct {
	r             resolver.Resolver
	authDirectory string
}

// NewParser creates a new authentication annotation parser
func NewParser(authDirectory string, r resolver.Resolver) parser.IngressAnnotation {
	return auth{r, authDirectory}
}

// Parse parses the annotations contained in the ingress
// rule used to add authentication in the paths defined in the rule
// and generated an htpasswd compatible file to be used as source
// during the authentication process
func (a auth) Parse(ing *networking.Ingress) (interface{}, error) {
	at, err := parser.GetStringAnnotation("auth-type", ing)
	if err != nil {
		return nil, err
	}

	if !authTypeRegex.MatchString(at) {
		return nil, ing_errors.NewLocationDenied("invalid authentication type")
	}

	var secretType string
	secretType, err = parser.GetStringAnnotation("auth-secret-type", ing)
	if err != nil {
		secretType = fileAuth
	}

	s, err := parser.GetStringAnnotation("auth-secret", ing)
	if err != nil {
		return nil, ing_errors.LocationDenied{
			Reason: fmt.Errorf("error reading secret name from annotation: %w", err),
		}
	}

	sns, sname, err := cache.SplitMetaNamespaceKey(s)
	if err != nil {
		return nil, ing_errors.LocationDenied{
			Reason: fmt.Errorf("error reading secret name from annotation: %w", err),
		}
	}

	if sns == "" {
		sns = ing.Namespace
	}

	name := fmt.Sprintf("%v/%v", sns, sname)
	secret, err := a.r.GetSecret(name)
	if err != nil {
		return nil, ing_errors.LocationDenied{
			Reason: fmt.Errorf("unexpected error reading secret %s: %w", name, err),
		}
	}

	realm, _ := parser.GetStringAnnotation("auth-realm", ing)

	passFilename := fmt.Sprintf("%v/%v-%v-%v.passwd", a.authDirectory, ing.GetNamespace(), ing.UID, secret.UID)

	switch secretType {
	case fileAuth:
		err = dumpSecretAuthFile(passFilename, secret)
		if err != nil {
			return nil, err
		}
	case mapAuth:
		err = dumpSecretAuthMap(passFilename, secret)
		if err != nil {
			return nil, err
		}
	default:
		return nil, ing_errors.LocationDenied{
			Reason: fmt.Errorf("invalid auth-secret-type in annotation, must be 'auth-file' or 'auth-map': %w", err),
		}
	}

	return &Config{
		Type:       at,
		Realm:      realm,
		File:       passFilename,
		Secured:    true,
		FileSHA:    file.SHA1(passFilename),
		Secret:     name,
		SecretType: secretType,
	}, nil
}

// ParseByMCI parses the annotations contained in the multiclusteringress
// rule used to add authentication in the paths defined in the rule
// and generated an htpasswd compatible file to be used as source
// during the authentication process
func (a auth) ParseByMCI(mci *karmadanetworking.MultiClusterIngress) (interface{}, error) {
	at, err := parser.GetStringAnnotationFromMCI("auth-type", mci)
	if err != nil {
		return nil, err
	}

	if !authTypeRegex.MatchString(at) {
		return nil, ing_errors.NewLocationDenied("invalid authentication type")
	}

	var secretType string
	secretType, err = parser.GetStringAnnotationFromMCI("auth-secret-type", mci)
	if err != nil {
		secretType = fileAuth
	}

	s, err := parser.GetStringAnnotationFromMCI("auth-secret", mci)
	if err != nil {
		return nil, ing_errors.LocationDenied{
			Reason: fmt.Errorf("error reading secret name from annotation: %w", err),
		}
	}

	sns, sname, err := cache.SplitMetaNamespaceKey(s)
	if err != nil {
		return nil, ing_errors.LocationDenied{
			Reason: fmt.Errorf("error reading secret name from annotation: %w", err),
		}
	}

	if sns == "" {
		sns = mci.Namespace
	}

	name := fmt.Sprintf("%v/%v", sns, sname)
	secret, err := a.r.GetSecret(name)
	if err != nil {
		return nil, ing_errors.LocationDenied{
			Reason: fmt.Errorf("unexpected error reading secret %s: %w", name, err),
		}
	}

	realm, _ := parser.GetStringAnnotationFromMCI("auth-realm", mci)

	passFilename := fmt.Sprintf("%v/%v-%v-%v.passwd", a.authDirectory, mci.GetNamespace(), mci.UID, secret.UID)

	switch secretType {
	case fileAuth:
		err = dumpSecretAuthFile(passFilename, secret)
		if err != nil {
			return nil, err
		}
	case mapAuth:
		err = dumpSecretAuthMap(passFilename, secret)
		if err != nil {
			return nil, err
		}
	default:
		return nil, ing_errors.LocationDenied{
			Reason: fmt.Errorf("invalid auth-secret-type in annotation, must be 'auth-file' or 'auth-map': %w", err),
		}
	}

	return &Config{
		Type:       at,
		Realm:      realm,
		File:       passFilename,
		Secured:    true,
		FileSHA:    file.SHA1(passFilename),
		Secret:     name,
		SecretType: secretType,
	}, nil
}

// dumpSecret dumps the content of a secret into a file
// in the expected format for the specified authorization
func dumpSecretAuthFile(filename string, secret *api.Secret) error {
	val, ok := secret.Data["auth"]
	if !ok {
		return ing_errors.LocationDenied{
			Reason: fmt.Errorf("the secret %s does not contain a key with value auth", secret.Name),
		}
	}

	err := os.WriteFile(filename, val, file.ReadWriteByUser)
	if err != nil {
		return ing_errors.LocationDenied{
			Reason: fmt.Errorf("unexpected error creating password file: %w", err),
		}
	}

	return nil
}

func dumpSecretAuthMap(filename string, secret *api.Secret) error {
	builder := &strings.Builder{}
	for user, pass := range secret.Data {
		builder.WriteString(user)
		builder.WriteString(":")
		builder.WriteString(string(pass))
		builder.WriteString("\n")
	}

	err := os.WriteFile(filename, []byte(builder.String()), file.ReadWriteByUser)
	if err != nil {
		return ing_errors.LocationDenied{
			Reason: fmt.Errorf("unexpected error creating password file: %w", err),
		}
	}

	return nil
}

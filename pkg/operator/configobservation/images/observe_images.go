package images

import (
	"bytes"
	"encoding/json"
	"reflect"

	"github.com/golang/glog"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/openshift/library-go/pkg/operator/configobserver"
	"github.com/openshift/library-go/pkg/operator/events"

	"github.com/openshift/cluster-kube-apiserver-operator/pkg/operator/configobservation"
)

// ObserveInternalRegistryHostname reads the internal registry hostname from the cluster configuration as provided by
// the registry operator.
func ObserveInternalRegistryHostname(genericListers configobserver.Listers, recorder events.Recorder, existingConfig map[string]interface{}) (map[string]interface{}, []error) {
	listers := genericListers.(configobservation.Listers)
	errs := []error{}
	prevObservedConfig := map[string]interface{}{}

	internalRegistryHostnamePath := []string{"imagePolicyConfig", "internalRegistryHostname"}
	currentInternalRegistryHostname, _, err := unstructured.NestedString(existingConfig, internalRegistryHostnamePath...)
	if err != nil {
		return prevObservedConfig, append(errs, err)
	}
	if len(currentInternalRegistryHostname) > 0 {
		if err := unstructured.SetNestedField(prevObservedConfig, currentInternalRegistryHostname, internalRegistryHostnamePath...); err != nil {
			errs = append(errs, err)
		}
	}

	if !listers.ImageConfigSynced() {
		glog.Warning("images.config.openshift.io not synced")
		return prevObservedConfig, errs
	}

	observedConfig := map[string]interface{}{}
	configImage, err := listers.ImageConfigLister.Get("cluster")
	if errors.IsNotFound(err) {
		glog.Warningf("image.config.openshift.io/cluster: not found")
		return observedConfig, errs
	}
	if err != nil {
		return prevObservedConfig, errs
	}

	internalRegistryHostName := configImage.Status.InternalRegistryHostname
	if len(internalRegistryHostName) > 0 {
		if err := unstructured.SetNestedField(observedConfig, internalRegistryHostName, internalRegistryHostnamePath...); err != nil {
			errs = append(errs, err)
		}
		if internalRegistryHostName != currentInternalRegistryHostname {
			recorder.Eventf("ObserveInternalRegistryHostnameChanged", "Internal registry hostname changed to %q", internalRegistryHostName)
		}
	}
	return observedConfig, errs
}

// ObserveExternalRegistryHostnames maps the user provided+generated external registry hostnames to the kube api server
// configuration.
func ObserveExternalRegistryHostnames(genericListers configobserver.Listers, recorder events.Recorder, existingConfig map[string]interface{}) (map[string]interface{}, []error) {
	listers := genericListers.(configobservation.Listers)
	var errs []error
	prevObservedConfig := map[string]interface{}{}

	// first observe all the existing config values so that if we get any errors
	// we can at least return those.
	externalRegistryHostnamePath := []string{"imagePolicyConfig", "externalRegistryHostnames"}
	o, _, err := unstructured.NestedSlice(existingConfig, externalRegistryHostnamePath...)
	if err != nil {
		return prevObservedConfig, append(errs, err)
	}
	if len(o) > 0 {
		err := unstructured.SetNestedSlice(prevObservedConfig, o, externalRegistryHostnamePath...)
		if err != nil {
			return prevObservedConfig, append(errs, err)
		}
	}

	if !listers.ImageConfigSynced() {
		glog.Warning("images.config.openshift.io not synced")
		return prevObservedConfig, errs
	}

	// now gather the cluster config and turn it into the observed config
	observedConfig := map[string]interface{}{}
	configImage, err := listers.ImageConfigLister.Get("cluster")
	if errors.IsNotFound(err) {
		glog.Warningf("image.config.openshift.io/cluster: not found")
		return observedConfig, errs
	}
	if err != nil {
		return prevObservedConfig, append(errs, err)
	}

	// User provided values take precedence, first entry in the array
	// has special significance.
	externalRegistryHostnames := configImage.Spec.ExternalRegistryHostnames
	externalRegistryHostnames = append(externalRegistryHostnames, configImage.Status.ExternalRegistryHostnames...)

	if len(externalRegistryHostnames) > 0 {
		hostnames, err := convert(externalRegistryHostnames)
		if err != nil {
			return prevObservedConfig, append(errs, err)
		}
		err = unstructured.SetNestedField(observedConfig, hostnames, externalRegistryHostnamePath...)
		if err != nil {
			return prevObservedConfig, append(errs, err)
		}
	}

	if !reflect.DeepEqual(o, externalRegistryHostnames) {
		recorder.Eventf("ObserveExternalRegistryHostnameChanged", "External registry hostname changed to %v", externalRegistryHostnames)
	}

	return observedConfig, errs
}

// ObserveAllowedRegistriesForImport maps the user provided list of allowed registries for importing images to the kube api server
// configuration.
func ObserveAllowedRegistriesForImport(genericListers configobserver.Listers, recorder events.Recorder, existingConfig map[string]interface{}) (map[string]interface{}, []error) {
	listers := genericListers.(configobservation.Listers)
	var errs []error
	prevObservedConfig := map[string]interface{}{}

	// first observe all the existing config values so that if we get any errors
	// we can at least return those.
	allowedRegistriesForImportPath := []string{"imagePolicyConfig", "allowedRegistriesForImport"}
	o, _, err := unstructured.NestedSlice(existingConfig, allowedRegistriesForImportPath...)
	if err != nil {
		return prevObservedConfig, append(errs, err)
	}
	if len(o) > 0 {
		err := unstructured.SetNestedSlice(prevObservedConfig, o, allowedRegistriesForImportPath...)
		if err != nil {
			return prevObservedConfig, append(errs, err)
		}
	}

	if !listers.ImageConfigSynced() {
		glog.Warning("images.config.openshift.io not synced")
		return prevObservedConfig, errs
	}

	// now gather the cluster config and turn it into the observed config
	observedConfig := map[string]interface{}{}
	configImage, err := listers.ImageConfigLister.Get("cluster")
	if errors.IsNotFound(err) {
		glog.Warningf("image.config.openshift.io/cluster: not found")
		return observedConfig, errs
	}
	if err != nil {
		return prevObservedConfig, append(errs, err)
	}

	if len(configImage.Spec.AllowedRegistriesForImport) > 0 {
		allowed, err := convert(configImage.Spec.AllowedRegistriesForImport)
		if err != nil {
			return prevObservedConfig, append(errs, err)
		}
		err = unstructured.SetNestedField(observedConfig, allowed, allowedRegistriesForImportPath...)
		if err != nil {
			return prevObservedConfig, append(errs, err)
		}
	}

	if !reflect.DeepEqual(o, configImage.Spec.AllowedRegistriesForImport) {
		recorder.Eventf("ObserveAllowedRegistriesForImport", "Allowed registries for import changed to %v", configImage.Spec.AllowedRegistriesForImport)
	}

	return observedConfig, errs
}

// convert converts an arbitrary object into the json decoded equivalent by
// first encoding it into a json string and then decoding the string and
// returning it.
func convert(o interface{}) (interface{}, error) {
	if o == nil {
		return nil, nil
	}
	buf := &bytes.Buffer{}
	if err := json.NewEncoder(buf).Encode(o); err != nil {
		return nil, err
	}

	ret := []interface{}{}
	if err := json.NewDecoder(buf).Decode(&ret); err != nil {
		return nil, err
	}

	return ret, nil
}
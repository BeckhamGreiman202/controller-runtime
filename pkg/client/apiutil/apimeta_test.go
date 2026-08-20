package apiutil

import (
	"net/http"
	"sync"
	"testing"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
)

type fakeDiscoveryClient struct {
	discovery.DiscoveryInterface
	mu        sync.Mutex
	resources []*metav1.APIResourceList
}

func (f *fakeDiscoveryClient) ServerGroupsAndResources() ([]*metav1.APIGroup, []*metav1.APIResourceList, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return nil, f.resources, nil
}

func TestRESTMapperStaleCache(t *testing.T) {
	resources := []*metav1.APIResourceList{
		{
			GroupVersion: "example.com/v1beta1",
			APIResources: []metav1.APIResource{
				{Name: "foos", Namespaced: true, Kind: "Foo"},
			},
		},
	}

	fdc := &fakeDiscoveryClient{resources: resources}
	gr := restmapper.GetAPIGroupResources(fdc)
	deferredMapper := restmapper.NewDeferredDiscoveryRESTMapper(gr)
	expander := restmapper.NewShortcutExpander(deferredMapper, fdc)

	mapper := &restMapper{
		RESTMapper:          expander,
		discoveryRESTMapper: deferredMapper,
	}

	// 1. Perform lookup for example.com/v1beta1
	gk := schema.GroupKind{Group: "example.com", Kind: "Foo"}
	mapping, err := mapper.RESTMapping(gk)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mapping.GroupVersionKind.Version != "v1beta1" {
		t.Errorf("expected version v1beta1, got %s", mapping.GroupVersionKind.Version)
	}

	// 2. Simulate API server change by adding v1 and making it preferred
	fdc.mu.Lock()
	fdc.resources = []*metav1.APIResourceList{
		{
			GroupVersion: "example.com/v1",
			APIResources: []metav1.APIResource{
				{Name: "foos", Namespaced: true, Kind: "Foo"},
			},
		},
		{
			GroupVersion: "example.com/v1beta1",
			APIResources: []metav1.APIResource{
				{Name: "foos", Namespaced: true, Kind: "Foo"},
			},
		},
	}
	fdc.mu.Unlock()

	// 3. Reset the mapper
	mapper.Reset()

	// 4. Perform lookup again and assert it resolves to the new version
	mapping, err = mapper.RESTMapping(gk)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mapping.GroupVersionKind.Version != "v1" {
		t.Errorf("expected version v1, got %s", mapping.GroupVersionKind.Version)
	}
}

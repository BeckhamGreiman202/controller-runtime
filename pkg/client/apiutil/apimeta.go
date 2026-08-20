package apiutil

import (
	"net/http"
	"sync"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
)

// NewDynamicRESTMapper returns a dynamic RESTMapper for the given config.
func NewDynamicRESTMapper(cfg *rest.Config, hc ...*http.Client) (meta.RESTMapper, error) {
	var httpClient *http.Client
	if len(hc) > 0 {
		httpClient = hc[0]
	} else {
		var err error
		httpClient, err = rest.HTTPClientFor(cfg)
		if err != nil {
			return nil, err
		}
	}

	dc, err := discovery.NewDiscoveryClientForConfigAndClient(cfg, httpClient)
	if err != nil {
		return nil, err
	}

	gr := restmapper.GetAPIGroupResources(dc)
	deferredMapper := restmapper.NewDeferredDiscoveryRESTMapper(gr)
	expander := restmapper.NewShortcutExpander(deferredMapper, dc)

	return &restMapper{
		RESTMapper:          expander,
		discoveryRESTMapper: deferredMapper,
	}, nil
}

type restMapper struct {
	meta.RESTMapper
	mu                  sync.Mutex
	discoveryRESTMapper *restmapper.DeferredDiscoveryRESTMapper
}

func (m *restMapper) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.discoveryRESTMapper.Reset()
}

func (m *restMapper) RESTMapping(gk schema.GroupKind, versions ...string) (*meta.RESTMapping, error) {
	mapping, err := m.RESTMapper.RESTMapping(gk, versions...)
	if err != nil && meta.IsNoMatchError(err) {
		m.Reset()
		return m.RESTMapper.RESTMapping(gk, versions...)
	}
	return mapping, err
}

func (m *restMapper) RESTMappings(gk schema.GroupKind, versions ...string) ([]*meta.RESTMapping, error) {
	mappings, err := m.RESTMapper.RESTMappings(gk, versions...)
	if err != nil && meta.IsNoMatchError(err) {
		m.Reset()
		return m.RESTMapper.RESTMappings(gk, versions...)
	}
	return mappings, err
}

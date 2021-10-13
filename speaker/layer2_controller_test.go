// SPDX-License-Identifier:Apache-2.0

package main

import (
	"net"
	"os"
	"sort"
	"testing"

	"go.universe.tf/metallb/internal/config"
	"go.universe.tf/metallb/internal/k8s"

	"github.com/go-kit/kit/log"
	v1 "k8s.io/api/core/v1"
	discovery "k8s.io/api/discovery/v1beta1"
)

type fakeSpeakerList struct {
	speakers map[string]bool
}

func (sl *fakeSpeakerList) UsableSpeakers() map[string]bool {
	return sl.speakers
}

func (sl *fakeSpeakerList) Rejoin() {}

func compareUseableNodesReturnedValue(a, b []string) bool {
	if &a == &b {
		return true
	}
	if len(a) != len(b) {
		return false
	}
	for i, v := range a {
		if b[i] != v {
			return false
		}
	}
	return true
}

func TestUsableNodes(t *testing.T) {
	c, err := newController(controllerConfig{
		MyNode: "iris1",
		Logger: log.NewLogfmtLogger(log.NewSyncWriter(os.Stderr)),
	})
	if err != nil {
		t.Fatalf("creating controller: %s", err)
	}
	c.client = &testK8S{t: t}

	tests := []struct {
		desc string

		eps k8s.EpsOrSlices

		usableSpeakers map[string]bool

		cExpectedResult []string
	}{
		{
			desc: "Two endpoints, different hosts",
			eps: k8s.EpsOrSlices{
				EpVal: &v1.Endpoints{
					Subsets: []v1.EndpointSubset{
						{
							Addresses: []v1.EndpointAddress{
								{
									IP:       "2.3.4.5",
									NodeName: strptr("iris1"),
								},
								{
									IP:       "2.3.4.15",
									NodeName: strptr("iris2"),
								},
							},
						},
					},
				},
				Type: k8s.Eps,
			},
			usableSpeakers:  map[string]bool{"iris1": true, "iris2": true},
			cExpectedResult: []string{"iris1", "iris2"},
		}, {
			desc: "Two endpoints, same host",
			eps: k8s.EpsOrSlices{
				EpVal: &v1.Endpoints{
					Subsets: []v1.EndpointSubset{
						{
							Addresses: []v1.EndpointAddress{
								{
									IP:       "2.3.4.5",
									NodeName: strptr("iris1"),
								},
								{
									IP:       "2.3.4.15",
									NodeName: strptr("iris1"),
								},
							},
						},
					},
				},
				Type: k8s.Eps,
			},
			usableSpeakers:  map[string]bool{"iris1": true, "iris2": true},
			cExpectedResult: []string{"iris1"},
		}, {
			desc: "Two endpoints, same host, one is not ready",
			eps: k8s.EpsOrSlices{
				EpVal: &v1.Endpoints{
					Subsets: []v1.EndpointSubset{
						{
							Addresses: []v1.EndpointAddress{
								{
									IP:       "2.3.4.5",
									NodeName: strptr("iris1"),
								},
							},
							NotReadyAddresses: []v1.EndpointAddress{
								{
									IP:       "2.3.4.15",
									NodeName: strptr("iris1"),
								},
							},
						},
					},
				},
				Type: k8s.Eps,
			},
			usableSpeakers:  map[string]bool{"iris1": true, "iris2": true},
			cExpectedResult: []string{"iris1"},
		},
	}

	for _, test := range tests {
		response := usableNodes(test.eps, test.usableSpeakers)
		sort.Strings(response)
		if !compareUseableNodesReturnedValue(response, test.cExpectedResult) {
			t.Errorf("%q: shouldAnnounce for controller returned incorrect result, expected '%s', but received '%s'", test.desc, test.cExpectedResult, response)
		}
	}
}

func TestUsableNodesEPSlices(t *testing.T) {
	c, err := newController(controllerConfig{
		MyNode: "iris1",
		Logger: log.NewLogfmtLogger(log.NewSyncWriter(os.Stderr)),
	})
	if err != nil {
		t.Fatalf("creating controller: %s", err)
	}
	c.client = &testK8S{t: t}

	tests := []struct {
		desc string

		eps k8s.EpsOrSlices

		usableSpeakers map[string]bool

		cExpectedResult []string
	}{
		{
			desc: "Two endpoints, different hosts, multi slice",
			eps: k8s.EpsOrSlices{
				SlicesVal: []*discovery.EndpointSlice{
					{
						Endpoints: []discovery.Endpoint{
							{
								Addresses: []string{
									"2.3.4.5",
								},
								Topology: map[string]string{
									"kubernetes.io/hostname": "iris1",
								},
								Conditions: discovery.EndpointConditions{
									Ready: boolPtr(true),
								},
							},
						},
					},
					{
						Endpoints: []discovery.Endpoint{
							{
								Addresses: []string{
									"2.3.4.15",
								},
								Topology: map[string]string{
									"kubernetes.io/hostname": "iris2",
								},
								Conditions: discovery.EndpointConditions{
									Ready: boolPtr(true),
								},
							},
						},
					},
				},
				Type: k8s.Slices,
			},
			usableSpeakers:  map[string]bool{"iris1": true, "iris2": true},
			cExpectedResult: []string{"iris1", "iris2"},
		},
		{
			desc: "Two endpoints, different hosts",
			eps: k8s.EpsOrSlices{
				SlicesVal: []*discovery.EndpointSlice{
					{
						Endpoints: []discovery.Endpoint{
							{
								Addresses: []string{
									"2.3.4.5",
								},
								Topology: map[string]string{
									"kubernetes.io/hostname": "iris1",
								},
								Conditions: discovery.EndpointConditions{
									Ready: boolPtr(true),
								},
							},
							{
								Addresses: []string{
									"2.3.4.15",
								},
								Topology: map[string]string{
									"kubernetes.io/hostname": "iris2",
								},
								Conditions: discovery.EndpointConditions{
									Ready: boolPtr(true),
								},
							},
						},
					},
				},
				Type: k8s.Slices,
			},
			usableSpeakers:  map[string]bool{"iris1": true, "iris2": true},
			cExpectedResult: []string{"iris1", "iris2"},
		},
		{
			desc: "Two endpoints, same host",
			eps: k8s.EpsOrSlices{
				SlicesVal: []*discovery.EndpointSlice{
					{
						Endpoints: []discovery.Endpoint{
							{
								Addresses: []string{
									"2.3.4.5",
								},
								Topology: map[string]string{
									"kubernetes.io/hostname": "iris1",
								},
								Conditions: discovery.EndpointConditions{
									Ready: boolPtr(true),
								},
							},
							{
								Addresses: []string{
									"2.3.4.15",
								},
								Topology: map[string]string{
									"kubernetes.io/hostname": "iris1",
								},
								Conditions: discovery.EndpointConditions{
									Ready: boolPtr(true),
								},
							},
						},
					},
				},
				Type: k8s.Slices,
			},
			usableSpeakers:  map[string]bool{"iris1": true, "iris2": true},
			cExpectedResult: []string{"iris1"},
		},

		{
			desc: "Two endpoints, same host, one is not ready",
			eps: k8s.EpsOrSlices{
				SlicesVal: []*discovery.EndpointSlice{
					{
						Endpoints: []discovery.Endpoint{
							{
								Addresses: []string{
									"2.3.4.5",
								},
								Topology: map[string]string{
									"kubernetes.io/hostname": "iris1",
								},
								Conditions: discovery.EndpointConditions{
									Ready: boolPtr(true),
								},
							},
							{
								Addresses: []string{
									"2.3.4.15",
								},
								Topology: map[string]string{
									"kubernetes.io/hostname": "iris1",
								},
								Conditions: discovery.EndpointConditions{
									Ready: boolPtr(false),
								},
							},
						},
					},
				},
				Type: k8s.Slices,
			},
			usableSpeakers:  map[string]bool{"iris1": true, "iris2": true},
			cExpectedResult: []string{"iris1"},
		},
	}

	for _, test := range tests {
		response := usableNodes(test.eps, test.usableSpeakers)
		sort.Strings(response)
		if !compareUseableNodesReturnedValue(response, test.cExpectedResult) {
			t.Errorf("%q: shouldAnnounce for controller returned incorrect result, expected '%s', but received '%s'", test.desc, test.cExpectedResult, response)
		}
	}
}

func TestShouldAnnounce(t *testing.T) {
	fakeSL := &fakeSpeakerList{
		speakers: map[string]bool{
			"iris1": true,
			"iris2": true,
		},
	}
	c1, err := newController(controllerConfig{
		MyNode: "iris1",
		Logger: log.NewLogfmtLogger(log.NewSyncWriter(os.Stderr)),
		SList:  fakeSL,
	})
	if err != nil {
		t.Fatalf("creating controller: %s", err)
	}
	c1.client = &testK8S{t: t}

	c2, err := newController(controllerConfig{
		MyNode: "iris2",
		Logger: log.NewLogfmtLogger(log.NewSyncWriter(os.Stderr)),
		SList:  fakeSL,
	})
	if err != nil {
		t.Fatalf("creating controller: %s", err)
	}
	c2.client = &testK8S{t: t}

	tests := []struct {
		desc string

		balancer string
		config   *config.Config
		svcs     []*v1.Service
		eps      map[string]k8s.EpsOrSlices

		c1ExpectedResult map[string]string
		c2ExpectedResult map[string]string
	}{
		{
			desc:     "One service, two endpoints, one host, controller 1 should announce",
			balancer: "test1",
			config: &config.Config{
				Pools: map[string]*config.Pool{
					"default": {
						Protocol: config.Layer2,
						CIDR:     []*net.IPNet{ipnet("10.20.30.0/24")},
					},
				},
			},
			svcs: []*v1.Service{
				{
					Spec: v1.ServiceSpec{
						Type:                  "LoadBalancer",
						ExternalTrafficPolicy: v1.ServiceExternalTrafficPolicyTypeLocal,
					},
					Status: statusAssigned("10.20.30.1"),
				},
			},
			eps: map[string]k8s.EpsOrSlices{
				"10.20.30.1": {
					EpVal: &v1.Endpoints{
						Subsets: []v1.EndpointSubset{
							{
								Addresses: []v1.EndpointAddress{
									{
										IP:       "2.3.4.5",
										NodeName: strptr("iris1"),
									},
									{
										IP:       "2.3.4.15",
										NodeName: strptr("iris1"),
									},
								},
							},
						},
					},
					Type: k8s.Eps,
				},
			},
			c1ExpectedResult: map[string]string{
				"10.20.30.1": "",
			},
			c2ExpectedResult: map[string]string{
				"10.20.30.1": "notOwner",
			},
		},

		{
			desc:     "One service, two endpoints, one host, neither endpoint is ready, no controller should announce",
			balancer: "test1",
			config: &config.Config{
				Pools: map[string]*config.Pool{
					"default": {
						Protocol: config.Layer2,
						CIDR:     []*net.IPNet{ipnet("10.20.30.0/24")},
					},
				},
			},
			svcs: []*v1.Service{
				{
					Spec: v1.ServiceSpec{
						Type:                  "LoadBalancer",
						ExternalTrafficPolicy: v1.ServiceExternalTrafficPolicyTypeLocal,
					},
					Status: statusAssigned("10.20.30.1"),
				},
			},
			eps: map[string]k8s.EpsOrSlices{
				"10.20.30.1": {
					EpVal: &v1.Endpoints{
						Subsets: []v1.EndpointSubset{
							{
								NotReadyAddresses: []v1.EndpointAddress{
									{
										IP:       "2.3.4.5",
										NodeName: strptr("iris1"),
									},
									{
										IP:       "2.3.4.15",
										NodeName: strptr("iris1"),
									},
								},
							},
						},
					},
					Type: k8s.Eps,
				},
			},
			c1ExpectedResult: map[string]string{
				"10.20.30.1": "notOwner",
			},
			c2ExpectedResult: map[string]string{
				"10.20.30.1": "notOwner",
			},
		},
		{
			desc:     "One service, two endpoints across two hosts, controller2 should announce",
			balancer: "test1",
			config: &config.Config{
				Pools: map[string]*config.Pool{
					"default": {
						Protocol: config.Layer2,
						CIDR:     []*net.IPNet{ipnet("10.20.30.0/24")},
					},
				},
			},
			svcs: []*v1.Service{
				{
					Spec: v1.ServiceSpec{
						Type:                  "LoadBalancer",
						ExternalTrafficPolicy: v1.ServiceExternalTrafficPolicyTypeLocal,
					},
					Status: statusAssigned("10.20.30.1"),
				},
			},
			eps: map[string]k8s.EpsOrSlices{
				"10.20.30.1": {
					EpVal: &v1.Endpoints{
						Subsets: []v1.EndpointSubset{
							{
								Addresses: []v1.EndpointAddress{
									{
										IP:       "2.3.4.5",
										NodeName: strptr("iris1"),
									},
									{
										IP:       "2.3.4.15",
										NodeName: strptr("iris2"),
									},
								},
							},
						},
					},
					Type: k8s.Eps,
				},
			},
			c1ExpectedResult: map[string]string{
				"10.20.30.1": "notOwner",
			},
			c2ExpectedResult: map[string]string{
				"10.20.30.1": "",
			},
		}, {
			desc: "One service, two endpoints across two hosts, neither endpoint is ready, no controllers should announce",
			config: &config.Config{
				Pools: map[string]*config.Pool{
					"default": {
						Protocol: config.Layer2,
						CIDR:     []*net.IPNet{ipnet("10.20.30.0/24")},
					},
				},
			},
			svcs: []*v1.Service{
				{
					Spec: v1.ServiceSpec{
						Type:                  "LoadBalancer",
						ExternalTrafficPolicy: v1.ServiceExternalTrafficPolicyTypeLocal,
					},
					Status: statusAssigned("10.20.30.1"),
				},
			},
			eps: map[string]k8s.EpsOrSlices{
				"10.20.30.1": {
					EpVal: &v1.Endpoints{
						Subsets: []v1.EndpointSubset{
							{
								NotReadyAddresses: []v1.EndpointAddress{
									{
										IP:       "2.3.4.5",
										NodeName: strptr("iris1"),
									},
									{
										IP:       "2.3.4.15",
										NodeName: strptr("iris2"),
									},
								},
							},
						},
					},
					Type: k8s.Eps,
				},
			},
			c1ExpectedResult: map[string]string{
				"10.20.30.1": "notOwner",
			},
			c2ExpectedResult: map[string]string{
				"10.20.30.1": "notOwner",
			},
		}, {
			desc: "One service, two endpoints across two hosts, controller 2 is not ready, controller 1 should announce",
			config: &config.Config{
				Pools: map[string]*config.Pool{
					"default": {
						Protocol: config.Layer2,
						CIDR:     []*net.IPNet{ipnet("10.20.30.0/24")},
					},
				},
			},
			svcs: []*v1.Service{
				{
					Spec: v1.ServiceSpec{
						Type:                  "LoadBalancer",
						ExternalTrafficPolicy: v1.ServiceExternalTrafficPolicyTypeLocal,
					},
					Status: statusAssigned("10.20.30.1"),
				},
			},
			eps: map[string]k8s.EpsOrSlices{
				"10.20.30.1": {
					EpVal: &v1.Endpoints{
						Subsets: []v1.EndpointSubset{
							{
								Addresses: []v1.EndpointAddress{
									{
										IP:       "2.3.4.5",
										NodeName: strptr("iris1"),
									},
								},
								NotReadyAddresses: []v1.EndpointAddress{
									{
										IP:       "2.3.4.15",
										NodeName: strptr("iris2"),
									},
								},
							},
						},
					},
					Type: k8s.Eps,
				},
			},
			c1ExpectedResult: map[string]string{
				"10.20.30.1": "",
			},
			c2ExpectedResult: map[string]string{
				"10.20.30.1": "notOwner",
			},
		}, {
			desc: "Two services each with two endpoints across across two hosts, controller 1 should announce the second, controller 2 the first",
			config: &config.Config{
				Pools: map[string]*config.Pool{
					"default": {
						Protocol: config.Layer2,
						CIDR:     []*net.IPNet{ipnet("10.20.30.0/24")},
					},
				},
			},
			svcs: []*v1.Service{
				{
					Spec: v1.ServiceSpec{
						Type:                  "LoadBalancer",
						ExternalTrafficPolicy: v1.ServiceExternalTrafficPolicyTypeLocal,
					},
					Status: statusAssigned("10.20.30.1"),
				},
				{
					Spec: v1.ServiceSpec{
						Type:                  "LoadBalancer",
						ExternalTrafficPolicy: v1.ServiceExternalTrafficPolicyTypeLocal,
					},
					Status: statusAssigned("10.20.30.2"),
				},
			},
			eps: map[string]k8s.EpsOrSlices{
				"10.20.30.1": {
					EpVal: &v1.Endpoints{
						Subsets: []v1.EndpointSubset{
							{
								Addresses: []v1.EndpointAddress{
									{
										IP:       "2.3.4.5",
										NodeName: strptr("iris1"),
									},
									{
										IP:       "2.3.4.15",
										NodeName: strptr("iris2"),
									},
								},
							},
						},
					},
					Type: k8s.Eps,
				},
				"10.20.30.2": {
					EpVal: &v1.Endpoints{
						Subsets: []v1.EndpointSubset{
							{
								Addresses: []v1.EndpointAddress{
									{
										IP:       "2.3.4.25",
										NodeName: strptr("iris1"),
									},
									{
										IP:       "2.3.4.35",
										NodeName: strptr("iris2"),
									},
								},
							},
						},
					},
					Type: k8s.Eps,
				},
			},
			c1ExpectedResult: map[string]string{
				"10.20.30.1": "notOwner",
				"10.20.30.2": "",
			},
			c2ExpectedResult: map[string]string{
				"10.20.30.1": "",
				"10.20.30.2": "notOwner",
			},
		}, {
			desc: "Two services each with two endpoints across across two hosts, one service has an endpoint not ready on controller 2, controller 2 should not announce for the service with the not ready endpoint",
			config: &config.Config{
				Pools: map[string]*config.Pool{
					"default": {
						Protocol: config.Layer2,
						CIDR:     []*net.IPNet{ipnet("10.20.30.0/24")},
					},
				},
			},
			svcs: []*v1.Service{
				{
					Spec: v1.ServiceSpec{
						Type:                  "LoadBalancer",
						ExternalTrafficPolicy: v1.ServiceExternalTrafficPolicyTypeLocal,
					},
					Status: statusAssigned("10.20.30.1"),
				},
				{
					Spec: v1.ServiceSpec{
						Type:                  "LoadBalancer",
						ExternalTrafficPolicy: v1.ServiceExternalTrafficPolicyTypeLocal,
					},
					Status: statusAssigned("10.20.30.2"),
				},
			},
			eps: map[string]k8s.EpsOrSlices{
				"10.20.30.1": {
					EpVal: &v1.Endpoints{
						Subsets: []v1.EndpointSubset{
							{
								Addresses: []v1.EndpointAddress{
									{
										IP:       "2.3.4.5",
										NodeName: strptr("iris1"),
									},
									{
										IP:       "2.3.4.15",
										NodeName: strptr("iris2"),
									},
								},
							},
						},
					},
					Type: k8s.Eps,
				},
				"10.20.30.2": {
					EpVal: &v1.Endpoints{
						Subsets: []v1.EndpointSubset{
							{
								Addresses: []v1.EndpointAddress{
									{
										IP:       "2.3.4.25",
										NodeName: strptr("iris1"),
									},
								},
								NotReadyAddresses: []v1.EndpointAddress{
									{
										IP:       "2.3.4.35",
										NodeName: strptr("iris2"),
									},
								},
							},
						},
					},
					Type: k8s.Eps,
				},
			},
			c1ExpectedResult: map[string]string{
				"10.20.30.1": "notOwner",
				"10.20.30.2": "",
			},
			c2ExpectedResult: map[string]string{
				"10.20.30.1": "",
				"10.20.30.2": "notOwner",
			},
		}, {
			desc: "Two services each with two endpoints across across two hosts, one service has an endpoint not ready on controller 1, the other service has an endpoint not ready on controller 2. Each controller should announce for the service with the ready endpoint on that controller",
			config: &config.Config{
				Pools: map[string]*config.Pool{
					"default": {
						Protocol: config.Layer2,
						CIDR:     []*net.IPNet{ipnet("10.20.30.0/24")},
					},
				},
			},
			svcs: []*v1.Service{
				{
					Spec: v1.ServiceSpec{
						Type:                  "LoadBalancer",
						ExternalTrafficPolicy: v1.ServiceExternalTrafficPolicyTypeLocal,
					},
					Status: statusAssigned("10.20.30.1"),
				},
				{
					Spec: v1.ServiceSpec{
						Type:                  "LoadBalancer",
						ExternalTrafficPolicy: v1.ServiceExternalTrafficPolicyTypeLocal,
					},
					Status: statusAssigned("10.20.30.2"),
				},
			},
			eps: map[string]k8s.EpsOrSlices{
				"10.20.30.1": {
					EpVal: &v1.Endpoints{
						Subsets: []v1.EndpointSubset{
							{
								Addresses: []v1.EndpointAddress{
									{
										IP:       "2.3.4.5",
										NodeName: strptr("iris2"),
									},
								},
								NotReadyAddresses: []v1.EndpointAddress{
									{
										IP:       "2.3.4.15",
										NodeName: strptr("iris1"),
									},
								},
							},
						},
					},
					Type: k8s.Eps,
				},
				"10.20.30.2": {
					EpVal: &v1.Endpoints{
						Subsets: []v1.EndpointSubset{
							{
								Addresses: []v1.EndpointAddress{
									{
										IP:       "2.3.4.25",
										NodeName: strptr("iris1"),
									},
								},
								NotReadyAddresses: []v1.EndpointAddress{
									{
										IP:       "2.3.4.35",
										NodeName: strptr("iris2"),
									},
								},
							},
						},
					},
					Type: k8s.Eps,
				},
			},
			c1ExpectedResult: map[string]string{
				"10.20.30.1": "notOwner",
				"10.20.30.2": "",
			},
			c2ExpectedResult: map[string]string{
				"10.20.30.1": "",
				"10.20.30.2": "notOwner",
			},
		}, {
			desc: "One service with three endpoints across across two hosts, controller 2 hosts two endpoints controller 2 should announce for the service",
			config: &config.Config{
				Pools: map[string]*config.Pool{
					"default": {
						Protocol: config.Layer2,
						CIDR:     []*net.IPNet{ipnet("10.20.30.0/24")},
					},
				},
			},
			svcs: []*v1.Service{
				{
					Spec: v1.ServiceSpec{
						Type:                  "LoadBalancer",
						ExternalTrafficPolicy: v1.ServiceExternalTrafficPolicyTypeLocal,
					},
					Status: statusAssigned("10.20.30.1"),
				},
			},
			eps: map[string]k8s.EpsOrSlices{
				"10.20.30.1": {
					EpVal: &v1.Endpoints{
						Subsets: []v1.EndpointSubset{
							{
								Addresses: []v1.EndpointAddress{
									{
										IP:       "2.3.4.5",
										NodeName: strptr("iris1"),
									},
									{
										IP:       "2.3.4.15",
										NodeName: strptr("iris2"),
									},
									{
										IP:       "2.3.4.25",
										NodeName: strptr("iris2"),
									},
								},
							},
						},
					},
					Type: k8s.Eps,
				},
			},
			c1ExpectedResult: map[string]string{
				"10.20.30.1": "notOwner",
			},
			c2ExpectedResult: map[string]string{
				"10.20.30.1": "",
			},
		}, {
			desc: "One service with three endpoints across across two hosts, controller 1 hosts two endpoints controller 2 should announce for the service",
			config: &config.Config{
				Pools: map[string]*config.Pool{
					"default": {
						Protocol: config.Layer2,
						CIDR:     []*net.IPNet{ipnet("10.20.30.0/24")},
					},
				},
			},
			svcs: []*v1.Service{
				{
					Spec: v1.ServiceSpec{
						Type:                  "LoadBalancer",
						ExternalTrafficPolicy: v1.ServiceExternalTrafficPolicyTypeLocal,
					},
					Status: statusAssigned("10.20.30.1"),
				},
			},
			eps: map[string]k8s.EpsOrSlices{
				"10.20.30.1": {
					EpVal: &v1.Endpoints{
						Subsets: []v1.EndpointSubset{
							{
								Addresses: []v1.EndpointAddress{
									{
										IP:       "2.3.4.5",
										NodeName: strptr("iris1"),
									},
									{
										IP:       "2.3.4.15",
										NodeName: strptr("iris2"),
									},
									{
										IP:       "2.3.4.25",
										NodeName: strptr("iris1"),
									},
								},
							},
						},
					},
					Type: k8s.Eps,
				},
			},
			c1ExpectedResult: map[string]string{
				"10.20.30.1": "notOwner",
			},
			c2ExpectedResult: map[string]string{
				"10.20.30.1": "",
			},
		}, {
			desc: "One service with three endpoints across across two hosts, controller 2 hosts two endpoints, one of which is not ready, controller 1 should announce for the service",
			config: &config.Config{
				Pools: map[string]*config.Pool{
					"default": {
						Protocol: config.Layer2,
						CIDR:     []*net.IPNet{ipnet("10.20.30.0/24")},
					},
				},
			},
			svcs: []*v1.Service{
				{
					Spec: v1.ServiceSpec{
						Type:                  "LoadBalancer",
						ExternalTrafficPolicy: v1.ServiceExternalTrafficPolicyTypeLocal,
					},
					Status: statusAssigned("10.20.30.1"),
				},
			},
			eps: map[string]k8s.EpsOrSlices{
				"10.20.30.1": {
					EpVal: &v1.Endpoints{
						Subsets: []v1.EndpointSubset{
							{
								Addresses: []v1.EndpointAddress{
									{
										IP:       "2.3.4.5",
										NodeName: strptr("iris1"),
									},
									{
										IP:       "2.3.4.15",
										NodeName: strptr("iris2"),
									},
								},
								NotReadyAddresses: []v1.EndpointAddress{
									{
										IP:       "2.3.4.25",
										NodeName: strptr("iris2"),
									},
								},
							},
						},
					},
					Type: k8s.Eps,
				},
			},
			c1ExpectedResult: map[string]string{
				"10.20.30.1": "notOwner",
			},
			c2ExpectedResult: map[string]string{
				"10.20.30.1": "",
			},
		}, {
			desc: "One service with three endpoints across across two hosts, controller 1 hosts two endpoints, one of which is not ready, controller 2 should announce for the service",
			config: &config.Config{
				Pools: map[string]*config.Pool{
					"default": {
						Protocol: config.Layer2,
						CIDR:     []*net.IPNet{ipnet("10.20.30.0/24")},
					},
				},
			},
			svcs: []*v1.Service{
				{
					Spec: v1.ServiceSpec{
						Type:                  "LoadBalancer",
						ExternalTrafficPolicy: v1.ServiceExternalTrafficPolicyTypeLocal,
					},
					Status: statusAssigned("10.20.30.1"),
				},
			},
			eps: map[string]k8s.EpsOrSlices{
				"10.20.30.1": {
					EpVal: &v1.Endpoints{
						Subsets: []v1.EndpointSubset{
							{
								Addresses: []v1.EndpointAddress{
									{
										IP:       "2.3.4.5",
										NodeName: strptr("iris1"),
									},
									{
										IP:       "2.3.4.15",
										NodeName: strptr("iris2"),
									},
								},
								NotReadyAddresses: []v1.EndpointAddress{
									{
										IP:       "2.3.4.25",
										NodeName: strptr("iris1"),
									},
								},
							},
						},
					},
					Type: k8s.Eps,
				},
			},
			c1ExpectedResult: map[string]string{
				"10.20.30.1": "notOwner",
			},
			c2ExpectedResult: map[string]string{
				"10.20.30.1": "",
			},
		}, {
			desc: "One service with three endpoints across across two hosts, controller 2 hosts two endpoints, both of which are not ready, controller 1 should announce for the service",
			config: &config.Config{
				Pools: map[string]*config.Pool{
					"default": {
						Protocol: config.Layer2,
						CIDR:     []*net.IPNet{ipnet("10.20.30.0/24")},
					},
				},
			},
			svcs: []*v1.Service{
				{
					Spec: v1.ServiceSpec{
						Type:                  "LoadBalancer",
						ExternalTrafficPolicy: v1.ServiceExternalTrafficPolicyTypeLocal,
					},
					Status: statusAssigned("10.20.30.1"),
				},
			},
			eps: map[string]k8s.EpsOrSlices{
				"10.20.30.1": {
					EpVal: &v1.Endpoints{
						Subsets: []v1.EndpointSubset{
							{
								Addresses: []v1.EndpointAddress{
									{
										IP:       "2.3.4.5",
										NodeName: strptr("iris1"),
									},
								},
								NotReadyAddresses: []v1.EndpointAddress{
									{
										IP:       "2.3.4.15",
										NodeName: strptr("iris2"),
									},
									{
										IP:       "2.3.4.25",
										NodeName: strptr("iris2"),
									},
								},
							},
						},
					},
					Type: k8s.Eps,
				},
			},
			c1ExpectedResult: map[string]string{
				"10.20.30.1": "",
			},
			c2ExpectedResult: map[string]string{
				"10.20.30.1": "notOwner",
			},
		},
	}

	l := log.NewNopLogger()
	for _, test := range tests {
		if test.config != nil {
			if c1.SetConfig(l, test.config) == k8s.SyncStateError {
				t.Errorf("%q: SetConfig failed", test.desc)
			}
			if c2.SetConfig(l, test.config) == k8s.SyncStateError {
				t.Errorf("%q: SetConfig failed", test.desc)
			}
		}

		for _, svc := range test.svcs {
			lbIP := net.ParseIP(svc.Status.LoadBalancer.Ingress[0].IP)
			lbIP_s := lbIP.String()
			pool := c1.config.Pools[poolFor(c1.config.Pools, lbIP)]
			response1 := c1.protocols[pool.Protocol].ShouldAnnounce(l, "balancer", lbIP, svc, test.eps[lbIP_s])
			response2 := c2.protocols[pool.Protocol].ShouldAnnounce(l, "balancer", lbIP, svc, test.eps[lbIP_s])
			if response1 != test.c1ExpectedResult[lbIP_s] {
				t.Errorf("%q: shouldAnnounce for controller 1 for service %s returned incorrect result, expected '%s', but received '%s'", test.desc, lbIP_s, test.c1ExpectedResult[lbIP_s], response1)
			}
			if response2 != test.c2ExpectedResult[lbIP_s] {
				t.Errorf("%q: shouldAnnounce for controller 2 for service %s returned incorrect result, expected '%s', but received '%s'", test.desc, lbIP_s, test.c2ExpectedResult[lbIP_s], response2)
			}
		}
	}
}

func TestShouldAnnounceEPSlices(t *testing.T) {
	fakeSL := &fakeSpeakerList{
		speakers: map[string]bool{
			"iris1": true,
			"iris2": true,
		},
	}
	c1, err := newController(controllerConfig{
		MyNode: "iris1",
		Logger: log.NewLogfmtLogger(log.NewSyncWriter(os.Stderr)),
		SList:  fakeSL,
	})
	if err != nil {
		t.Fatalf("creating controller: %s", err)
	}
	c1.client = &testK8S{t: t}

	c2, err := newController(controllerConfig{
		MyNode: "iris2",
		Logger: log.NewLogfmtLogger(log.NewSyncWriter(os.Stderr)),
		SList:  fakeSL,
	})
	if err != nil {
		t.Fatalf("creating controller: %s", err)
	}
	c2.client = &testK8S{t: t}

	tests := []struct {
		desc string

		balancer string
		config   *config.Config
		svcs     []*v1.Service
		eps      map[string]k8s.EpsOrSlices

		c1ExpectedResult map[string]string
		c2ExpectedResult map[string]string
	}{
		{
			desc:     "One service, two endpoints, one host, controller 1 should announce",
			balancer: "test1",
			config: &config.Config{
				Pools: map[string]*config.Pool{
					"default": {
						Protocol: config.Layer2,
						CIDR:     []*net.IPNet{ipnet("10.20.30.0/24")},
					},
				},
			},
			svcs: []*v1.Service{
				{
					Spec: v1.ServiceSpec{
						Type:                  "LoadBalancer",
						ExternalTrafficPolicy: v1.ServiceExternalTrafficPolicyTypeLocal,
					},
					Status: statusAssigned("10.20.30.1"),
				},
			},
			eps: map[string]k8s.EpsOrSlices{
				"10.20.30.1": {
					SlicesVal: []*discovery.EndpointSlice{
						{
							Endpoints: []discovery.Endpoint{
								{
									Addresses: []string{
										"2.3.4.5",
									},
									Topology: map[string]string{
										"kubernetes.io/hostname": "iris1",
									},
									Conditions: discovery.EndpointConditions{
										Ready: boolPtr(true),
									},
								},
								{
									Addresses: []string{
										"2.3.4.15",
									},
									Topology: map[string]string{
										"kubernetes.io/hostname": "iris1",
									},
									Conditions: discovery.EndpointConditions{
										Ready: boolPtr(true),
									},
								},
							},
						},
					},
					Type: k8s.Slices,
				},
			},
			c1ExpectedResult: map[string]string{
				"10.20.30.1": "",
			},
			c2ExpectedResult: map[string]string{
				"10.20.30.1": "notOwner",
			},
		}, {
			desc: "One service, two endpoints, one host, neither endpoint is ready, no controller should announce",
			config: &config.Config{
				Pools: map[string]*config.Pool{
					"default": {
						Protocol: config.Layer2,
						CIDR:     []*net.IPNet{ipnet("10.20.30.0/24")},
					},
				},
			},
			svcs: []*v1.Service{
				{
					Spec: v1.ServiceSpec{
						Type:                  "LoadBalancer",
						ExternalTrafficPolicy: v1.ServiceExternalTrafficPolicyTypeLocal,
					},
					Status: statusAssigned("10.20.30.1"),
				},
			},
			eps: map[string]k8s.EpsOrSlices{
				"10.20.30.1": {
					SlicesVal: []*discovery.EndpointSlice{
						{
							Endpoints: []discovery.Endpoint{
								{
									Addresses: []string{
										"2.3.4.5",
									},
									Topology: map[string]string{
										"kubernetes.io/hostname": "iris1",
									},
									Conditions: discovery.EndpointConditions{
										Ready: boolPtr(false),
									},
								},
								{
									Addresses: []string{
										"2.3.4.15",
									},
									Topology: map[string]string{
										"kubernetes.io/hostname": "iris1",
									},
									Conditions: discovery.EndpointConditions{
										Ready: boolPtr(false),
									},
								},
							},
						},
					},
					Type: k8s.Slices,
				},
			},
			c1ExpectedResult: map[string]string{
				"10.20.30.1": "notOwner",
			},
			c2ExpectedResult: map[string]string{
				"10.20.30.1": "notOwner",
			},
		}, {
			desc: "One service, two endpoints across two hosts, controller2 should announce",
			config: &config.Config{
				Pools: map[string]*config.Pool{
					"default": {
						Protocol: config.Layer2,
						CIDR:     []*net.IPNet{ipnet("10.20.30.0/24")},
					},
				},
			},
			svcs: []*v1.Service{
				{
					Spec: v1.ServiceSpec{
						Type:                  "LoadBalancer",
						ExternalTrafficPolicy: v1.ServiceExternalTrafficPolicyTypeLocal,
					},
					Status: statusAssigned("10.20.30.1"),
				},
			},
			eps: map[string]k8s.EpsOrSlices{
				"10.20.30.1": {
					SlicesVal: []*discovery.EndpointSlice{
						{
							Endpoints: []discovery.Endpoint{
								{
									Addresses: []string{
										"2.3.4.5",
									},
									Topology: map[string]string{
										"kubernetes.io/hostname": "iris1",
									},
									Conditions: discovery.EndpointConditions{
										Ready: boolPtr(true),
									},
								},
								{
									Addresses: []string{
										"2.3.4.15",
									},
									Topology: map[string]string{
										"kubernetes.io/hostname": "iris2",
									},
									Conditions: discovery.EndpointConditions{
										Ready: boolPtr(true),
									},
								},
							},
						},
					},
					Type: k8s.Slices,
				},
			},
			c1ExpectedResult: map[string]string{
				"10.20.30.1": "notOwner",
			},
			c2ExpectedResult: map[string]string{
				"10.20.30.1": "",
			},
		}, {
			desc: "One service, two endpoints across two hosts, neither endpoint is ready, no controllers should announce",
			config: &config.Config{
				Pools: map[string]*config.Pool{
					"default": {
						Protocol: config.Layer2,
						CIDR:     []*net.IPNet{ipnet("10.20.30.0/24")},
					},
				},
			},
			svcs: []*v1.Service{
				{
					Spec: v1.ServiceSpec{
						Type:                  "LoadBalancer",
						ExternalTrafficPolicy: v1.ServiceExternalTrafficPolicyTypeLocal,
					},
					Status: statusAssigned("10.20.30.1"),
				},
			},
			eps: map[string]k8s.EpsOrSlices{
				"10.20.30.1": {
					SlicesVal: []*discovery.EndpointSlice{
						{
							Endpoints: []discovery.Endpoint{
								{
									Addresses: []string{
										"2.3.4.5",
									},
									Topology: map[string]string{
										"kubernetes.io/hostname": "iris1",
									},
									Conditions: discovery.EndpointConditions{
										Ready: boolPtr(false),
									},
								},
								{
									Addresses: []string{
										"2.3.4.15",
									},
									Topology: map[string]string{
										"kubernetes.io/hostname": "iris2",
									},
									Conditions: discovery.EndpointConditions{
										Ready: boolPtr(false),
									},
								},
							},
						},
					},
					Type: k8s.Slices,
				},
			},
			c1ExpectedResult: map[string]string{
				"10.20.30.1": "notOwner",
			},
			c2ExpectedResult: map[string]string{
				"10.20.30.1": "notOwner",
			},
		}, {
			desc: "One service, two endpoints across two hosts, controller 2 is not ready, controller 1 should announce",
			config: &config.Config{
				Pools: map[string]*config.Pool{
					"default": {
						Protocol: config.Layer2,
						CIDR:     []*net.IPNet{ipnet("10.20.30.0/24")},
					},
				},
			},
			svcs: []*v1.Service{
				{
					Spec: v1.ServiceSpec{
						Type:                  "LoadBalancer",
						ExternalTrafficPolicy: v1.ServiceExternalTrafficPolicyTypeLocal,
					},
					Status: statusAssigned("10.20.30.1"),
				},
			},
			eps: map[string]k8s.EpsOrSlices{
				"10.20.30.1": {
					SlicesVal: []*discovery.EndpointSlice{
						{
							Endpoints: []discovery.Endpoint{
								{
									Addresses: []string{
										"2.3.4.5",
									},
									Topology: map[string]string{
										"kubernetes.io/hostname": "iris1",
									},
									Conditions: discovery.EndpointConditions{
										Ready: boolPtr(true),
									},
								},
								{
									Addresses: []string{
										"2.3.4.15",
									},
									Topology: map[string]string{
										"kubernetes.io/hostname": "iris2",
									},
									Conditions: discovery.EndpointConditions{
										Ready: boolPtr(false),
									},
								},
							},
						},
					},
					Type: k8s.Slices,
				},
			},
			c1ExpectedResult: map[string]string{
				"10.20.30.1": "",
			},
			c2ExpectedResult: map[string]string{
				"10.20.30.1": "notOwner",
			},
		}, {
			desc: "Two services each with two endpoints across across two hosts, each controller should announce one service",
			config: &config.Config{
				Pools: map[string]*config.Pool{
					"default": {
						Protocol: config.Layer2,
						CIDR:     []*net.IPNet{ipnet("10.20.30.0/24")},
					},
				},
			},
			svcs: []*v1.Service{
				{
					Spec: v1.ServiceSpec{
						Type:                  "LoadBalancer",
						ExternalTrafficPolicy: v1.ServiceExternalTrafficPolicyTypeLocal,
					},
					Status: statusAssigned("10.20.30.1"),
				},
				{
					Spec: v1.ServiceSpec{
						Type:                  "LoadBalancer",
						ExternalTrafficPolicy: v1.ServiceExternalTrafficPolicyTypeLocal,
					},
					Status: statusAssigned("10.20.30.2"),
				},
			},
			eps: map[string]k8s.EpsOrSlices{
				"10.20.30.1": {
					SlicesVal: []*discovery.EndpointSlice{
						{
							Endpoints: []discovery.Endpoint{
								{
									Addresses: []string{
										"2.3.4.5",
									},
									Topology: map[string]string{
										"kubernetes.io/hostname": "iris1",
									},
									Conditions: discovery.EndpointConditions{
										Ready: boolPtr(true),
									},
								},
								{
									Addresses: []string{
										"2.3.4.15",
									},
									Topology: map[string]string{
										"kubernetes.io/hostname": "iris2",
									},
									Conditions: discovery.EndpointConditions{
										Ready: boolPtr(true),
									},
								},
							},
						},
					},
					Type: k8s.Slices,
				},
				"10.20.30.2": {
					SlicesVal: []*discovery.EndpointSlice{
						{
							Endpoints: []discovery.Endpoint{
								{
									Addresses: []string{
										"2.3.4.25",
									},
									Topology: map[string]string{
										"kubernetes.io/hostname": "iris1",
									},
									Conditions: discovery.EndpointConditions{
										Ready: boolPtr(true),
									},
								},
								{
									Addresses: []string{
										"2.3.4.35",
									},
									Topology: map[string]string{
										"kubernetes.io/hostname": "iris2",
									},
									Conditions: discovery.EndpointConditions{
										Ready: boolPtr(true),
									},
								},
							},
						},
					},
					Type: k8s.Slices,
				},
			},
			c1ExpectedResult: map[string]string{
				"10.20.30.1": "notOwner",
				"10.20.30.2": "",
			},
			c2ExpectedResult: map[string]string{
				"10.20.30.1": "",
				"10.20.30.2": "notOwner",
			},
		}, {
			desc: "Two services each with two endpoints across across two hosts, one service has an endpoint not ready on controller 2, each controller should announce one service",
			config: &config.Config{
				Pools: map[string]*config.Pool{
					"default": {
						Protocol: config.Layer2,
						CIDR:     []*net.IPNet{ipnet("10.20.30.0/24")},
					},
				},
			},
			svcs: []*v1.Service{
				{
					Spec: v1.ServiceSpec{
						Type:                  "LoadBalancer",
						ExternalTrafficPolicy: v1.ServiceExternalTrafficPolicyTypeLocal,
					},
					Status: statusAssigned("10.20.30.1"),
				},
				{
					Spec: v1.ServiceSpec{
						Type:                  "LoadBalancer",
						ExternalTrafficPolicy: v1.ServiceExternalTrafficPolicyTypeLocal,
					},
					Status: statusAssigned("10.20.30.2"),
				},
			},
			eps: map[string]k8s.EpsOrSlices{
				"10.20.30.1": {
					SlicesVal: []*discovery.EndpointSlice{
						{
							Endpoints: []discovery.Endpoint{
								{
									Addresses: []string{
										"2.3.4.5",
									},
									Topology: map[string]string{
										"kubernetes.io/hostname": "iris1",
									},
									Conditions: discovery.EndpointConditions{
										Ready: boolPtr(true),
									},
								},
								{
									Addresses: []string{
										"2.3.4.15",
									},
									Topology: map[string]string{
										"kubernetes.io/hostname": "iris2",
									},
									Conditions: discovery.EndpointConditions{
										Ready: boolPtr(true),
									},
								},
							},
						},
					},
					Type: k8s.Slices,
				},
				"10.20.30.2": {
					SlicesVal: []*discovery.EndpointSlice{
						{
							Endpoints: []discovery.Endpoint{
								{
									Addresses: []string{
										"2.3.4.25",
									},
									Topology: map[string]string{
										"kubernetes.io/hostname": "iris1",
									},
									Conditions: discovery.EndpointConditions{
										Ready: boolPtr(true),
									},
								},
								{
									Addresses: []string{
										"2.3.4.35",
									},
									Topology: map[string]string{
										"kubernetes.io/hostname": "iris2",
									},
									Conditions: discovery.EndpointConditions{
										Ready: boolPtr(false),
									},
								},
							},
						},
					},
					Type: k8s.Slices,
				},
			},
			c1ExpectedResult: map[string]string{
				"10.20.30.1": "notOwner",
				"10.20.30.2": "",
			},
			c2ExpectedResult: map[string]string{
				"10.20.30.1": "",
				"10.20.30.2": "notOwner",
			},
		}, {
			desc: "Two services each with two endpoints across across two hosts, one service has an endpoint not ready on controller 1, the other service has an endpoint not ready on controller 2. Each controller should announce for the service with the ready endpoint on that controller",
			config: &config.Config{
				Pools: map[string]*config.Pool{
					"default": {
						Protocol: config.Layer2,
						CIDR:     []*net.IPNet{ipnet("10.20.30.0/24")},
					},
				},
			},
			svcs: []*v1.Service{
				{
					Spec: v1.ServiceSpec{
						Type:                  "LoadBalancer",
						ExternalTrafficPolicy: v1.ServiceExternalTrafficPolicyTypeLocal,
					},
					Status: statusAssigned("10.20.30.1"),
				},
				{
					Spec: v1.ServiceSpec{
						Type:                  "LoadBalancer",
						ExternalTrafficPolicy: v1.ServiceExternalTrafficPolicyTypeLocal,
					},
					Status: statusAssigned("10.20.30.2"),
				},
			},
			eps: map[string]k8s.EpsOrSlices{
				"10.20.30.1": {
					SlicesVal: []*discovery.EndpointSlice{
						{
							Endpoints: []discovery.Endpoint{
								{
									Addresses: []string{
										"2.3.4.5",
									},
									Topology: map[string]string{
										"kubernetes.io/hostname": "iris2",
									},
									Conditions: discovery.EndpointConditions{
										Ready: boolPtr(true),
									},
								},
								{
									Addresses: []string{
										"2.3.4.15",
									},
									Topology: map[string]string{
										"kubernetes.io/hostname": "iris1",
									},
									Conditions: discovery.EndpointConditions{
										Ready: boolPtr(false),
									},
								},
							},
						},
					},
					Type: k8s.Slices,
				},
				"10.20.30.2": {
					SlicesVal: []*discovery.EndpointSlice{
						{
							Endpoints: []discovery.Endpoint{
								{
									Addresses: []string{
										"2.3.4.25",
									},
									Topology: map[string]string{
										"kubernetes.io/hostname": "iris1",
									},
									Conditions: discovery.EndpointConditions{
										Ready: boolPtr(true),
									},
								},
								{
									Addresses: []string{
										"2.3.4.35",
									},
									Topology: map[string]string{
										"kubernetes.io/hostname": "iris2",
									},
									Conditions: discovery.EndpointConditions{
										Ready: boolPtr(false),
									},
								},
							},
						},
					},
					Type: k8s.Slices,
				},
			},
			c1ExpectedResult: map[string]string{
				"10.20.30.1": "notOwner",
				"10.20.30.2": "",
			},
			c2ExpectedResult: map[string]string{
				"10.20.30.1": "",
				"10.20.30.2": "notOwner",
			},
		}, {
			desc: "One service with three endpoints across across two hosts, controller 2 hosts two endpoints controller 2 should announce for the service",
			config: &config.Config{
				Pools: map[string]*config.Pool{
					"default": {
						Protocol: config.Layer2,
						CIDR:     []*net.IPNet{ipnet("10.20.30.0/24")},
					},
				},
			},
			svcs: []*v1.Service{
				{
					Spec: v1.ServiceSpec{
						Type:                  "LoadBalancer",
						ExternalTrafficPolicy: v1.ServiceExternalTrafficPolicyTypeLocal,
					},
					Status: statusAssigned("10.20.30.1"),
				},
			},
			eps: map[string]k8s.EpsOrSlices{
				"10.20.30.1": {
					SlicesVal: []*discovery.EndpointSlice{
						{
							Endpoints: []discovery.Endpoint{
								{
									Addresses: []string{
										"2.3.4.5",
									},
									Topology: map[string]string{
										"kubernetes.io/hostname": "iris1",
									},
									Conditions: discovery.EndpointConditions{
										Ready: boolPtr(true),
									},
								},
								{
									Addresses: []string{
										"2.3.4.15",
									},
									Topology: map[string]string{
										"kubernetes.io/hostname": "iris2",
									},
									Conditions: discovery.EndpointConditions{
										Ready: boolPtr(true),
									},
								},
							},
						},
						{
							Endpoints: []discovery.Endpoint{
								{
									Addresses: []string{
										"2.3.4.25",
									},
									Topology: map[string]string{
										"kubernetes.io/hostname": "iris2",
									},
									Conditions: discovery.EndpointConditions{
										Ready: boolPtr(true),
									},
								},
							},
						},
					},
					Type: k8s.Slices,
				},
			},
			c1ExpectedResult: map[string]string{
				"10.20.30.1": "notOwner",
			},
			c2ExpectedResult: map[string]string{
				"10.20.30.1": "",
			},
		}, {
			desc: "One service with three endpoints across across two hosts, controller 1 hosts two endpoints controller 2 should announce for the service",
			config: &config.Config{
				Pools: map[string]*config.Pool{
					"default": {
						Protocol: config.Layer2,
						CIDR:     []*net.IPNet{ipnet("10.20.30.0/24")},
					},
				},
			},
			svcs: []*v1.Service{
				{
					Spec: v1.ServiceSpec{
						Type:                  "LoadBalancer",
						ExternalTrafficPolicy: v1.ServiceExternalTrafficPolicyTypeLocal,
					},
					Status: statusAssigned("10.20.30.1"),
				},
			},
			eps: map[string]k8s.EpsOrSlices{
				"10.20.30.1": {
					SlicesVal: []*discovery.EndpointSlice{
						{
							Endpoints: []discovery.Endpoint{
								{
									Addresses: []string{
										"2.3.4.5",
									},
									Topology: map[string]string{
										"kubernetes.io/hostname": "iris1",
									},
									Conditions: discovery.EndpointConditions{
										Ready: boolPtr(true),
									},
								},
								{
									Addresses: []string{
										"2.3.4.15",
									},
									Topology: map[string]string{
										"kubernetes.io/hostname": "iris2",
									},
									Conditions: discovery.EndpointConditions{
										Ready: boolPtr(true),
									},
								},
								{
									Addresses: []string{
										"2.3.4.25",
									},
									Topology: map[string]string{
										"kubernetes.io/hostname": "iris1",
									},
									Conditions: discovery.EndpointConditions{
										Ready: boolPtr(true),
									},
								},
							},
						},
					},
					Type: k8s.Slices,
				},
			},
			c1ExpectedResult: map[string]string{
				"10.20.30.1": "notOwner",
			},
			c2ExpectedResult: map[string]string{
				"10.20.30.1": "",
			},
		}, {
			desc: "One service with three endpoints across across two hosts, controller 2 hosts two endpoints, one of which is not ready, controller 2 should announce for the service",
			config: &config.Config{
				Pools: map[string]*config.Pool{
					"default": {
						Protocol: config.Layer2,
						CIDR:     []*net.IPNet{ipnet("10.20.30.0/24")},
					},
				},
			},
			svcs: []*v1.Service{
				{
					Spec: v1.ServiceSpec{
						Type:                  "LoadBalancer",
						ExternalTrafficPolicy: v1.ServiceExternalTrafficPolicyTypeLocal,
					},
					Status: statusAssigned("10.20.30.1"),
				},
			},
			eps: map[string]k8s.EpsOrSlices{
				"10.20.30.1": {
					SlicesVal: []*discovery.EndpointSlice{
						{
							Endpoints: []discovery.Endpoint{
								{
									Addresses: []string{
										"2.3.4.5",
									},
									Topology: map[string]string{
										"kubernetes.io/hostname": "iris1",
									},
									Conditions: discovery.EndpointConditions{
										Ready: boolPtr(true),
									},
								},
								{
									Addresses: []string{
										"2.3.4.15",
									},
									Topology: map[string]string{
										"kubernetes.io/hostname": "iris2",
									},
									Conditions: discovery.EndpointConditions{
										Ready: boolPtr(true),
									},
								},
								{
									Addresses: []string{
										"2.3.4.25",
									},
									Topology: map[string]string{
										"kubernetes.io/hostname": "iris2",
									},
									Conditions: discovery.EndpointConditions{
										Ready: boolPtr(true),
									},
								},
							},
						},
					},
					Type: k8s.Slices,
				},
			},
			c1ExpectedResult: map[string]string{
				"10.20.30.1": "notOwner",
			},
			c2ExpectedResult: map[string]string{
				"10.20.30.1": "",
			},
		}, {
			desc: "One service with three endpoints across across two hosts, controller 1 hosts two endpoints, one of which is not ready, controller 1 should announce for the service",
			config: &config.Config{
				Pools: map[string]*config.Pool{
					"default": {
						Protocol: config.Layer2,
						CIDR:     []*net.IPNet{ipnet("10.20.30.0/24")},
					},
				},
			},
			svcs: []*v1.Service{
				{
					Spec: v1.ServiceSpec{
						Type:                  "LoadBalancer",
						ExternalTrafficPolicy: v1.ServiceExternalTrafficPolicyTypeLocal,
					},
					Status: statusAssigned("10.20.30.1"),
				},
			},
			eps: map[string]k8s.EpsOrSlices{
				"10.20.30.1": {
					SlicesVal: []*discovery.EndpointSlice{
						{
							Endpoints: []discovery.Endpoint{
								{
									Addresses: []string{
										"2.3.4.5",
									},
									Topology: map[string]string{
										"kubernetes.io/hostname": "iris1",
									},
									Conditions: discovery.EndpointConditions{
										Ready: boolPtr(true),
									},
								},
								{
									Addresses: []string{
										"2.3.4.15",
									},
									Topology: map[string]string{
										"kubernetes.io/hostname": "iris2",
									},
									Conditions: discovery.EndpointConditions{
										Ready: boolPtr(true),
									},
								},
								{
									Addresses: []string{
										"2.3.4.25",
									},
									Topology: map[string]string{
										"kubernetes.io/hostname": "iris1",
									},
									Conditions: discovery.EndpointConditions{
										Ready: boolPtr(false),
									},
								},
							},
						},
					},
					Type: k8s.Slices,
				},
			},
			c1ExpectedResult: map[string]string{
				"10.20.30.1": "notOwner",
			},
			c2ExpectedResult: map[string]string{
				"10.20.30.1": "",
			},
		}, {
			desc: "One service with three endpoints across across two hosts, controller 2 hosts two endpoints, both of which are not ready, controller 1 should announce for the service",
			config: &config.Config{
				Pools: map[string]*config.Pool{
					"default": {
						Protocol: config.Layer2,
						CIDR:     []*net.IPNet{ipnet("10.20.30.0/24")},
					},
				},
			},
			svcs: []*v1.Service{
				{
					Spec: v1.ServiceSpec{
						Type:                  "LoadBalancer",
						ExternalTrafficPolicy: v1.ServiceExternalTrafficPolicyTypeLocal,
					},
					Status: statusAssigned("10.20.30.1"),
				},
			},
			eps: map[string]k8s.EpsOrSlices{
				"10.20.30.1": {
					SlicesVal: []*discovery.EndpointSlice{
						{
							Endpoints: []discovery.Endpoint{
								{
									Addresses: []string{
										"2.3.4.5",
									},
									Topology: map[string]string{
										"kubernetes.io/hostname": "iris1",
									},
									Conditions: discovery.EndpointConditions{
										Ready: boolPtr(true),
									},
								},
								{
									Addresses: []string{
										"2.3.4.15",
									},
									Topology: map[string]string{
										"kubernetes.io/hostname": "iris2",
									},
									Conditions: discovery.EndpointConditions{
										Ready: boolPtr(false),
									},
								},
								{
									Addresses: []string{
										"2.3.4.25",
									},
									Topology: map[string]string{
										"kubernetes.io/hostname": "iris2",
									},
									Conditions: discovery.EndpointConditions{
										Ready: boolPtr(false),
									},
								},
							},
						},
					},
					Type: k8s.Slices,
				},
			},
			c1ExpectedResult: map[string]string{
				"10.20.30.1": "",
			},
			c2ExpectedResult: map[string]string{
				"10.20.30.1": "notOwner",
			},
		},
	}

	l := log.NewNopLogger()
	for _, test := range tests {
		if test.config != nil {
			if c1.SetConfig(l, test.config) == k8s.SyncStateError {
				t.Errorf("%q: SetConfig failed", test.desc)
			}
			if c2.SetConfig(l, test.config) == k8s.SyncStateError {
				t.Errorf("%q: SetConfig failed", test.desc)
			}
		}

		for _, svc := range test.svcs {
			lbIP := net.ParseIP(svc.Status.LoadBalancer.Ingress[0].IP)
			lbIP_s := lbIP.String()
			pool := c1.config.Pools[poolFor(c1.config.Pools, lbIP)]
			response1 := c1.protocols[pool.Protocol].ShouldAnnounce(l, "test1", lbIP, svc, test.eps[lbIP_s])
			response2 := c2.protocols[pool.Protocol].ShouldAnnounce(l, "test1", lbIP, svc, test.eps[lbIP_s])
			if response1 != test.c1ExpectedResult[lbIP_s] {
				t.Errorf("%q: shouldAnnounce for controller 1 for service %s returned incorrect result, expected '%s', but received '%s'", test.desc, lbIP_s, test.c1ExpectedResult[lbIP_s], response1)
			}
			if response2 != test.c2ExpectedResult[lbIP_s] {
				t.Errorf("%q: shouldAnnounce for controller 2 for service %s returned incorrect result, expected '%s', but received '%s'", test.desc, lbIP_s, test.c2ExpectedResult[lbIP_s], response2)
			}
		}
	}
}

func boolPtr(b bool) *bool {
	return &b
}

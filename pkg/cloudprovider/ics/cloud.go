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

package ics

import (
	"io"
	"runtime"

	v1 "k8s.io/api/core/v1"
	"k8s.io/klog"

	cloudprovider "k8s.io/cloud-provider"

	"github.com/inspur-ics/cloud-provider-ics/pkg/cloudprovider/ics/server"
	cm "github.com/inspur-ics/cloud-provider-ics/pkg/common/connectionmanager"
	k8s "github.com/inspur-ics/cloud-provider-ics/pkg/common/kubernetes"
)

const (
	// ProviderName is the name of the cloud provider registered with
	// Kubernetes.
	ProviderName string = "ics"
	// ClientName is the user agent passed into the controller client builder.
	ClientName string = "ics-cloud-controller-manager"
)

func init() {
	cloudprovider.RegisterCloudProvider(ProviderName, func(config io.Reader) (cloudprovider.Interface, error) {
		cpiConfig, err := ReadCPIConfig(config)
		if err != nil {
			return nil, err
		}
		return newICS(cpiConfig, true)
	})
}

// Creates new Controller node interface and returns
func newICS(cfg *CPIConfig, finalize ...bool) (*ICS, error) {
	vs, err := buildICSFromConfig(cfg)
	if err != nil {
		return nil, err
	}
	if len(finalize) == 1 && finalize[0] {
		// optional for use in tests
		runtime.SetFinalizer(vs, logout)
	}
	return vs, nil
}

// Initialize initializes the cloud provider.
func (vs *ICS) Initialize(clientBuilder cloudprovider.ControllerClientBuilder, stop <-chan struct{}) {
	client, err := clientBuilder.Client(ClientName)
	if err == nil {
		klog.V(1).Info("Kubernetes Client Init Succeeded")

		vs.informMgr = k8s.NewInformer(client, true)

		connMgr := cm.NewConnectionManager(&vs.cfg.Config, vs.informMgr, client)
		vs.connectionManager = connMgr
		vs.nodeManager.connectionManager = connMgr

		vs.informMgr.AddNodeListener(vs.nodeAdded, vs.nodeDeleted, nil)

		vs.informMgr.Listen()

		//if running secrets, init them
		connMgr.InitializeSecretLister()

		if !vs.cfg.Global.APIDisable {
			klog.V(1).Info("Starting the API Server")
			vs.server.Start()
		} else {
			klog.V(1).Info("API Server is disabled")
		}
	} else {
		klog.Errorf("Kubernetes Client Init Failed: %v", err)
	}
}

// LoadBalancer returns a balancer interface. Also returns true if the
// interface is supported, false otherwise.
func (vs *ICS) LoadBalancer() (cloudprovider.LoadBalancer, bool) {
	klog.Warning("The ics cloud provider does not support load balancers")
	return nil, false
}

// Instances returns an instances interface. Also returns true if the
// interface is supported, false otherwise.
func (vs *ICS) Instances() (cloudprovider.Instances, bool) {
	klog.V(6).Info("Calling the Instances interface on ics cloud provider")
	return vs.instances, true
}

// Zones returns a zones interface. Also returns true if the interface
// is supported, false otherwise.
func (vs *ICS) Zones() (cloudprovider.Zones, bool) {
	klog.V(6).Info("Calling the Zones interface on ics cloud provider")
	return vs.zones, true
}

// Clusters returns a clusters interface.  Also returns true if the interface
// is supported, false otherwise.
func (vs *ICS) Clusters() (cloudprovider.Clusters, bool) {
	klog.Warning("The ics cloud provider does not support clusters")
	return nil, false
}

// Routes returns a routes interface along with whether the interface
// is supported.
func (vs *ICS) Routes() (cloudprovider.Routes, bool) {
	klog.Warning("The ics cloud provider does not support routes")
	return nil, false
}

// ProviderName returns the cloud provider ID.
func (vs *ICS) ProviderName() string {
	return ProviderName
}

// ScrubDNS is not implemented.
// TODO(akutz) Add better documentation for this function.
func (vs *ICS) ScrubDNS(nameservers, searches []string) (nsOut, srchOut []string) {
	return nil, nil
}

// HasClusterID returns true if a ClusterID is required and set/
func (vs *ICS) HasClusterID() bool {
	return true
}

// Initializes ics from ics CloudProvider Configuration
func buildICSFromConfig(cfg *CPIConfig) (*ICS, error) {
	nm := newNodeManager(cfg, nil)

	vs := ICS{
		cfg:         cfg,
		nodeManager: nm,
		instances:   newInstances(nm),
		zones:       newZones(nm, cfg.Labels.Zone, cfg.Labels.Region),
		server:      server.NewServer(cfg.Global.APIBinding, nm),
	}
	return &vs, nil
}

func logout(vs *ICS) {
	vs.connectionManager.Logout()
}

// Notification handler when node is added into k8s cluster.
func (vs *ICS) nodeAdded(obj interface{}) {
	node, ok := obj.(*v1.Node)
	if node == nil || !ok {
		klog.Warningf("nodeAdded: unrecognized object %+v", obj)
		return
	}

	vs.nodeManager.RegisterNode(node)
}

// Notification handler when node is removed from k8s cluster.
func (vs *ICS) nodeDeleted(obj interface{}) {
	node, ok := obj.(*v1.Node)
	if node == nil || !ok {
		klog.Warningf("nodeDeleted: unrecognized object %+v", obj)
		return
	}

	vs.nodeManager.UnregisterNode(node)
}

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

package connectionmanager

import (
	"context"
	"sort"
	"strings"
	"time"

	"k8s.io/klog"

	icslib "github.com/inspur-ics/cloud-provider-ics/pkg/common/icslib"
)

// ListAllVCandDCPairs returns all VC/DC pairs
func (cm *ConnectionManager) ListAllVCandDCPairs(ctx context.Context) ([]*ListDiscoveryInfo, error) {
	klog.V(4).Infof("ListAllVCandDCPairs called")

	listOfVCAndDCPairs := make([]*ListDiscoveryInfo, 0)

//ics
	for _, vsi := range cm.IcsInstanceMap {
		var datacenterObjs []*icslib.Datacenter
//ics
		var err error
		for i := 0; i < NumConnectionAttempts; i++ {
			err = cm.Connect(ctx, vsi)
			if err == nil {
				break
			}
			time.Sleep(time.Duration(RetryAttemptDelaySecs) * time.Second)
		}

		if err != nil {
			klog.Error("Connect error vc:", err)
			continue
		}
//ics
		if vsi.Cfg.Datacenters == "" {
			datacenterObjs, err = icslib.GetAllDatacenter(ctx, vsi.Conn)
//ics
			if err != nil {
				klog.Error("GetAllDatacenter error dc:", err)
				continue
			}
		} else {
			datacenters := strings.Split(vsi.Cfg.Datacenters, ",")
			for _, dc := range datacenters {
				dc = strings.TrimSpace(dc)
				if dc == "" {
					continue
				}
//ics
				datacenterObj, err := icslib.GetDatacenter(ctx, vsi.Conn, dc)
//ics
				if err != nil {
					klog.Error("GetDatacenter error dc:", err)
					continue
				}
				datacenterObjs = append(datacenterObjs, datacenterObj)
			}
		}

		for _, datacenterObj := range datacenterObjs {
			listOfVCAndDCPairs = append(listOfVCAndDCPairs, &ListDiscoveryInfo{
				TenantRef:  vsi.Cfg.TenantRef,
				VcServer:   vsi.Cfg.VCenterIP,
				DataCenter: datacenterObj,
			})
		}
	}

	sort.Slice(listOfVCAndDCPairs, func(i, j int) bool {
		return strings.Compare(listOfVCAndDCPairs[i].VcServer, listOfVCAndDCPairs[j].VcServer) > 0 &&
			strings.Compare(listOfVCAndDCPairs[i].DataCenter.Name(), listOfVCAndDCPairs[j].DataCenter.Name()) > 0
	})

	return listOfVCAndDCPairs, nil
}

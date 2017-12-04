// Copyright (c) 2017 Cisco and/or its affiliates.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at:
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package l2plugin

import (
	l2ba "github.com/ligato/vpp-agent/plugins/defaultplugins/l2plugin/bin_api/l2"
	l2na "github.com/ligato/vpp-agent/plugins/defaultplugins/l2plugin/model/l2"
	"testing"
	"github.com/ligato/cn-infra/logging/measure"
	"github.com/ligato/cn-infra/logging"
)


var testDataInBD = &l2na.BridgeDomains_BridgeDomain{
	Interfaces: []*l2na.BridgeDomains_BridgeDomain_Interfaces{
		{Name:"dummyName", BridgedVirtualInterface:true, SplitHorizonGroup:2104},
	}
}

func TestConfigureBridgeDomain(t *testing.T) {
	bdConfigurator := BDConfigurator{
	}
	bdConfigurator.ConfigureBridgeDomain(testDataInBD);
}

// ConfigureBridgeDomain for newly created bridge domain.
func (plugin *BDConfigurator) ConfigureBridgeDomain(bridgeDomainInput *l2.BridgeDomains_BridgeDomain) error {
	plugin.Log.Println("Configuring VPP Bridge Domain", bridgeDomainInput.Name)

	if !plugin.vppValidateBridgeDomainBVI(bridgeDomainInput) {
		return nil
	}

	bridgeDomainIndex := plugin.BridgeDomainIDSeq

	// Create bridge domain with respective index.
	err := vppcalls.VppAddBridgeDomain(bridgeDomainIndex, bridgeDomainInput, plugin.Log, plugin.vppChan,
		measure.GetTimeLog(l2ba.BridgeDomainAddDel{}, plugin.Stopwatch))
	// Increment global index
	plugin.BridgeDomainIDSeq++
	if err != nil {
		plugin.Log.WithField("Bridge domain name", bridgeDomainInput.Name).Error(err)
		return err
	}

	// Register created bridge domain.
	plugin.BdIndexes.RegisterName(bridgeDomainInput.Name, bridgeDomainIndex, nil)
	plugin.Log.WithFields(logging.Fields{"Name": bridgeDomainInput.Name, "Index": bridgeDomainIndex}).Debug("Bridge domain registered.")

	// Find all interfaces belonging to this bridge domain and set them up.
	allInterfaces, configuredInterfaces, bviInterfaceName := vppcalls.VppSetAllInterfacesToBridgeDomain(bridgeDomainInput, bridgeDomainIndex,
		plugin.SwIfIndexes, plugin.Log, plugin.vppChan, measure.GetTimeLog(vpe.SwInterfaceSetL2Bridge{}, plugin.Stopwatch))
	plugin.registerInterfaceToBridgeDomainPairs(allInterfaces, configuredInterfaces, bviInterfaceName, bridgeDomainIndex)

	// Resolve ARP termination table entries.
	arpTerminationTable := bridgeDomainInput.GetArpTerminationTable()
	if arpTerminationTable != nil && len(arpTerminationTable) != 0 {
		arpTable := bridgeDomainInput.ArpTerminationTable
		for _, arpEntry := range arpTable {
			err := vppcalls.VppAddArpTerminationTableEntry(bridgeDomainIndex, arpEntry.PhysAddress, arpEntry.IpAddress,
				plugin.Log, plugin.vppChan, measure.GetTimeLog(vpe.BdIPMacAddDel{}, plugin.Stopwatch))
			if err != nil {
				plugin.Log.Error(err)
			}
		}
	} else {
		plugin.Log.WithField("Bridge domain name", bridgeDomainInput.Name).Debug("No ARP termination entries to set")
	}

	// Push to bridge domain state.
	errLookup := plugin.LookupBridgeDomainDetails(bridgeDomainIndex, bridgeDomainInput.Name)
	if errLookup != nil {
		plugin.Log.WithField("bdName", bridgeDomainInput.Name).Error(errLookup)
		return errLookup
	}

	return nil
}


}

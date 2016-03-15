// Copyright 2015 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package maas

import (
	"encoding/json"
	"fmt"

	"github.com/juju/errors"
	"github.com/juju/gomaasapi"

	"github.com/juju/juju/instance"
	"github.com/juju/juju/network"
)

////////////////////////////////////////////////////////////////////////////////
// TODO(dimitern): The types below should be part of gomaasapi.
// LKK Card: https://canonical.leankit.com/Boards/View/101652562/119310616

type maasLinkMode string

const (
	modeUnknown maasLinkMode = ""
	modeAuto    maasLinkMode = "auto"
	modeStatic  maasLinkMode = "static"
	modeDHCP    maasLinkMode = "dhcp"
	modeLinkUp  maasLinkMode = "link_up"
)

type maasInterfaceLink struct {
	ID        int          `json:"id"`
	Subnet    *maasSubnet  `json:"subnet,omitempty"`
	IPAddress string       `json:"ip_address,omitempty"`
	Mode      maasLinkMode `json:"mode"`
}

type maasInterfaceType string

const (
	typeUnknown  maasInterfaceType = ""
	typePhysical maasInterfaceType = "physical"
	typeVLAN     maasInterfaceType = "vlan"
	typeBond     maasInterfaceType = "bond"
)

type maasInterface struct {
	ID      int               `json:"id"`
	Name    string            `json:"name"`
	Type    maasInterfaceType `json:"type"`
	Enabled bool              `json:"enabled"`

	MACAddress  string   `json:"mac_address"`
	VLAN        maasVLAN `json:"vlan"`
	EffectveMTU int      `json:"effective_mtu"`

	Links []maasInterfaceLink `json:"links"`

	Parents  []string `json:"parents"`
	Children []string `json:"children"`

	ResourceURI string `json:"resource_uri"`
}

type maasVLAN struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	VID         int    `json:"vid"`
	MTU         int    `json:"mtu"`
	Fabric      string `json:"fabric"`
	ResourceURI string `json:"resource_uri"`
}

type maasSubnet struct {
	ID          int      `json:"id"`
	Name        string   `json:"name"`
	Space       string   `json:"space"`
	VLAN        maasVLAN `json:"vlan"`
	GatewayIP   string   `json:"gateway_ip"`
	DNSServers  []string `json:"dns_servers"`
	CIDR        string   `json:"cidr"`
	ResourceURI string   `json:"resource_uri"`
}

func parseInterfaces(jsonBytes []byte) ([]maasInterface, error) {
	var interfaces []maasInterface
	if err := json.Unmarshal(jsonBytes, &interfaces); err != nil {
		return nil, errors.Annotate(err, "parsing interfaces")
	}
	return interfaces, nil
}

// maasObjectNetworkInterfaces implements environs.NetworkInterfaces() using the
// new (1.9+) MAAS API, parsing the node details JSON embedded into the given
// maasObject to extract all the relevant InterfaceInfo fields. It returns an
// error satisfying errors.IsNotSupported() if it cannot find the required
// "interface_set" node details field.
func maasObjectNetworkInterfaces(maasObject *gomaasapi.MAASObject, subnetsMap map[string]network.Id) ([]network.InterfaceInfo, error) {

	interfaceSet, ok := maasObject.GetMap()["interface_set"]
	if !ok || interfaceSet.IsNil() {
		// This means we're using an older MAAS API.
		return nil, errors.NotSupportedf("interface_set")
	}

	// TODO(dimitern): Change gomaasapi JSONObject to give access to the raw
	// JSON bytes directly, rather than having to do call MarshalJSON just so
	// the result can be unmarshaled from it.
	//
	// LKK Card: https://canonical.leankit.com/Boards/View/101652562/119311323

	rawBytes, err := interfaceSet.MarshalJSON()
	if err != nil {
		return nil, errors.Annotate(err, "cannot get interface_set JSON bytes")
	}

	interfaces, err := parseInterfaces(rawBytes)
	if err != nil {
		return nil, errors.Trace(err)
	}

	infos := make([]network.InterfaceInfo, 0, len(interfaces))
	for i, iface := range interfaces {
		nicInfo := network.InterfaceInfo{
			DeviceIndex:   i,
			MACAddress:    iface.MACAddress,
			ProviderId:    network.Id(fmt.Sprintf("%v", iface.ID)),
			VLANTag:       iface.VLAN.VID,
			InterfaceName: iface.Name,
			Disabled:      !iface.Enabled,
			NoAutoStart:   !iface.Enabled,
			// This is not needed anymore, but the provisioner still validates it's set.
			NetworkName: network.DefaultPrivate,
		}

		for _, link := range iface.Links {
			switch link.Mode {
			case modeUnknown:
				nicInfo.ConfigType = network.ConfigUnknown
			case modeDHCP:
				nicInfo.ConfigType = network.ConfigDHCP
			case modeStatic, modeLinkUp:
				nicInfo.ConfigType = network.ConfigStatic
			default:
				nicInfo.ConfigType = network.ConfigManual
			}

			if link.IPAddress == "" {
				logger.Warningf("interface %q has no address", iface.Name)
			} else {
				// We set it here initially without a space, just so we don't
				// lose it when we have no linked subnet below.
				nicInfo.Address = network.NewAddress(link.IPAddress)
			}

			if link.Subnet == nil {
				logger.Warningf("interface %q link %d missing subnet", iface.Name, link.ID)
				infos = append(infos, nicInfo)
				continue
			}

			sub := link.Subnet
			nicInfo.CIDR = sub.CIDR
			nicInfo.ProviderSubnetId = network.Id(fmt.Sprintf("%v", sub.ID))

			// Now we know the subnet and space, we can update the address to
			// store the space with it.
			nicInfo.Address = network.NewAddressOnSpace(sub.Space, link.IPAddress)
			spaceId, ok := subnetsMap[string(sub.CIDR)]
			if !ok {
				// The space we found is not recognised, no
				// provider id available.
				logger.Warningf("interface %q link %d has unrecognised space %q", iface.Name, link.ID, sub.Space)
			} else {

				nicInfo.Address.SpaceProviderId = spaceId
			}

			gwAddr := network.NewAddressOnSpace(sub.Space, sub.GatewayIP)
			nicInfo.DNSServers = network.NewAddressesOnSpace(sub.Space, sub.DNSServers...)
			if ok {
				gwAddr.SpaceProviderId = spaceId
				for i := range nicInfo.DNSServers {
					nicInfo.DNSServers[i].SpaceProviderId = spaceId
				}
			}
			nicInfo.GatewayAddress = gwAddr
			nicInfo.MTU = sub.VLAN.MTU

			// Each link we represent as a separate InterfaceInfo, but with the
			// same name and device index, just different addres, subnet, etc.
			infos = append(infos, nicInfo)
		}
	}
	return infos, nil
}

// NetworkInterfaces implements Environ.NetworkInterfaces.
func (environ *maasEnviron) NetworkInterfaces(instId instance.Id) ([]network.InterfaceInfo, error) {
	inst, err := environ.getInstance(instId)
	if err != nil {
		return nil, errors.Trace(err)
	}
	subnetsMap, err := environ.subnetToSpaceIds()
	if err != nil {
		return nil, errors.Trace(err)
	}
	mi := inst.(*maasInstance)
	return maasObjectNetworkInterfaces(mi.maasObject, subnetsMap)
}

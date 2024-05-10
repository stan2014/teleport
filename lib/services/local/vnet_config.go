// Teleport
// Copyright (C) 2024 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package local

import (
	"context"
	"net"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/gen/proto/go/teleport/vnet/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local/generic"
)

const (
	vnetConfigPrefix        = "vnet_config"
	vnetConfigSingletonName = "vnet-config"
)

type VnetConfigService struct {
	svc *generic.ServiceWrapper[*vnet.VnetConfig]
}

func NewVnetConfigService(backend backend.Backend) (*VnetConfigService, error) {
	svc, err := generic.NewServiceWrapper(
		backend,
		types.KindVnetConfig,
		vnetConfigPrefix,
		services.MarshalProtoResource[*vnet.VnetConfig],
		services.UnmarshalProtoResource[*vnet.VnetConfig],
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &VnetConfigService{
		svc: svc,
	}, nil
}

func (s *VnetConfigService) GetVnetConfig(ctx context.Context) (*vnet.VnetConfig, error) {
	return s.svc.GetResource(ctx, vnetConfigSingletonName)
}

func (s *VnetConfigService) CreateVnetConfig(ctx context.Context, vnetConfig *vnet.VnetConfig) (*vnet.VnetConfig, error) {
	if err := validateVnetConfig(vnetConfig); err != nil {
		return nil, trace.Wrap(err)
	}
	return s.svc.CreateResource(ctx, vnetConfig)
}

func (s *VnetConfigService) UpdateVnetConfig(ctx context.Context, vnetConfig *vnet.VnetConfig) (*vnet.VnetConfig, error) {
	if err := validateVnetConfig(vnetConfig); err != nil {
		return nil, trace.Wrap(err)
	}
	return s.svc.ConditionalUpdateResource(ctx, vnetConfig)
}

func (s *VnetConfigService) UpsertVnetConfig(ctx context.Context, vnetConfig *vnet.VnetConfig) (*vnet.VnetConfig, error) {
	if err := validateVnetConfig(vnetConfig); err != nil {
		return nil, trace.Wrap(err)
	}
	return s.svc.UpsertResource(ctx, vnetConfig)
}

func (s *VnetConfigService) DeleteVnetConfig(ctx context.Context) error {
	return s.svc.DeleteResource(ctx, vnetConfigSingletonName)
}

func validateVnetConfig(vnetConfig *vnet.VnetConfig) error {
	if vnetConfig.GetKind() != types.KindVnetConfig {
		return trace.BadParameter("kind must be %q", types.KindVnetConfig)
	}
	if vnetConfig.GetVersion() != types.V1 {
		return trace.BadParameter("version must be %q", types.V1)
	}
	if vnetConfig.GetMetadata().GetName() != vnetConfigSingletonName {
		return trace.BadParameter("name must be %q", vnetConfigSingletonName)
	}
	if cidrRange := vnetConfig.GetSpec().GetIpv4CidrRange(); cidrRange != "" {
		ip, _, err := net.ParseCIDR(cidrRange)
		if err != nil {
			return trace.Wrap(err, "parsing ipv4_cidr_range")
		}
		if ip4 := ip.To4(); ip4 == nil {
			return trace.BadParameter("ipv4_cidr_range must be valid IPv4")
		}
	}
	return nil
}

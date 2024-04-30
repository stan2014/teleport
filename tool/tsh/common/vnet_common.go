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

package common

import (
	"context"
	"crypto/tls"
	"log/slog"
	"net"
	"strings"

	"github.com/gravitational/trace"
	"github.com/vulcand/predicate/builder"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/srv/alpnproxy"
	"github.com/gravitational/teleport/lib/vnet"
)

type tcpAppResolver struct {
	cf          *CLIConf
	clientStore *client.Store
}

func newTCPAppResolver(cf *CLIConf) *tcpAppResolver {
	clientStore := client.NewFSClientStore(cf.HomePath)
	return &tcpAppResolver{
		cf:          cf,
		clientStore: clientStore,
	}
}

// ResolveTCPHandler takes a fully-qualified domain name and, if it should be valid for a currently connected
// app, returns a TCPHandler that should handle all future VNet TCP connections to that FQDN.
func (r *tcpAppResolver) ResolveTCPHandler(ctx context.Context, fqdn string) (handler vnet.TCPHandler, match bool, err error) {
	profileNames, err := r.clientStore.ListProfiles()
	if err != nil {
		return nil, false, trace.Wrap(err, "listing profiles")
	}
	appPublicAddr := strings.TrimSuffix(fqdn, ".")
	for _, profileName := range profileNames {
		if !isSubdomain(fqdn, profileName) {
			// TODO(nklaassen): handle custom DNS zones and leaf clusters.
			continue
		}

		tc, err := r.getTeleportClient(ctx, profileName)
		if err != nil {
			return nil, false, trace.Wrap(err, "getting Teleport client")
		}

		appServers, err := tc.ListAppServersWithFilters(ctx, &proto.ListResourcesRequest{
			ResourceType: types.KindAppServer,
			PredicateExpression: builder.Equals(
				builder.Identifier("resource.spec.public_addr"),
				builder.String(appPublicAddr),
			).String(),
		})
		if err != nil {
			return nil, false, trace.Wrap(err, "listing application servers")
		}

		for _, appServer := range appServers {
			app := appServer.GetApp()
			if app.GetPublicAddr() == appPublicAddr && app.IsTCP() {
				appHandler, err := newTCPAppHandler(ctx, r, profileName, app)
				if err != nil {
					return nil, false, trace.Wrap(err)
				}
				return appHandler, true, nil
			}
		}
	}
	return nil, false, nil
}

func (r *tcpAppResolver) getTeleportClient(ctx context.Context, profileName string) (*client.TeleportClient, error) {
	// TODO(nklaassen): handle leaf clusters.
	// TODO(nklaassen/ravicious): cache clients and handle certificate expiry.
	tc, err := makeClientForProxy(r.cf, profileName)
	return tc, trace.Wrap(err)
}

type tcpAppHandler struct {
	lp  *alpnproxy.LocalProxy
	app types.Application
}

func newTCPAppHandler(ctx context.Context, r *tcpAppResolver, profileName string, app types.Application) (*tcpAppHandler, error) {
	tc, err := r.getTeleportClient(ctx, profileName)
	if err != nil {
		return nil, trace.Wrap(err, "getting Teleport client")
	}

	slog.With("app", app, "app_name", app.GetName()).DebugContext(ctx, "logging in to app")
	appCerts, err := getAppCert(ctx, tc, app)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// TODO(nklaassen/ravicious): add middleware to refresh expired app certs and handle per-session MFA.
	lp, err := alpnproxy.NewLocalProxy(
		makeBasicLocalProxyConfig(r.cf, tc, nil /*listener*/),
		alpnproxy.WithALPNProtocol(alpnProtocolForApp(app)),
		alpnproxy.WithClientCerts(appCerts),
		alpnproxy.WithClusterCAsIfConnUpgrade(ctx, tc.RootClusterCACertPool),
	)
	if err != nil {
		return nil, trace.Wrap(err, "creating local proxy")
	}

	return &tcpAppHandler{
		lp:  lp,
		app: app,
	}, nil
}

// HandleTCP handles a TCP connection from VNet and proxies it to the application.
func (h *tcpAppHandler) HandleTCPConnector(ctx context.Context, connector func() (net.Conn, error)) error {
	return trace.Wrap(h.lp.HandleTCPConnector(ctx, connector), "handling TCP connector")
}

func getAppCert(ctx context.Context, tc *client.TeleportClient, app types.Application) (tls.Certificate, error) {
	cert, needLogin, err := loadAppCertificate(tc, app.GetName())
	if err == nil && needLogin == false {
		return cert, nil
	}
	if !needLogin {
		return tls.Certificate{}, trace.Wrap(err, "loading app certificate")
	}

	profile, err := tc.ProfileStatus()
	if err != nil {
		return tls.Certificate{}, trace.Wrap(err, "loading profile")
	}

	routeToApp := proto.RouteToApp{
		Name:        app.GetName(),
		PublicAddr:  app.GetPublicAddr(),
		ClusterName: tc.SiteName,
	}

	// TODO (Joerger): DELETE IN v17.0.0
	clusterClient, err := tc.ConnectToCluster(ctx)
	if err != nil {
		return tls.Certificate{}, trace.Wrap(err)
	}
	rootClient, err := clusterClient.ConnectToRootCluster(ctx)
	if err != nil {
		return tls.Certificate{}, trace.Wrap(err)
	}
	routeToApp.SessionID, err = auth.TryCreateAppSessionForClientCertV15(ctx, rootClient, tc.Username, routeToApp)
	if err != nil {
		return tls.Certificate{}, trace.Wrap(err)
	}

	err = tc.ReissueUserCerts(ctx, client.CertCacheKeep, client.ReissueParams{
		RouteToCluster: profile.Cluster,
		RouteToApp:     routeToApp,
		AccessRequests: profile.ActiveRequests.AccessRequests,
	})
	if err != nil {
		return tls.Certificate{}, trace.Wrap(err, "logging in to app")
	}

	cert, needLogin, err = loadAppCertificate(tc, app.GetName())
	if err != nil {
		return tls.Certificate{}, trace.Wrap(err, "loading app certificate after login")
	}
	if needLogin {
		return tls.Certificate{}, trace.Errorf("still need login after login: this is a bug")
	}
	return cert, nil
}

func isSubdomain(appFQDN, proxyAddress string) bool {
	// Fully-qualify the proxy address
	if !strings.HasSuffix(proxyAddress, ".") {
		proxyAddress = proxyAddress + "."
	}
	return strings.HasSuffix(appFQDN, "."+proxyAddress)
}

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

package clientcache

import (
	"context"
	"slices"
	"strings"
	"sync"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/singleflight"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/client"
)

// Cache stores clients keyed by cluster URI.
// Safe for concurrent access.
// Closes all clients and wipes the cache on Clear.
type Cache struct {
	cfg Config
	mu  sync.RWMutex
	// clients keep mapping between cluster URI
	// (both root and leaf) and cluster clients
	clients map[string]*client.ClusterClient
	// group prevents duplicate requests to create clients
	// for a given cluster URI
	group singleflight.Group
}

type NewClientFunc func(ctx context.Context, profileName, leafClusterName string) (*client.TeleportClient, error)
type RetryWithReloginFunc func(context.Context, func() error) error

// Config describes the client cache configuration.
type Config struct {
	NewClientFunc        NewClientFunc
	RetryWithReloginFunc RetryWithReloginFunc
	Log                  logrus.FieldLogger
}

func (c *Config) checkAndSetDefaults() error {
	if c.NewClientFunc == nil {
		return trace.BadParameter("NewClientFunc is required")
	}
	if c.RetryWithReloginFunc == nil {
		return trace.BadParameter("RetryWithReloginFunc is required")
	}
	if c.Log == nil {
		c.Log = logrus.WithField(teleport.ComponentKey, "clientcache")
	}
	return nil
}

func key(profileName, leafClusterName string) string {
	if leafClusterName == "" {
		return profileName
	}
	return profileName + "/" + leafClusterName
}

// New creates an instance of Cache.
func New(c Config) (*Cache, error) {
	if err := c.checkAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &Cache{
		cfg:     c,
		clients: make(map[string]*client.ClusterClient),
	}, nil
}

// Get returns a client from the cache if there is one,
// otherwise it dials the remote server.
// The caller should not close the returned client.
func (c *Cache) Get(ctx context.Context, profileName, leafClusterName string) (*client.ClusterClient, error) {
	k := key(profileName, leafClusterName)
	groupClt, err, _ := c.group.Do(k, func() (any, error) {
		if fromCache := c.getFromCache(k); fromCache != nil {
			c.cfg.Log.WithField("cluster", k).Info("Retrieved client from cache.")
			return fromCache, nil
		}

		tc, err := c.cfg.NewClientFunc(ctx, profileName, leafClusterName)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		var newClient *client.ClusterClient
		if err := c.cfg.RetryWithReloginFunc(ctx, func() error {
			clt, err := tc.ConnectToCluster(ctx)
			if err != nil {
				return trace.Wrap(err)
			}
			newClient = clt
			return nil
		}); err != nil {
			return nil, trace.Wrap(err)
		}

		// Save the client in the cache, so we don't have to build a new connection next time.
		c.addToCache(k, newClient)

		c.cfg.Log.WithField("cluster", k).Info("Added client to cache.")

		return newClient, nil
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	clt, ok := groupClt.(*client.ClusterClient)
	if !ok {
		return nil, trace.BadParameter("unexpected type %T received for cluster client", groupClt)
	}

	return clt, nil
}

// ClearForRoot closes and removes clients from the cache
// for the root cluster and its leaf clusters.
func (c *Cache) ClearForRoot(profileName string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	var (
		errors  []error
		deleted []string
	)

	for k, clt := range c.clients {
		if k == profileName || strings.HasPrefix(k, profileName+"/") {
			if err := clt.Close(); err != nil {
				errors = append(errors, err)
			}
			deleted = append(deleted, k)
			delete(c.clients, k)
		}
	}

	c.cfg.Log.WithFields(
		logrus.Fields{"cluster": profileName, "clients": deleted},
	).Info("Invalidated cached clients for root cluster.")

	return trace.NewAggregate(errors...)

}

// Clear closes and removes all clients.
func (c *Cache) Clear() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	var errors []error
	for _, clt := range c.clients {
		if err := clt.Close(); err != nil {
			errors = append(errors, err)
		}
	}
	clear(c.clients)

	return trace.NewAggregate(errors...)
}

func (c *Cache) addToCache(k string, clusterClient *client.ClusterClient) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.clients[k] = clusterClient
}

func (c *Cache) getFromCache(k string) *client.ClusterClient {
	c.mu.RLock()
	defer c.mu.RUnlock()

	clt := c.clients[k]
	return clt
}

// NoCache is a client cache implementation that returns a new client
// on each call to Get.
//
// ClearForRoot and Clear still work as expected.
type NoCache struct {
	mu            sync.Mutex
	newClientFunc NewClientFunc
	clients       []noCacheClient
}

type noCacheClient struct {
	k      string
	client *client.ClusterClient
}

func NewNoCache(newClientFunc NewClientFunc) *NoCache {
	return &NoCache{
		newClientFunc: newClientFunc,
	}
}

func (c *NoCache) Get(ctx context.Context, profileName, leafClusterName string) (*client.ClusterClient, error) {
	clusterClient, err := c.newClientFunc(ctx, profileName, leafClusterName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	newClient, err := clusterClient.ConnectToCluster(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	c.mu.Lock()
	c.clients = append(c.clients, noCacheClient{
		k:      key(profileName, leafClusterName),
		client: newClient,
	})
	c.mu.Unlock()

	return newClient, nil
}

func (c *NoCache) ClearForRoot(profileName string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	var (
		errors []error
	)

	c.clients = slices.DeleteFunc(c.clients, func(ncc noCacheClient) bool {
		belongsToCluster := ncc.k == profileName || strings.HasPrefix(ncc.k, profileName+"/")

		if belongsToCluster {
			if err := ncc.client.Close(); err != nil {
				errors = append(errors, err)
			}
		}

		return belongsToCluster
	})

	return trace.NewAggregate(errors...)
}

func (c *NoCache) Clear() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	var errors []error
	for _, ncc := range c.clients {
		if err := ncc.client.Close(); err != nil {
			errors = append(errors, err)
		}
	}
	c.clients = nil

	return trace.NewAggregate(errors...)
}

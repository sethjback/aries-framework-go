/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package didexchange

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/btcsuite/btcutil/base58"
	"github.com/stretchr/testify/require"

	"github.com/hyperledger/aries-framework-go/pkg/didcomm/common/service"
	"github.com/hyperledger/aries-framework-go/pkg/didcomm/protocol/decorator"
	"github.com/hyperledger/aries-framework-go/pkg/didcomm/protocol/didexchange"
	"github.com/hyperledger/aries-framework-go/pkg/didcomm/protocol/route"
	mockprotocol "github.com/hyperledger/aries-framework-go/pkg/internal/mock/didcomm/protocol"
	mocksvc "github.com/hyperledger/aries-framework-go/pkg/internal/mock/didcomm/protocol/didexchange"
	mockroute "github.com/hyperledger/aries-framework-go/pkg/internal/mock/didcomm/protocol/route"
	mockkms "github.com/hyperledger/aries-framework-go/pkg/internal/mock/kms/legacykms"
	mockprovider "github.com/hyperledger/aries-framework-go/pkg/internal/mock/provider"
	mockstore "github.com/hyperledger/aries-framework-go/pkg/internal/mock/storage"
	mockvdri "github.com/hyperledger/aries-framework-go/pkg/internal/mock/vdri"
	"github.com/hyperledger/aries-framework-go/pkg/store/connection"
)

func TestNew(t *testing.T) {
	t.Run("test new client", func(t *testing.T) {
		svc, err := didexchange.New(&mockprotocol.MockProvider{
			ServiceMap: map[string]interface{}{
				route.Coordination: &mockroute.MockRouteSvc{},
			},
		})
		require.NoError(t, err)
		require.NotNil(t, svc)

		_, err = New(&mockprovider.Provider{
			TransientStorageProviderValue: mockstore.NewMockStoreProvider(),
			StorageProviderValue:          mockstore.NewMockStoreProvider(),
			ServiceMap: map[string]interface{}{
				didexchange.DIDExchange: svc,
				route.Coordination:      &mockroute.MockRouteSvc{},
			},
		})
		require.NoError(t, err)
	})

	t.Run("test error from get service from context", func(t *testing.T) {
		_, err := New(&mockprovider.Provider{ServiceErr: fmt.Errorf("service error")})
		require.Error(t, err)
		require.Contains(t, err.Error(), "service error")
	})

	t.Run("test error from cast service", func(t *testing.T) {
		_, err := New(&mockprovider.Provider{ServiceValue: nil})
		require.Error(t, err)
		require.Contains(t, err.Error(), "cast service to DIDExchange Service failed")
	})

	t.Run("test route service cast error", func(t *testing.T) {
		_, err := New(&mockprovider.Provider{
			TransientStorageProviderValue: mockstore.NewMockStoreProvider(),
			StorageProviderValue:          mockstore.NewMockStoreProvider(),
			ServiceMap: map[string]interface{}{
				didexchange.DIDExchange: &mocksvc.MockDIDExchangeSvc{},
				route.Coordination:      &mocksvc.MockDIDExchangeSvc{},
			},
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), "cast service to Route Service failed")
	})

	t.Run("test error from open store", func(t *testing.T) {
		svc, err := didexchange.New(&mockprotocol.MockProvider{
			ServiceMap: map[string]interface{}{
				route.Coordination: &mockroute.MockRouteSvc{},
			},
		})
		require.NoError(t, err)
		require.NotNil(t, svc)

		_, err = New(&mockprovider.Provider{
			TransientStorageProviderValue: mockstore.NewMockStoreProvider(),
			StorageProviderValue: &mockstore.MockStoreProvider{
				ErrOpenStoreHandle: fmt.Errorf("failed to open store")},
			ServiceMap: map[string]interface{}{
				didexchange.DIDExchange: svc,
				route.Coordination:      &mockroute.MockRouteSvc{},
			},
			InboundEndpointValue: "endpoint"})
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to open store")
	})

	t.Run("test error from open transient store", func(t *testing.T) {
		svc, err := didexchange.New(&mockprotocol.MockProvider{
			ServiceMap: map[string]interface{}{
				route.Coordination: &mockroute.MockRouteSvc{},
			},
		})
		require.NoError(t, err)
		require.NotNil(t, svc)

		_, err = New(&mockprovider.Provider{
			StorageProviderValue: mockstore.NewMockStoreProvider(),
			TransientStorageProviderValue: &mockstore.MockStoreProvider{
				ErrOpenStoreHandle: fmt.Errorf("failed to open transient store")},
			ServiceMap: map[string]interface{}{
				didexchange.DIDExchange: svc,
				route.Coordination:      &mockroute.MockRouteSvc{},
			},
			InboundEndpointValue: "endpoint"})
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to open transient store")
	})
}

func TestClient_CreateInvitation(t *testing.T) {
	t.Run("test success", func(t *testing.T) {
		svc, err := didexchange.New(&mockprotocol.MockProvider{
			ServiceMap: map[string]interface{}{
				route.Coordination: &mockroute.MockRouteSvc{},
			},
		})
		require.NoError(t, err)
		require.NotNil(t, svc)

		c, err := New(&mockprovider.Provider{
			TransientStorageProviderValue: mockstore.NewMockStoreProvider(),
			StorageProviderValue:          mockstore.NewMockStoreProvider(),
			ServiceMap: map[string]interface{}{
				didexchange.DIDExchange: svc,
				route.Coordination:      &mockroute.MockRouteSvc{},
			},
			KMSValue:             &mockkms.CloseableKMS{CreateEncryptionKeyValue: "sample-key"},
			InboundEndpointValue: "endpoint"})

		require.NoError(t, err)
		inviteReq, err := c.CreateInvitation("agent")
		require.NoError(t, err)
		require.NotNil(t, inviteReq)
		require.NotEmpty(t, inviteReq.Label)
		require.NotEmpty(t, inviteReq.ID)
		require.Nil(t, inviteReq.RoutingKeys)
		require.Equal(t, "endpoint", inviteReq.ServiceEndpoint)
	})

	t.Run("test error from createSigningKey", func(t *testing.T) {
		svc, err := didexchange.New(&mockprotocol.MockProvider{
			ServiceMap: map[string]interface{}{
				route.Coordination: &mockroute.MockRouteSvc{},
			},
		})
		require.NoError(t, err)
		require.NotNil(t, svc)

		c, err := New(&mockprovider.Provider{
			TransientStorageProviderValue: mockstore.NewMockStoreProvider(),
			StorageProviderValue:          mockstore.NewMockStoreProvider(),
			ServiceMap: map[string]interface{}{
				didexchange.DIDExchange: svc,
				route.Coordination:      &mockroute.MockRouteSvc{},
			},
			KMSValue: &mockkms.CloseableKMS{CreateKeyErr: fmt.Errorf("createKeyErr")}})
		require.NoError(t, err)
		_, err = c.CreateInvitation("agent")
		require.Error(t, err)
		require.Contains(t, err.Error(), "createKeyErr")
	})

	t.Run("test error from save record", func(t *testing.T) {
		store := &mockstore.MockStore{
			Store:  make(map[string][]byte),
			ErrPut: fmt.Errorf("store error"),
		}

		svc, err := didexchange.New(&mockprotocol.MockProvider{
			StoreProvider: mockstore.NewCustomMockStoreProvider(store),
			ServiceMap: map[string]interface{}{
				route.Coordination: &mockroute.MockRouteSvc{},
			},
		})
		require.NoError(t, err)
		require.NotNil(t, svc)

		c, err := New(&mockprovider.Provider{
			TransientStorageProviderValue: mockstore.NewMockStoreProvider(),
			StorageProviderValue:          mockstore.NewCustomMockStoreProvider(store),
			ServiceMap: map[string]interface{}{
				didexchange.DIDExchange: svc,
				route.Coordination:      &mockroute.MockRouteSvc{},
			},
			KMSValue: &mockkms.CloseableKMS{}})
		require.NoError(t, err)
		_, err = c.CreateInvitation("agent")
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to save invitation")
	})

	t.Run("test success with router registered", func(t *testing.T) {
		endpoint := "http://router.example.com"
		routingKeys := []string{"abc", "xyz"}

		svc, err := didexchange.New(&mockprotocol.MockProvider{
			ServiceMap: map[string]interface{}{
				route.Coordination: &mockroute.MockRouteSvc{},
			},
		})
		require.NoError(t, err)
		require.NotNil(t, svc)

		c, err := New(&mockprovider.Provider{
			TransientStorageProviderValue: mockstore.NewMockStoreProvider(),
			StorageProviderValue:          mockstore.NewMockStoreProvider(),
			ServiceMap: map[string]interface{}{
				didexchange.DIDExchange: svc,
				route.Coordination:      &mockroute.MockRouteSvc{RoutingKeys: routingKeys, RouterEndpoint: endpoint},
			},
			KMSValue:             &mockkms.CloseableKMS{CreateEncryptionKeyValue: "sample-key"},
			InboundEndpointValue: "endpoint",
		})
		require.NoError(t, err)

		inviteReq, err := c.CreateInvitation("agent")
		require.NoError(t, err)
		require.NotNil(t, inviteReq)
		require.NotEmpty(t, inviteReq.Label)
		require.NotEmpty(t, inviteReq.ID)
		require.Equal(t, endpoint, inviteReq.ServiceEndpoint)
		require.Equal(t, routingKeys, inviteReq.RoutingKeys)
	})

	t.Run("test create invitation with router config error", func(t *testing.T) {
		svc, err := didexchange.New(&mockprotocol.MockProvider{
			ServiceMap: map[string]interface{}{
				route.Coordination: &mockroute.MockRouteSvc{},
			},
		})
		require.NoError(t, err)
		require.NotNil(t, svc)

		c, err := New(&mockprovider.Provider{
			TransientStorageProviderValue: mockstore.NewMockStoreProvider(),
			StorageProviderValue:          mockstore.NewMockStoreProvider(),
			ServiceMap: map[string]interface{}{
				didexchange.DIDExchange: svc,
				route.Coordination:      &mockroute.MockRouteSvc{ConfigErr: errors.New("router config error")},
			},
			KMSValue:             &mockkms.CloseableKMS{CreateEncryptionKeyValue: "sample-key"},
			InboundEndpointValue: "endpoint",
		})
		require.NoError(t, err)

		inviteReq, err := c.CreateInvitation("agent")
		require.Error(t, err)
		require.Contains(t, err.Error(), "create invitation - fetch router config")
		require.Nil(t, inviteReq)
	})

	t.Run("test create invitation with adding key to router error", func(t *testing.T) {
		endpoint := "http://router.example.com"
		routingKeys := []string{"abc", "xyz"}

		svc, err := didexchange.New(&mockprotocol.MockProvider{
			ServiceMap: map[string]interface{}{
				route.Coordination: &mockroute.MockRouteSvc{},
			},
		})
		require.NoError(t, err)
		require.NotNil(t, svc)

		c, err := New(&mockprovider.Provider{
			TransientStorageProviderValue: mockstore.NewMockStoreProvider(),
			StorageProviderValue:          mockstore.NewMockStoreProvider(),
			ServiceMap: map[string]interface{}{
				didexchange.DIDExchange: svc,
				route.Coordination: &mockroute.MockRouteSvc{
					RoutingKeys:    routingKeys,
					RouterEndpoint: endpoint,
					AddKeyErr:      errors.New("failed to add key to the router"),
				},
			},
			KMSValue:             &mockkms.CloseableKMS{CreateEncryptionKeyValue: "sample-key"},
			InboundEndpointValue: "endpoint",
		})
		require.NoError(t, err)

		inviteReq, err := c.CreateInvitation("agent")
		require.Error(t, err)
		require.Contains(t, err.Error(), "create invitation - add key to the router")
		require.Nil(t, inviteReq)
	})
}

func TestClient_CreateInvitationWithDID(t *testing.T) {
	t.Run("test success", func(t *testing.T) {
		svc, err := didexchange.New(&mockprotocol.MockProvider{
			ServiceMap: map[string]interface{}{
				route.Coordination: &mockroute.MockRouteSvc{},
			},
		})
		require.NoError(t, err)
		require.NotNil(t, svc)

		c, err := New(&mockprovider.Provider{
			TransientStorageProviderValue: mockstore.NewMockStoreProvider(),
			StorageProviderValue:          mockstore.NewMockStoreProvider(),
			ServiceMap: map[string]interface{}{
				didexchange.DIDExchange: svc,
				route.Coordination:      &mockroute.MockRouteSvc{},
			},
			KMSValue:             &mockkms.CloseableKMS{CreateEncryptionKeyValue: "sample-key"},
			InboundEndpointValue: "endpoint"})
		require.NoError(t, err)

		const label = "agent"
		const id = "did:sidetree:123"
		inviteReq, err := c.CreateInvitationWithDID(label, id)
		require.NoError(t, err)
		require.NotNil(t, inviteReq)
		require.Equal(t, label, inviteReq.Label)
		require.NotEmpty(t, inviteReq.ID)
		require.Equal(t, id, inviteReq.DID)
	})

	t.Run("test error from save invitation", func(t *testing.T) {
		store := &mockstore.MockStore{
			Store:  make(map[string][]byte),
			ErrPut: fmt.Errorf("store error"),
		}

		svc, err := didexchange.New(&mockprotocol.MockProvider{
			StoreProvider: mockstore.NewCustomMockStoreProvider(store),
			ServiceMap: map[string]interface{}{
				route.Coordination: &mockroute.MockRouteSvc{},
			},
		})
		require.NoError(t, err)
		require.NotNil(t, svc)

		c, err := New(&mockprovider.Provider{
			TransientStorageProviderValue: mockstore.NewMockStoreProvider(),
			StorageProviderValue:          mockstore.NewCustomMockStoreProvider(store),
			ServiceMap: map[string]interface{}{
				didexchange.DIDExchange: svc,
				route.Coordination:      &mockroute.MockRouteSvc{},
			},
			KMSValue: &mockkms.CloseableKMS{}})
		require.NoError(t, err)

		_, err = c.CreateInvitationWithDID("agent", "did:sidetree:123")
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to save invitation")
	})
}

func TestClient_QueryConnectionByID(t *testing.T) {
	const (
		connID   = "id1"
		threadID = "thid1"
	)

	t.Run("test success", func(t *testing.T) {
		svc, err := didexchange.New(&mockprotocol.MockProvider{
			ServiceMap: map[string]interface{}{
				route.Coordination: &mockroute.MockRouteSvc{},
			},
		})
		require.NoError(t, err)
		require.NotNil(t, svc)

		c, err := New(&mockprovider.Provider{
			TransientStorageProviderValue: mockstore.NewMockStoreProvider(),
			StorageProviderValue:          mockstore.NewMockStoreProvider(),
			ServiceMap: map[string]interface{}{
				didexchange.DIDExchange: svc,
				route.Coordination:      &mockroute.MockRouteSvc{},
			},
			KMSValue:             &mockkms.CloseableKMS{CreateEncryptionKeyValue: "sample-key"},
			InboundEndpointValue: "endpoint"})
		require.NoError(t, err)

		connRec := &connection.Record{ConnectionID: connID, ThreadID: threadID, State: "complete"}

		require.NoError(t, err)
		require.NoError(t, c.connectionStore.SaveConnectionRecord(connRec))
		result, err := c.GetConnection(connID)
		require.NoError(t, err)
		require.Equal(t, "complete", result.State)
		require.Equal(t, "id1", result.ConnectionID)
	})

	t.Run("test error", func(t *testing.T) {
		const errMsg = "query connection error"
		svc, err := didexchange.New(&mockprotocol.MockProvider{
			ServiceMap: map[string]interface{}{
				route.Coordination: &mockroute.MockRouteSvc{},
			},
		})
		require.NoError(t, err)
		require.NotNil(t, svc)

		store := &mockstore.MockStore{
			Store:  make(map[string][]byte),
			ErrGet: fmt.Errorf(errMsg),
		}

		c, err := New(&mockprovider.Provider{
			TransientStorageProviderValue: mockstore.NewCustomMockStoreProvider(store),
			StorageProviderValue:          mockstore.NewMockStoreProvider(),
			ServiceMap: map[string]interface{}{
				didexchange.DIDExchange: svc,
				route.Coordination:      &mockroute.MockRouteSvc{},
			},
			KMSValue:             &mockkms.CloseableKMS{CreateEncryptionKeyValue: "sample-key"},
			InboundEndpointValue: "endpoint"})
		require.NoError(t, err)

		connRec := &connection.Record{ConnectionID: connID, ThreadID: threadID, State: "complete"}

		require.NoError(t, err)
		require.NoError(t, c.connectionStore.SaveConnectionRecord(connRec))
		_, err = c.GetConnection(connID)
		require.Error(t, err)
		require.Contains(t, err.Error(), errMsg)
	})

	t.Run("test data not found", func(t *testing.T) {
		svc, err := didexchange.New(&mockprotocol.MockProvider{
			ServiceMap: map[string]interface{}{
				route.Coordination: &mockroute.MockRouteSvc{},
			},
		})
		require.NoError(t, err)
		require.NotNil(t, svc)

		c, err := New(&mockprovider.Provider{
			TransientStorageProviderValue: mockstore.NewMockStoreProvider(),
			StorageProviderValue:          mockstore.NewMockStoreProvider(),
			ServiceMap: map[string]interface{}{
				didexchange.DIDExchange: svc,
				route.Coordination:      &mockroute.MockRouteSvc{},
			},
			KMSValue:             &mockkms.CloseableKMS{CreateEncryptionKeyValue: "sample-key"},
			InboundEndpointValue: "endpoint"})
		require.NoError(t, err)

		result, err := c.GetConnection(connID)
		require.Error(t, err)
		require.True(t, errors.Is(err, ErrConnectionNotFound))
		require.Nil(t, result)
	})
}

func TestClient_GetConnection(t *testing.T) {
	connID := "id1"
	threadID := "thid1"

	t.Run("test failure", func(t *testing.T) {
		svc, err := didexchange.New(&mockprotocol.MockProvider{
			ServiceMap: map[string]interface{}{
				route.Coordination: &mockroute.MockRouteSvc{},
			},
		})
		require.NoError(t, err)
		require.NotNil(t, svc)
		s := &mockstore.MockStore{Store: make(map[string][]byte), ErrGet: ErrConnectionNotFound}
		c, err := New(&mockprovider.Provider{
			TransientStorageProviderValue: mockstore.NewMockStoreProvider(),
			StorageProviderValue:          mockstore.NewMockStoreProvider(),
			ServiceMap: map[string]interface{}{
				didexchange.DIDExchange: svc,
				route.Coordination:      &mockroute.MockRouteSvc{},
			},
		})
		require.NoError(t, err)
		connRec := &connection.Record{ConnectionID: connID, ThreadID: threadID, State: "complete"}
		connBytes, err := json.Marshal(connRec)
		require.NoError(t, err)
		require.NoError(t, s.Put("conn_id1", connBytes))
		result, err := c.GetConnection(connID)
		require.Equal(t, err.Error(), ErrConnectionNotFound.Error())
		require.Nil(t, result)
	})
}

func TestClientGetConnectionAtState(t *testing.T) {
	// create service
	svc, err := didexchange.New(&mockprotocol.MockProvider{
		ServiceMap: map[string]interface{}{
			route.Coordination: &mockroute.MockRouteSvc{},
		},
	})
	require.NoError(t, err)
	require.NotNil(t, svc)

	// create client
	c, err := New(&mockprovider.Provider{
		TransientStorageProviderValue: mockstore.NewMockStoreProvider(),
		StorageProviderValue:          mockstore.NewMockStoreProvider(),
		ServiceMap: map[string]interface{}{
			didexchange.DIDExchange: svc,
			route.Coordination:      &mockroute.MockRouteSvc{},
		},
	})
	require.NoError(t, err)

	// not found
	result, err := c.GetConnectionAtState("id1", "complete")
	require.Equal(t, err.Error(), ErrConnectionNotFound.Error())
	require.Nil(t, result)
}

func TestClient_RemoveConnection(t *testing.T) {
	svc, err := didexchange.New(&mockprotocol.MockProvider{
		ServiceMap: map[string]interface{}{
			route.Coordination: &mockroute.MockRouteSvc{},
		},
	})
	require.NoError(t, err)
	require.NotNil(t, svc)

	c, err := New(&mockprovider.Provider{
		TransientStorageProviderValue: mockstore.NewMockStoreProvider(),
		StorageProviderValue:          mockstore.NewMockStoreProvider(),
		ServiceMap: map[string]interface{}{
			didexchange.DIDExchange: svc,
			route.Coordination:      &mockroute.MockRouteSvc{},
		},
	})
	require.NoError(t, err)

	err = c.RemoveConnection("sample-id")
	require.NoError(t, err)
}

func TestClient_HandleInvitation(t *testing.T) {
	t.Run("test success", func(t *testing.T) {
		c, err := New(&mockprovider.Provider{
			TransientStorageProviderValue: mockstore.NewMockStoreProvider(),
			StorageProviderValue:          mockstore.NewMockStoreProvider(),
			ServiceMap: map[string]interface{}{
				didexchange.DIDExchange: &mocksvc.MockDIDExchangeSvc{},
				route.Coordination:      &mockroute.MockRouteSvc{},
			},
			KMSValue:             &mockkms.CloseableKMS{CreateEncryptionKeyValue: "sample-key"},
			InboundEndpointValue: "endpoint"})

		require.NoError(t, err)
		inviteReq, err := c.CreateInvitation("agent")
		require.NoError(t, err)

		connectionID, err := c.HandleInvitation(inviteReq)
		require.NoError(t, err)
		require.NotEmpty(t, connectionID)
	})

	t.Run("test error from handle msg", func(t *testing.T) {
		c, err := New(&mockprovider.Provider{
			TransientStorageProviderValue: mockstore.NewMockStoreProvider(),
			StorageProviderValue:          mockstore.NewMockStoreProvider(),
			ServiceMap: map[string]interface{}{
				didexchange.DIDExchange: &mocksvc.MockDIDExchangeSvc{HandleFunc: func(msg service.DIDCommMsg) (string, error) {
					return "", fmt.Errorf("handle error")
				}},
				route.Coordination: &mockroute.MockRouteSvc{},
			},

			KMSValue: &mockkms.CloseableKMS{CreateEncryptionKeyValue: "sample-key"}, InboundEndpointValue: "endpoint"})
		require.NoError(t, err)
		inviteReq, err := c.CreateInvitation("agent")
		require.NoError(t, err)

		_, err = c.HandleInvitation(inviteReq)
		require.Error(t, err)
		require.Contains(t, err.Error(), "handle error")
	})
}

func TestClient_CreateImplicitInvitation(t *testing.T) {
	t.Run("test success", func(t *testing.T) {
		c, err := New(&mockprovider.Provider{
			TransientStorageProviderValue: mockstore.NewMockStoreProvider(),
			StorageProviderValue:          mockstore.NewMockStoreProvider(),
			ServiceMap: map[string]interface{}{
				didexchange.DIDExchange: &mocksvc.MockDIDExchangeSvc{},
				route.Coordination:      &mockroute.MockRouteSvc{},
			},
			KMSValue:             &mockkms.CloseableKMS{CreateEncryptionKeyValue: "sample-key"},
			InboundEndpointValue: "endpoint"})
		require.NoError(t, err)

		connectionID, err := c.CreateImplicitInvitation("alice", "did:example:123")
		require.NoError(t, err)
		require.NotEmpty(t, connectionID)
	})

	t.Run("test error from service", func(t *testing.T) {
		c, err := New(&mockprovider.Provider{
			TransientStorageProviderValue: mockstore.NewMockStoreProvider(),
			StorageProviderValue:          mockstore.NewMockStoreProvider(),
			ServiceMap: map[string]interface{}{
				didexchange.DIDExchange: &mocksvc.MockDIDExchangeSvc{
					ImplicitInvitationErr: errors.New("implicit error")},
				route.Coordination: &mockroute.MockRouteSvc{},
			},
			KMSValue:             &mockkms.CloseableKMS{CreateEncryptionKeyValue: "sample-key"},
			InboundEndpointValue: "endpoint"})
		require.NoError(t, err)

		connectionID, err := c.CreateImplicitInvitation("Alice", "did:example:123")
		require.Error(t, err)
		require.Contains(t, err.Error(), "implicit error")
		require.Empty(t, connectionID)
	})
}

func TestClient_CreateImplicitInvitationWithDID(t *testing.T) {
	inviter := &DIDInfo{Label: "alice", DID: "did:example:alice"}
	invitee := &DIDInfo{Label: "bob", DID: "did:example:bob"}

	t.Run("test success", func(t *testing.T) {
		c, err := New(&mockprovider.Provider{
			TransientStorageProviderValue: mockstore.NewMockStoreProvider(),
			StorageProviderValue:          mockstore.NewMockStoreProvider(),
			ServiceMap: map[string]interface{}{
				didexchange.DIDExchange: &mocksvc.MockDIDExchangeSvc{},
				route.Coordination:      &mockroute.MockRouteSvc{},
			},
			KMSValue:             &mockkms.CloseableKMS{CreateEncryptionKeyValue: "sample-key"},
			InboundEndpointValue: "endpoint"})
		require.NoError(t, err)

		connectionID, err := c.CreateImplicitInvitationWithDID(inviter, invitee)
		require.NoError(t, err)
		require.NotEmpty(t, connectionID)
	})

	t.Run("test error from service", func(t *testing.T) {
		c, err := New(&mockprovider.Provider{
			TransientStorageProviderValue: mockstore.NewMockStoreProvider(),
			StorageProviderValue:          mockstore.NewMockStoreProvider(),
			ServiceMap: map[string]interface{}{
				didexchange.DIDExchange: &mocksvc.MockDIDExchangeSvc{
					ImplicitInvitationErr: errors.New("implicit with DID error")},
				route.Coordination: &mockroute.MockRouteSvc{},
			},
			KMSValue:             &mockkms.CloseableKMS{CreateEncryptionKeyValue: "sample-key"},
			InboundEndpointValue: "endpoint"})
		require.NoError(t, err)

		connectionID, err := c.CreateImplicitInvitationWithDID(inviter, invitee)
		require.Error(t, err)
		require.Contains(t, err.Error(), "implicit with DID error")
		require.Empty(t, connectionID)
	})

	t.Run("test missing required DID info", func(t *testing.T) {
		c, err := New(&mockprovider.Provider{
			TransientStorageProviderValue: mockstore.NewMockStoreProvider(),
			StorageProviderValue:          mockstore.NewMockStoreProvider(),
			ServiceMap: map[string]interface{}{
				didexchange.DIDExchange: &mocksvc.MockDIDExchangeSvc{},
				route.Coordination:      &mockroute.MockRouteSvc{},
			},
			KMSValue:             &mockkms.CloseableKMS{CreateEncryptionKeyValue: "sample-key"},
			InboundEndpointValue: "endpoint"})
		require.NoError(t, err)

		connectionID, err := c.CreateImplicitInvitationWithDID(inviter, nil)
		require.Error(t, err)
		require.Contains(t, err.Error(), "missing inviter and/or invitee public DID(s)")
		require.Empty(t, connectionID)

		connectionID, err = c.CreateImplicitInvitationWithDID(nil, invitee)
		require.Error(t, err)
		require.Contains(t, err.Error(), "missing inviter and/or invitee public DID(s)")
		require.Empty(t, connectionID)
	})
}

func TestClient_QueryConnectionsByParams(t *testing.T) {
	t.Run("test get all connections", func(t *testing.T) {
		svc, err := didexchange.New(&mockprotocol.MockProvider{
			ServiceMap: map[string]interface{}{
				route.Coordination: &mockroute.MockRouteSvc{},
			},
		})
		require.NoError(t, err)
		require.NotNil(t, svc)

		storageProvider := mockstore.NewMockStoreProvider()
		c, err := New(&mockprovider.Provider{
			TransientStorageProviderValue: mockstore.NewMockStoreProvider(),
			StorageProviderValue:          storageProvider,
			ServiceMap: map[string]interface{}{
				didexchange.DIDExchange: svc,
				route.Coordination:      &mockroute.MockRouteSvc{},
			},
		})
		require.NoError(t, err)

		const count = 10
		const keyPrefix = "conn_"
		const state = "completed"
		for i := 0; i < count; i++ {
			val, e := json.Marshal(&connection.Record{
				ConnectionID: string(i),
				State:        state,
			})
			require.NoError(t, e)
			require.NoError(t, storageProvider.Store.Put(fmt.Sprintf("%sabc%d", keyPrefix, i), val))
		}

		results, err := c.QueryConnections(&QueryConnectionsParams{})
		require.NoError(t, err)
		require.Len(t, results, count)
		for _, result := range results {
			require.NotEmpty(t, result.ConnectionID)
		}
	})

	t.Run("test get connections by state param", func(t *testing.T) {
		svc, err := didexchange.New(&mockprotocol.MockProvider{
			ServiceMap: map[string]interface{}{
				route.Coordination: &mockroute.MockRouteSvc{},
			},
		})
		require.NoError(t, err)
		require.NotNil(t, svc)

		storageProvider := mockstore.NewMockStoreProvider()
		c, err := New(&mockprovider.Provider{
			TransientStorageProviderValue: mockstore.NewMockStoreProvider(),
			StorageProviderValue:          storageProvider,
			ServiceMap: map[string]interface{}{
				didexchange.DIDExchange: svc,
				route.Coordination:      &mockroute.MockRouteSvc{},
			},
		})
		require.NoError(t, err)

		const count = 10
		const countWithState = 5
		const keyPrefix = "conn_"
		const state = "completed"
		for i := 0; i < count; i++ {
			var queryState string
			if i < countWithState {
				queryState = state
			}

			val, e := json.Marshal(&connection.Record{
				ConnectionID: string(i),
				State:        queryState,
			})
			require.NoError(t, e)
			require.NoError(t, storageProvider.Store.Put(fmt.Sprintf("%sabc%d", keyPrefix, i), val))
		}

		results, err := c.QueryConnections(&QueryConnectionsParams{})
		require.NoError(t, err)
		require.Len(t, results, count)
		for _, result := range results {
			require.NotEmpty(t, result.ConnectionID)
		}

		results, err = c.QueryConnections(&QueryConnectionsParams{State: state})
		require.NoError(t, err)
		require.Len(t, results, countWithState)
		for _, result := range results {
			require.NotEmpty(t, result.ConnectionID)
			require.Equal(t, result.State, state)
		}
	})

	t.Run("test get connections error", func(t *testing.T) {
		svc, err := didexchange.New(&mockprotocol.MockProvider{
			ServiceMap: map[string]interface{}{
				route.Coordination: &mockroute.MockRouteSvc{},
			},
		})
		require.NoError(t, err)
		require.NotNil(t, svc)
		const keyPrefix = "conn_"

		storageProvider := mockstore.NewMockStoreProvider()
		c, err := New(&mockprovider.Provider{
			TransientStorageProviderValue: mockstore.NewMockStoreProvider(),
			StorageProviderValue:          storageProvider,
			ServiceMap: map[string]interface{}{
				didexchange.DIDExchange: svc,
				route.Coordination:      &mockroute.MockRouteSvc{},
			},
		})
		require.NoError(t, err)

		require.NoError(t, storageProvider.Store.Put(fmt.Sprintf("%sabc", keyPrefix), []byte("----")))

		results, err := c.QueryConnections(&QueryConnectionsParams{})
		require.Error(t, err)
		require.Empty(t, results)
	})
}

func TestServiceEvents(t *testing.T) {
	transientStore := mockstore.NewMockStoreProvider()
	store := mockstore.NewMockStoreProvider()
	didExSvc, err := didexchange.New(&mockprotocol.MockProvider{
		TransientStoreProvider: transientStore,
		StoreProvider:          store,
		ServiceMap: map[string]interface{}{
			route.Coordination: &mockroute.MockRouteSvc{},
		},
	})
	require.NoError(t, err)

	// create the client
	c, err := New(&mockprovider.Provider{
		TransientStorageProviderValue: transientStore,
		StorageProviderValue:          store,
		ServiceMap: map[string]interface{}{
			didexchange.DIDExchange: didExSvc,
			route.Coordination:      &mockroute.MockRouteSvc{},
		},
		KMSValue: &mockkms.CloseableKMS{CreateEncryptionKeyValue: "sample-key"}})
	require.NoError(t, err)
	require.NotNil(t, c)

	// register action event channel
	aCh := make(chan service.DIDCommAction, 10)
	err = c.RegisterActionEvent(aCh)
	require.NoError(t, err)

	go func() {
		service.AutoExecuteActionEvent(aCh)
	}()

	// register message event channel
	mCh := make(chan service.StateMsg, 10)
	err = c.RegisterMsgEvent(mCh)
	require.NoError(t, err)

	stateMsg := make(chan service.StateMsg)

	go func() {
		for e := range mCh {
			if e.Type == service.PostState && e.StateID == "responded" {
				stateMsg <- e
			}
		}
	}()

	// send connection request message
	id := "valid-thread-id"
	newDidDoc, err := (&mockvdri.MockVDRIRegistry{}).Create("test")
	require.NoError(t, err)

	invitation, err := c.CreateInvitation("alice")
	require.NoError(t, err)

	request, err := json.Marshal(
		&didexchange.Request{
			Type:  didexchange.RequestMsgType,
			ID:    id,
			Label: "test",
			Thread: &decorator.Thread{
				PID: invitation.ID,
			},
			Connection: &didexchange.Connection{
				DID:    newDidDoc.ID,
				DIDDoc: newDidDoc,
			},
		},
	)
	require.NoError(t, err)

	msg, err := service.ParseDIDCommMsgMap(request)
	require.NoError(t, err)
	_, err = didExSvc.HandleInbound(msg, "", "")
	require.NoError(t, err)

	select {
	case e := <-stateMsg:
		switch v := e.Properties.(type) {
		case Event:
			props := v
			conn, err := c.GetConnectionAtState(props.ConnectionID(), e.StateID)
			require.NoError(t, err)
			require.Equal(t, e.StateID, conn.State)
		default:
			require.Fail(t, "unable to cast to did exchange event")
		}
	case <-time.After(5 * time.Second):
		require.Fail(t, "tests are not validated due to timeout")
	}
}

func TestAcceptExchangeRequest(t *testing.T) {
	store := mockstore.NewMockStoreProvider()
	didExSvc, err := didexchange.New(&mockprotocol.MockProvider{
		StoreProvider: store,
		ServiceMap: map[string]interface{}{
			route.Coordination: &mockroute.MockRouteSvc{},
		},
	})
	require.NoError(t, err)

	// create the client
	c, err := New(&mockprovider.Provider{
		TransientStorageProviderValue: mockstore.NewMockStoreProvider(),
		StorageProviderValue:          store,
		ServiceMap: map[string]interface{}{
			didexchange.DIDExchange: didExSvc,
			route.Coordination:      &mockroute.MockRouteSvc{},
		},
		KMSValue: &mockkms.CloseableKMS{CreateEncryptionKeyValue: "sample-key"}},
	)
	require.NoError(t, err)
	require.NotNil(t, c)

	// register action event channel
	aCh := make(chan service.DIDCommAction, 10)
	err = c.RegisterActionEvent(aCh)
	require.NoError(t, err)

	go func() {
		for e := range aCh {
			prop, ok := e.Properties.(Event)
			if !ok {
				require.Fail(t, "Failed to cast the event properties to service.Event")
			}

			require.NoError(t, c.AcceptExchangeRequest(prop.ConnectionID(), "", ""))
		}
	}()

	// register message event channel
	mCh := make(chan service.StateMsg, 10)
	err = c.RegisterMsgEvent(mCh)
	require.NoError(t, err)

	done := make(chan struct{})

	go func() {
		for e := range mCh {
			if e.Type == service.PostState && e.StateID == "responded" {
				close(done)
			}
		}
	}()

	invitation, err := c.CreateInvitation("alice")
	require.NoError(t, err)
	// send connection request message
	id := "valid-thread-id"
	newDidDoc, err := (&mockvdri.MockVDRIRegistry{}).Create("test")
	require.NoError(t, err)

	request, err := json.Marshal(
		&didexchange.Request{
			Type:  didexchange.RequestMsgType,
			ID:    id,
			Label: "test",
			Thread: &decorator.Thread{
				PID: invitation.ID,
			},
			Connection: &didexchange.Connection{
				DID:    newDidDoc.ID,
				DIDDoc: newDidDoc,
			},
		},
	)
	require.NoError(t, err)

	msg, err := service.ParseDIDCommMsgMap(request)
	require.NoError(t, err)
	_, err = didExSvc.HandleInbound(msg, "", "")
	require.NoError(t, err)

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		require.Fail(t, "tests are not validated due to timeout")
	}

	err = c.AcceptExchangeRequest("invalid-id", "", "")
	require.Error(t, err)
	require.Contains(t, err.Error(), "did exchange client - accept exchange request:")
}

func TestAcceptInvitation(t *testing.T) {
	store := mockstore.NewMockStoreProvider()
	didExSvc, err := didexchange.New(&mockprotocol.MockProvider{
		StoreProvider: store,
		ServiceMap: map[string]interface{}{
			route.Coordination: &mockroute.MockRouteSvc{},
		},
	})
	require.NoError(t, err)

	// create the client
	c, err := New(&mockprovider.Provider{
		TransientStorageProviderValue: mockstore.NewMockStoreProvider(),
		StorageProviderValue:          store,
		ServiceMap: map[string]interface{}{
			didexchange.DIDExchange: didExSvc,
			route.Coordination:      &mockroute.MockRouteSvc{},
		},
	})
	require.NoError(t, err)
	require.NotNil(t, c)

	t.Run("accept invitation - success", func(t *testing.T) {
		// register action event channel
		aCh := make(chan service.DIDCommAction, 10)
		err = c.RegisterActionEvent(aCh)
		require.NoError(t, err)

		go func() {
			for e := range aCh {
				_, ok := e.Properties.(Event)
				require.True(t, ok, "Failed to cast the event properties to service.Event")

				// ignore action event
			}
		}()

		// register message event channel
		mCh := make(chan service.StateMsg, 10)
		err = c.RegisterMsgEvent(mCh)
		require.NoError(t, err)

		done := make(chan struct{})

		go func() {
			for e := range mCh {
				prop, ok := e.Properties.(Event)
				if !ok {
					require.Fail(t, "Failed to cast the event properties to service.Event")
				}

				if e.Type == service.PostState && e.StateID == "invited" {
					require.NoError(t, c.AcceptInvitation(prop.ConnectionID(), "", ""))
				}

				if e.Type == service.PostState && e.StateID == "requested" {
					close(done)
				}
			}
		}()

		pubKey, _ := generateKeyPair()
		// send connection invitation message
		invitation, jsonErr := json.Marshal(
			&didexchange.Invitation{
				Type:          InvitationMsgType,
				ID:            "abc",
				Label:         "test",
				RecipientKeys: []string{pubKey},
			},
		)
		require.NoError(t, jsonErr)

		msg, svcErr := service.ParseDIDCommMsgMap(invitation)
		require.NoError(t, svcErr)
		_, err = didExSvc.HandleInbound(msg, "", "")
		require.NoError(t, err)

		select {
		case <-done:
		case <-time.After(5 * time.Second):
			require.Fail(t, "tests are not validated due to timeout")
		}
	})

	t.Run("accept invitation - error", func(t *testing.T) {
		err = c.AcceptInvitation("invalid-id", "", "")
		require.Error(t, err)
		require.Contains(t, err.Error(), "did exchange client - accept exchange invitation")
	})
}
func generateKeyPair() (string, []byte) {
	pubKey, privKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		panic(err)
	}

	return base58.Encode(pubKey[:]), privKey
}

/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package didexchange

import (
	"fmt"

	"github.com/hyperledger/aries-framework-go/pkg/store/connection"
	"github.com/hyperledger/aries-framework-go/pkg/store/did"
)

const (
	myNSPrefix = "my"
	// TODO: https://github.com/hyperledger/aries-framework-go/issues/556 It will not be constant, this namespace
	//  will need to be figured with verification key
	theirNSPrefix = "their"
)

// newConnectionStore returns new connection store instance
func newConnectionStore(p provider) (*connectionStore, error) {
	recorder, err := connection.NewRecorder(p)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize connection recorder: %w", err)
	}

	didConnStore, err := did.New(p)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize did connection store: %w", err)
	}

	return &connectionStore{Recorder: recorder, Store: didConnStore}, nil
}

// connectionStore takes care of connection and DID related persistence features
// TODO merge connection stores [Issue #1004]
type connectionStore struct {
	*connection.Recorder
	*did.Store
}

// saveConnectionRecord saves the connection record against the connection id  in the store
func (c *connectionStore) saveConnectionRecord(record *connection.Record) error {
	err := c.SaveConnectionRecord(record)
	if err != nil {
		return fmt.Errorf(" failed to save connection record : %w", err)
	}

	if record.State == stateNameCompleted {
		if err := c.SaveDIDByResolving(record.TheirDID, record.RecipientKeys...); err != nil {
			return fmt.Errorf(" failed to save DID by resolving : %w", err)
		}
	}

	return nil
}

// saveConnectionRecordWithMapping saves newly created connection record against the connection id in the store
// and it creates mapping from namespaced ThreadID to connection ID
func (c *connectionStore) saveConnectionRecordWithMapping(record *connection.Record) error {
	err := c.SaveConnectionRecordWithMappings(record)
	if err != nil {
		return err
	}

	if record.MyDID != "" {
		if err := c.SaveDIDByResolving(record.MyDID); err != nil {
			return err
		}
	}

	return nil
}

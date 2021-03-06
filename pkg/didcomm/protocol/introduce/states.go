/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package introduce

import (
	"errors"
	"fmt"

	"github.com/hyperledger/aries-framework-go/pkg/didcomm/common/model"
	"github.com/hyperledger/aries-framework-go/pkg/didcomm/common/service"
	"github.com/hyperledger/aries-framework-go/pkg/didcomm/protocol/decorator"
	"github.com/hyperledger/aries-framework-go/pkg/didcomm/protocol/didexchange"
)

const (
	// common states
	stateNameNoop       = "noop"
	stateNameStart      = "start"
	stateNameAbandoning = "abandoning"
	stateNameDone       = "done"

	// introducer states
	stateNameArranging  = "arranging"
	stateNameDelivering = "delivering"
	stateNameConfirming = "confirming"

	// introducee states
	stateNameRequesting = "requesting"
	stateNameDeciding   = "deciding"
	stateNameWaiting    = "waiting"
)

// The introduce protocol's state.
type state interface {
	// Name of this state.
	Name() string
	// Whether this state allows transitioning into the next state.
	CanTransitionTo(next state) bool
	// Executes this state, returning a followup state to be immediately executed as well.
	// The 'noOp' state should be returned if the state has no followup.
	ExecuteInbound(messenger service.Messenger, msg *metaData) (followup state, err error)
	ExecuteOutbound(messenger service.Messenger, msg *metaData) (followup state, err error)
}

// noOp state
type noOp struct {
}

func (s *noOp) Name() string {
	return stateNameNoop
}

func (s *noOp) CanTransitionTo(_ state) bool {
	return false
}

func (s *noOp) ExecuteInbound(_ service.Messenger, _ *metaData) (state, error) {
	return nil, errors.New("cannot execute no-op")
}

func (s *noOp) ExecuteOutbound(_ service.Messenger, _ *metaData) (state, error) {
	return nil, errors.New("cannot execute no-op")
}

// start state
type start struct {
}

func (s *start) Name() string {
	return stateNameStart
}

func (s *start) CanTransitionTo(next state) bool {
	// Introducer can go to arranging or delivering state
	// Introducee can go to deciding
	switch next.Name() {
	case stateNameArranging, stateNameDeciding, stateNameRequesting, stateNameAbandoning:
		return true
	}

	return false
}

func (s *start) ExecuteInbound(_ service.Messenger, _ *metaData) (state, error) {
	return nil, errors.New("start: ExecuteInbound function is not supposed to be used")
}

func (s *start) ExecuteOutbound(_ service.Messenger, _ *metaData) (state, error) {
	return nil, errors.New("start: ExecuteOutbound function is not supposed to be used")
}

// done state
type done struct {
}

func (s *done) Name() string {
	return stateNameDone
}

func (s *done) CanTransitionTo(next state) bool {
	// done is the last state there is no possibility for the next state
	return false
}

func (s *done) ExecuteInbound(_ service.Messenger, _ *metaData) (state, error) {
	return &noOp{}, nil
}

func (s *done) ExecuteOutbound(_ service.Messenger, _ *metaData) (state, error) {
	return nil, errors.New("done: ExecuteOutbound function is not supposed to be used")
}

// arranging state
type arranging struct {
}

func (s *arranging) Name() string {
	return stateNameArranging
}

func (s *arranging) CanTransitionTo(next state) bool {
	return next.Name() == stateNameArranging || next.Name() == stateNameDone ||
		next.Name() == stateNameAbandoning || next.Name() == stateNameDelivering
}

func (s *arranging) ExecuteInbound(messenger service.Messenger, m *metaData) (state, error) {
	// after receiving a response we need to determine whether it is skip proposal or no
	// if this is skip proposal we do not need to send a proposal to another introducee
	// we just simply go to Delivering state
	if m.Msg.Type() == ResponseMsgType && isSkipProposal(m) {
		return &delivering{}, nil
	}

	if approve, ok := getApproveFromMsg(m.Msg); ok && !approve {
		return &abandoning{}, nil
	}

	var recipient *Recipient

	// sends Proposal according to the WaitCount
	if m.WaitCount == initialWaitCount {
		recipient = m.Recipients[0]
	} else {
		recipient = m.Recipients[1]
	}

	// TODO: Send should be replaced with ReplyTo. [Issue #1159]
	return &noOp{}, messenger.Send(service.NewDIDCommMsgMap(Proposal{
		Type:   ProposalMsgType,
		To:     recipient.To,
		Thread: &decorator.Thread{ID: m.ThreadID},
	}), recipient.MyDID, recipient.TheirDID)
}

func (s *arranging) ExecuteOutbound(messenger service.Messenger, m *metaData) (state, error) {
	if err := messenger.Send(m.Msg.(service.DIDCommMsgMap), m.myDID, m.theirDID); err != nil {
		return nil, fmt.Errorf("arranging: Send: %w", err)
	}

	return &noOp{}, nil
}

// delivering state
type delivering struct {
}

func (s *delivering) Name() string {
	return stateNameDelivering
}

func (s *delivering) CanTransitionTo(next state) bool {
	return next.Name() == stateNameConfirming || next.Name() == stateNameDone || next.Name() == stateNameAbandoning
}

// toDestIDx returns destination index based on introducee index
func toDestIDx(idx int) int {
	if idx == 0 {
		return 1
	}

	return 0
}

func getApproveFromMsg(msg service.DIDCommMsg) (bool, bool) {
	if msg.Type() != ResponseMsgType {
		return false, false
	}

	r := Response{}

	if err := msg.Decode(&r); err != nil {
		return false, false
	}

	return r.Approve, true
}

func sendProblemReport(messenger service.Messenger, m *metaData, recipients []*Recipient) (state, error) {
	for _, recipient := range recipients {
		// TODO: add description code to the ProblemReport message [Issues #1160]
		problem := service.NewDIDCommMsgMap(model.ProblemReport{Type: ProblemReportMsgType})

		if err := messenger.ReplyToNested(m.ThreadID, problem, recipient.MyDID, recipient.TheirDID); err != nil {
			return nil, fmt.Errorf("send problem-report: %w", err)
		}
	}

	return &done{}, nil
}

func deliveringSkipInvitation(messenger service.Messenger, m *metaData, recipients []*Recipient) (state, error) {
	// for skip proposal, we always have only one recipient e.g recipients[0]
	err := messenger.ReplyToNested(m.ThreadID,
		service.NewDIDCommMsgMap(m.dependency.Invitation()),
		recipients[0].MyDID, recipients[0].TheirDID,
	)
	if err != nil {
		return nil, fmt.Errorf("send inbound invitation (skip): %w", err)
	}

	return &done{}, nil
}

func (s *delivering) ExecuteInbound(messenger service.Messenger, m *metaData) (state, error) {
	if approve, ok := getApproveFromMsg(m.Msg); ok && !approve {
		return &abandoning{}, nil
	}

	if isSkipProposal(m) {
		return deliveringSkipInvitation(messenger, m, m.Recipients)
	}

	// edge case: no one shared the invitation
	if m.Invitation == nil {
		return &abandoning{}, nil
	}

	recipient := m.Recipients[toDestIDx(m.IntroduceeIndex)]

	msgMap := service.NewDIDCommMsgMap(m.Invitation)

	if err := messenger.ReplyToNested(m.ThreadID, msgMap, recipient.MyDID, recipient.TheirDID); err != nil {
		return nil, fmt.Errorf("send inbound invitation: %w", err)
	}

	return &confirming{}, nil
}

func (s *delivering) ExecuteOutbound(_ service.Messenger, _ *metaData) (state, error) {
	return nil, errors.New("delivering: ExecuteOutbound function is not supposed to be used")
}

// confirming state
type confirming struct {
}

func (s *confirming) Name() string {
	return stateNameConfirming
}

func (s *confirming) CanTransitionTo(next state) bool {
	return next.Name() == stateNameDone || next.Name() == stateNameAbandoning
}

func (s *confirming) ExecuteInbound(messenger service.Messenger, m *metaData) (state, error) {
	recipient := m.Recipients[m.IntroduceeIndex]

	msgMap := service.NewDIDCommMsgMap(model.Ack{
		Type:   AckMsgType,
		Thread: &decorator.Thread{ID: m.ThreadID},
	})

	// TODO: Send should be replaced with ReplyTo. [Issue #1159]
	if err := messenger.Send(msgMap, recipient.MyDID, recipient.TheirDID); err != nil {
		return nil, fmt.Errorf("send ack: %w", err)
	}

	return &done{}, nil
}

func (s *confirming) ExecuteOutbound(_ service.Messenger, _ *metaData) (state, error) {
	return nil, errors.New("confirming: ExecuteOutbound function is not supposed to be used")
}

// abandoning state
type abandoning struct {
}

func (s *abandoning) Name() string {
	return stateNameAbandoning
}

func (s *abandoning) CanTransitionTo(next state) bool {
	return next.Name() == stateNameDone
}

func fillRecipient(recipients []*Recipient, m *metaData) []*Recipient {
	// for the first recipient, we may do not have a destination
	// in that case, we need to get destination from the inbound message
	// NOTE: it happens after receiving the Request message.
	if len(recipients) == 0 {
		return append(recipients, &Recipient{
			MyDID:    m.myDID,
			TheirDID: m.theirDID,
		})
	}

	if recipients[0].MyDID == "" {
		recipients[0].MyDID = m.myDID
	}

	if recipients[0].TheirDID == "" {
		recipients[0].TheirDID = m.theirDID
	}

	return recipients
}

func (s *abandoning) ExecuteInbound(messenger service.Messenger, m *metaData) (state, error) {
	var recipients []*Recipient

	if m.Msg.Type() == RequestMsgType {
		recipients = fillRecipient(nil, m)
	}

	if m.Msg.Type() == ResponseMsgType {
		recipients = fillRecipient(m.Recipients, m)
	}

	if approve, ok := getApproveFromMsg(m.Msg); ok && !approve {
		if m.WaitCount == 1 {
			return &done{}, nil
		}
		// if we receive the second Response with Approve=false
		// report-problem should be sent only to the first introducee
		recipients = recipients[:1]
	}

	return sendProblemReport(messenger, m, recipients)
}

func (s *abandoning) ExecuteOutbound(_ service.Messenger, _ *metaData) (state, error) {
	return nil, errors.New("abandoning: ExecuteOutbound function is not supposed to be used")
}

// deciding state
type deciding struct {
}

func (s *deciding) Name() string {
	return stateNameDeciding
}

func (s *deciding) CanTransitionTo(next state) bool {
	return next.Name() == stateNameWaiting || next.Name() == stateNameDone || next.Name() == stateNameAbandoning
}

func (s *deciding) ExecuteInbound(messenger service.Messenger, m *metaData) (state, error) {
	var inv *didexchange.Invitation

	if m.dependency != nil {
		inv = m.dependency.Invitation()
	}

	var st state = &waiting{}
	if m.disapprove {
		st = &abandoning{}
	}

	msgMap := service.NewDIDCommMsgMap(Response{
		Type:       ResponseMsgType,
		Invitation: inv,
		Approve:    !m.disapprove,
	})

	return st, messenger.ReplyTo(m.Msg.ID(), msgMap)
}

func (s *deciding) ExecuteOutbound(_ service.Messenger, _ *metaData) (state, error) {
	return nil, errors.New("deciding: ExecuteOutbound function is not supposed to be used")
}

// waiting state
type waiting struct {
}

func (s *waiting) Name() string {
	return stateNameWaiting
}

func (s *waiting) CanTransitionTo(next state) bool {
	return next.Name() == stateNameDone || next.Name() == stateNameAbandoning
}

func (s *waiting) ExecuteInbound(_ service.Messenger, _ *metaData) (state, error) {
	return &noOp{}, nil
}

func (s *waiting) ExecuteOutbound(_ service.Messenger, _ *metaData) (state, error) {
	return nil, errors.New("waiting: ExecuteOutbound function is not supposed to be used")
}

// requesting state
type requesting struct {
}

func (s *requesting) Name() string {
	return stateNameRequesting
}

func (s *requesting) CanTransitionTo(next state) bool {
	return next.Name() == stateNameDeciding || next.Name() == stateNameAbandoning || next.Name() == stateNameDone
}

func (s *requesting) ExecuteInbound(_ service.Messenger, _ *metaData) (state, error) {
	return nil, errors.New("requesting: ExecuteInbound function is not supposed to be used")
}

func (s *requesting) ExecuteOutbound(messenger service.Messenger, m *metaData) (state, error) {
	return &noOp{}, messenger.Send(m.Msg.(service.DIDCommMsgMap), m.myDID, m.theirDID)
}

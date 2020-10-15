// Copyright 2020 The gVisor Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package tcp

import "gvisor.dev/gvisor/pkg/tcpip/seqnum"

// sackRecovery stores the variables related to TCP SACK loss recovery
// algorithm.
//
// +stateify savable
type sackRecovery struct {
	s *sender
}

func newSACKRecovery(s *sender) *sackRecovery {
	return &sackRecovery{s: s}
}

func (sr *sackRecovery) Update() {
	sr.s.state = SACKRecovery
	sr.s.ep.stack.Stats().TCP.SACKRecovery.Increment()
}

// handleSACKRecovery implements the loss recovery phase as described in RFC6675
// section 5, step C.
func (sr *sackRecovery) handleSACKRecovery(limit int, end seqnum.Value) (dataSent bool) {
	sr.s.SetPipe()

	if smss := int(sr.s.ep.scoreboard.SMSS()); limit > smss {
		// Cap segment size limit to s.smss as SACK recovery requires
		// that all retransmissions or new segments send during recovery
		// be of <= SMSS.
		limit = smss
	}

	nextSegHint := sr.s.writeList.Front()
	for sr.s.outstanding < sr.s.sndCwnd {
		var nextSeg *segment
		var rescueRtx bool
		nextSeg, nextSegHint, rescueRtx = sr.s.NextSeg(nextSegHint)
		if nextSeg == nil {
			return dataSent
		}
		if !sr.s.isAssignedSequenceNumber(nextSeg) || sr.s.sndNxt.LessThanEq(nextSeg.sequenceNumber) {
			// New data being sent.

			// Step C.3 described below is handled by
			// maybeSendSegment which increments sndNxt when
			// a segment is transmitted.
			//
			// Step C.3 "If any of the data octets sent in
			// (C.1) are above HighData, HighData must be
			// updated to reflect the transmission of
			// previously unsent data."
			//
			// We pass s.smss as the limit as the Step 2) requires that
			// new data sent should be of size s.smss or less.
			if sent := sr.s.maybeSendSegment(nextSeg, limit, end); !sent {
				return dataSent
			}
			dataSent = true
			sr.s.outstanding++
			sr.s.writeNext = nextSeg.Next()
			continue
		}

		// Now handle the retransmission case where we matched either step 1,3 or 4
		// of the NextSeg algorithm.
		// RFC 6675, Step C.4.
		//
		// "The estimate of the amount of data outstanding in the network
		// must be updated by incrementing pipe by the number of octets
		// transmitted in (C.1)."
		sr.s.outstanding++
		dataSent = true
		sr.s.sendSegment(nextSeg)

		segEnd := nextSeg.sequenceNumber.Add(nextSeg.logicalLen())
		if rescueRtx {
			// We do the last part of rule (4) of NextSeg here to update
			// RescueRxt as until this point we don't know if we are going
			// to use the rescue transmission.
			sr.s.fr.rescueRxt = sr.s.fr.last
		} else {
			// RFC 6675, Step C.2
			//
			// "If any of the data octets sent in (C.1) are below
			// HighData, HighRxt MUST be set to the highest sequence
			// number of the retransmitted segment unless NextSeg ()
			// rule (4) was invoked for this retransmission."
			sr.s.fr.highRxt = segEnd - 1
		}
	}
	return dataSent
}

func (sr *sackRecovery) DoRecovery(seg *segment, rtx bool) {
	if rtx {
		sr.s.resendSegment()
	}

	ack := seg.ackNumber
	// We are in fast recovery mode. Ignore the ack if it's out of
	// range.
	if !ack.InRange(sr.s.sndUna, sr.s.sndNxt+1) {
		return
	}

	// RFC 6675 recovery algorithm step C 1-5.
	end := sr.s.sndUna.Add(sr.s.sndWnd)
	dataSent := sr.handleSACKRecovery(sr.s.maxPayloadSize, end)
	if dataSent {
		// We sent data, so we should stop the keepalive timer to ensure
		// that no keepalives are sent while there is pending data.
		sr.s.ep.disableKeepaliveTimer()
	}

	// If the sender has advertized zero receive window and we have
	// data to be sent out, start zero window probing to query the
	// the remote for it's receive window size.
	if sr.s.writeNext != nil && sr.s.sndWnd == 0 {
		sr.s.enableZeroWindowProbing()
	}

	// Enable the timer if we have pending data and it's not enabled yet.
	if !sr.s.resendTimer.enabled() && sr.s.sndUna != sr.s.sndNxt {
		sr.s.resendTimer.enable(sr.s.rto)
	}
	// If we have no more pending data, start the keepalive timer.
	if sr.s.sndUna == sr.s.sndNxt {
		sr.s.ep.resetKeepaliveTimer(false)
	}
}

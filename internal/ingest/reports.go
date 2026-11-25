// Package ingest connects native evidence to the one canonical result ledger.
package ingest

import (
	"networkmemories/nm-outbreak-go/internal/core"
	"networkmemories/nm-outbreak-go/internal/reports"
)

type Sink struct {
	Engine      *core.Engine
	ArchiveSink reports.Sink
}

func (s *Sink) Archive(report reports.Envelope) error {
	var session core.Session
	eligible := false
	if report.Protocol == "snap" && report.Command == 0x6301 && report.Normalized != nil && report.PayloadLogin == report.ClaimedLogin {
		if actor, round, err := s.Engine.APPResultContext(report.Game, report.ClaimedLogin, report.RemoteHost); err == nil {
			session = actor
			report.OwnerID = actor.Account.UserID
			report.SessionID = actor.ID
			report.RoundID = round
			eligible = true
		}
	}
	if report.Protocol == "kddi" && report.Command == 0x6138 && report.JapaneseFile2 != nil {
		if actor, round, err := s.Engine.KDDIReportRound(report.SessionID, string(report.JapaneseFile2.GameSession)); err == nil && actor.Game == report.Game {
			report.OwnerID = actor.Account.UserID
			report.RoundID = round
		}
	}
	// Japanese 6138 is structurally decoded. Its result values stay in the
	// archive until native runs establish which values qualify for the ledger.
	if err := s.ArchiveSink.Archive(report); err != nil {
		return err
	}
	if eligible {
		return s.Engine.RecordResult(session.ID, *report.Normalized)
	}
	return nil // receipt only; unattributed/opaque uploads do not touch rankings.
}

package domain

// The health gate, applied again when the feed is READ — not only when a
// signal is stored.
//
// The ingest gate (SignalWatch.Matches) has been right since it shipped. The
// problem it cannot solve is the stock: on 15/09/2026 the account's feed held
// 62 signals of which 45 failed the gate, and the twenty the screen actually
// showed were a drone survey, two orders of braided safety netting, car
// servicing, room thermometers, "televisions and ceiling fans", and six budget
// entries from a city council that were not purchases at all.
//
// None of those got in past the gate. They arrived while production was
// running a pre-gate image — every automated API deploy had been broken since
// 01/09 by a directory owned by a uid that does not exist on that machine, so
// the gate merged on 09/09 and reached production days later. By then the rows
// were stored, and nothing in this system ever looks at a stored row again.
//
// That is the actual defect, and it outlives this particular batch: the gate
// has changed three times in a week and will change again, and every change
// leaves the whole existing stock behind. Filtering on read is what makes the
// feed heal itself — a row that stops qualifying disappears, and a row that
// starts qualifying comes back.
//
// Deliberately NOT a delete and NOT a dismissal:
//
//   - `dismissed_at` means a HUMAN said "not for us". Stamping it in bulk
//     would put words in the seller's mouth and turn the dismissal rate — the
//     one number that says whether the radar is improving — into fiction.
//   - A delete is irreversible, and the gate has false negatives too: among
//     the 45 rejected sat "Aquisição de Luva 6,5 e Dispositivo para infusão",
//     which is unambiguously a health purchase. Filtering gives that one back
//     the day the gate learns "infusão"; deleting loses it for good.
//
// (Most of the other `luva` rows were correctly rejected, and for a reason
// worth writing down: in Portuguese a *luva* is both a glove and a pipe
// coupling. "LUVA DE REDUÇÃO EM PVC" is plumbing.)

// SignalPassesDerivedGate reports whether a stored signal still deserves a
// place in the feed.
//
// fromDerivedWatch says the row was collected by a watch this system built out
// of the account's own catalogue. A HAND-WRITTEN watch is exempt, exactly as
// it is at ingest: somebody typed those words on purpose and the product has
// no business second-guessing them.
//
// Pushed sources are exempt too. A signal delivered by an integration the
// account pays for was claimed by the act of pushing it; it will never have a
// watch row, and judging it by a health heuristic would empty every vendor
// feed the product supports.
func SignalPassesDerivedGate(s *Signal, fromDerivedWatch bool) bool {
	if s == nil {
		return false
	}
	if !fromDerivedWatch || !IsPolledSource(s.Source) {
		return true
	}
	return looksLikeHealthTender(s)
}

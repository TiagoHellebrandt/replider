package domain

import "context"

// Virus scanning for inbound attachments.
//
// The product asks people to open files sent by strangers it prospected
// itself. Storing those files and handing back a signed URL without ever
// looking at them makes the platform the delivery mechanism.

// ScanStatus is what is known about a stored file.
type ScanStatus string

const (
	// ScanPending is written the moment the file lands and before any
	// scanner has seen it. It is the DEFAULT so a crash between store and
	// scan leaves a file marked unknown rather than clean.
	ScanPending ScanStatus = "pending"
	// ScanClean means a scanner looked and found nothing.
	ScanClean ScanStatus = "clean"
	// ScanInfected means a scanner found something. The bytes are kept for
	// evidence but must never be handed to a browser.
	ScanInfected ScanStatus = "infected"
	// ScanSkipped means no scanner is configured, or the file was past the
	// size the scanner accepts. Distinct from clean ON PURPOSE: "we did not
	// look" and "we looked and it was fine" are different sentences, and
	// collapsing them is how a UI ends up claiming safety it never checked.
	ScanSkipped ScanStatus = "skipped"
	// ScanFailed means the scanner was reachable and errored. Also not
	// clean.
	ScanFailed ScanStatus = "failed"
)

// Downloadable reports whether a file in this state may be served to a user.
//
// Only a real clean verdict qualifies. Everything else — pending, skipped,
// failed, infected — is withheld and explained. Serving an unscanned file
// while calling it safe is the failure this exists to prevent; serving it
// while SAYING it is unscanned is a decision the reader can make, and that
// is a separate, explicit action in the UI.
func (s ScanStatus) Downloadable() bool { return s == ScanClean }

// ScanVerdict is one scanner's answer about one file.
type ScanVerdict struct {
	Status ScanStatus
	// Signature names what was found ("Eicar-Test-Signature"). Empty unless
	// Status is ScanInfected. Shown to the user, because "infectado" with no
	// name is impossible to argue with when it is a false positive.
	Signature string
}

// AttachmentScanner inspects bytes before anyone can download them.
//
// A narrow port with one method: the implementation is an external daemon
// that can be absent, slow or wrong, and none of that belongs in the use
// case. An unconfigured scanner is represented by a nil interface, and the
// caller records ScanSkipped rather than pretending.
type AttachmentScanner interface {
	// Scan returns a verdict. An error means the scan could not be
	// completed — callers must record ScanFailed, never ScanClean.
	Scan(ctx context.Context, data []byte) (ScanVerdict, error)
}

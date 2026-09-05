// SPDX-License-Identifier: GPL-3.0-or-later

package kw

import (
	"errors"
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// TestStreamError_CarriesEachBooksOwnCauseSentence is erratum E13 made
// falsifiable: the two documents give the SAME token DIFFERENT causes, and
// a diagnostic that quotes the wrong one quotes the wrong document.
func TestStreamError_CarriesEachBooksOwnCauseSentence(t *testing.T) {
	tests := []struct {
		token    string
		book     Book
		cause    string
		citation string
	}{
		{"E;", Book590, "A communication error occurred, such as an overrun or framing error during a serial data transmission", "590:110-112"},
		{"E;", Book480, "A communication error occurred, such as an overrun or framing error during a serial data transmission", "480:140-142"},
		{"O;", Book590, "A receive buffer overrun error occurred", "590:113"},
		{"O;", Book480, "Receive data was sent but processing was not completed", "480:143-144"},
	}
	for _, tt := range tests {
		t.Run(tt.token+" "+tt.book.String(), func(t *testing.T) {
			e := newStreamError(tt.token, tt.book)
			if e.Token != tt.token {
				t.Errorf("Token = %q, want %q", e.Token, tt.token)
			}
			if e.Book != tt.book {
				t.Errorf("Book = %v, want %v", e.Book, tt.book)
			}
			if e.Cause != tt.cause {
				t.Errorf("Cause = %q, want the book's own sentence %q", e.Cause, tt.cause)
			}
			if e.Citation != tt.citation {
				t.Errorf("Citation = %q, want %q", e.Citation, tt.citation)
			}
			msg := e.Error()
			for _, want := range []string{tt.token, tt.cause, tt.citation} {
				if !strings.Contains(msg, want) {
					t.Errorf("Error() = %q, want it to contain %q", msg, want)
				}
			}
			if !errors.Is(e, ErrStream) {
				t.Error("errors.Is(e, ErrStream) = false, want true")
			}
		})
	}
}

// TestStreamError_IsNeverARejection is the negative that carries the design.
// A stream-health token is a LINK failure; ErrRejected is a command refusal.
// Collapsing them is the conflation §"Acknowledgement conventions" exists to
// prevent, and a driver that reached for errors.Is(err, ErrRejected) must
// not find one here.
func TestStreamError_IsNeverARejection(t *testing.T) {
	for _, token := range []string{"E;", "O;"} {
		for _, b := range []Book{Book590, Book480} {
			e := newStreamError(token, b)
			if errors.Is(e, transport.ErrRejected) {
				t.Errorf("%s on %v matches transport.ErrRejected — a link failure is not a refusal", token, b)
			}
		}
	}
}

// TestStreamError_MustNotWrapAFrameTooLongError is FatalFramer's own
// contract clause, pinned here because the engine delivers the cause as an
// ordinary reader error as well as closing the port, and the consumer of
// that event tests for *FrameTooLongError first — a wrapped one would be
// reported as recoverable contamination rather than as the link having
// ended.
func TestStreamError_MustNotWrapAFrameTooLongError(t *testing.T) {
	e := newStreamError("E;", Book480)
	var tooLong *transport.FrameTooLongError
	if errors.As(e, &tooLong) {
		t.Error("a StreamError errors.As-matches *transport.FrameTooLongError — the engine would report the link's end as recoverable contamination")
	}
	if errors.Is(e, transport.ErrFrameTooLong) {
		t.Error("a StreamError matches transport.ErrFrameTooLong")
	}
}

// TestNewStreamError_RefusesAFrameThatIsNotAToken pins the constructor
// against being handed anything else: it is reached only from IsFatal,
// which has already recognised the token, and a nil return there would
// silently make a fatal frame ordinary.
func TestNewStreamError_RefusesAFrameThatIsNotAToken(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("newStreamError accepted a frame that is not a stream-health token")
		}
	}()
	_ = newStreamError("?;", Book590)
}

// TestRejectionError_NamesBothCausesAndTheTransientSentence pins the one
// thing a driver can honestly tell a user about "?;": that the document
// gives TWO indistinguishable causes for it, and that the document also
// says the message may not appear at all.
func TestRejectionError_NamesBothCausesAndTheTransientSentence(t *testing.T) {
	for _, tt := range []struct {
		book      Book
		citation  string
		transient string
	}{
		{Book590, "590:100-105", "590:106-108"},
		{Book480, "480:130-135", "480:136-138"},
	} {
		t.Run(tt.book.String(), func(t *testing.T) {
			e := NewRejectionError(tt.book, "MC007;")
			if !errors.Is(e, transport.ErrRejected) {
				t.Error("errors.Is(e, transport.ErrRejected) = false — the typed rejection must WRAP the transport sentinel, not replace it")
			}
			msg := e.Error()
			for _, want := range []string{
				"MC007;",
				"Command syntax was incorrect",
				"Command was not executed due to the current status of the transceiver",
				"may not appear due to microprocessor transients",
				tt.citation,
				tt.transient,
			} {
				if !strings.Contains(msg, want) {
					t.Errorf("Error() = %q, want it to contain %q", msg, want)
				}
			}
			var rej *RejectionError
			if !errors.As(error(e), &rej) {
				t.Error("errors.As could not recover *RejectionError")
			}
		})
	}
}

// TestTimeoutError_IsNeverAnInferenceOfAbsence is the sharpest of the three
// and the reason the family exists at all. Both books say the NAK itself is
// unreliable — "Occasionally, this message may not appear due to
// microprocessor transients in the transceiver" (590:106-108, 480:136-138)
// — so a read that times out carries NO information: it is neither "absent"
// nor "rejected". The message must say so, in the place a user reads it.
func TestTimeoutError_IsNeverAnInferenceOfAbsence(t *testing.T) {
	e := NewTimeoutError(Book590, "MR0007;")
	if !errors.Is(e, transport.ErrTimeout) {
		t.Error("errors.Is(e, transport.ErrTimeout) = false — the typed timeout must WRAP the transport sentinel")
	}
	if errors.Is(e, transport.ErrRejected) {
		t.Error("a timeout matches transport.ErrRejected — silence is not a refusal")
	}
	msg := e.Error()
	for _, want := range []string{"MR0007;", "590:106-108", "not evidence"} {
		if !strings.Contains(msg, want) {
			t.Errorf("Error() = %q, want it to contain %q", msg, want)
		}
	}
	var to *TimeoutError
	if !errors.As(error(e), &to) {
		t.Error("errors.As could not recover *TimeoutError")
	}
}

// SPDX-License-Identifier: GPL-3.0-or-later

package fakeic905

// defaultImageChannels is how many channels a fresh Radio holds, and
// defaultImageGroup is which group they sit in: group 0, the first of the
// printed "00 00 ~ 00 99: Memory channel group".
//
// The COUNT is invented. Nothing in either artefact says how many channels an
// IC-905 ships occupied, or which. Ten is enough for a consumer to seed round,
// small enough to read in a test failure, and stated here so that no reader
// mistakes it for a fact. doc.go register entry 5.
const (
	defaultImageGroup    = 0
	defaultImageChannels = 10
)

// defaultImage is the channel map a Radio starts with when no WithRecord or
// WithEmpty option replaces part of it: ten occupied channels in group 0, each
// holding a record of the printed length, and nothing anywhere else.
//
// THE CONTENT IS INVENTED, AND IT IS ALL-ZERO. This is doc.go register entry 5,
// and the reason for the choice is worth stating: the reference guide prints
// each field's PERMITTED VALUES and never a shipped default, so there is
// nothing here to source a factory record from. Every byte of a default record
// is therefore this package's invention, and the least invention available is a
// record of zeros — a fake that shipped plausible-looking frequencies and call
// signs would be putting a fiction in front of anyone who ran the fake and read
// what came back.
//
// Two things follow, and both are deliberate:
//
//   - The zeros are not a claim that a real IC-905 answers zeros. They are a
//     placeholder, and a consumer that needs a record with content seeds one
//     with WithRecord, which is what that Option is for.
//   - The fake still does not interpret them. It does not know that byte 5 of
//     the printed block carries a fixed nibble, or that byte 13 is documented
//     as 00 or 01; it is storing sixty-four zeros because sixty-four zeros
//     invent the least, not because anything here read the legend.
//
// A FRESH MAP PER RADIO. The returned map is independent, so an Option applied
// to one fake cannot reach another's channels.
func defaultImage() map[chanAddr]MemState {
	img := make(map[chanAddr]MemState, defaultImageChannels)
	for ch := 0; ch < defaultImageChannels; ch++ {
		img[addrOf(defaultImageGroup, ch)] = MemState{Record: make([]byte, printedRecordLen)}
	}
	return img
}

// defaultIdentityToken is what a Radio answers "Read transceiver ID" with when
// no WithIdentityToken option pins something else.
//
// IT IS ARBITRARY, AND CONSPICUOUSLY SO. The reply's value is undocumented: the
// wire facts this package was built from give the command (cn 19, sc 00) and
// state that the request carries no data bytes, and say nothing whatever about
// what comes back. A fake that answered AC, or 09 05, or anything else with the
// shape of a fact would be asserting one nobody has, and every consumer that
// then matched on it would be testing this package's guess rather than its own
// driver.
//
// DE AD was chosen precisely because no reader could mistake it for a fact
// about an IC-905. doc.go register entry 4, and see WithIdentityToken, which
// exists so a consumer can prove its driver RECORDS whatever it gets rather
// than matching a particular value.
var defaultIdentityToken = []byte{0xDE, 0xAD}

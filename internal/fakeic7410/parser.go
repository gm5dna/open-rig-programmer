// SPDX-License-Identifier: GPL-3.0-or-later

package fakeic7410

// wireFrame and reassembler deliberately do not share production framing
// code; this package's own tests exercise the resulting independent byte
// grammar (matrix §1 row 2, §3.10 — the printed frame skeletons).
type wireFrame struct {
	raw      []byte
	to, from byte
	data     []byte
}
type reassembler struct{ b []byte }

// push feeds in bytes and returns every complete frame found: two or more
// 0xFE bytes (preamble), an address pair, data, and a terminating 0xFD.
// Data bytes are not escaped — the scan for a terminator looks only for
// the first 0xFD after the address pair, so an interior 0xFD truncates
// the frame there and the bytes after it are re-scanned from the next
// 0xFE 0xFE run, matching the printed framing the sibling fakes record.
func (a *reassembler) push(in []byte) []wireFrame {
	a.b = append(a.b, in...)
	var out []wireFrame
	for {
		start := -1
		for i := 0; i+1 < len(a.b); i++ {
			if a.b[i] == 0xfe && a.b[i+1] == 0xfe {
				start = i
				break
			}
		}
		if start < 0 {
			if len(a.b) > 0 && a.b[len(a.b)-1] == 0xfe {
				a.b = []byte{0xfe}
			} else {
				a.b = nil
			}
			break
		}
		a.b = a.b[start:]
		k := 0
		for k < len(a.b) && a.b[k] == 0xfe {
			k++
		}
		if len(a.b) < k+2 {
			break
		}
		end := -1
		for i := k + 2; i < len(a.b); i++ {
			if a.b[i] == 0xfd {
				end = i
				break
			}
		}
		if end < 0 {
			break
		}
		raw := append([]byte(nil), a.b[:end+1]...)
		out = append(out, wireFrame{raw: raw, to: a.b[k], from: a.b[k+1], data: append([]byte(nil), a.b[k+2:end]...)})
		a.b = append([]byte(nil), a.b[end+1:]...)
	}
	return out
}

func buildFrame(to, from byte, p ...byte) []byte {
	b := append([]byte{0xfe, 0xfe, to, from}, p...)
	return append(b, 0xfd)
}

// answer frames a reply to the controller that sent f from the radio
// address this Radio was configured with, which dispatch has already
// proved equal to f's destination byte. There is no literal source
// address here, so WithRadioAddress moves it on every reply, not just the
// listening address.
func (r *Radio) answer(f wireFrame, p ...byte) []byte {
	return buildFrame(f.from, r.addr, p...)
}

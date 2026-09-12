// SPDX-License-Identifier: GPL-3.0-or-later

package fakeic7700

// wireFrame and reassembler deliberately do not share production framing
// code; the tests in fakeic7700_test.go exercise the resulting independent
// byte grammar.
type wireFrame struct {
	raw      []byte
	to, from byte
	data     []byte
}
type reassembler struct{ b []byte }

// push appends data and returns every complete frame now available, in
// order. See doc.go, "Framing": extra leading 0xFE is preamble padding and
// the whole run is skipped; the first 0xFD after the address pair ends the
// frame, so data bytes are not escaped.
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

// buildFrame assembles FE FE <to> <from> <p...> FD. MANUAL-EVIDENCED shape
// (matrix §3.10, PDF p.202).
func buildFrame(to, from byte, p ...byte) []byte {
	b := append([]byte{0xfe, 0xfe, to, from}, p...)
	return append(b, 0xfd)
}

// answer frames a reply to the controller that sent f, from the radio
// address this Radio was configured with, which the dispatch filter has
// already proved equal to f's destination byte.
func (r *Radio) answer(f wireFrame, p ...byte) []byte {
	return buildFrame(f.from, r.addr, p...)
}

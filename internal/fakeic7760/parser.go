package fakeic7760

type parsed struct {
	to, from byte
	payload  []byte
}

func parseFrame(b []byte) (parsed, bool) {
	if len(b) < 7 || b[0] != 0xFE || b[1] != 0xFE || b[len(b)-1] != 0xFD {
		return parsed{}, false
	}
	return parsed{b[2], b[3], append([]byte(nil), b[4:len(b)-1]...)}, true
}

type reassembler struct {
	buf []byte
	max int
}

func newReassembler(n int) *reassembler { return &reassembler{max: n} }
func (a *reassembler) push(in []byte) [][]byte {
	var out [][]byte
	for _, b := range in {
		if len(a.buf) == 0 {
			if b == 0xFE {
				a.buf = []byte{b}
			}
			continue
		}
		if len(a.buf) == 1 {
			if b == 0xFE {
				a.buf = append(a.buf, b)
			} else {
				a.buf = nil
			}
			continue
		}
		if b == 0xFE {
			a.buf = []byte{b}
			continue
		}
		a.buf = append(a.buf, b)
		if len(a.buf) > a.max {
			a.buf = nil
			continue
		}
		if b == 0xFD {
			out = append(out, append([]byte(nil), a.buf...))
			a.buf = nil
		}
	}
	return out
}

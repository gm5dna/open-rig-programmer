package fakeic7851

// wireFrame and reassembler deliberately do not share production framing
// code; TestMemoryReadWriteAndDirection and TestEchoAndFloodAreConfigurable
// exercise the resulting independent byte grammar.
type wireFrame struct {
	raw      []byte
	to, from byte
	data     []byte
}
type reassembler struct{ b []byte }

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
func buildAnswer(p ...byte) []byte { return buildFrame(0xe0, 0x8e, p...) }

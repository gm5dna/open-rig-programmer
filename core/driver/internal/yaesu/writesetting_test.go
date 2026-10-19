// SPDX-License-Identifier: GPL-3.0-or-later

package yaesu

import (
	"context"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gm5dna/open-rig-programmer/core/cat"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// scriptPort is a recording transport.Port: every ';'-terminated frame
// written is logged and answered with whatever reply returns for it.
type scriptPort struct {
	mu     sync.Mutex
	writes []string
	buf    string
	reply  func(frame string) string
	rx     chan []byte
	done   chan struct{}
}

func newScriptPort(reply func(string) string) *scriptPort {
	return &scriptPort{reply: reply, rx: make(chan []byte, 16), done: make(chan struct{})}
}

func (p *scriptPort) Write(b []byte) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.buf += string(b)
	for {
		i := strings.IndexByte(p.buf, ';')
		if i < 0 {
			return len(b), nil
		}
		frame := p.buf[:i+1]
		p.buf = p.buf[i+1:]
		p.writes = append(p.writes, frame)
		if r := p.reply(frame); r != "" {
			p.rx <- []byte(r)
		}
	}
}

func (p *scriptPort) Read(b []byte) (int, error) {
	select {
	case d := <-p.rx:
		return copy(b, d), nil
	case <-p.done:
		return 0, io.EOF
	}
}

func (p *scriptPort) Close() error { close(p.done); return nil }

func (p *scriptPort) sent() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]string(nil), p.writes...)
}

// TestWriteSetting_FourOutcomes drives the shared body against a recording
// port for the four outcomes: written and verified, refused ("?;"), verify
// mismatch, and pre-flight refusal (no wire traffic at all).
func TestWriteSetting_FourOutcomes(t *testing.T) {
	const id, set = "010101", "EX010101123;"
	addr, err := cat.FT710.ParseEXAddress(id)
	if err != nil {
		t.Fatal(err)
	}
	dialect := cat.DialectWithEXWriteForTest(cat.FT710, addr, cat.Domain{Lo: 0, Hi: 999, Step: 1}, 3)
	p := &Params{Name: "test", Model: "TEST"}

	run := func(t *testing.T, value string, reply func(string) string) (driver.SettingWriteResult, error, []string) {
		t.Helper()
		port := newScriptPort(reply)
		eng, err := transport.NewEngine(port, dialect)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = eng.Close() })
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		res, err := WriteSetting(ctx, eng, dialect, p, id, value)
		return res, err, port.sent()
	}
	readBack := func(v string) func(string) string {
		return func(f string) string {
			if f == "EX010101;" {
				return "EX010101" + v + ";"
			}
			return ""
		}
	}

	t.Run("accepted", func(t *testing.T) {
		res, err, sent := run(t, "123", readBack("123"))
		if err != nil || res.Outcome != driver.SettingWriteAccepted || res.Observed != "123" {
			t.Errorf("res=%+v err=%v", res, err)
		}
		if len(sent) != 2 || sent[0] != set || sent[1] != "EX010101;" {
			t.Errorf("sent %q, want the Set then the read", sent)
		}
	})
	t.Run("refused", func(t *testing.T) {
		res, err, _ := run(t, "123", func(f string) string {
			if f == set {
				return "?;"
			}
			return ""
		})
		if !errors.Is(err, cat.ErrRejected) || res.Outcome != driver.SettingWriteRefused || !res.Step.Sent {
			t.Errorf("res=%+v err=%v", res, err)
		}
	})
	t.Run("verify mismatch", func(t *testing.T) {
		res, err, _ := run(t, "123", readBack("000"))
		var mm *driver.SettingVerifyMismatchError
		if !errors.As(err, &mm) || res.Outcome != driver.SettingWriteVerifyMismatch || mm.Observed != "000" {
			t.Errorf("res=%+v err=%v", res, err)
		}
	})
	t.Run("pre-flight refusal", func(t *testing.T) {
		for _, value := range []string{"1000", "12", "abc"} {
			res, err, sent := run(t, value, readBack("123"))
			if err == nil || res != (driver.SettingWriteResult{}) || len(sent) != 0 {
				t.Errorf("value %q: res=%+v err=%v sent=%q, want a zero result, an error and no traffic", value, res, err, sent)
			}
		}
	})
}

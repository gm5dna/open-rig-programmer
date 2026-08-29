// SPDX-License-Identifier: GPL-3.0-or-later

package spec

import (
	"strings"
	"testing"
)

func TestTransmitValidation(t *testing.T) {
	t.Run("unspecified is refused", func(t *testing.T) {
		c := validTestCapabilities()
		c.Transmit = TransmitUnspecified
		if err := c.Validate(); err == nil || !strings.Contains(err.Error(), "Transmit must be declared") {
			t.Fatalf("Validate() = %v, want Transmit declaration error", err)
		}
	})

	for _, field := range []Field{FieldTxFrequency, FieldToneTx} {
		t.Run("receive-only refuses "+string(field), func(t *testing.T) {
			c := validTestCapabilities()
			c.Transmit = ReceiveOnly
			c.Banks[0].Fields[field] = FieldSupport{Read: Supported}
			if err := c.Validate(); err == nil || !strings.Contains(err.Error(), string(field)) || !strings.Contains(err.Error(), "ReceiveOnly") {
				t.Fatalf("Validate() = %v, want ReceiveOnly %s error", err, field)
			}
		})
	}

	t.Run("receive-only refuses a transmitting tone semantic", func(t *testing.T) {
		c := validTestCapabilities()
		c.Transmit = ReceiveOnly
		c.ToneModes = []ToneMode{{Value: "TONE", Semantics: ToneModeCTCSS}}
		if err := c.Validate(); err == nil || !strings.Contains(err.Error(), "NeedsTxTone") {
			t.Fatalf("Validate() = %v, want NeedsTxTone error", err)
		}
	})

	t.Run("receive-only receive squelch is valid", func(t *testing.T) {
		c := validTestCapabilities()
		c.Transmit = ReceiveOnly
		c.ToneModes = []ToneMode{{Value: "TSQL", Semantics: ToneModeCTCSSRxSquelch}}
		if err := c.Validate(); err != nil {
			t.Fatalf("Validate() = %v", err)
		}
	})
}

func TestToneModeCTCSSRxSquelchNeedsOnlyReceiveTone(t *testing.T) {
	m := ToneMode{Value: "TSQL", Semantics: ToneModeCTCSSRxSquelch}
	if m.NeedsTxTone() {
		t.Error("ToneModeCTCSSRxSquelch.NeedsTxTone() = true")
	}
	if !m.NeedsRxTone() {
		t.Error("ToneModeCTCSSRxSquelch.NeedsRxTone() = false")
	}
}

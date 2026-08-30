// SPDX-License-Identifier: GPL-3.0-or-later

package ic7851_test

import (
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var frozenSHA256 = map[string]string{
	"IC-7851-field-ledger.csv":       "8a1a68264f19334996e2a3fee247bde80032c2ed3a90081e4fa2d26fa460ecea",
	"IC-7851-field-ledger.md":        "a98fe3d04670ff93123e804e9a28da1fbcff5f4be8fc368fc77e6984ca32c494",
	"IC-7851-geometry-witness.csv":   "7a42821e6d49cdb62bc1e57ae7800f8294c92b84847a185e04bdea2f0acab7ab",
	"IC-7851-geometry-witness.md":    "3f325a5e3653f502a43965a5b1eea8c7915cb558e2d0c89c88237ea2a08ff1ba",
	"IC-7851-golden-assumptions.csv": "8f78ab66e9b7479fc379578a6c5f882e9683c5c27ab8d95e1b9484b04b3c1c30",
	"IC-7851-golden-provenance.md":   "52cb56c4674d153d411b051068c6bd68bebc6aaa875d7f370b139067d1fdd449",
	"IC-7851-transcription-b.csv":    "a383134e6a251fb584bb13fe58925704c03292bdcc99b153fd48335d1d0a99f7",
	"IC-7851-transcription-b.md":     "27db66d645b5509d16145f208d9c7808d38ca623e6e403237705357b88423018",
	"IC-7851-vectors.golden":         "7b0b2fb57c7fa6ccd4373dd181dd2df9432f99cbf7a18fd536b1e711036a63c3",
}

func TestEvidenceFrozen(t *testing.T) {
	for name, want := range frozenSHA256 {
		data, err := os.ReadFile(filepath.Join("testdata", name))
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(data)
		got := hex.EncodeToString(sum[:])
		if got != want {
			t.Errorf("frozen evidence %s changed: got %s want %s", name, got, want)
		}
	}
	seen := 0
	err := filepath.WalkDir("testdata", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !(strings.HasSuffix(d.Name(), ".csv") || strings.HasSuffix(d.Name(), ".md") || strings.HasSuffix(d.Name(), ".golden")) {
			return nil
		}
		seen++
		if _, ok := frozenSHA256[d.Name()]; !ok {
			t.Errorf("unmanifested evidence %s", path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if seen != len(frozenSHA256) {
		t.Errorf("evidence count = %d, manifest = %d", seen, len(frozenSHA256))
	}
}

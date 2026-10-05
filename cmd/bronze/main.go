package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ShafiudeenKameel/exchange-recon-pipeline/internal/classify"
	"github.com/ShafiudeenKameel/exchange-recon-pipeline/internal/fixtures"
)

// Lands mock exchange payloads UNTOUCHED into data/bronze/*.json
// (replayable source of truth — classification happens later).
func main() {
	root := "data/bronze"
	if err := os.MkdirAll(root, 0o755); err != nil {
		panic(err)
	}
	sets := map[string][]classify.RawRecord{
		"fixture1_fee_third_asset.json":  fixtures.Fixture1FeeInThirdAsset(),
		"fixture2_clean_internal.json":   fixtures.Fixture2CleanInternalTransfer(),
		"fixture3_false_match_trap.json": fixtures.Fixture3FalseMatchTrap(),
		"fixture4_multileg.json":         fixtures.Fixture4MultiLegOrder(),
	}
	for name, recs := range sets {
		path := filepath.Join(root, name)
		f, err := os.Create(path)
		if err != nil {
			panic(err)
		}
		enc := json.NewEncoder(f)
		enc.SetIndent("", "  ")
		if err := enc.Encode(recs); err != nil {
			panic(err)
		}
		_ = f.Close()
		fmt.Println("wrote", path)
	}
}

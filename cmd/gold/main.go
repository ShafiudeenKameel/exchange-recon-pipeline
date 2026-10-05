package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/ShafiudeenKameel/exchange-recon-pipeline/internal/classify"
	"github.com/ShafiudeenKameel/exchange-recon-pipeline/internal/fixtures"
	"github.com/ShafiudeenKameel/exchange-recon-pipeline/internal/gold"
	"github.com/ShafiudeenKameel/exchange-recon-pipeline/internal/pipeline"
)

func main() {
	which := "all"
	if len(os.Args) > 1 {
		which = os.Args[1]
	}
	jobs := map[string][]classify.RawRecord{
		"fixture1_fee_third_asset":  fixtures.Fixture1FeeInThirdAsset(),
		"fixture2_clean_internal":   fixtures.Fixture2CleanInternalTransfer(),
		"fixture3_false_match_trap": fixtures.Fixture3FalseMatchTrap(),
		"fixture4_multileg":         fixtures.Fixture4MultiLegOrder(),
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	for name, recs := range jobs {
		if which != "all" && which != name {
			continue
		}
		fmt.Printf("=== %s ===\n", name)
		_ = enc.Encode(gold.Publish(pipeline.Process(recs)))
	}
}

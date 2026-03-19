//go:build darwin

// boxxy-scan: work-stealing network scanner with Waymo detection
//
// Uses Chase-Lev deque per worker, GMP-style scheduling (1/61 global check),
// random peer stealing with bulk steal-half for amortization.
//
// BlackHat Go Ch.2 (TCP), Ch.33 (Reconnaissance), ATT&CK TA0043
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/bmorphism/boxxy/internal/scan"
)

func main() {
	iface := flag.String("i", "en0", "network interface")
	count := flag.Int("n", 500, "packet count")
	mode := flag.String("mode", "worksteal", "scan mode: errgroup or worksteal")
	narrativeDir := flag.String("narrative", "", "directory for narrative sheaf persistence (enables temporal tracking)")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	opts := scan.ScanOpts{Interface: *iface, PacketCount: *count}

	fmt.Fprintf(os.Stderr, "boxxy-scan: %s mode on %s (%d pkts)\n", *mode, *iface, *count)

	var result *scan.ScanResult
	var err error
	start := time.Now()

	switch *mode {
	case "worksteal", "ws":
		result, err = scan.RunWorkStealing(ctx, opts)
	case "errgroup", "eg":
		result, err = scan.Run(ctx, opts)
	default:
		result, err = scan.Run(ctx, opts)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.Encode(result)

	elapsed := time.Since(start)
	fmt.Fprintf(os.Stderr, "\n=== SCAN (%s, %.1fs) ===\n", *mode, elapsed.Seconds())
	fmt.Fprintf(os.Stderr, "Devices: %d | Waymo: %d | GF(3): %d\n",
		len(result.Devices), len(result.WaymoCands), result.GF3Sum)
	for _, d := range result.WaymoCands {
		trit := map[int]string{1: "+", 0: "0", -1: "-"}[d.GF3Trit]
		fmt.Fprintf(os.Stderr, "  [%s] %.0f%% %s %s [%s] %v\n",
			trit, d.WaymoScore*100, d.MAC, d.IP, d.Vendor, d.Flags)
	}
	if len(result.WaymoCands) == 0 {
		fmt.Fprintf(os.Stderr, "  (no Waymo candidates)\n")
	}

	// Narrative sheaf: accumulate temporal scan data
	if *narrativeDir != "" {
		narrative, loadErr := scan.LoadNarrative(*narrativeDir)
		if loadErr != nil {
			fmt.Fprintf(os.Stderr, "narrative: load error: %v\n", loadErr)
			os.Exit(1)
		}

		prevLen := len(narrative.Snapshots)
		narrative.AddSnapshot(result)

		// Verify sheaf condition (H⁰ obstruction check)
		violations := narrative.VerifySheafCondition()

		// Save updated narrative
		if saveErr := narrative.Save(*narrativeDir); saveErr != nil {
			fmt.Fprintf(os.Stderr, "narrative: save error: %v\n", saveErr)
			os.Exit(1)
		}

		fmt.Fprintf(os.Stderr, "\n=== NARRATIVE (sheaf on I_%d) ===\n", len(narrative.Snapshots))
		fmt.Fprintf(os.Stderr, "Snapshots: %d → %d | Intervals: %d\n",
			prevLen, len(narrative.Snapshots), len(narrative.Values))

		if narrative.IsBalanced() {
			fmt.Fprintf(os.Stderr, "GF(3) balance: F([0,%d]) = 0 ✓\n", len(narrative.Snapshots)-1)
		} else {
			v, _ := narrative.SheafValue(0, len(narrative.Snapshots)-1)
			fmt.Fprintf(os.Stderr, "GF(3) balance: F([0,%d]) = %d (unbalanced)\n",
				len(narrative.Snapshots)-1, v.GF3)
		}

		fixed := narrative.FrobeniusFixed()
		if len(fixed) > 0 {
			fmt.Fprintf(os.Stderr, "Frobenius-fixed intervals: %d (GF(3) ⊂ GF(9))\n", len(fixed))
		}

		if len(violations) > 0 {
			fmt.Fprintf(os.Stderr, "⚠ Sheaf violations: %d\n", len(violations))
			for _, v := range violations {
				fmt.Fprintf(os.Stderr, "  %s\n", v)
			}
		} else {
			fmt.Fprintf(os.Stderr, "Sheaf condition: H⁰ = 0 ✓\n")
		}
	}
}

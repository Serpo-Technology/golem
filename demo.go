package main

import (
	"fmt"
	"sync/atomic"
	"time"
)

// fastForwardNights compresses months of nightly consolidation into the
// demo printout.
const fastForwardNights = 90

// runDemo is the self-contained, in-RAM show: one synthetic channel, a
// burst of observations, a serotonin-triggered sleep, and a retrieval
// test. It exercises the same components the daemon runs, minus
// persistence, and exits on its own.
func runDemo() {
	cfg := DefaultConfig()

	fmt.Println("GOLEM waking up. Observing channel 'syslog'.")
	fmt.Println("Raw lines are normalized to templates, so a Ping with a fresh seq")
	fmt.Println("number is still the same Ping. Only new SHAPES reach consciousness.")
	fmt.Println("The log bursts for a few seconds, then goes quiet. Quiet builds")
	fmt.Println("serotonin, and high serotonin plus silent input triggers SLEEP.")
	fmt.Println("--------------------------------------------------------------------")

	obs := make(chan Observation, 16)
	stop := make(chan struct{})

	predictor := NewPredictor(cfg.PredictorWindow)
	chem := NewNeurochemistry()
	hippo := NewHippocampus()

	go LogSensor("syslog", obs, stop, new(atomic.Int64))

	homeostasis := time.NewTicker(200 * time.Millisecond)
	defer homeostasis.Stop()
	failsafe := time.NewTimer(30 * time.Second)
	defer failsafe.Stop()

	lastObs := time.Now()
	lastSurprise := time.Now()

	// The Global Workspace: a single select loop that is GOLEM's "consciousness."
	for {
		select {
		case o := <-obs:
			lastObs = time.Now()
			tpl := Normalize(o.Content)
			s := predictor.Surprise(tpl)
			predictor.Learn(tpl) // model updates toward reality

			if s < cfg.SurpriseThreshold {
				continue // Thalamus drops it. Nothing wakes up.
			}

			// Surprise! Spike alertness, push sleep away, and lay down a
			// memory of the moment.
			lastSurprise = time.Now()
			chem.SpikeNoradrenaline(s * naSpikePerBit)
			chem.SuppressSleep(sleepSuppressOnSurprise)
			frag := &Fragment{
				Channel:  o.Channel,
				Content:  o.Content,
				Template: tpl,
				Surprise: s,
				Limbic:   chem.Snapshot(),
				Time:     o.At,
			}
			id := hippo.Encode(frag)
			fmt.Printf("[!] %.2f bits  %-44q -> %q  NA=%.2f  #%d\n",
				s, o.Content, tpl, frag.Limbic.Noradrenaline, id)

		case <-homeostasis.C:
			chem.Decay() // noradrenaline and dopamine drift toward baseline
			if time.Since(lastSurprise) > cfg.CalmBeforeSleep() {
				chem.BuildSleepPressure()
			}
			if chem.Serotonin() >= cfg.SleepPressureGate && time.Since(lastObs) >= cfg.InputSilence() {
				close(stop)
				demoSleepAndRecall(cfg, hippo, chem, time.Since(lastObs))
				return
			}

		case <-failsafe.C:
			close(stop)
			fmt.Println("Failsafe timer elapsed; forcing the sleep cycle.")
			demoSleepAndRecall(cfg, hippo, chem, time.Since(lastObs))
			return
		}
	}
}

// demoSleepAndRecall runs consolidation (one night, then a fast-forward
// across many more) and finishes with a retrieval test that queries memory
// by feeling rather than by content.
func demoSleepAndRecall(cfg Config, hippo *Hippocampus, chem *Neurochemistry, quietFor time.Duration) {
	fmt.Println("--------------------------------------------------------------------")
	fmt.Printf("Input quiet for %.1fs, serotonin %.2f -> entering SLEEP. %d fragment(s) held.\n",
		quietFor.Seconds(), chem.Serotonin(), hippo.Count())

	if hippo.Count() == 0 {
		fmt.Println("  (nothing surprising happened this window; memory is empty)")
		return
	}

	pruned := hippo.Sleep(cfg.NightlyDecay, cfg.PruneFloor)
	fmt.Printf("Night 1: weights decayed, surprising memories resisting; %d pruned.\n", pruned)

	for range fastForwardNights {
		pruned += hippo.Sleep(cfg.NightlyDecay, cfg.PruneFloor)
	}
	fmt.Printf("Fast-forward %d more nights: %d pruned in total, %d remain.\n",
		fastForwardNights, pruned, hippo.Count())

	for _, f := range hippo.Dump() {
		fmt.Printf("  #%-2d %.2f bits  weight %.2f  %q\n", f.ID, f.Surprise, f.Weight, f.Content)
	}

	fmt.Println("\nRetrieval query \"show me when things felt wrong\":")
	if f := hippo.RecallByStress(); f != nil {
		fmt.Printf("  -> #%d %q (NA=%.2f at encoding); recall refreshed its weight to %.2f.\n",
			f.ID, f.Content, f.Limbic.Noradrenaline, f.Weight)
	} else {
		fmt.Println("  Nothing left in memory; every moment faded.")
	}
}

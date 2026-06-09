package main

import (
	"fmt"
	"time"
)

const (
	// surpriseThreshold is the bar (in bits) a template must clear to reach
	// consciousness. Below it, the Thalamus silently drops the data — this is
	// the ~0% CPU path that the whole design is built around.
	surpriseThreshold = 2.0

	// predictorWindow is the habituation timescale: roughly how many recent
	// observations the channel model remembers.
	predictorWindow = 256
)

func main() {
	fmt.Println("GOLEM waking up. Observing channel 'syslog'.")
	fmt.Println("Raw lines are normalized to templates, so a Ping with a fresh seq")
	fmt.Println("number is still the same Ping. Only new SHAPES reach consciousness.")
	fmt.Println("--------------------------------------------------------------------")

	obs := make(chan Observation, 16)
	stop := make(chan struct{})

	predictor := NewPredictor(predictorWindow)
	chem := NewNeurochemistry()
	hippo := NewHippocampus()

	go LogSensor(obs, stop)

	window := time.After(6 * time.Second)
	homeostasis := time.NewTicker(200 * time.Millisecond)
	defer homeostasis.Stop()

	// The Global Workspace: a single select loop that is GOLEM's "consciousness."
	for {
		select {
		case <-window:
			close(stop)
			sleepAndRecall(hippo)
			return

		case <-homeostasis.C:
			chem.Decay() // neurochemistry drifts back toward baseline each tick

		case o := <-obs:
			tpl := Normalize(o.Content)
			s := predictor.Surprise(tpl)
			predictor.Learn(tpl) // model updates toward reality

			if s < surpriseThreshold {
				continue // Thalamus drops it. Nothing wakes up.
			}

			// Surprise! Spike alertness and lay down a memory of the moment.
			chem.SpikeNoradrenaline(s * 0.12)
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
		}
	}
}

// sleepAndRecall runs the consolidation cycle and then a retrieval test that
// queries memory by feeling rather than by content.
func sleepAndRecall(hippo *Hippocampus) {
	fmt.Println("--------------------------------------------------------------------")
	fmt.Printf("Quiet input -> entering SLEEP. %d fragment(s) held.\n", hippo.Count())

	pruned := hippo.Sleep(0.10, 0.10)
	fmt.Printf("Consolidation: decayed all weights 10%%, pruned %d faded fragment(s).\n", pruned)

	fmt.Println("\nMemory after consolidation:")
	if hippo.Count() == 0 {
		fmt.Println("  (nothing surprising happened this window — CPU stayed near zero)")
	}
	for _, f := range hippo.Dump() {
		fmt.Printf("  #%d  %-20q  surprise=%.2f bits  weight=%.2f\n",
			f.ID, f.Content, f.Surprise, f.Weight)
	}

	fmt.Println("\nRetrieval test — \"show me when things felt wrong\":")
	if f := hippo.RecallByStress(); f != nil {
		fmt.Printf("  GOLEM recalls fragment #%d: %q (felt NA=%.2f at the time).\n",
			f.ID, f.Content, f.Limbic.Noradrenaline)
	} else {
		fmt.Println("  Nothing stressful in memory.")
	}
}

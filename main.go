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

	// sleepPressureGate is the serotonin level at which GOLEM is ready to
	// sleep, provided input has also paused for at least inputSilence.
	sleepPressureGate = 0.8
	inputSilence      = time.Second

	// calmBeforeSleep is how long after the last surprise sleep pressure
	// starts building. Routine, well-predicted input does not block it.
	calmBeforeSleep = 1500 * time.Millisecond

	// nightlyDecay and pruneFloor govern consolidation; fastForwardNights
	// compresses months of nightly cycles into the demo printout.
	nightlyDecay      = 0.10
	pruneFloor        = 0.10
	fastForwardNights = 90
)

func main() {
	fmt.Println("GOLEM waking up. Observing channel 'syslog'.")
	fmt.Println("Raw lines are normalized to templates, so a Ping with a fresh seq")
	fmt.Println("number is still the same Ping. Only new SHAPES reach consciousness.")
	fmt.Println("The log bursts for a few seconds, then goes quiet. Quiet builds")
	fmt.Println("serotonin, and high serotonin plus silent input triggers SLEEP.")
	fmt.Println("--------------------------------------------------------------------")

	obs := make(chan Observation, 16)
	stop := make(chan struct{})

	predictor := NewPredictor(predictorWindow)
	chem := NewNeurochemistry()
	hippo := NewHippocampus()

	go LogSensor(obs, stop)

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

			if s < surpriseThreshold {
				continue // Thalamus drops it. Nothing wakes up.
			}

			// Surprise! Spike alertness, push sleep away, and lay down a
			// memory of the moment.
			lastSurprise = time.Now()
			chem.SpikeNoradrenaline(s * 0.12)
			chem.SuppressSleep(0.15)
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
			if time.Since(lastSurprise) > calmBeforeSleep {
				chem.BuildSleepPressure()
			}
			if chem.Serotonin() >= sleepPressureGate && time.Since(lastObs) >= inputSilence {
				close(stop)
				sleepAndRecall(hippo, chem, time.Since(lastObs))
				return
			}

		case <-failsafe.C:
			close(stop)
			fmt.Println("Failsafe timer elapsed; forcing the sleep cycle.")
			sleepAndRecall(hippo, chem, time.Since(lastObs))
			return
		}
	}
}

// sleepAndRecall runs consolidation (one night, then a fast-forward across
// many more) and finishes with a retrieval test that queries memory by
// feeling rather than by content.
func sleepAndRecall(hippo *Hippocampus, chem *Neurochemistry, quietFor time.Duration) {
	fmt.Println("--------------------------------------------------------------------")
	fmt.Printf("Input quiet for %.1fs, serotonin %.2f -> entering SLEEP. %d fragment(s) held.\n",
		quietFor.Seconds(), chem.Serotonin(), hippo.Count())

	if hippo.Count() == 0 {
		fmt.Println("  (nothing surprising happened this window; memory is empty)")
		return
	}

	pruned := hippo.Sleep(nightlyDecay, pruneFloor)
	fmt.Printf("Night 1: weights decayed, surprising memories resisting; %d pruned.\n", pruned)

	for night := 0; night < fastForwardNights; night++ {
		pruned += hippo.Sleep(nightlyDecay, pruneFloor)
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

package main

import "testing"

const (
	testFloor  = 2.0
	testZ      = 2.5
	testWarmup = 30
)

// On a quiet channel, a rare multi-bit event is an alarm, but nothing
// below the absolute floor ever is.
func TestCalibratorQuietChannel(t *testing.T) {
	c := &Calibrator{}
	for range 100 {
		c.Gate(0.1, testFloor, testZ, testWarmup)
	}
	if !c.Gate(5.0, testFloor, testZ, testWarmup) {
		t.Fatal("a 5-bit event on a quiet channel should alarm")
	}
	if c.Gate(1.5, testFloor, testZ, testWarmup) {
		t.Fatal("below the absolute floor never alarms")
	}
}

// On a channel whose routine surprise is already high (diverse templates),
// routine chatter must stay below the gate while a genuine spike clears it.
// This is the cross-channel fairness the fixed threshold could not give.
func TestCalibratorChattyChannel(t *testing.T) {
	c := &Calibrator{}
	for i := range 200 {
		c.Gate(4.5+float64(i%2), testFloor, testZ, testWarmup)
	}
	if c.Gate(5.5, testFloor, testZ, testWarmup) {
		t.Fatal("routine 5.5-bit chatter should not alarm a chatty channel")
	}
	if !c.Gate(11.0, testFloor, testZ, testWarmup) {
		t.Fatal("an 11-bit spike should alarm even a chatty channel")
	}
}

// When a channel turns permanently chaotic, chaos becomes the baseline and
// the alarms stop: calibration habituates at the channel level just as the
// predictor habituates at the template level.
func TestCalibratorHabituatesToNewRegime(t *testing.T) {
	c := &Calibrator{}
	for range 100 {
		c.Gate(0.1, testFloor, testZ, testWarmup)
	}
	if !c.Gate(6.0, testFloor, testZ, testWarmup) {
		t.Fatal("the first 6-bit event should alarm")
	}
	alarms := 0
	for range 300 {
		if c.Gate(6.0, testFloor, testZ, testWarmup) {
			alarms++
		}
	}
	if alarms > 200 {
		t.Fatalf("constant 6-bit chatter should stop alarming, got %d/300", alarms)
	}
}

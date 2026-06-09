package main

import (
	"fmt"
	"testing"
)

// A fresh anomaly must clear the consciousness gate, and repetition must
// drag it back below the gate on the window's timescale.
func TestHabituation(t *testing.T) {
	p := NewPredictor(100)
	for i := 0; i < 60; i++ {
		p.Learn("Ping seq=<n> latency=<n>ms")
	}

	first := p.Surprise("Disk full on <path>")
	if first < surpriseThreshold {
		t.Fatalf("fresh anomaly should clear the gate, got %.2f bits", first)
	}

	for i := 0; i < 25; i++ {
		p.Learn("Disk full on <path>")
	}
	after := p.Surprise("Disk full on <path>")
	if after >= surpriseThreshold {
		t.Fatalf("repeated anomaly should habituate below %.1f bits, got %.2f", surpriseThreshold, after)
	}
	if after >= first {
		t.Fatalf("surprise should fall with repetition: first %.2f, after %.2f", first, after)
	}
}

// When the channel's content changes wholesale, the old regime must fade
// from the model and become surprising again.
func TestForgettingDrift(t *testing.T) {
	p := NewPredictor(100)
	for i := 0; i < 300; i++ {
		p.Learn("old normal")
	}
	if s := p.Surprise("old normal"); s > 0.5 {
		t.Fatalf("the current regime should be unsurprising, got %.2f bits", s)
	}

	for i := 0; i < 400; i++ {
		p.Learn("new normal")
	}
	if s := p.Surprise("old normal"); s < surpriseThreshold {
		t.Fatalf("a forgotten regime should be surprising again, got %.2f bits", s)
	}
}

// A pathological stream of all-unique templates must not grow the model
// without bound. This is the difference between a demo and a daemon.
func TestBoundedMemory(t *testing.T) {
	p := NewPredictor(100)
	for i := 0; i < 10000; i++ {
		p.Learn(fmt.Sprintf("unique line %d", i))
	}
	if len(p.counts) > 600 {
		t.Fatalf("vocabulary should stay bounded, got %d entries", len(p.counts))
	}
	if p.total > 101 {
		t.Fatalf("total mass should stay within the window, got %.1f", p.total)
	}
}

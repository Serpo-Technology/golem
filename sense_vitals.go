package main

import (
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"time"
)

// VitalsSensor samples the machine's vital signs and emits an observation
// only when a reading crosses into a different band. Numbers are bucketed
// into words before they leave the sensor: "cpu load high" is a shape the
// predictor can learn, while "load is 3.41" never repeats. Band changes
// are rare, which is exactly what makes them informative.
func VitalsSensor(out chan Observation, stop <-chan struct{}, dropped *atomic.Int64) {
	tick := time.NewTicker(30 * time.Second)
	defer tick.Stop()

	last := map[string]string{}
	report := func() {
		for key, band := range collectVitals() {
			if band == "" || last[key] == band {
				continue
			}
			last[key] = band
			emit(out, dropped, Observation{Channel: "vitals", Content: band, At: time.Now()})
		}
	}

	report()
	for {
		select {
		case <-stop:
			return
		case <-tick.C:
			report()
		}
	}
}

func collectVitals() map[string]string {
	v := map[string]string{}
	if load, ok := loadAverage(); ok {
		v["load"] = loadBand(load, runtime.NumCPU())
	}
	if lvl, ok := memoryPressureLevel(); ok {
		v["memory"] = pressureBand(lvl)
	}
	if free, ok := diskFreePct("/"); ok {
		v["disk"] = diskBand(free)
	}
	return v
}

func loadAverage() (float64, bool) {
	raw, err := exec.Command("/usr/sbin/sysctl", "-n", "vm.loadavg").Output()
	if err != nil {
		return 0, false
	}
	fields := strings.Fields(strings.Trim(strings.TrimSpace(string(raw)), "{}"))
	if len(fields) < 1 {
		return 0, false
	}
	f, err := strconv.ParseFloat(fields[0], 64)
	return f, err == nil
}

func memoryPressureLevel() (int, bool) {
	raw, err := exec.Command("/usr/sbin/sysctl", "-n", "kern.memorystatus_vm_pressure_level").Output()
	if err != nil {
		return 0, false
	}
	n, err := strconv.Atoi(strings.TrimSpace(string(raw)))
	return n, err == nil
}

func diskFreePct(path string) (float64, bool) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil || st.Blocks == 0 {
		return 0, false
	}
	return 100 * float64(st.Bavail) / float64(st.Blocks), true
}

func loadBand(load1 float64, cpus int) string {
	if cpus < 1 {
		cpus = 1
	}
	per := load1 / float64(cpus)
	switch {
	case per < 0.7:
		return "cpu load normal"
	case per < 1.5:
		return "cpu load elevated"
	default:
		return "cpu load high"
	}
}

func pressureBand(level int) string {
	switch level {
	case 1:
		return "memory pressure normal"
	case 2:
		return "memory pressure warning"
	case 4:
		return "memory pressure critical"
	default:
		return ""
	}
}

func diskBand(freePct float64) string {
	switch {
	case freePct > 20:
		return "disk space normal"
	case freePct > 10:
		return "disk space low"
	default:
		return "disk space critical"
	}
}

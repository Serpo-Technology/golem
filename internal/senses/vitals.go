package senses

import (
	"os"
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
// are rare, which is exactly what makes them informative. The readings are
// taken per platform, sysctl on macOS, /proc on Linux, so the same sensor
// reports the host it actually runs on; an unsupported platform stays quiet.
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
	if band := memoryBand(); band != "" {
		v["memory"] = band
	}
	if free, ok := diskFreePct("/"); ok {
		v["disk"] = diskBand(free)
	}
	return v
}

// loadAverage reads the one-minute load: /proc/loadavg on Linux, the
// vm.loadavg sysctl on macOS.
func loadAverage() (float64, bool) {
	switch runtime.GOOS {
	case "linux":
		raw, err := os.ReadFile("/proc/loadavg")
		if err != nil {
			return 0, false
		}
		return parseLoadavgFirst(string(raw))
	case "darwin":
		raw, err := exec.Command("/usr/sbin/sysctl", "-n", "vm.loadavg").Output()
		if err != nil {
			return 0, false
		}
		return parseLoadavgFirst(string(raw))
	}
	return 0, false
}

// parseLoadavgFirst reads the first load figure from either form: the Linux
// "0.52 0.58 0.59 1/890 12345" or the macOS "{ 1.84 1.74 1.71 }".
func parseLoadavgFirst(s string) (float64, bool) {
	fields := strings.Fields(strings.Trim(strings.TrimSpace(s), "{}"))
	if len(fields) < 1 {
		return 0, false
	}
	f, err := strconv.ParseFloat(fields[0], 64)
	return f, err == nil
}

// memoryBand reports the memory band per platform: the macOS pressure level,
// or the share of memory still available on Linux.
func memoryBand() string {
	switch runtime.GOOS {
	case "linux":
		raw, err := os.ReadFile("/proc/meminfo")
		if err != nil {
			return ""
		}
		if pct, ok := memAvailablePctFrom(string(raw)); ok {
			return memBandLinux(pct)
		}
	case "darwin":
		raw, err := exec.Command("/usr/sbin/sysctl", "-n", "kern.memorystatus_vm_pressure_level").Output()
		if err != nil {
			return ""
		}
		if n, err := strconv.Atoi(strings.TrimSpace(string(raw))); err == nil {
			return pressureBand(n)
		}
	}
	return ""
}

// memAvailablePctFrom reads MemAvailable against MemTotal from /proc/meminfo.
func memAvailablePctFrom(meminfo string) (float64, bool) {
	var total, avail float64
	haveTotal, haveAvail := false, false
	for _, line := range strings.Split(meminfo, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		switch fields[0] {
		case "MemTotal:":
			if v, err := strconv.ParseFloat(fields[1], 64); err == nil {
				total, haveTotal = v, true
			}
		case "MemAvailable:":
			if v, err := strconv.ParseFloat(fields[1], 64); err == nil {
				avail, haveAvail = v, true
			}
		}
	}
	if !haveTotal || !haveAvail || total == 0 {
		return 0, false
	}
	return 100 * avail / total, true
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

// pressureBand maps the macOS memory pressure level to a word.
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

// memBandLinux maps the share of memory still available to the same words the
// macOS pressure level uses, so the band reads alike on either host.
func memBandLinux(availPct float64) string {
	switch {
	case availPct > 20:
		return "memory pressure normal"
	case availPct > 10:
		return "memory pressure warning"
	default:
		return "memory pressure critical"
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

#!/usr/bin/env python3
"""Feed the host's senses to a containerized brain. Sample this machine's vital
signs, system log, and frontmost window, and pipe each as an observation over
TCP. The brain in the container cannot see the host, so this adapter is how it
gets the host's senses, the same way golem-listen gives it ears.

The readings and bands mirror the in-binary senses, so the brain sees the same
shapes whether it sensed itself natively or was fed from here. Cross-platform:
load and disk from the standard library, memory / log / window per OS.

    python3 golem-senses.py [--addr 127.0.0.1:7777] [--only vitals,screen,syslog]
"""
import argparse
import json
import os
import platform
import shutil
import socket
import subprocess
import threading
import time


def observe(addr, channel, text):
    host, port = addr.rsplit(":", 1)
    try:
        with socket.create_connection((host, int(port)), timeout=10) as s:
            s.sendall((json.dumps({"cmd": "observe", "channel": channel, "content": text}) + "\n").encode())
            s.makefile("rb").readline()
    except OSError as err:
        print("send failed:", err)


def load_band(load1, cpus):
    per = load1 / max(cpus, 1)
    return "cpu load normal" if per < 0.7 else "cpu load elevated" if per < 1.5 else "cpu load high"


def disk_band(free_pct):
    return "disk space normal" if free_pct > 20 else "disk space low" if free_pct > 10 else "disk space critical"


def mem_band():
    sysname = platform.system()
    if sysname == "Linux":
        try:
            info = {}
            for line in open("/proc/meminfo"):
                f = line.split()
                if len(f) >= 2:
                    info[f[0].rstrip(":")] = float(f[1])
            total, avail = info.get("MemTotal"), info.get("MemAvailable")
            if total:
                pct = 100 * avail / total
                return "memory pressure normal" if pct > 20 else "memory pressure warning" if pct > 10 else "memory pressure critical"
        except (OSError, ValueError, ZeroDivisionError):
            pass
    elif sysname == "Darwin":
        try:
            lvl = subprocess.run(["sysctl", "-n", "kern.memorystatus_vm_pressure_level"],
                                 capture_output=True, text=True).stdout.strip()
            return {"1": "memory pressure normal", "2": "memory pressure warning",
                    "4": "memory pressure critical"}.get(lvl, "")
        except OSError:
            pass
    return ""


def vitals_loop(addr):
    last = {}
    cpus = os.cpu_count() or 1
    while True:
        bands = {}
        if hasattr(os, "getloadavg"):
            bands["load"] = load_band(os.getloadavg()[0], cpus)
        total, _, free = shutil.disk_usage("/")
        if total:
            bands["disk"] = disk_band(100 * free / total)
        m = mem_band()
        if m:
            bands["memory"] = m
        for key, band in bands.items():
            if band and last.get(key) != band:
                last[key] = band
                observe(addr, "vitals", band)
        time.sleep(30)


def frontmost():
    sysname = platform.system()
    try:
        if sysname == "Darwin":
            return subprocess.run(["osascript", "-e",
                "tell application \"System Events\" to get name of first application process whose frontmost is true"],
                capture_output=True, text=True).stdout.strip()
        if sysname == "Linux":
            return subprocess.run(["xdotool", "getactivewindow", "getwindowname"],
                capture_output=True, text=True).stdout.strip()
    except OSError:
        return ""
    return ""


def screen_loop(addr):
    last = ""
    while True:
        name = frontmost()
        if name and name != last:
            last = name
            observe(addr, "screen", "frontmost app " + name)
        time.sleep(5)


def syslog_cmd():
    sysname = platform.system()
    if sysname == "Darwin":
        return ["log", "stream", "--style", "compact",
                "--predicate", "messageType == error OR messageType == fault"]
    if sysname == "Linux":
        return ["journalctl", "-f", "-n", "0", "-p", "err", "-o", "cat"]
    return None


def syslog_loop(addr):
    cmd = syslog_cmd()
    if not cmd:
        return
    while True:
        try:
            proc = subprocess.Popen(cmd, stdout=subprocess.PIPE, text=True)
        except OSError:
            return
        for line in proc.stdout:
            line = line.strip()
            if line and not line.startswith(("Filtering the log data", "Timestamp")):
                observe(addr, "syslog", line[:240])
        time.sleep(5)


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--addr", default="127.0.0.1:7777", help="brain TCP address")
    ap.add_argument("--only", default="vitals,screen,syslog",
                    help="comma-separated subset of senses to feed")
    args = ap.parse_args()
    want = {s.strip() for s in args.only.split(",") if s.strip()}
    loops = {"vitals": vitals_loop, "screen": screen_loop, "syslog": syslog_loop}
    threads = [threading.Thread(target=loops[s], args=(args.addr,), daemon=True)
               for s in want if s in loops]
    for t in threads:
        t.start()
    print("feeding host senses to", args.addr, "::", ", ".join(sorted(want)))
    try:
        while True:
            time.sleep(3600)
    except KeyboardInterrupt:
        pass


if __name__ == "__main__":
    main()

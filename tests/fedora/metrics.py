"""Measure only the app (and optional --proxy-worker), excluding GNOME and fixtures."""
import os
import pathlib
import sys
import time


def sample(pid):
    proc = pathlib.Path("/proc") / str(pid)
    # The fields after the command start at field 3 in proc_pid_stat(5).
    fields = (proc / "stat").read_text().rsplit(")", 1)[1].split()
    memory = {}
    for line in (proc / "smaps_rollup").read_text().splitlines():
        key, _, value = line.partition(":")
        if key in ("Rss", "Pss"):
            memory[key] = int(value.split()[0])
    return int(fields[11]) + int(fields[12]), memory["Rss"], memory["Pss"]


mode, root, duration = sys.argv[1:]
root, duration = int(root), float(duration)
roles = {"app": root}
if mode == "connected":
    children = set()
    for task in (pathlib.Path("/proc") / str(root) / "task").iterdir():
        try:
            children.update(map(int, (task / "children").read_text().split()))
        except FileNotFoundError:
            continue
    workers = [
        pid for pid in children
        if b"--proxy-worker" in (pathlib.Path("/proc") / str(pid) / "cmdline").read_bytes().split(b"\0")
    ]
    if len(workers) != 1:
        raise SystemExit("Expected exactly one worker for the test app")
    roles["worker"] = workers[0]

start = time.monotonic()
samples = {role: [sample(pid)] for role, pid in roles.items()}
while time.monotonic() - start < duration:
    time.sleep(1)
    for role, pid in roles.items():
        samples[role].append(sample(pid))

elapsed = time.monotonic() - start
ticks = os.sysconf("SC_CLK_TCK")
for role, readings in samples.items():
    cpu = 100 * (readings[-1][0] - readings[0][0]) / ticks / elapsed
    rss = sum(reading[1] for reading in readings) / len(readings) / 1024
    pss = sum(reading[2] for reading in readings) / len(readings) / 1024
    print(f"WIREPROXY_METRICS {mode} {role} duration={elapsed:.1f}s CPU={cpu:.2f}% RSS={rss:.1f}MiB PSS={pss:.1f}MiB")

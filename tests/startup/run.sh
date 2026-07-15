#!/usr/bin/env bash
# Compare process-cold pre-main inittrace data for two puregotk revisions.
# Usage: tests/startup/run.sh [baseline-rev] [head-rev] [runs-per-revision]
set -euo pipefail

root=$(git rev-parse --show-toplevel)
baseline=${1:-origin/main}
head=${2:-HEAD}
runs=${3:-10}
if (( runs < 10 || runs % 2 != 0 )); then
	echo "runs-per-revision must be an even number >= 10" >&2
	exit 2
fi

state=${XDG_STATE_HOME:-"$HOME/.local/state"}/puregotk/startup
umask 077
mkdir -p "$state"
run_id="$(date -u +%Y%m%dT%H%M%SZ)-$(git rev-parse --short "$head")"
out="$state/$run_id"
mkdir -m 700 "$out"
work=$(mktemp -d)
trap 'git -C "$root" worktree remove --force "$work/baseline" 2>/dev/null || true; git -C "$root" worktree remove --force "$work/head" 2>/dev/null || true; rm -rf "$work"' EXIT

git -C "$root" worktree add --detach --quiet "$work/baseline" "$baseline"
git -C "$root" worktree add --detach --quiet "$work/head" "$head"

# Both clean revision worktrees receive byte-identical helper sources, then
# resolve imports from their own revision. The temporary worktrees are removed.
harness="$root/tests/startup"
for revision in baseline head; do
	tree="$work/$revision"
	mkdir -p "$tree/tests/startup"
	cp -R "$harness/blank" "$harness/firstuse" "$tree/tests/startup/"
	cmp "$harness/blank/main.go" "$tree/tests/startup/blank/main.go"
	cmp "$harness/firstuse/main.go" "$tree/tests/startup/firstuse/main.go"
	go build -C "$tree" -o "$out/$revision-blank" ./tests/startup/blank
	go build -C "$tree" -o "$out/$revision-firstuse" ./tests/startup/firstuse
done

{
	echo "baseline=$(git -C "$work/baseline" rev-parse HEAD)"
	echo "head=$(git -C "$work/head" rev-parse HEAD)"
	echo "runs_per_revision=$runs"
	echo "order=alternating; odd pairs baseline,head; even pairs head,baseline"
	uname -a
	go version
} >"$out/metadata.txt"

# Ten fresh execs per revision, with five runs in each position for runs=10.
baseline_n=0
head_n=0
for ((pair = 0; pair < runs; pair++)); do
	if (( pair % 2 == 0 )); then order=(baseline head); else order=(head baseline); fi
	for revision in "${order[@]}"; do
		if [[ $revision == baseline ]]; then
			((++baseline_n))
			n=$baseline_n
		else
			((++head_n))
			n=$head_n
		fi
		GODEBUG=inittrace=1 "$out/$revision-blank" >"$out/$revision-$n.stdout" 2>"$out/$revision-$n.inittrace"
	done
done
for revision in baseline head; do
	"$out/$revision-firstuse" >"$out/$revision-firstuse.stdout" 2>"$out/$revision-firstuse.stderr"
done

python3 - "$out" "$runs" <<'PY'
import glob
import os
import re
import statistics
import sys

out = sys.argv[1]
runs = int(sys.argv[2])
# inittrace rows report package, clock time, bytes, and allocs.  Only this
# module's rows are summed: whole-process and dependency initialization are
# intentionally excluded.
row = re.compile(r'^init github\.com/bnema/puregotk(?:/[^ ]*)? @[^,]*, ([0-9.]+) ms clock, ([0-9]+) bytes, ([0-9]+) allocs')
values = {}
for revision in ('baseline', 'head'):
    samples = []
    for path in sorted(glob.glob(os.path.join(out, revision + '-*.inittrace')),
                       key=lambda p: int(os.path.basename(p).split('-')[1].split('.')[0])):
        clock = byte = alloc = 0
        for line in open(path, encoding='utf-8', errors='replace'):
            m = row.match(line)
            if m:
                clock += float(m.group(1))
                byte += int(m.group(2))
                alloc += int(m.group(3))
        samples.append((clock, byte, alloc))
    if len(samples) != runs or not any(sample != (0, 0, 0) for sample in samples):
        raise SystemExit(f'{revision}: incomplete or irrelevant inittrace samples: {samples}')
    values[revision] = samples

with open(os.path.join(out, 'summary.txt'), 'w', encoding='utf-8') as report:
    report.write('metric: sum of github.com/bnema/puregotk inittrace rows before main\n')
    for revision, samples in values.items():
        report.write(f'{revision} raw samples (clock_ms,bytes,allocs): {samples}\n')
        report.write(f'{revision} medians (clock_ms,bytes,allocs): ' +
                     str(tuple(statistics.median(x[i] for x in samples) for i in range(3))) + '\n')
    base = statistics.median(x[0] for x in values['baseline'])
    head = statistics.median(x[0] for x in values['head'])
    reduction = (base - head) / base * 100 if base else 0
    report.write(f'clock reduction: {reduction:.2f}%\n')
PY
chmod 600 "$out"/*
printf 'private raw evidence: %s\n' "$out"
cat "$out/summary.txt"

# Startup benchmark

`blank` imports exactly v4 GTK, GDK, GLib, Gio, and Adwaita without using their
APIs. `firstuse` invokes one safe, display-independent symbol from each package
to verify deferred resolution separately from startup.

Run from the repository root:

```sh
XDG_STATE_HOME="$HOME/.local/state" tests/startup/run.sh 7fe22e85 HEAD 10
```

The runner creates detached worktrees for the two revisions, compiles the same
helper source against each, and executes 10 fresh processes per revision. It
balances order (five baseline-first and five head-first). Each blank process has
`GODEBUG=inittrace=1`; `summary.txt` sums only inittrace rows whose package is
`github.com/bnema/puregotk`, so it measures relevant pre-main binding work, not
whole-process startup. It reports raw `(clock_ms, bytes, allocs)` samples and
medians. Both first-use binaries must exit successfully.

Raw binaries, traces, metadata, and summaries are private: they are written
with mode 0700/0600 below `$XDG_STATE_HOME/puregotk/startup` and must not be
committed. Process-cold here means a new executable process; OS page-cache state
is not claimed to be cold.

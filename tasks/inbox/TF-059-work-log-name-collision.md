# TF-059: Avoid log name collisions within one second

## Goal

Two `taskfactory work` runs for the same task and adapter that start in the
same second must both get a log file instead of the second one refusing.

## Scope

The log file name from `workprompt.LogPath` ends in a UTC time with one second
resolution, and `OpenLog` refuses to overwrite. Found while verifying TF-056:
a second run started within the same second failed with "refusing to
overwrite existing log". Add a short numeric suffix on collision (for example
`-2`) or use sub-second time, keeping the documented directory layout, the
never-overwrite rule and the sort order by name.

## Notes

Low priority; it only affects scripted back-to-back runs. Unrefined: write the
ready contract with a test that opens two logs with the same timestamp.

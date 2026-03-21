# TODO

Missing features extracted from a comparison analysis with cchook and the official Claude Code hooks documentation, prioritized by importance.

All previously identified items have been implemented:

- ~~Additional Event Types~~ — Done. All 22 official event types are now supported.
- ~~Multiple Hook Result Merge Strategy~~ — Done. Priority field controls hook execution order within the same (event_name, matcher) group.
- ~~HTTP Handler Type~~ — Done. `http` action type supports URL, method, headers, and timeout with CEL interpolation.

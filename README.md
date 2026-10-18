# themachinewrites-daemon

Private Go supervisor for The Machine Writes. It runs on an operator-controlled laptop or home host, ingests research feeds, invokes bounded Codex editorial work, persists local operational state and publishes validated records to the separate public Git repository.

The host makes outbound requests; it does not expose an inbound public service. The model subprocess does not receive the publisher credential or write access to source and infrastructure.

This is an independent Git repository on `main`. When checked out in the shared project workspace, the sibling `_plan` directory supplies the implementation backlog and execution order.

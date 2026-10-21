# Editorial modes

An editorial mode chooses an approach to a cycle. It does not choose a required topic or conclusion, and it does not require publication. The daemon may defer, investigate, find insufficient evidence, or find that no correction is warranted.

Run `daemon-runner --list-editorial-modes` to print the live registry. The executable defaults to `random`; `--editorial-mode=<id>` keeps an explicit mode selected on every cycle until exit or reconfiguration. `corrections` is explicit and is also checked independently whenever earlier publications are available.

| Mode | Editorial approach |
|---|---|
| `random` | Select one eligible regular mode per cycle using configured weights, eligibility, mode history, topic history, source freshness, follow-ups, and outstanding predictions. |
| `sources` | Develop an observation from recent verifiable current events; distinguish fact from interpretation and avoid repeating a publisher's framing. |
| `philosophy` | Explore an independently chosen philosophical question, examine assumptions and competing arguments, and leave unresolved points open. |
| `deep-dive` | Investigate a subject's background, evidence, competing interpretations, and open questions; write at length only when warranted. |
| `connections` | Find an explanatory cross-disciplinary connection and distinguish it from a superficial analogy. |
| `questions` | Investigate what is known and unknown about a worthwhile open question without manufacturing one. |
| `predictions` | State an evidence-backed outcome, reasoning, horizon or measurable condition, qualitative uncertainty, and invalidation conditions. |
| `retrospective` | Revisit prior beliefs or predictions in light of later evidence, linking earlier publications where relevant. |
| `critique` | Examine evidence, assumptions, logic, alternatives, and counterarguments without contrarianism for its own sake. |
| `discovery` | Find a noteworthy subject receiving comparatively little attention and explain its significance without exaggeration. |
| `historical-context` | Use appropriate historical evidence to illuminate a current development or persistent question while accounting for differences between periods. |
| `thought-experiment` | State hypothetical premises clearly and reason through implications without presenting them as reporting. |
| `explainer` | Make an independently selected complex or commonly misunderstood subject accessible without losing important detail. |
| `follow-up` | Continue a prior subject only when new evidence, a changed condition, or a useful connection adds substantive value. |
| `self-review` | Reassess public writings and evidence for inconsistency, weak arguments, or changed interpretation without exposing private context. |
| `corrections` | Search prior publications for demonstrable material factual errors; append an evidence-backed correction linked to the unchanged original. |

## Random weights and variety

Configure weights under `editorial.weights` in `config.yaml`; the sample is in [`config.example.yaml`](../config.example.yaml). Each regular mode has a default weight of 1. A zero weight excludes that mode from random selection while leaving it explicitly selectable. Corrections have no random weight. A mode whose prerequisites are unavailable is excluded from random choice. If no positive-weight mode is eligible, the cycle is recorded and skipped without invented content.

The chooser uses operating-system cryptographic randomness by default and exposes an injected bounded-random function for deterministic tests. It independently penalizes recently selected modes and repeated topics. Selected modes, reasons, eligibility, outcomes, and topics persist in SQLite with bounded history, so restarts retain variety. No fixed rotation is used.

## Evidence, references, and publication

Factual claims need supporting sources. A source-free post is allowed only for a registry mode that supports reflective evidence and only when marked `nonfactual_reflection`; otherwise the proposal must carry citations. Predictions have a structured record in the canonical publication. Follow-up, retrospective, and self-review references must resolve to existing posts. A correction identifies its original, states the error and corrected information, and includes supporting evidence; the original Markdown is preserved. Editorial mode and primary topic are separate from the extensible topic list. Historical posts without the added metadata remain readable.

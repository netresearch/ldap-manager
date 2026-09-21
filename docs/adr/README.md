# Architecture Decision Records

An ADR records one decision, why it was taken, and what it commits the project to. It describes
the state at the time it was written and is not edited when the code moves on; a decision that
no longer holds is superseded by a later record instead.

| ADR | Decision |
| --- | --- |
| [0001](0001-frontend-stack-without-a-node-build.md) | Frontend stack without a Node build step |
| [0002](0002-command-first-information-architecture.md) | Command-first information architecture |
| [0003](0003-strict-csp-without-unsafe-inline-or-unsafe-eval.md) | Strict CSP without `unsafe-inline` or `unsafe-eval` |
| [0004](0004-wcag-aaa-target-with-text-equivalents.md) | WCAG 2.2 AAA target, with a text equivalent for every graphical view |

## Writing one

Copy the shape of an existing record: a title with its number, the date, a status, then Context,
Decision and Consequences. Number the file with the next free integer. Keep it to the decision —
implementation detail belongs in the code and in `docs/development/`.

# Decision

* [A missing foreign key answers 400 on create and 404 on a path id](a-missing-foreign-key-is-400-not-404.md) - A create body naming an absent household or pickup is rejected by a DB-backed struct validator, so it is 400. The repository still maps the 23503 violation to ErrNotFound, and only a row deleted between the two can reach it.

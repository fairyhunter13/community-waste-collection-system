# Constraint

* [The codecov ignore list has to mirror the COVERPKG filter](codecov-ignore-mirrors-the-coverpkg-filter.md) - The Makefile drops internal/observability from -coverpkg, so no coverage data is produced for it at all. Removing it from codecov.yml does not measure it; it reports it as uncovered.

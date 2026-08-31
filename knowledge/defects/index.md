# Defect

* [The Newman smoke test asserted a field that every response carries](the-smoke-test-asserted-what-never-fails.md) - All 17 envelope checks asserted success:true even on 4xx, so a run where about ten requests failed still passed. The failure they hid was folder order: Delete Household ran before the folders that needed the household.

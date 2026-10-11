# CI webhook-to-workflow verification

2026-10-11: the argo-events domain-check sensor path was verified end to end —
a push to main fired the github-webhooks event, the domain-check-sensor matched
the main-branch dependency, and a domain-check-build workflow was submitted on
this commit (bead declarat-78776a9c).

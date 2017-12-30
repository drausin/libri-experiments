# Experiments

This directory contains sets of experiments run against libri clusters:
- [exp01](exp01): How does performance degrade with increasing user load?

Each experiment is oriented around a high-level question or goal and contains individual trials
that represent discrete steps toward that question/goal. In this experiment documentation, we 
strive to meet the following standards:
- trials should be **reproducible**, so all the configs and shell commands are given for 
anyone to run the same trial themselves
- **raw result data** should be included, be they Grafana screenshots or Prometheus query results 
- trials should be **incremental**, meaning that they rarely contain nice, tidy results but more
honestly reflect the actual experimental progression
- discussion should be honest about **negative results**, since what didn't work is often just as 
import as what did

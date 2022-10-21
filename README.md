# pprof-breakdown

This repo contains the code and data used to analyze the efficiency of the pprof breakdown proposal.

## View Data

The results are best viewed in [this spreadsheet](https://docs.google.com/spreadsheets/d/158gORmju85Z1rwGtTEL71yrkmLHQw4KGPMYKrHfm8qU/edit#gid=1729544497).

For the individual workloads, see [workloads.go](./testdata/workloads/workloads.go).

Alternatively you can take a look at the files in `testdata/pprof-inputs` and `testdata/pprof-outputs`, perhaps using protoc:

```
cat testdata/pprof-outputs/printf-10s.none.breakdown.pprof | protoc --decode perftools.profiles.Profile /path/to/profile.proto
```

## Reproduce Results

If you're interested in reproducing the results, please:

1. Build this [fork of Go](https://github.com/felixge/go/pull/3) which adds the pprof breakdown feature to the CPU profiler

2. Run the code below:

```
PATH="/path/to/github.com/felixge/go/bin:$PATH" make pprof-inputs pprof-outputs
```

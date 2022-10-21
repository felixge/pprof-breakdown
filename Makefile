.PHONY: pprof-outputs
pprof-outputs:
	go run . testdata/pprof-inputs testdata/pprof-outputs

.PHONY: pprof-inputs
pprof-inputs:
	cd ./testdata/workloads && PPROF_BREAKDOWN=true go run . -duration 10s -timeline=false -cpuprofile=../pprof-inputs/json-heatmap-10s.pprof json
	cd ./testdata/workloads && PPROF_BREAKDOWN=true go run . -duration 10s -timeline=true -cpuprofile=../pprof-inputs/json-timeline-10s.pprof json
	cd ./testdata/workloads && PPROF_BREAKDOWN=true go run . -duration 60s -timeline=false -cpuprofile=../pprof-inputs/json-heatmap-60s.pprof json
	cd ./testdata/workloads && PPROF_BREAKDOWN=true go run . -duration 60s -timeline=true -cpuprofile=../pprof-inputs/json-timeline-60s.pprof json
	cd ./testdata/workloads && PPROF_BREAKDOWN=true go run . -duration 10s -timeline=false -cpuprofile=../pprof-inputs/printf-heatmap-10s.pprof printf
	cd ./testdata/workloads && PPROF_BREAKDOWN=true go run . -duration 10s -timeline=true -cpuprofile=../pprof-inputs/printf-timeline-10s.pprof printf
	cd ./testdata/workloads && PPROF_BREAKDOWN=true go run . -duration 60s -timeline=false -cpuprofile=../pprof-inputs/printf-heatmap-60s.pprof printf
	cd ./testdata/workloads && PPROF_BREAKDOWN=true go run . -duration 60s -timeline=true -cpuprofile=../pprof-inputs/printf-timeline-60s.pprof printf


.PHONY: clean
clean:
	rm -f testdata/pprof-outputs/*.pprof

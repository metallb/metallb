# Prometheus-plugin example

The example showcases to expose metrics using prometheus plugin.

To start the example:
```
./prometheus-plugin [--http-port <PORT_NUM>]
```
By default the prometheus stats are exposed at port 9191.

The example exposes two registries. The first contains runtime statistics.
```
curl localhost:9191/metrics
# HELP go_gc_duration_seconds A summary of the GC invocation durations.
# TYPE go_gc_duration_seconds summary
go_gc_duration_seconds{quantile="0"} 0
go_gc_duration_seconds{quantile="0.25"} 0
go_gc_duration_seconds{quantile="0.5"} 0
go_gc_duration_seconds{quantile="0.75"} 0
go_gc_duration_seconds{quantile="1"} 0
go_gc_duration_seconds_sum 0
go_gc_duration_seconds_count 0
# HELP go_goroutines Number of goroutines that currently exist.
# TYPE go_goroutines gauge
go_goroutines 12
# HELP go_memstats_alloc_bytes Number of bytes allocated and still in use.
# TYPE go_memstats_alloc_bytes gauge
go_memstats_alloc_bytes 838576
# HELP go_memstats_alloc_bytes_total Total number of bytes allocated, even if freed.
# TYPE go_memstats_alloc_bytes_total counter
go_memstats_alloc_bytes_total 838576
# HELP go_memstats_buck_hash_sys_bytes Number of bytes used by the profiling bucket hash table.
# TYPE go_memstats_buck_hash_sys_bytes gauge
go_memstats_buck_hash_sys_bytes 3043
# HELP go_memstats_frees_total Total number of frees.
# TYPE go_memstats_frees_total counter
go_memstats_frees_total 372
# HELP go_memstats_gc_sys_bytes Number of bytes used for garbage collection system metadata.
# TYPE go_memstats_gc_sys_bytes gauge
go_memstats_gc_sys_bytes 169984
# HELP go_memstats_heap_alloc_bytes Number of heap bytes allocated and still in use.
# TYPE go_memstats_heap_alloc_bytes gauge
...output is truncated
```

The second includes user defined metrics.
```
curl localhost:9191/custom
# HELP Countdown This gauge is decremented by 1 each second, once it reaches 0 the gauge is removed.
# TYPE Countdown gauge
Countdown 45
# HELP Vector This gauge groups multiple similar metrics.
# TYPE Vector gauge
Vector{answer="42",order="1",type="vector"} 1
Vector{answer="42",order="2",type="vector"} 1
Vector{answer="42",order="3",type="vector"} 1
Vector{answer="42",order="4",type="vector"} 1
Vector{answer="42",order="5",type="vector"} 1
Vector{answer="42",order="6",type="vector"} 1
Vector{answer="42",order="7",type="vector"} 1
Vector{answer="42",order="8",type="vector"} 1
Vector{answer="42",order="9",type="vector"} 1
```

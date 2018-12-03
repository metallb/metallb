## Tracer

A simple utility able to log measured time periods various events. To create a new tracer, call:

`t := NewTracer(name string, log logging.Logger)`

Tracer object can store a new entry with `t.LogTime(entity string, start Time)` where `entity` is a string 
representation of a measured object (name of a function, structure or just simple string) and `start` is a start time. 

Tracer can measure repeating event (in a loop for example). Every event will be stored with the particular index.

Use `t.Get()` to read all measurements. The Trace object contains a list of entries and overall time duration.

Last method is `t.Clear()` which removes all entries from the internal database.
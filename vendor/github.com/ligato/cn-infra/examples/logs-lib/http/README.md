# Logs-lib HTTP Example

To run the example, simply type:
```
go run server.go
```

List all registered loggers and their current log level via HTTP GET:
```
curl localhost:8080/list
```

Modify log level remotely via HTTP POST:
```
curl -X PUT localhost:8080/set/{loggerName}/{logLevel}
```

Example of setting log level for custom and default loggers:
```
curl -X PUT localhost:8080/set/MyLogger/debug
curl -X PUT localhost:8080/set/defaultLogger/debug
```
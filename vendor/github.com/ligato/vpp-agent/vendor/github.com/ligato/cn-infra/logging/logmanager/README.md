# Log Manager

Log manager plugin allows to view and modify log levels of loggers using REST API.

**API**
- List all registered loggers:

    ```curl -X GET http://<host>:<port>/log/list```
- Set log level for a registered logger:
   ```curl -X PUT http://<host>:<port>/log/<logger-name>/<log-level>```
 
   `<log-level>` is one of `debug`,`info`,`warning`,`error`,`fatal`,`panic`
   
`<host>` and `<port>` are determined by configuration of rest.Plugin.
 
**Config file**

- Logger config file is composed of two parts: the default level applied for all plugins, 
  and a map where every logger can have its own log level defined. See config file 
  [example](../logging.conf) to learn how to define it.
  
  **Note:** initial log level can be set using environmental variable `INITIAL_LOGLVL`. The 
  variable replaces default-level from configuration file. However, loggers (partial definition)
  replace default value set by environmental variable for specific loggers defined.  
 
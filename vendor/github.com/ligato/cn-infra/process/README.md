# Process manager

The process manager plugin provides a set of methods to create a plugin-defined process instance implementing a set
of methods to manage and monitor it. 

There are several ways how to obtain a process instance.

**New process with options:** using method `NewProcess(<cmd>, <options>...)` which requires a command 
and a set of optional parameters. 
**New process from template:** using method `NewProcessFromTemplate(<tmp>)` which requires template as a parameter
**Attach to existing process:** using method `AttachProcess(<pid>)`. The process ID is required to order to attach.

## Management

Note: since application (management plugin) is parent of all processes, application termination causes all
started processes to stop as well. This can be changed with *Detach* option (see process options).

Process management methods:

* `Start()` starts the plugin-defined process, stores the instance and does initial status file read
* `Restart()` tries to gracefully stop (force stop if fails) the process and starts it again. If the instance 
is not running, it is started.
* `Stop()` stops the instance using SIGTERM signal. Process is not guaranteed to be stopped. Note that 
child processes (not detached) may end up as defunct if stopped this way. 
* `StopAndWait()` stops the instance using SIGTERM signal and waits until the process completes. 
* `Kill()` force-stops the process using SIGKILL signal and releases all the resources used.
* `Wait()` waits until the process completes.
* `Delete()` releases all process resources (stops the instance and all internal channels) and removes it.
Template won't be affected.
* `Signal()` allows user to send custom signal to a process. Note that some signals may cause unexpected 
behavior in process handling.

Process monitor methods:

* `IsAlive()` returns true if process is running
* `GetNotificationChan()` returns channel where process status notifications will be sent. Useful only when process
is created via template with 'notify' field set to true. In other cases, the channel is provided by user.
* `GetName` returns process name as defined in status file
* `GetPid()` returns process ID
* `UpdateStatus()` updates internal status of the plugin and returns the actual status file
* `GetCommand()` returns original process command. Always empty for attached processes.
* `GetArguments()` returns original arguments the process was run with. Always empty for attached processes.
* `GetStartTime()` returns time stamp when the process was started for the last time
* `GetUpTime()` returns process up-time in nanoseconds

## Status watcher

Every process is watched for status changes (it does not matter which way it was crated) despite the process
is running or not. The watcher uses standard statuses (running, sleeping, idle, etc.). The state is read 
from process status file and every change is reported. The plugin also defines two plugin-wide statues:
* **Terminated** - if the process is not running or does not respond
* **Unavailable** - if the process is running but the status cannot be obtained
The process status is periodically polled and notifications can be sent to the user defined channel. In case 
process was crated via template, channel was initialized in the plugin and can be obtained via `GetNotificationChan()`.

## Process options

Following options are available for processes. All options can be defined in the API method as well as in the template.
All of them are optional.

**Arguments:** takes string array as parameter, process will be started with given arguments. 
**Restarts:** takes a number as a parameter, defines count of automatic restarts when the process 
state becomes terminated.
**Detach:** no parameters, started process detaches from the parent application and will be given to current user.
This setting allows the process to run even after the parent was terminated.   
**Template:** requires name, and run-on-startup flag. This setup creates a template on process creation.
The template path has to be set in the plugin.
**Notify:** allows user to provide a notification channel for status changes.

## Templates

The template is a file which defines process configuration for plugin manager. All templates should be stored 
in the path defined in the plugin config file. Example can be found [here](pm.conf).

```
./process-manager-plugin -process-manager-config=<path-to-file>
```

The template can be either written by hand using 
[proto model](template/model/process/process.proto), or generated with the *Template* option while creating a new 
process. 

On the plugin init, all templates are read, and those with *run-on-startup* set to 'true' are also immediately started.
The template contains several fields defining process name, command, arguments and all the other fields from options.

The plugin API allows to read templates directly with `GetTemplate(<name)` or `GetAllTmplates()`. The template object
can be used as parameter to start a new process from it. 
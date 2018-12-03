# Client v1

Client v1 (i.e. the first version) defines an API that allows to manage
configuration of default plugins and the Linux plugin.
How the configuration is transported between APIs and the plugins
is fully abstracted from the user.

The API calls can be split into two groups:
 - **resync** applies a given (full) configuration. An existing
   configuration, if present, is replaced. The name is an abbreviation
   of *resynchronization*. It is used initially and after any system
   event that may leave the configuration out-of-sync while the set
   of outdated configuration options is impossible to determine locally
   (e.g. temporarily lost connection to data store).
 - **data change** allows to deliver incremental changes
   of a configuration.

There are two implementations:
 - **local client** runs inside the same process as the agent
   and delivers configuration through go channels directly
   to the plugins.
 - **remote client** stores the configuration using the given
   `keyval.broker`.

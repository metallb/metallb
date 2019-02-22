# FileDB - file system configuration reader plugin

The fileDB plugin allows to use the file system of a operating system as a key-value data store. The filesystem
plugin watches for pre-defined files or directories, reads a configuration and sends response events according
to changes.

All the configuration is resynced in the beginning (as for standard key-value data store). Configuration files
then can be added, updated, moved, renamed or removed, plugin makes all the necessary changes.

Important note: fileDB as datastore is read-only from the plugin perspective, changes from within the plugin
are not allowed.

## Configuration

All files/directories used as a data store must be defined in configuration file. Location of the file
can be defined either by the command line flag `filedb-config` or set via the `FILEDB_CONFIG`
environment variable.

## Supported formats

* JSON `(*.json)`
* YAML `(*.yaml)`

## Data structure

Plugin currently supports only JSON and YAML-formatted data. The format of the file is as follows for JSON:

```
{
    "data": [
        {
            "key": "<key>",
            "value": {
                <proto-modelled data>
            }
        },
        {
            "key": "<key>",
            "value": {
                <proto-modelled data>
            }
        },
        ...
    ]
}

``` 

For YAML:

```

---
data:
    -
        key: '<key>'
        value: '<modelled data>'

```

Key has to contain also instance prefix with micro service label, so plugin knows which parts of the configuration 
are intended for it. All configuration is stored internally in local database. It allows to compare events and 
respond with correct 'previous' value for a given key. 

## Data state propagation

Data types supporting status propagation (like interfaces or bridge domains) can store the state in filesystem.
There is a field in configuration file called `status-path` which has to be set in order to store the status.
Status data will be stored in the same format as for configuration, type is defined by the file extension 
(JSON or YAML).

Data will not be propagated if target path directory.

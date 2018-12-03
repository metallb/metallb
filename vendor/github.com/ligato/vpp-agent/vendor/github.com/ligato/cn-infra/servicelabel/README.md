# Service Label

The `servicelabel` is a small Core Agent Plugin, which other plugins can use to
obtain the microservice label, i.e. the string used to identify the particular VNF.
The label is primarily used to prefix keys in etcd datastore so that the configurations
of different VNFs do not get mixed up.

**API**

described in [doc.go](doc.go)

**Configuration**

- the serviceLabel can be set either by commandline flag `microservice-label` or environment variable `MICROSERVICE_LABEL`

**Dependencies**

\-

**Example**

Example of retrieving and using the microservice label:
```
plugin.Label = servicelabel.GetAgentLabel()
dbw.Watch(dataChan, cfg.SomeConfigKeyPrefix(plugin.Label))
```



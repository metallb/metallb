# E2E Tests
The MetalLB E2E test suite can be run by creating a configured development cluster:

```
inv dev-env -p layer2
```
or
```
inv dev-env -p bgp
```

and running the E2E tests against the development cluster:

```
inv e2etest
```

The test suite will detect the development cluster protocol and run the
appropriate tests against that cluster. Be sure to cleanup any previously created
development clusters using `inv dev-env-cleanup`.


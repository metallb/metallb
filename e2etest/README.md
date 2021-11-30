# E2E Tests
The MetalLB E2E test suite can be run by creating a development cluster:

```
inv dev-env
```

and running the E2E tests against the development cluster:

```
inv e2etest
```

Run only BGP test suite:

```
inv e2etest --focus BGP
```

Run only L2 test suite:

```
inv e2etest --focus L2
```

The test suite will run the appropriate tests against the cluster.
Be sure to cleanup any previously created development clusters using `inv dev-env-cleanup`.


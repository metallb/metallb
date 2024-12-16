# metallb

![Version: 0.0.0](https://img.shields.io/badge/Version-0.0.0-informational?style=flat-square) ![Type: application](https://img.shields.io/badge/Type-application-informational?style=flat-square) ![AppVersion: v0.0.0](https://img.shields.io/badge/AppVersion-v0.0.0-informational?style=flat-square)

A network load-balancer implementation for Kubernetes using standard routing protocols

**Homepage:** <https://metallb.universe.tf>

## Source Code

* <https://github.com/metallb/metallb>

## Requirements

Kubernetes: `>= 1.19.0-0`

| Repository | Name | Version |
|------------|------|---------|
|  | crds | 0.0.0 |
| https://metallb.github.io/frr-k8s | frr-k8s | 0.0.16 |

## Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| controller.affinity | object | `{}` |  |
| controller.enabled | bool | `true` |  |
| controller.extraContainers | list | `[]` |  |
| controller.image.pullPolicy | string | `nil` |  |
| controller.image.repository | string | `"quay.io/metallb/controller"` |  |
| controller.image.tag | string | `nil` |  |
| controller.labels | object | `{}` |  |
| controller.livenessProbe.enabled | bool | `true` |  |
| controller.livenessProbe.failureThreshold | int | `3` |  |
| controller.livenessProbe.initialDelaySeconds | int | `10` |  |
| controller.livenessProbe.periodSeconds | int | `10` |  |
| controller.livenessProbe.successThreshold | int | `1` |  |
| controller.livenessProbe.timeoutSeconds | int | `1` |  |
| controller.logLevel | string | `"info"` | Controller log level. Must be one of: `all`, `debug`, `info`, `warn`, `error` or `none` |
| controller.nodeSelector | object | `{}` |  |
| controller.podAnnotations | object | `{}` |  |
| controller.priorityClassName | string | `""` |  |
| controller.readinessProbe.enabled | bool | `true` |  |
| controller.readinessProbe.failureThreshold | int | `3` |  |
| controller.readinessProbe.initialDelaySeconds | int | `10` |  |
| controller.readinessProbe.periodSeconds | int | `10` |  |
| controller.readinessProbe.successThreshold | int | `1` |  |
| controller.readinessProbe.timeoutSeconds | int | `1` |  |
| controller.resources | object | `{}` |  |
| controller.runtimeClassName | string | `""` |  |
| controller.securityContext.fsGroup | int | `65534` |  |
| controller.securityContext.runAsNonRoot | bool | `true` |  |
| controller.securityContext.runAsUser | int | `65534` |  |
| controller.serviceAccount.annotations | object | `{}` |  |
| controller.serviceAccount.create | bool | `true` |  |
| controller.serviceAccount.name | string | `""` |  |
| controller.strategy.type | string | `"RollingUpdate"` |  |
| controller.tlsCipherSuites | string | `""` |  |
| controller.tlsMinVersion | string | `"VersionTLS12"` |  |
| controller.tolerations | list | `[]` |  |
| crds.enabled | bool | `true` |  |
| crds.validationFailurePolicy | string | `"Fail"` |  |
| frrk8s.enabled | bool | `false` |  |
| frrk8s.external | bool | `false` |  |
| frrk8s.namespace | string | `""` |  |
| fullnameOverride | string | `""` |  |
| imagePullSecrets | list | `[]` |  |
| loadBalancerClass | string | `""` |  |
| nameOverride | string | `""` |  |
| prometheus.controllerMetricsTLSSecret | string | `""` |  |
| prometheus.metricsPort | int | `7472` |  |
| prometheus.namespace | string | `""` |  |
| prometheus.podMonitor.additionalLabels | object | `{}` |  |
| prometheus.podMonitor.annotations | object | `{}` |  |
| prometheus.podMonitor.enabled | bool | `false` |  |
| prometheus.podMonitor.interval | string | `nil` |  |
| prometheus.podMonitor.jobLabel | string | `"app.kubernetes.io/name"` |  |
| prometheus.podMonitor.metricRelabelings | list | `[]` |  |
| prometheus.podMonitor.relabelings | list | `[]` |  |
| prometheus.prometheusRule.additionalLabels | object | `{}` |  |
| prometheus.prometheusRule.addressPoolExhausted.enabled | bool | `true` |  |
| prometheus.prometheusRule.addressPoolExhausted.labels.severity | string | `"critical"` |  |
| prometheus.prometheusRule.addressPoolUsage.enabled | bool | `true` |  |
| prometheus.prometheusRule.addressPoolUsage.thresholds[0].labels.severity | string | `"warning"` |  |
| prometheus.prometheusRule.addressPoolUsage.thresholds[0].percent | int | `75` |  |
| prometheus.prometheusRule.addressPoolUsage.thresholds[1].labels.severity | string | `"warning"` |  |
| prometheus.prometheusRule.addressPoolUsage.thresholds[1].percent | int | `85` |  |
| prometheus.prometheusRule.addressPoolUsage.thresholds[2].labels.severity | string | `"critical"` |  |
| prometheus.prometheusRule.addressPoolUsage.thresholds[2].percent | int | `95` |  |
| prometheus.prometheusRule.annotations | object | `{}` |  |
| prometheus.prometheusRule.bgpSessionDown.enabled | bool | `true` |  |
| prometheus.prometheusRule.bgpSessionDown.labels.severity | string | `"critical"` |  |
| prometheus.prometheusRule.configNotLoaded.enabled | bool | `true` |  |
| prometheus.prometheusRule.configNotLoaded.labels.severity | string | `"warning"` |  |
| prometheus.prometheusRule.enabled | bool | `false` |  |
| prometheus.prometheusRule.extraAlerts | list | `[]` |  |
| prometheus.prometheusRule.staleConfig.enabled | bool | `true` |  |
| prometheus.prometheusRule.staleConfig.labels.severity | string | `"warning"` |  |
| prometheus.rbacPrometheus | bool | `true` |  |
| prometheus.rbacProxy.pullPolicy | string | `nil` |  |
| prometheus.rbacProxy.repository | string | `"gcr.io/kubebuilder/kube-rbac-proxy"` |  |
| prometheus.rbacProxy.tag | string | `"v0.12.0"` |  |
| prometheus.scrapeAnnotations | bool | `false` |  |
| prometheus.serviceAccount | string | `""` |  |
| prometheus.serviceMonitor.controller.additionalLabels | object | `{}` |  |
| prometheus.serviceMonitor.controller.annotations | object | `{}` |  |
| prometheus.serviceMonitor.controller.tlsConfig.insecureSkipVerify | bool | `true` |  |
| prometheus.serviceMonitor.enabled | bool | `false` |  |
| prometheus.serviceMonitor.interval | string | `nil` |  |
| prometheus.serviceMonitor.jobLabel | string | `"app.kubernetes.io/name"` |  |
| prometheus.serviceMonitor.metricRelabelings | list | `[]` |  |
| prometheus.serviceMonitor.relabelings | list | `[]` |  |
| prometheus.serviceMonitor.speaker.additionalLabels | object | `{}` |  |
| prometheus.serviceMonitor.speaker.annotations | object | `{}` |  |
| prometheus.serviceMonitor.speaker.tlsConfig.insecureSkipVerify | bool | `true` |  |
| prometheus.speakerMetricsTLSSecret | string | `""` |  |
| rbac.create | bool | `true` |  |
| speaker.affinity | object | `{}` |  |
| speaker.enabled | bool | `true` |  |
| speaker.excludeInterfaces.enabled | bool | `true` |  |
| speaker.extraContainers | list | `[]` |  |
| speaker.frr.enabled | bool | `true` |  |
| speaker.frr.image.pullPolicy | string | `nil` |  |
| speaker.frr.image.repository | string | `"quay.io/frrouting/frr"` |  |
| speaker.frr.image.tag | string | `"9.1.0"` |  |
| speaker.frr.metricsPort | int | `7473` |  |
| speaker.frr.resources | object | `{}` |  |
| speaker.frrMetrics.resources | object | `{}` |  |
| speaker.ignoreExcludeLB | bool | `false` |  |
| speaker.image.pullPolicy | string | `nil` |  |
| speaker.image.repository | string | `"quay.io/metallb/speaker"` |  |
| speaker.image.tag | string | `nil` |  |
| speaker.labels | object | `{}` |  |
| speaker.livenessProbe.enabled | bool | `true` |  |
| speaker.livenessProbe.failureThreshold | int | `3` |  |
| speaker.livenessProbe.initialDelaySeconds | int | `10` |  |
| speaker.livenessProbe.periodSeconds | int | `10` |  |
| speaker.livenessProbe.successThreshold | int | `1` |  |
| speaker.livenessProbe.timeoutSeconds | int | `1` |  |
| speaker.logLevel | string | `"info"` | Speaker log level. Must be one of: `all`, `debug`, `info`, `warn`, `error` or `none` |
| speaker.memberlist.enabled | bool | `true` |  |
| speaker.memberlist.mlBindAddrOverride | string | `""` |  |
| speaker.memberlist.mlBindPort | int | `7946` |  |
| speaker.memberlist.mlSecretKeyPath | string | `"/etc/ml_secret_key"` |  |
| speaker.nodeSelector | object | `{}` |  |
| speaker.podAnnotations | object | `{}` |  |
| speaker.priorityClassName | string | `""` |  |
| speaker.readinessProbe.enabled | bool | `true` |  |
| speaker.readinessProbe.failureThreshold | int | `3` |  |
| speaker.readinessProbe.initialDelaySeconds | int | `10` |  |
| speaker.readinessProbe.periodSeconds | int | `10` |  |
| speaker.readinessProbe.successThreshold | int | `1` |  |
| speaker.readinessProbe.timeoutSeconds | int | `1` |  |
| speaker.reloader.resources | object | `{}` |  |
| speaker.resources | object | `{}` |  |
| speaker.runtimeClassName | string | `""` |  |
| speaker.securityContext | object | `{}` |  |
| speaker.serviceAccount.annotations | object | `{}` |  |
| speaker.serviceAccount.create | bool | `true` |  |
| speaker.serviceAccount.name | string | `""` |  |
| speaker.startupProbe.enabled | bool | `true` |  |
| speaker.startupProbe.failureThreshold | int | `30` |  |
| speaker.startupProbe.periodSeconds | int | `5` |  |
| speaker.tolerateMaster | bool | `true` |  |
| speaker.tolerations | list | `[]` |  |
| speaker.updateStrategy.type | string | `"RollingUpdate"` |  |

----------------------------------------------
Autogenerated from chart metadata using [helm-docs v1.10.0](https://github.com/norwoodj/helm-docs/releases/v1.10.0)

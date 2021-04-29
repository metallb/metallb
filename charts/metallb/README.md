metallb
=======
A network load-balancer implementation for Kubernetes using standard routing protocols

Current chart version is `0.9.6`

Source code can be found [here](https://metallb.universe.tf)



## Chart Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| configInline | object | `{}` |  |
| controller.affinity | object | `{}` |  |
| controller.enabled | bool | `true` |  |
| controller.image.pullPolicy | string | `nil` |  |
| controller.image.repository | string | `"quay.io/metallb/controller"` |  |
| controller.image.tag | string | `nil` |  |
| controller.livenessProbe.enabled | bool | `true` |  |
| controller.livenessProbe.failureThreshold | int | `3` |  |
| controller.livenessProbe.initialDelaySeconds | int | `10` |  |
| controller.livenessProbe.periodSeconds | int | `10` |  |
| controller.livenessProbe.successThreshold | int | `1` |  |
| controller.livenessProbe.timeoutSeconds | int | `1` |  |
| controller.nodeSelector | object | `{}` |  |
| controller.podAnnotations | object | `{}` |  |
| controller.readinessProbe.enabled | bool | `true` |  |
| controller.readinessProbe.failureThreshold | int | `3` |  |
| controller.readinessProbe.initialDelaySeconds | int | `10` |  |
| controller.readinessProbe.periodSeconds | int | `10` |  |
| controller.readinessProbe.successThreshold | int | `1` |  |
| controller.readinessProbe.timeoutSeconds | int | `1` |  |
| controller.resources | object | `{}` |  |
| controller.serviceAccount.annotations | object | `{}` |  |
| controller.serviceAccount.create | bool | `true` |  |
| controller.serviceAccount.name | string | `""` |  |
| controller.tolerations | list | `[]` |  |
| existingConfigMap | string | `""` |  |
| fullnameOverride | string | `""` |  |
| imagePullSecrets | list | `[]` |  |
| nameOverride | string | `""` |  |
| prometheus.podMonitor.enabled | bool | `false` |  |
| prometheus.podMonitor.interval | string | `nil` |  |
| prometheus.podMonitor.jobLabel | string | `"app.kubernetes.io/name"` |  |
| prometheus.podMonitor.metricRelabelings | list | `[]` |  |
| prometheus.podMonitor.relabelings | list | `[]` |  |
| prometheus.prometheusRule.addressPoolExhausted.enabled | bool | `true` |  |
| prometheus.prometheusRule.addressPoolExhausted.labels.severity | string | `"alert"` |  |
| prometheus.prometheusRule.addressPoolUsage.enabled | bool | `true` |  |
| prometheus.prometheusRule.addressPoolUsage.thresholds[0].labels.severity | string | `"warning"` |  |
| prometheus.prometheusRule.addressPoolUsage.thresholds[0].percent | int | `75` |  |
| prometheus.prometheusRule.addressPoolUsage.thresholds[1].labels.severity | string | `"warning"` |  |
| prometheus.prometheusRule.addressPoolUsage.thresholds[1].percent | int | `85` |  |
| prometheus.prometheusRule.addressPoolUsage.thresholds[2].labels.severity | string | `"alert"` |  |
| prometheus.prometheusRule.addressPoolUsage.thresholds[2].percent | int | `95` |  |
| prometheus.prometheusRule.bgpSessionDown.enabled | bool | `true` |  |
| prometheus.prometheusRule.bgpSessionDown.labels.severity | string | `"alert"` |  |
| prometheus.prometheusRule.configNotLoaded.enabled | bool | `true` |  |
| prometheus.prometheusRule.configNotLoaded.labels.severity | string | `"warning"` |  |
| prometheus.prometheusRule.enabled | bool | `false` |  |
| prometheus.prometheusRule.extraAlerts | list | `[]` |  |
| prometheus.prometheusRule.staleConfig.enabled | bool | `true` |  |
| prometheus.prometheusRule.staleConfig.labels.severity | string | `"warning"` |  |
| prometheus.scrapeAnnotations | bool | `false` |  |
| psp.create | bool | `true` |  |
| rbac.create | bool | `true` |  |
| speaker.affinity | object | `{}` |  |
| speaker.enabled | bool | `true` |  |
| speaker.image.pullPolicy | string | `nil` |  |
| speaker.image.repository | string | `"quay.io/metallb/speaker"` |  |
| speaker.image.tag | string | `nil` |  |
| speaker.livenessProbe.enabled | bool | `true` |  |
| speaker.livenessProbe.failureThreshold | int | `3` |  |
| speaker.livenessProbe.initialDelaySeconds | int | `10` |  |
| speaker.livenessProbe.periodSeconds | int | `10` |  |
| speaker.livenessProbe.successThreshold | int | `1` |  |
| speaker.livenessProbe.timeoutSeconds | int | `1` |  |
| speaker.memberlist.enabled | bool | `true` |  |
| speaker.memberlist.mlBindPort | int | `7946` |  |
| speaker.nodeSelector | object | `{}` |  |
| speaker.podAnnotations | object | `{}` |  |
| speaker.readinessProbe.enabled | bool | `true` |  |
| speaker.readinessProbe.failureThreshold | int | `3` |  |
| speaker.readinessProbe.initialDelaySeconds | int | `10` |  |
| speaker.readinessProbe.periodSeconds | int | `10` |  |
| speaker.readinessProbe.successThreshold | int | `1` |  |
| speaker.readinessProbe.timeoutSeconds | int | `1` |  |
| speaker.resources | object | `{}` |  |
| speaker.serviceAccount.annotations | object | `{}` |  |
| speaker.serviceAccount.create | bool | `true` |  |
| speaker.serviceAccount.name | string | `""` |  |
| speaker.tolerateMaster | bool | `true` |  |
| speaker.tolerations | list | `[]` |  |

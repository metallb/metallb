+++
tags = ["config"]
title = "Configuration"
weight = 20
+++

On top of [Hugo's global configuration options](https://gohugo.io/overview/configuration/), the Relearn theme lets you define further options unique to the theme in your `hugo.toml`.

Note that some of these options are explained in detail in other sections of this documentation.

## All config options

The values reflect the options active in this documentation. The defaults can be taken from the [annotated example](#annotated-config-options) below.

{{< multiconfig file=hugo >}}
[params]
{{% include "config/_default/params.toml" %}}
{{< /multiconfig >}}

## Annotated config options

````toml {title="hugo.toml"}
[params]
{{% include "config/_default/params.toml" %}}
````

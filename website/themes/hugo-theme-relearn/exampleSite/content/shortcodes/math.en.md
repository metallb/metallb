+++
description = "Beautiful math and chemical formulae"
title = "Math"
+++

The `math` shortcode generates beautiful formatted math and chemical formulae using the [MathJax](https://mathjax.org/) library.

{{< math align="center" >}}
$$\left( \sum_{k=1}^n a_k b_k \right)^2 \leq \left( \sum_{k=1}^n a_k^2 \right) \left( \sum_{k=1}^n b_k^2 \right)$$
{{< /math >}}

{{% notice note %}}
This only works in modern browsers.
{{% /notice %}}

## Usage

While the examples are using shortcodes with named parameter it is recommended to use codefences instead. This is because more and more other software supports Math codefences (eg. GitHub) and so your markdown becomes more portable.

You are free to also call this shortcode from your own partials.

{{< tabs groupid="shortcode-parameter">}}
{{% tab title="codefence" %}}

````md
```math { align="center" }
$$\left( \sum_{k=1}^n a_k b_k \right)^2 \leq \left( \sum_{k=1}^n a_k^2 \right) \left( \sum_{k=1}^n b_k^2 \right)$$
```
````

{{% /tab %}}
{{% tab title="shortcode" %}}

````go
{{</* math align="center" */>}}
$$\left( \sum_{k=1}^n a_k b_k \right)^2 \leq \left( \sum_{k=1}^n a_k^2 \right) \left( \sum_{k=1}^n b_k^2 \right)$$
{{</* /math */>}}
````

{{% /tab %}}
{{% tab title="partial" %}}

````go
{{ partial "shortcodes/math.html" (dict
  "page"    .
  "content" "$$left( \sum_{k=1}^n a_k b_k \right)^2 \leq \left( \sum_{k=1}^n a_k^2 \right) \left( \sum_{k=1}^n b_k^2 \right)$$"
  "align"   "center"
)}}

````

{{% /tab %}}
{{< /tabs >}}

### Parameter

| Name                  | Default          | Notes       |
|-----------------------|------------------|-------------|
| **align**             | `center`         | Allowed values are `left`, `center` or `right`. |
| _**&lt;content&gt;**_ | _&lt;empty&gt;_  | Your formulae. |

## Configuration

MathJax is configured with default settings. You can customize MathJax's default settings for all of your files thru a JSON object in your `hugo.toml` or override these settings per page thru your pages frontmatter.

The JSON object of your `hugo.toml` / frontmatter is forwarded into MathJax's configuration object.

See [MathJax documentation](https://docs.mathjax.org/en/latest/options/index.html) for all allowed settings.

### Global Configuration File

{{< multiconfig file=hugo >}}
[params]
  mathJaxInitialize = "{ \"chtml\": { \"displayAlign\": \"left\" } }"
{{< /multiconfig >}}

### Page's Frontmatter

{{< multiconfig fm=true >}}
mathJaxInitialize = "{ \"chtml\": { \"displayAlign\": \"left\" } }"
{{< /multiconfig >}}

## Examples

### Inline Math

````md
Inline math is generated if you use a single `$` as a delimiter around your formulae: {{</* math */>}}$\sqrt{3}${{</* /math */>}}
````

Inline math is generated if you use a single `$` as a delimiter around your formulae: {{< math >}}$\sqrt{3}${{< /math >}}

### Blocklevel Math with Right Alignment

````md
If you delimit your formulae by two consecutive `$$` it generates a new block.

{{</* math align="right" */>}}
$$\left( \sum_{k=1}^n a_k b_k \right)^2 \leq \left( \sum_{k=1}^n a_k^2 \right) \left( \sum_{k=1}^n b_k^2 \right)$$
{{</* /math */>}}
````

If you delimit your formulae by two consecutive `$$` it generates a new block.

{{< math align="right" >}}
$$\left( \sum_{k=1}^n a_k b_k \right)^2 \leq \left( \sum_{k=1}^n a_k^2 \right) \left( \sum_{k=1}^n b_k^2 \right)$$
{{< /math >}}

### Codefence

You can also use codefences.

````md
```math
$$\left( \sum_{k=1}^n a_k b_k \right)^2 \leq \left( \sum_{k=1}^n a_k^2 \right) \left( \sum_{k=1}^n b_k^2 \right)$$
```
````

````math
$$\left( \sum_{k=1}^n a_k b_k \right)^2 \leq \left( \sum_{k=1}^n a_k^2 \right) \left( \sum_{k=1}^n b_k^2 \right)$$
````

### Chemical Formulae

````md
{{</* math */>}}
$$\ce{Hg^2+ ->[I-] HgI2 ->[I-] [Hg^{II}I4]^2-}$$
{{</* /math */>}}
`````

{{< math >}}
$$\ce{Hg^2+ ->[I-] HgI2 ->[I-] [Hg^{II}I4]^2-}$$
{{< /math >}}

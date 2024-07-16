+++
description = "Get value of site params"
title = "SiteParam"
+++

The `siteparam` shortcode prints values of site params.

## Usage

While the examples are using shortcodes with named parameter you are free to use positional aswell or call this shortcode from your own partials.

{{< tabs groupid="shortcode-parameter">}}
{{% tab title="shortcode" %}}

````go
{{%/* siteparam name="editURL" */%}}
````

{{% /tab %}}
{{% tab title="shortcode (positional)" %}}

````go
{{%/* siteparam "editURL" */%}}
````

{{% /tab %}}
{{% tab title="partial" %}}

````go
{{ partial "shortcodes/siteparam.html" (dict
  "page" .
  "name" "editURL"
)}}
````

{{% /tab %}}
{{< /tabs >}}

### Parameter

| Name                 | Position | Default          | Notes       |
|----------------------|----------|------------------|-------------|
| **name**             | 1        | _&lt;empty&gt;_  | The name of the site param to be displayed. |

## Examples

### `editURL` from `hugo.toml`

```go
`editURL` value: {{%/* siteparam name="editURL" */%}}
```

`editURL` value: {{% siteparam name="editURL" %}}

### Nested parameter with Markdown and HTML formatting

To use formatted parameter, add this in your `hugo.toml`:

{{< multiconfig file=hugo >}}
[markup.goldmark.renderer]
  unsafe = true
{{< /multiconfig >}}

Now values containing Markdown will be formatted correctly.

{{< multiconfig file=hugo >}}
[params]
  [params.siteparam.test]
    text = "A **nested** parameter <b>with</b> formatting"
{{< /multiconfig >}}

```go
Formatted parameter: {{%/* siteparam name="siteparam.test.text" */%}}
```

Formatted parameter: {{% siteparam name="siteparam.test.text" %}}

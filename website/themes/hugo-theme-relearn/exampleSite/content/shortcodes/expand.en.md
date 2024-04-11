+++
description = "Expandable/collapsible sections of text"
title = "Expand"
+++

The `expand` shortcode displays an expandable/collapsible section of text.

{{% expand title="Expand me..." %}}Thank you!

That's some text with a footnote[^1]

[^1]: And that's the footnote.

That's some more text with a footnote.[^someid]

[^someid]:
    Anything of interest goes here.

    Blue light glows blue.
{{% /expand %}}

{{% notice note %}}
This only works in modern browsers flawlessly. While Internet Explorer 11 has issues in displaying it, the functionality still works.
{{% /notice %}}

## Usage

While the examples are using shortcodes with named parameter you are free to use positional as well or also call this shortcode from your own partials.

{{< tabs groupid="shortcode-parameter">}}
{{% tab title="shortcode" %}}

````go
{{%/* expand title="Expand me..." */%}}Thank you!{{%/* /expand */%}}
````

{{% /tab %}}
{{% tab title="shortcode (positional)" %}}

````go
{{%/* expand "Expand me..." */%}}Thank you!{{%/* /expand */%}}
````

{{% /tab %}}
{{% tab title="partial" %}}

````go
{{ partial "shortcodes/expand.html" (dict
  "page"  .
  "title" "Expand me..."
  "content" "Thank you!"
)}}
````

{{% /tab %}}
{{< /tabs >}}

### Parameter

| Name                  | Position | Default          | Notes       |
|-----------------------|----------|------------------|-------------|
| **title**             | 1        | `"Expand me..."` | Arbitrary text to appear next to the expand/collapse icon. |
| **open**              | 2        | `false`          | When `true` the content text will be initially shown as expanded. |
| _**&lt;content&gt;**_ |          | _&lt;empty&gt;_  | Arbitrary text to be displayed on expand. |

## Examples

### All Defaults

````go
{{%/* expand */%}}Yes, you did it!{{%/* /expand */%}}
````

{{% expand %}}Yes, you did it!{{% /expand %}}

### Initially Expanded

````go
{{%/* expand title="Expand me..." open="true" */%}}No need to press you!{{%/* /expand */%}}
````

{{% expand title="Expand me..." open="true" %}}No need to press you!{{% /expand %}}

### Arbitrary Text

````go
{{%/* expand title="Show me almost **endless** possibilities" */%}}
You can add standard markdown syntax:

- multiple paragraphs
- bullet point lists
- _emphasized_, **bold** and even **_bold emphasized_** text
- [links](https://example.com)
- etc.

```plaintext
...and even source code
```

> the possibilities are endless (almost - including other shortcodes may or may not work)
{{%/* /expand */%}}
````

{{% expand title="Show me almost **endless** possibilities" %}}
You can add standard markdown syntax:

- multiple paragraphs
- bullet point lists
- _emphasized_, **bold** and even **_bold emphasized_** text
- [links](https://example.com)
- etc.

```plaintext
...and even source code
```

> the possibilities are endless (almost - including other shortcodes may or may not work)
{{% /expand %}}

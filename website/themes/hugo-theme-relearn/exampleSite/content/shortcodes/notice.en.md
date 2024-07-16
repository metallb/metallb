+++
description = "Disclaimers to help you structure your page"
title = "Notice"
+++

The `notice` shortcode shows various types of disclaimers with adjustable color, title and icon to help you structure your page.

{{% notice style="primary" title="There may be pirates" icon="skull-crossbones" %}}
It is all about the boxes.
{{% /notice %}}

## Usage

While the examples are using shortcodes with named parameter you are free to use positional as well or also call this shortcode from your own partials.

{{< tabs groupid="shortcode-parameter">}}
{{% tab title="shortcode" %}}

````go
{{%/* notice style="primary" title="There may be pirates" icon="skull-crossbones" */%}}
It is all about the boxes.
{{%/* /notice */%}}
````

{{% /tab %}}
{{% tab title="shortcode (positional)" %}}

````go
{{%/* notice primary "There may be pirates" "skull-crossbones" */%}}
It is all about the boxes.
{{%/* /notice */%}}
````

{{% /tab %}}
{{% tab title="partial" %}}

````go
{{ partial "shortcodes/notice.html" (dict
  "page"  .
  "style" "primary"
  "title" "There may be pirates"
  "icon" "skull-crossbones"
  "content" "It is all about the boxes."
)}}
````

{{% /tab %}}
{{< /tabs >}}

### Parameter

| Name      | Position | Default   | Notes       |
|-----------|----------|-----------|-------------|
| **style** | 1        | `default` | The style scheme used for the box.<br><br>- by severity: `info`, `note`, `tip`, `warning`<br>- by brand color: `primary`, `secondary`, `accent`<br>- by color: `blue`, `green`, `grey`, `orange`, `red`<br>- by special color: `default`, `transparent`, `code` |
| **color** |          | see notes | The [CSS color value](https://developer.mozilla.org/en-US/docs/Web/CSS/color_value) to be used. If not set, the chosen color depends on the **style**. Any given value will overwrite the default.<br><br>- for severity styles: a nice matching color for the severity<br>- for all other styles: the corresponding color |
| **title** | 2        | see notes | Arbitrary text for the box title. Depending on the **style** there may be a default title. Any given value will overwrite the default.<br><br>- for severity styles: the matching title for the severity<br>- for all other styles: _&lt;empty&gt;_<br><br>If you want no title for a severity style, you have to set this parameter to `" "` (a non empty string filled with spaces) |
| **icon**  | 3        | see notes | [Font Awesome icon name](shortcodes/icon#finding-an-icon) set to the left of the title. Depending on the **style** there may be a default icon. Any given value will overwrite the default.<br><br>- for severity styles: a nice matching icon for the severity<br>- for all other styles: _&lt;empty&gt;_<br><br>If you want no icon for a severity style, you have to set this parameter to `" "` (a non empty string filled with spaces) |
| _**&lt;content&gt;**_ |          | _&lt;empty&gt;_ | Arbitrary text to be displayed in box. |

## Examples

### By Severity

#### Info with markup

````go
{{%/* notice style="info" */%}}
An **information** disclaimer

You can add standard markdown syntax:

- multiple paragraphs
- bullet point lists
- _emphasized_, **bold** and even ***bold emphasized*** text
- [links](https://example.com)
- etc.

```plaintext
...and even source code
```

> the possibilities are endless (almost - including other shortcodes may or may not work)
{{%/* /notice */%}}
````

{{% notice style="info" %}}
An **information** disclaimer

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
{{% /notice %}}

#### Note

````go
{{%/* notice style="note" */%}}
A **notice** disclaimer
{{%/* /notice */%}}
````

{{% notice style="note" %}}
A **notice** disclaimer
{{% /notice %}}

#### Tip

````go
{{%/* notice style="tip" */%}}
A **tip** disclaimer
{{%/* /notice */%}}
````

{{% notice style="tip" %}}
A **tip** disclaimer
{{% /notice %}}

#### Warning

````go
{{%/* notice style="warning" */%}}
A **warning** disclaimer
{{%/* /notice */%}}
````

{{% notice style="warning" %}}
A **warning** disclaimer
{{% /notice %}}

#### Warning with Non-Default Title and Icon

````go
{{%/* notice style="warning" title="Here are dragons" icon="dragon" */%}}
A **warning** disclaimer
{{%/* /notice */%}}
````

{{% notice style="warning" title="Here are dragons" icon="dragon" %}}
A **warning** disclaimer
{{% /notice %}}

#### Warning without a Title and Icon

````go
{{%/* notice style="warning" title=" " icon=" " */%}}
A **warning** disclaimer
{{%/* /notice */%}}
````

{{% notice style="warning" title=" " icon=" " %}}
A **warning** disclaimer
{{% /notice %}}

### By Brand Colors

#### Primary with Title only

````go
{{%/* notice style="primary" title="Primary" */%}}
A **primary** disclaimer
{{%/* /notice */%}}
````

{{% notice style="primary" title="Primary" %}}
A **primary** disclaimer
{{% /notice %}}

#### Secondary with Icon only

````go
{{%/* notice style="secondary" icon="stopwatch" */%}}
A **secondary** disclaimer
{{%/* /notice */%}}
````

{{% notice style="secondary" icon="stopwatch" %}}
A **secondary** disclaimer
{{% /notice %}}

#### Accent

````go
{{%/* notice style="accent" */%}}
An **accent** disclaimer
{{%/* /notice */%}}
````

{{% notice style="accent" %}}
An **accent** disclaimer
{{% /notice %}}

### By Color

#### Blue without a Title and Icon

````go
{{%/* notice style="blue" */%}}
A **blue** disclaimer
{{%/* /notice */%}}
````

{{% notice style="blue" %}}
A **blue** disclaimer
{{% /notice %}}

#### Green with Title only

````go
{{%/* notice style="green" title="Green" */%}}
A **green** disclaimer
{{%/* /notice */%}}
````

{{% notice style="green" title="Green" %}}
A **green** disclaimer
{{% /notice %}}

#### Grey with Icon only

````go
{{%/* notice style="grey" icon="bug" */%}}
A **grey** disclaimer
{{%/* /notice */%}}
````

{{% notice style="grey" icon="bug" %}}
A **grey** disclaimer
{{% /notice %}}

#### Orange with Title and Icon

````go
{{%/* notice style="orange" title="Orange" icon="bug" */%}}
A **orange** disclaimer
{{%/* /notice */%}}
````

{{% notice style="orange" title="Orange" icon="bug" %}}
A **orange** disclaimer
{{% /notice %}}

#### Red without a Title and Icon

````go
{{%/* notice style="red" */%}}
A **red** disclaimer
{{%/* /notice */%}}
````

{{% notice style="red" %}}
A **red** disclaimer
{{% /notice %}}

### By Special Color

#### Default with Positional Parameter

````go
{{%/* notice default "Pay Attention to this Note!" "skull-crossbones" */%}}
Some serious information.
{{%/* /notice */%}}
````

{{% notice default "Pay Attention to this Note!" "skull-crossbones" %}}
Some serious information.
{{% /notice %}}

#### Transparent with Title and Icon

````go
{{%/* notice style="transparent" title="Pay Attention to this Note!" icon="skull-crossbones" */%}}
Some serious information.
{{%/* /notice */%}}
````

{{% notice style="transparent" title="Pay Attention to this Note!" icon="skull-crossbones" %}}
Some serious information.
{{% /notice %}}

### With User-Defined Color, Font Awesome Brand Icon and Markdown Title

````go
{{%/* notice color="fuchsia" title="**Hugo**" icon="fa-fw fab fa-hackerrank" */%}}
Victor? Is it you?
{{%/* /notice */%}}
````

{{% notice color="fuchsia" title="**Hugo**" icon="fa-fw fab fa-hackerrank" %}}
Victor? Is it you?
{{% /notice %}}

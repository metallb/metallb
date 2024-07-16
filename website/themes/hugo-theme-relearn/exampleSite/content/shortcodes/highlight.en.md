+++
description = "Render code with a syntax highlighter"
title = "Highlight"
+++

The `highlight` shortcode renders your code with a syntax highlighter.

{{< highlight lineNos="true" type="py" wrap="true" title="python" >}}
print("Hello World!")
{{< /highlight >}}

## Usage

This shortcode is fully compatible with Hugo's [`highlight` shortcode](https://gohugo.io/content-management/syntax-highlighting/#highlight-shortcode) but **offers some extensions**.

It is called interchangeably in the same way as Hugo's own shortcode providing positional parameter or by simply using codefences.

You are free to also call this shortcode from your own partials. In this case it resembles Hugo's [`highlight` function](https://gohugo.io/functions/highlight/) syntax if you call this shortcode as a partial using compatibility syntax.

While the examples are using shortcodes with named parameter it is recommended to use codefences instead. This is because more and more other software supports codefences (eg. GitHub) and so your markdown becomes more portable.

{{< tabs groupid="shortcode-parameter">}}
{{% tab title="codefence" %}}

````md
```py { lineNos="true" wrap="true" title="python" }
print("Hello World!")
```
````

{{% /tab %}}
{{% tab title="shortcode" %}}

````go
{{</* highlight lineNos="true" type="py" wrap="true" title="python" */>}}
print("Hello World!")
{{</* /highlight */>}}
````

{{% /tab %}}
{{% tab title="shortcode (positional)" %}}

````go
{{</* highlight py "lineNos=true,wrap=true,title=python" */>}}
print("Hello World!")
{{</* /highlight */>}}
````

{{% /tab %}}
{{% tab title="partial" %}}

````go
{{ partial "shortcodes/highlight.html" (dict
  "page"    .
  "content" "print(\"Hello World!\")"
  "lineNos" "true"
  "type"    "py"
  "wrap"    "true"
  "title"   "python"
)}}
````

{{% /tab %}}
{{% tab title="partial (compat)" %}}

````go
{{ partial "shortcodes/highlight.html" (dict
  "page"    .
  "content" "print(\"Hello World!\")"
  "options" "lineNos=true,wrap=true,title=python"
  "type"    "py"
)}}
````

{{% /tab %}}
{{< /tabs >}}

### Parameter

| Name                  | Position | Default          | Notes       |
|-----------------------|--------- | -----------------|-------------|
| **type**              | 1        | _&lt;empty&gt;_  | The language of the code to highlight. Choose from one of the [supported languages](https://gohugo.io/content-management/syntax-highlighting/#list-of-chroma-highlighting-languages). Case-insensitive. |
| **title**             |          | _&lt;empty&gt;_  | **Extension**. Arbitrary title for code. This displays the code like a [single tab](shortcodes/tab) if `hl_inline=false` (which is Hugos default). |
| **wrap**              |          | see notes        | **Extension**. When `true` the content may wrap on long lines otherwise it will be scrollable.<br><br>The default value can be set in your `hugo.toml` and overwritten via frontmatter. [See below](#configuration). |
| **options**           | 2        | _&lt;empty&gt;_  | An optional, comma-separated list of zero or more [Hugo supported options](https://gohugo.io/functions/highlight/#options) as well as extension parameter from this table. |
| _**&lt;option&gt;**_  |          | _&lt;empty&gt;_  | Any of [Hugo's supported options](https://gohugo.io/functions/highlight/#options). |
| _**&lt;content&gt;**_ |          | _&lt;empty&gt;_  | Your code to highlight. |

## Configuration

Default values for [Hugo's supported options](https://gohugo.io/functions/highlight/#options) can be set via [goldmark settings](https://gohugo.io/getting-started/configuration-markup/#highlight) in your `hugo.toml`

Default values for extension options can be set via params settings in your `hugo.toml` or be overwritten by frontmatter for each individual page.

### Global Configuration File

You can configure the color style used for code blocks in your [color variants stylesheet](basics/branding#syntax-highlightning) file.

#### Recommended Settings

{{< multiconfig file=hugo >}}
[markup]
  [markup.highlight]
    lineNumbersInTable = false
    noClasses = false
{{< /multiconfig >}}

#### Optional Settings

{{< multiconfig file=hugo >}}
[params]
  highlightWrap = true
{{< /multiconfig >}}

### Page's Frontmatter

{{< multiconfig fm=true >}}
highlightWrap = true
{{< /multiconfig >}}

## Examples

### Line Numbers with Starting Offset

As mentioned above, line numbers in a `table` layout will shift if code is wrapping, so better use `inline`. To make things easier for you, set `lineNumbersInTable = false` in your `hugo.toml` and add `lineNos = true` when calling the shortcode instead of the specific values `table` or `inline`.

````go
{{</* highlight lineNos="true" lineNoStart="666" type="py" */>}}
# the hardest part is to start writing code; here's a kickstart; just copy and paste this; it's free; the next lines will cost you serious credits
print("Hello")
print(" ")
print("World")
print("!")
{{</* /highlight */>}}
````

{{< highlight lineNos="true" lineNoStart="666" type="py" >}}
# the hardest part is to start writing code; here's a kickstart; just copy and paste this; it's free; the next lines will cost you serious credits
print("Hello")
print(" ")
print("World")
print("!")
{{< /highlight >}}

### Codefence with Title


````md
```py { title="python" }
# a bit shorter
print("Hello World!")
```
````

```py { title="python" }
# a bit shorter
print("Hello World!")
```



### With Wrap

````go
{{</* highlight type="py" wrap="true" hl_lines="2" */>}}
# Quicksort Python One-liner
lambda L: [] if L==[] else qsort([x for x in L[1:] if x< L[0]]) + L[0:1] + qsort([x for x in L[1:] if x>=L[0]])
# Some more stuff
{{</* /highlight */>}}
````

{{< highlight type="py" wrap="true" hl_lines="2" >}}
# Quicksort Python One-liner
lambda L: [] if L==[] else qsort([x for x in L[1:] if x< L[0]]) + L[0:1] + qsort([x for x in L[1:] if x>=L[0]])
# Some more stuff
{{< /highlight >}}

### Without Wrap

````go
{{</* highlight type="py" wrap="false" hl_lines="2" */>}}
# Quicksort Python One-liner
lambda L: [] if L==[] else qsort([x for x in L[1:] if x< L[0]]) + L[0:1] + qsort([x for x in L[1:] if x>=L[0]])
# Some more stuff
{{</* /highlight */>}}
````

{{< highlight type="py" wrap="false" hl_lines="2">}}
# Quicksort Python One-liner
lambda L: [] if L==[] else qsort([x for x in L[1:] if x< L[0]]) + L[0:1] + qsort([x for x in L[1:] if x>=L[0]])
# Some more stuff
{{< /highlight >}}

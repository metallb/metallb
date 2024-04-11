+++
alwaysopen = false
description = "List the child pages of a page"
title = "Children"
+++

The `children` shortcode lists the child pages of the current page and its descendants.

{{% children sort="weight" %}}

## Usage

While the examples are using shortcodes with named parameter you are free to also call this shortcode from your own partials.

{{< tabs groupid="shortcode-parameter">}}
{{% tab title="shortcode" %}}

````go
{{%/* children sort="weight" */%}}
````

{{% /tab %}}
{{% tab title="partial" %}}

````go
{{ partial "shortcodes/children.html" (dict
  "page" .
  "sort" "weight"
)}}
````

{{% /tab %}}
{{< /tabs >}}

### Parameter

| Name               | Default           | Notes       |
|--------------------|-------------------|-------------|
| **containerstyle** | `ul`              | Choose the style used to group all children. It could be any HTML tag name. |
| **style**          | `li`              | Choose the style used to display each descendant. It could be any HTML tag name. |
| **showhidden**     | `false`           | When `true`, child pages hidden from the menu will be displayed as well. |
| **description**    | `false`           | When `true` shows a short text under each page in the list. When no description or summary exists for the page, the first 70 words of the content is taken - [read more info about summaries on gohugo.io](https://gohugo.io/content/summaries/). |
| **depth**          | `1`               | The depth of descendants to display. For example, if the value is `2`, the shortcode will display two levels of child pages.  To get all descendants, set this value to a high  number eg. `999`. |
| **sort**           | `auto`            | The sort criteria of the displayed list.<br><br>- `auto` defaults to [`ordersectionsby` of the pages frontmatter](cont/frontmatter)<br>&nbsp;&nbsp;&nbsp;&nbsp;or to [`ordersectionsby` of the site configuration](basics/configuration)<br>&nbsp;&nbsp;&nbsp;&nbsp;or to `weight`<br>- `weight`<br>- `title`<br>- `linktitle`<br>- `modifieddate`<br>- `expirydate`<br>- `publishdate`<br>- `date`<br>- `length`<br>- `default` adhering to Hugo's default sort criteria|

## Examples

### All Default

````go
{{%/* children  */%}}
````

{{% children %}}

### With Description

````go
{{%/* children description="true" */%}}
````

{{%children description="true" %}}

### Infinite Depth and Hidden Pages

````go
{{%/* children depth="999" showhidden="true" */%}}
````

{{% children depth="999" showhidden="true" %}}

### Heading Styles for Container and Elements

````go
{{%/* children containerstyle="div" style="h2" depth="3" description="true" */%}}
````

{{% children containerstyle="div" style="h2" depth="3" description="true" %}}

### Divs for Group and Element Styles

````go
{{%/* children containerstyle="div" style="div" depth="3" */%}}
````

{{% children containerstyle="div" style="div" depth="3" %}}

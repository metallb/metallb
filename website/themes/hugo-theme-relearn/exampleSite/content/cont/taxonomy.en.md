+++
categories = ["taxonomy", "content"]
tags = "tutorial"
title = "Taxonomy"
weight = 8
+++

The Relearn theme supports Hugo's default taxonomies _tag_ and _category_ out of the box.

## Configuration

Just add tags and/or categories to any page. They can be given as a single string or an array of strings.

{{< multiconfig fm=true >}}
categories = ["taxonomy", "content"]
tags = "tutorial"
title = "Taxonomy"
{{< /multiconfig >}}

## Behavior

The tags are displayed at the top of the page in alphabetical order.

The categories are displayed at the bottom of the page in alphabetical order in the default implementation of the theme but can be customized by providing your own `content-footer.html` partial.

Each item is a link to a taxonomy page displaying all the articles with the given term.

## List all the tags

In the `hugo.toml`  file you can add a shortcut to display all the tags and categories

{{< multiconfig file=hugo >}}
[[menu.shortcuts]]
name = "<i class='fa-fw fas fa-tags'></i> Tags"
url = "/tags"

[[menu.shortcuts]]
name = "<i class='fa-fw fas fa-layer-group'></i> Categories"
url = "/categories"
{{< /multiconfig >}}

## Customization

If you define [custom taxonomies](https://gohugo.io/content-management/taxonomies/#configure-taxonomies) and want to display a list of them somewhere on your page (often in the `layouts/partials/content-footer.html`) you can call a partial that does the job for you:

````go
{{ partial "term-list.html" (dict
  "page" .
  "taxonomy" "categories"
  "icon" "layer-group"
) }}
````

### Parameter

| Name                  | Default         | Notes       |
|-----------------------|-----------------|-------------|
| **page**              | _&lt;empty&gt;_ | Mandatory reference to the page. |
| **taxonomy**          | _&lt;empty&gt;_ | The plural name of the taxonomy to display as used in your frontmatter. |
| **class**             | _&lt;empty&gt;_ | Additional CSS classes set on the outermost generated HTML element.<br><br>If set to `tags` you will get the visuals for displaying the _tags_ taxonomy, otherwise it will be a simple list of links as for the _categories_ taxonomy. |
| **style**             | `primary`       | The style scheme used if **class** is `tags`.<br><br>- by severity: `info`, `note`, `tip`, `warning`<br>- by brand color: `primary`, `secondary`, `accent`<br>- by color: `blue`, `green`, `grey`, `orange`, `red`<br>- by special color: `default`, `transparent`, `code` |
| **color**             | see notes       | The [CSS color value](https://developer.mozilla.org/en-US/docs/Web/CSS/color_value) to be used if **class** is `tags`. If not set, the chosen color depends on the **style**. Any given value will overwrite the default.<br><br>- for severity styles: a nice matching color for the severity<br>- for all other styles: the corresponding color |
| **icon**              | _&lt;empty&gt;_ | An optional [Font Awesome icon name](shortcodes/icon#finding-an-icon) set to the left of the list. |

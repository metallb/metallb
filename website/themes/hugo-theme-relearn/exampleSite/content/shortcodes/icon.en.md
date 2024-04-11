+++
aliases = "/cont/icons"
description = "Nice icons for your page"
title = "Icon"
+++

The `icon` shortcode displays icons using the [Font Awesome](https://fontawesome.com) library.

{{% icon exclamation-triangle %}}
{{% icon angle-double-up %}}
{{% icon skull-crossbones %}}

## Usage

While the examples are using shortcodes with positional parameter you are free to also call this shortcode from your own partials.

{{< tabs groupid="shortcode-parameter">}}
{{% tab title="shortcode" %}}

````go
{{%/* icon icon="exclamation-triangle" */%}}
{{%/* icon icon="angle-double-up" */%}}
{{%/* icon icon="skull-crossbones" */%}}
````

{{% /tab %}}
{{% tab title="shortcode (positional)" %}}

````go
{{%/* icon exclamation-triangle */%}}
{{%/* icon angle-double-up */%}}
{{%/* icon skull-crossbones */%}}
````

{{% /tab %}}
{{% tab title="partial" %}}

````go
{{ partial "shortcodes/icon.html" (dict
    "page" .
    "icon" "exclamation-triangle"
)}}
{{ partial "shortcodes/icon.html" (dict
    "page" .
    "icon" "angle-double-up"
)}}
{{ partial "shortcodes/icon.html" (dict
    "page" .
    "icon" "skull-crossbones"
)}}
````

{{% /tab %}}
{{< /tabs >}}

### Parameter

| Name                  | Position | Default         | Notes       |
|-----------------------|----------|-----------------|-------------|
| **icon**              | 1        | _&lt;empty&gt;_ | [Font Awesome icon name](#finding-an-icon) to be displayed. It will be displayed in the text color of its according context. |

### Finding an icon

Browse through the available icons in the [Font Awesome Gallery](https://fontawesome.com/v5/search?m=free). Notice that the **free** filter is enabled, as only the free icons are available by default.

Once on the Font Awesome page for a specific icon, for example the page for the [heart](https://fontawesome.com/v5/icons/heart?s=solid), copy the icon name and paste into the Markdown content.

### Customising Icons

Font Awesome provides many ways to modify the icon

- Change color (by default the icon will inherit the parent color)
- Increase or decrease size
- Rotate
- Combine with other icons

Check the full documentation on [web fonts with CSS](https://fontawesome.com/how-to-use/web-fonts-with-css) for more.

## Examples

### Standard Usage

````go
Built with {{%/* icon heart */%}} by Relearn and Hugo
````

Built with {{% icon heart %}} by Relearn and Hugo

### Advanced HTML Usage

While the shortcode simplifies using standard icons, the icon customization and other advanced features of the Font Awesome library require you to use HTML directly. Paste the `<i>` HTML into markup, and Font Awesome will load the relevant icon.

````html
Built with <i class="fas fa-heart"></i> by Relearn and Hugo
````

Built with <i class="fas fa-heart"></i> by Relearn and Hugo

To use these native HTML elements in your Markdown, add this in your `hugo.toml`:

````toml
[markup.goldmark.renderer]
    unsafe = true
````

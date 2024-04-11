+++
title = "Image Effects"
weight = 5
+++

The theme supports non-standard [image effects](cont/markdown#image-effects).

| Name     | Description                                                       |
| -------- | ----------------------------------------------------------------- |
| border   | Draws a light thin border around the image                        |
| lazy     | Lets the image be lazy loaded                                     |
| lightbox | The image will be clickable to show it enlarged                   |
| shadow   | Draws a shadow around the image to make it appear hovered/glowing |

As [described](cont/markdown#image-effects), you can add this to the URL query parameter, but this may be cumbersome to be done consistently for the whole page.

Instead, you can configure the defaults in your `hugo.toml` aswell as overriding these default in the pages frontmatter.

Explicitly set URL query parameter will override the defaults in effect for a page.

Without any settings in your `hugo.toml` this defaults to

{{< multiconfig file=hugo >}}
[params]
  [params.imageEffects]
    border = false
    lazy = true
    lightbox = true
    shadow = false
{{< /multiconfig >}}

This can be overridden in a pages frontmatter by eg.

{{< multiconfig fm=true >}}
[imageEffects]
  border = true
{{< /multiconfig >}}

Or by explicitly override settings by URL query parameter

````md {title="URL"}
![Minion](https://octodex.github.com/images/minion.png?lightbox=false&bg-white=true)
````

The settings applied to the above image would be

{{< multiconfig >}}
border = true
lazy = true
lightbox = false
shadow = false
bg-white = true
{{< /multiconfig >}}

This ends up in the following HTML where the parameter are converted to CSS classes.

````html {title="HTML"}
<img src="https://octodex.github.com/images/minion.png?lightbox=false&bg-white=true" loading="lazy" alt="Minion" class="bg-white border lazy nolightbox noshadow">
````


## Extending

As you can see in the above example, the `bg-white` parameter is not initially supported in the themes default settings. Nevertheless you are free to define arbitrary parameter by just adding them to the URL query parameter or set them in your `hugo.toml` or pages frontmatter.

{{% notice note %}}
If no extended parameter like `bg-white` in the example is set on the URL, a `class="nobg-white"` in the HTML will only be generated if a default value was set in the `hugo.toml` or pages frontmatter.
{{% /notice %}}

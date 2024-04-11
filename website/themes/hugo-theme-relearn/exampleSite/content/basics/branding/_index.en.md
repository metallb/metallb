+++
categories = ["custom", "theming"]
title = "Branding"
weight = 24
+++

The Relearn theme provides configuration options to change your your site's colors, favicon and logo. This allows you to easily align your site visuals to your desired style. Most of these options are exposed thru so called color variants.

A color variant lets you customize various visual effects of your site like almost any color, used fonts, color schemes of print, syntax highligtning, Mermaid and the OpenAPI shortcode, etc. It contains of a CSS file and optional configuration options in your `hugo.toml`.

The Relearn theme ships with a wide set of different color variants. You can use them as-is, copy them over and use them as a starting point for your customizations or just create completely new variants unique to your site. The [interactive variant generator](basics/generator) may help you with this task.

Once configured in your `hugo.toml`, you can select them with the variant selector at the bottom of the menu.

## Change the Variant (Simple) {#theme-variant}

### Single Variant

Set the `themeVariant` value to the name of your theme file. That's it! Your site will be displayed in this variant only.

{{< multiconfig file=hugo >}}
[params]
  themeVariant = "relearn-light"
{{< /multiconfig >}}

{{% notice note %}}
Your theme variant file must reside in your site's `static/css` directory or in the theme's `static/css` directory and the file name must start with `theme-` and end wit `.css`. In the above example, the path of your theme file must be `static/css/theme-relearn-light.css`.

If you want to make changes to a shipped color variant, create a copy in your site's `static/css` directory. Don't edit the file in the theme's directory!
{{% /notice %}}

### Multiple Variants

You can also set multiple variants. In this case, the first variant is the default chosen on first view and a variant selector will be shown in the menu footer if the array contains more than one entry.

{{< multiconfig file=hugo >}}
[params]
  themeVariant = [ "relearn-light", "relearn-dark" ]
{{< /multiconfig >}}

{{% notice tip %}}
The theme provides an advanced configuration mode, combining the functionality for multiple variants with the below possibilities of adjusting to your OS settings and syntax highlightning and even more!

Although all options documented here are still working, the advanced configuration options are the recommended way to configure your color variants. [See below](#theme-variant-advanced).
{{% /notice %}}

## Adjust to OS Settings

You can also cause the site to adjust to your OS settings for light/dark mode. Just set the `themeVariant` to `auto` to become an auto mode variant. That's it.

You can use the `auto` value with the single or multiple variants option. If you are using multiple variants, you can drop `auto` at any position in the option's array, but usually it makes sense to set it in the first position and make it the default.

{{< multiconfig file=hugo >}}
[params]
  themeVariant = [ "auto", "red" ]
{{< /multiconfig >}}

If you don't configure anything else, the theme will default to use `relearn-light` for light mode and `relearn-dark` for dark mode. These defaults are overwritten by the first two non-auto options of your `themeVariant` option if present.

In the above example, you would end with `red` for light mode and the default of `relearn-dark` for dark mode.

If you don't like that behavior, you can explicitly set `themeVariantAuto`. The first entry in the array is the color variant for light mode, the second for dark mode.

{{< multiconfig file=hugo >}}
[params]
  themeVariantAuto = [ "learn", "neon" ]
{{< /multiconfig >}}

## Change the Favicon

If your favicon is a SVG, PNG or ICO, just drop your image in your site's `static/images/` directory and name it `favicon.svg`, `favicon.png` or `favicon.ico` respectively.

If you want to adjust your favicon according to your OS settings for light/dark mode, add the image files `static/images/favicon-light.svg` and `static/images/favicon-dark.svg` to your site's directory, respectively, corresponding to your file format. In case some of the files are missing, the theme falls back to `favicon.svg` for each missing file. All supplied favicons must be of the same file format.

If no favicon file is found, the theme will lookup the alternative filename `logo` in the same location and will repeat the search for the list of supported file types.

If you need to change this default behavior, create a new file `layouts/partials/favicon.html` in your site's directory and write something like this:

````html {title="layouts/partials/favicon.html"}
<link rel="icon" href="/images/favicon.bmp" type="image/bmp">
````

## Change the Logo

Create a new file in `layouts/partials/logo.html` of your site. Then write any HTML you want. You could use an `img` HTML tag and reference an image created under the _static_ folder, or you could paste a SVG definition!

{{% notice note %}}
The size of the logo will adapt automatically.
{{% /notice %}}

## Syntax Highlightning

If you want to switch the syntax highlighting theme together with your color variant, you need to configure your installation [according to Hugo's documentation](https://gohugo.io/content-management/syntax-highlighting/) and provide a syntax highlighting stylesheet file.

You can use a one of the shipped stylesheet files or use Hugo to generate a file for you. The file must be written to `static/css/chroma-<NAME>.css`. To use it with your color variant you have to define `--CODE-theme: <NAME>` in the color variant stylesheet file.

For an example, take a look into [`theme-relearn-light.css`](https://github.com/McShelby/hugo-theme-relearn/blob/main/static/css/theme-relearn-light.css) and [`hugo.toml`](https://github.com/McShelby/hugo-theme-relearn/blob/main/exampleSite/config/_default/hugo.toml) of the exampleSite.

## Change the Variant (Advanced) {#theme-variant-advanced}

The theme offers a new way to configure theme variants and all of the aspects above inside of a single configuration item. This comes with some features previously unsupported.

Like with the [multiple variants](#multiple-variants) option, you are defining your theme variants in an array but now _not by simple strings_ **but in a table with suboptions**.

Again, in this case, the first variant is the default chosen on first view and a variant selector will be shown in the menu footer if the array contains more than one entry.

{{< multiconfig file=hugo >}}
[params]
  themeVariant = [ "relearn-light", "relearn-dark" ]
{{< /multiconfig >}}

you now write it that way:

{{< multiconfig file=hugo >}}
[params]
  [[params.themeVariant]]
    identifier = "relearn-light"
  [[params.themeVariant]]
    identifier = "relearn-dark"
{{< /multiconfig >}}

The `identifier` option is mandatory and equivalent to the string in the first example. Further options can be configured, see the table below.

### Parameter

| Name                  | Default         | Notes       |
|-----------------------|-----------------|-------------|
| identifier            | _&lt;empty&gt;_ | Must correspond to the name of a color variant either in your site's or the theme's directory in the form `static/css/theme-<IDENTIFIER>.css`. |
| name                  | see notes       | The name to be displayed in the variant selector. If not set, the identifier is used in a human readable form. |
| auto                  | _&lt;empty&gt;_ | If set, the variant is treated as an [auto mode variant](#adjust-to-os-settings). It has the same behavior as the `themeVariantAuto` option. The first entry in the array is the color variant for light mode, the second for dark mode. Defining auto mode variants with the advanced options has the benefit that you can now have multiple auto mode variants instead of just one with the simple options. |

### Example Configuration of This Site

{{< multiconfig file=hugo >}}
[params]
  [[params.themeVariant]]
    identifier = "relearn-auto"
    name = "Relearn Light/Dark"
    auto = []
  [[params.themeVariant]]
    identifier = "relearn-light"
  [[params.themeVariant]]
    identifier = "relearn-dark"
  [[params.themeVariant]]
    identifier = "zen-auto"
    name = "Zen Light/Dark"
    auto = [ "zen-light", "zen-dark" ]
  [[params.themeVariant]]
    identifier = "zen-light"
  [[params.themeVariant]]
    identifier = "zen-dark"
  [[params.themeVariant]]
    identifier = "neon"
{{< /multiconfig >}}

## Modify Shipped Variants

In case you like a shipped variant but only want to tweak some aspects, you have two choices:

1. Copy and change

    You can copy the shipped variant file from the theme's `static/css` directory to the site's `static/css` directory and either store it with the same name or give it a new name. Edit the settings and save the new file. Afterwards you can use it in your `hugo.toml` by the choosen name.

2. Create and import

    You can create a new variant file in the site's `static/css` directory and give it a new name. Import the shipped variant, add the settings you want to change and save the new file. Afterwards you can use it in your `hugo.toml` by the choosen name.

    For example, you want to use the `relearn-light` variant but want to change the syntax highlightning schema to the one used in the `neon` variant. For that, create a new `static/css/theme-my-branding.css` in your site's directory and add the following lines:

    ````css {title="static/css/theme-my-branding.css"}
    @import "theme-relearn-light.css";

    :root {
      --CODE-theme: neon; /* name of the chroma stylesheet file */
      --CODE-BLOCK-color: rgba( 226, 228, 229, 1 ); /* fallback color for code text */
      --CODE-BLOCK-BG-color: rgba( 40, 42, 54, 1 ); /* fallback color for code background */
    }
    ````

    Afterwards put this in your `hugo.toml` to use your new variant:

    {{< multiconfig file=hugo >}}
    [params]
      themeVariant = "my-branding"
    {{< /multiconfig >}}

    In comparison to _copy and change_, this has the advantage that you profit from any adjustments to the `relearn-light` variant but keep your modifications.

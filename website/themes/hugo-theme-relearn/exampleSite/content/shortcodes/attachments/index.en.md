+++
description = "List of files attached to a page"
hidden = "true"
title = "Attachments"
+++

The `attachments` shortcode displays a list of files attached to a page with adjustable color, title and icon.

{{% attachments sort="asc" /%}}

{{% notice warning %}}
This shortcode is deprecated in favor of the new [`resources` shortcode](shortcodes/resources). See [migration instructions](#migration) below.
{{% /notice %}}

## Migration

While this shortcode will still be available for some time, it does not receive support anymore. Start to migrate early, as it will be removed with the next major update of the theme.

The `resources` shortcode leverages Hugo's resource feature for page bundles. It has all the same parameter as the `attachments` shortcode but applies the `pattern` directly on a resources `Name` attribute.

To migrate your pages apply the following steps:

1. If a page is not already a [page bundle](https://gohugo.io/content-management/page-bundles/) convert it
2. Move your files to a valid destination inside of your page bundle (depending if you have a branch or a leaf bundle)
3. Change the calls from the `attachments` shortcode to the [`resources` shortcode](shortcodes/resources) and adjust the `pattern` parameter to the new directory layout and the resources [`Name` attribute](https://gohugo.io/methods/resource/name/).

Multilanguage features are not supported directly by the shortcode anymore but rely on Hugo's handling for resource translations.

## Usage

While the examples are using shortcodes with named parameter you are free to also call this shortcode from your own partials.

{{< tabs groupid="shortcode-parameter">}}
{{% tab title="shortcode" %}}

````go
{{%/* attachments sort="asc" /*/%}}
````

{{% /tab %}}
{{% tab title="partial" %}}

````go
{{ partial "shortcodes/attachments.html" (dict
  "page" .
  "sort" "asc"
)}}
````

{{% /tab %}}
{{< /tabs >}}

### Parameter

| Name        | Default         | Notes       |
|-------------|-----------------|-------------|
| **style**   | `transparent`   | The style scheme used for the box.<br><br>- by severity: `info`, `note`, `tip`, `warning`<br>- by brand color: `primary`, `secondary`, `accent`<br>- by color: `blue`, `green`, `grey`, `orange`, `red`<br>- by special color: `default`, `transparent`, `code` |
| **color**   | see notes       | The [CSS color value](https://developer.mozilla.org/en-US/docs/Web/CSS/color_value) to be used. If not set, the chosen color depends on the **style**. Any given value will overwrite the default.<br><br>- for severity styles: a nice matching color for the severity<br>- for all other styles: the corresponding color |
| **title**   | see notes       | Arbitrary text for the box title. Depending on the **style** there may be a default title. Any given value will overwrite the default.<br><br>- for severity styles: the matching title for the severity<br>- for all other styles: `Attachments`<br><br>If you want no title for a severity style, you have to set this parameter to `" "` (a non empty string filled with spaces) |
| **icon**    | see notes       | [Font Awesome icon name](shortcodes/icon#finding-an-icon) set to the left of the title. Depending on the **style** there may be a default icon. Any given value will overwrite the default.<br><br>- for severity styles: a nice matching icon for the severity<br>- for all other styles: `paperclip`<br><br>If you want no icon, you have to set this parameter to `" "` (a non empty d with spaces) |
| **sort**    | `asc`           | Sorting the output in `asc`ending or `desc`ending order. |
| **pattern** | `.*`            | A [regular expressions](https://en.wikipedia.org/wiki/Regular_expression), used to filter the attachments by file name. For example:<br><br>- to match a file suffix of 'jpg', use `.*\.jpg` (not `*.\.jpg`)<br>- to match file names ending in `jpg` or `png`, use `.*\.(jpg\|png)` |

## Setup

### Single language

The shortcode lists files found in a specific folder. The name of the folder depends on your page type (either branch bundle, leaf bundle or page).

1. If your page is a leaf bundle, attachments must be placed in a nested `index.files` folder, accordingly.

    > * content
    >   * _index.md
    >   * page
    >     * index.md
    >     * **index.files**
    >       * attachment.pdf

2. If your page is a branch bundle, attachments must be placed in a nested `_index.files` folder, accordingly.

    {{% badge style="warning" title=" " %}}Warning{{% /badge %}} This is only available for Hugo < `0.112.0`

    > * content
    >   * _index.md
    >   * page
    >     * _index.md
    >     * **_index.files**
    >       * attachment.pdf

3. For simple pages, attachments must be placed in a folder named like your page and ending with `.files`.

    {{% badge style="warning" title=" " %}}Warning{{% /badge %}} This is only available for Hugo < `0.112.0`

    > * content
    >   * _index.md
    >   * **page.files**
    >     * attachment.pdf
    >   * page.md

### Multilingual

Be aware that if you use a multilingual website, you will need to have as many folders as languages and the language code must be part of the folder name.

Eg. for a site in English and Piratish:

  > * content
  >   * index.en.md
  >   * index.pir.md
  >   * page
  >     * index.en.md
  >     * index.pir.md
  >     * **index.en.files**
  >       * attachment.pdf
  >     * **index.pir.files**
  >       * attachment.pdf

## Examples

### Custom Title, List of Attachments Ending in pdf or mp4

````go
{{%/* attachments title="Related **files**" pattern=".*\.(pdf|mp4)" /*/%}}
````

{{% attachments title="Related **files**" pattern=".*\.(pdf|mp4)" /%}}

### Info Styled Box, Descending Sort Order

````go
{{%/* attachments style="info" sort="desc" /*/%}}
````

{{% attachments style="info" sort="desc" /%}}

### With User-Defined Color and Font Awesome Brand Icon

````go
{{%/* attachments color="fuchsia" icon="fa-fw fab fa-hackerrank" /*/%}}
````

{{% attachments color="fuchsia" icon="fa-fw fab fa-hackerrank" /%}}

### Style, Color, Title and Icons

For further examples for **style**, **color**, **title** and **icon**, see the [`notice` shortcode](shortcodes/notice) documentation. The parameter are working the same way for both shortcodes, besides having different defaults.

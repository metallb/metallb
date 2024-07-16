+++
title = "Customization"
weight = 25
+++

## Usage scenarios

The theme is usable in different scenarios, requiring the following mandatory settings in your `hugo.toml`. All settings not mentioned can be set to your liking.

### Public Webserver from Root

{{< multiconfig file=hugo >}}
baseURL = "https://example.com/"
{{< /multiconfig >}}

### Public Webserver from Subdirectory

{{< multiconfig file=hugo >}}
baseURL = "https://example.com/mysite/"
relativeURLs = false
{{< /multiconfig >}}

### Private Webserver (LAN)

The same settings as with any of the public webserver usage scenarios or

{{< multiconfig file=hugo >}}
baseURL = "/"
relativeURLs = true
{{< /multiconfig >}}

### File System

{{< multiconfig file=hugo >}}
baseURL = "/"
relativeURLs = true
{{< /multiconfig >}}

{{% notice warning %}}
Using a `baseURL` with a subdirectory and `relativeURLs=true` are mutally exclusive due to the fact, that [Hugo does not apply the `baseURL` correctly](https://github.com/gohugoio/hugo/issues/12130).

If you need both, you have to generate your site twice but with different settings into separate directories.
{{% /notice %}}

{{% notice note %}}
Sublemental pages (like `sitemap.xml`, `rss.xml`) and generated social media links inside of your pages will always be generated with absolute URLs and will not work if you set `relativeURLs=true`.
{{% /notice %}}

{{% notice info %}}
If you are using `uglyURLs=false` (Hugo's default), the theme will append an additional `index.html` to all page links to make your site be servable from the file system. If you don't care about the file system and only serve your page via a webserver you can generate the links without this:

{{< multiconfig file=hugo >}}
[params]
  disableExplicitIndexURLs = true
{{< /multiconfig >}}
{{% /notice %}}

## Activate search

If not already present, add the following lines in your `hugo.toml` file.

{{< multiconfig file=hugo >}}
[outputs]
  home = ["html", "rss", "search"]
{{< /multiconfig >}}

This will generate a search index file at the root of your public folder ready to be consumed by the Lunr search library. Note that the `search` outputformat was named `json` in previous releases but was implemented differently. Although `json` still works, it is now deprecated.

{{% notice note %}}
If you want to use the search feature from the file system, migrating from an older installation of the theme, make sure to change your outputformat for the homepage from the now deprecated `json` to `search` [as seen below](#activate-search).
{{% /notice %}}

### Activate dedicated search page

You can add a dedicated search page for your page by adding the `searchpage` outputformat to your home page by adding the following lines in your `hugo.toml` file. This will cause Hugo to generate a new file `http://example.com/mysite/search.html`.

{{< multiconfig file=hugo >}}
[outputs]
  home = ["html", "rss", "search", "searchpage"]
{{< /multiconfig >}}

You can access this page by either clicking on the magnifier glass or by typing some search term and pressing `ENTER` inside of the menu's search box .

![Screenshot of the dedicated search page](search_page.png?&width=60pc)

{{% notice note %}}
To have Hugo create the dedicated search page successfully, you must not generate the URL `http://example.com/mysite/search.html` from your own content. This can happen if you set `uglyURLs=true` in your `hugo.toml` and defining a Markdown file `content/search.md`.

To make sure, there is no duplicate content for any given URL of your project, run `hugo --printPathWarnings`.
{{% /notice %}}

## Activate print support

You can activate print support to add the capability to print whole chapters or even the complete site. Just add the `print` output format to your home, section and page in your `hugo.toml` as seen below:

{{< multiconfig file=hugo >}}
[outputs]
  home = ["html", "rss", "print", "search"]
  section = ["html", "rss", "print"]
  page = ["html", "rss", "print"]
{{< /multiconfig >}}

This will add a little printer icon in the top bar. It will switch the page to print preview when clicked. You can then send this page to the printer by using your browser's usual print functionality.

{{% notice note %}}
The resulting URL will not be [configured ugly](https://gohugo.io/templates/output-formats/#configure-output-formats) in terms of [Hugo's URL handling](https://gohugo.io/content-management/urls/#ugly-urls) even if you've set `uglyURLs=true` in your `hugo.toml`. This is due to the fact that for one mime type only one suffix can be configured.

Nevertheless, if you're unhappy with the resulting URLs you can manually redefine `outputFormats.print` in your own `hugo.toml` to your liking.
{{% /notice %}}

## Home Button Configuration

If the `disableLandingPageButton` option is set to `false`, a Home button will appear
on the left menu. It is an alternative for clicking on the logo. To edit the
appearance, you will have to configure the `landingPageName` for the defined languages:

{{< multiconfig file=hugo >}}
[languages]
[languages.en]
[languages.en.params]
landingPageName = "<i class='fa-fw fas fa-home'></i> Home"
[languages.pir]
[languages.pir.params]
landingPageName = "<i class='fa-fw fas fa-home'></i> Arrr! Homme"
{{< /multiconfig >}}

If this option is not configured for a specific language, they will get their default values:

{{< multiconfig >}}
landingPageName = "<i class='fa-fw fas fa-home'></i> Home"
{{< /multiconfig >}}

The home button is going to look like this:

![Default Home Button](home_button_defaults.png?width=18.75rem)

## Social Media Meta Tags

You can add social media meta tags for the [Open Graph](https://gohugo.io/templates/internal/#open-graph) protocol and [Twitter Cards](https://gohugo.io/templates/internal/#twitter-cards) to your site. These are configured as mentioned in the Hugo docs.

## Change the Menu Width

The menu width adjusts automatically for different screen sizes.

| Name | Screen Width  | Menu Width |
| ---- | ------------- | ---------- |
| S    | < 48rem       | 14.375rem  |
| M    | 48rem - 60rem | 14.375rem  |
| L    | >= 60rem      | 18.75rem   |

The values for the screen width breakpoints aren't configurable.

If you want to adjust the menu width you can define the following CSS variables in your `custom-header.html`. Note that `--MENU-WIDTH-S` applies to the menu flyout width in mobile mode for small screen sizes.

````css
:root {
    --MENU-WIDTH-S: 14.375rem;
    --MENU-WIDTH-M: 14.375rem;
    --MENU-WIDTH-L: 18.75rem;
}
````

## Change the Main Area's Max Width

By default the main area width will only grow to a certain extent if more vertical screen space is available. This is done for readability purposes as long line are usually harder to read.

If you are unhappy with the default, you can define the following CSS variable in your `custom-header.html` and set the value to your liking. If you want to use all available space, select a really big value like `1000rem`;

````css
:root {
    --MAIN-WIDTH-MAX: 80.25rem;
}
````

## Own Shortcodes with JavaScript Dependencies

Certain shortcodes make use of additional dependencies like JavaScript and CSS files. The theme only loads these dependencies if the shortcode is used. To do so correctly the theme adds management code in various files.

You can you use this mechanism in your own shortcodes. Say you want to add a shortcode `myshortcode` that also requires the `jquery` JavaScript library.

1. Write the shortcode file `layouts/shortcodes/myshortcode.html` and add the following line

    ````go {title="layouts/shortcodes/myshortcode.html"}
   {{- .Page.Store.Set "hasMyShortcode" true }}
    ````

1. Add the following snippet to your `hugo.toml`

    {{< multiconfig file=hugo >}}
    [params.relearn.dependencies]
      [params.relearn.dependencies.myshortcode]
        name = "MyShortcode"
    {{< /multiconfig >}}

1. Add the dependency loader file `layouts/partials/dependencies/myshortcode.html`. The loader file will be called from multiple locations inside of the theme with the parameter `page` containing the current [page](https://gohugo.io/methods/page/) variable and `location` with one of the currently defined locations

    * `header`: if called at the end of the HTML `head` element
    * `footer`: if called at the end of the HTML `body` element

    ````go {title="layouts/partials/dependencies/myshortcode.html"}
    {{- if eq .location "footer" }}
      <script src="https://www.unpkg.com/jquery/dist/jquery.js"></script>
    {{- end }}
    ````

Character casing is relevant!

- the `name` setting in your `hugo.toml` must match the key (that needs to be prefixed with a `has`) you used for the store in your `layouts/shortcodes/myshortcode.html`.
- the key on `params.relearn.dependencies` in your `hugo.toml` must match the base file name of your loader file.

See the `math`, `mermaid` and `openapi` shortcodes for examples.

{{% notice note %}}
If you are really into customization of the theme and want to use the dependency loader for your own locations, you can do this by simply calling it from inside of your overriden partials

````go
{{- partial "dependencies.html" (dict "page" . "location" "mylocation") }}
````

{{% /notice %}}

## Output Formats

Certain parts of the theme can be changed for support of your own [output formats](https://gohugo.io/templates/output-formats/). Eg. if you define a new output format `PLAINTEXT` in your `hugo.toml`, you can add a file `layouts/partials/header.plaintext.html` to change the way, the page header should look like for that output format.

## React to Variant Switches in JavaScript

Once a color variant is fully loaded, either initially or by switching the color variant manually with the variant selector, the custom event `themeVariantLoaded` on the `document` will be dispatched. You can add an event listener and react to changes.

````javascript
document.addEventListener( 'themeVariantLoaded', function( e ){
  console.log( e.detail.variant ); // `relearn-light`
});
````

## Partials

The Relearn theme has been built to be as configurable as possible by defining multiple [partials](https://gohugo.io/templates/partials/)

In `themes/hugo-theme-relearn/layouts/partials/`, you will find all the partials defined for this theme. If you need to overwrite something, don't change the code directly. Instead [follow this page](https://gohugo.io/themes/customizing/). You'd create a new partial in the `layouts/partials` folder of your local project. This partial will have the priority.

This theme defines the following partials :

- `header.html`: the header of the page. See [output-formats](#output-formats)
- `footer.html`: the footer of the page. See [output-formats](#output-formats)
- `body.html`: the body of the page. The body may contain of one or many articles. See [output-formats](#output-formats)
- `article.html`: the output for a single article, can contain elements around your content. See [output-formats](#output-formats)
- `menu.html`: left menu. _Not meant to be overwritten_
- `search.html`: search box. _Not meant to be overwritten_
- `custom-header.html`: custom headers in page. Meant to be overwritten when adding CSS imports. Don't forget to include `style` HTML tag directive in your file.
- `custom-footer.html`:  custom footer in page. Meant to be overwritten when adding JavaScript. Don't forget to include `javascript` HTML tag directive in your file.
- `favicon.html`: the favicon
- `heading.html`: side-wide configuration to change the pages title headings.
- `heading-pre.html`: side-wide configuration to prepend to pages title headings. If you override this, it is your responsibility to take the page's `headingPre` setting into account.
- `heading-post.html`: side-wide configuration to append to pages title headings. If you override this, it is your responsibility to take the page's `headingPost` setting into account.
- `logo.html`: the logo, on top left hand corner
- `meta.html`: HTML meta tags, if you want to change default behavior
- `menu-pre.html`: side-wide configuration to prepend to menu items. If you override this, it is your responsibility to take the page's `menuPre` setting into account.
- `menu-post.html`: side-wide configuration to append to menu items. If you override this, it is your responsibility to take the page's `menuPost` setting into account.
- `menu-footer.html`: footer of the the left menu
- `toc.html`: table of contents
- `content.html`: the content page itself. This can be overridden if you want to display page's meta data above or below the content.
- `content-header.html`: header above the title, has a default implementation but you can overwrite it if you don't like it.
- `content-footer.html`: footer below the content, has a default implementation but you can overwrite it if you don't like it.

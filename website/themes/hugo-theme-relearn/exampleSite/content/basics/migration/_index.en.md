+++
disableToc = false
title = "What's New"
weight = 2
+++

This document shows you what's new in the latest release and flags it with one of the following badges. For a detailed list of changes, see the [history page](basics/history).

- {{% badge color="fuchsia" icon="fa-fw fab fa-hackerrank" title=" " %}}0.121.0{{% /badge %}} The minimum required Hugo version.

- {{% badge style="warning" title=" " %}}Breaking{{% /badge %}} A change that requires action by you after upgrading to assure the site is still functional.

- {{% badge style="note" title=" " %}}Change{{% /badge %}} A change in default behavior that may requires action by you if you want to revert it.

- {{% badge style="info" icon="plus-circle" title=" " %}}New{{% /badge %}} Marks new behavior you might find interesting or comes configurable.

<!--GH-ACTION-RELEASE-MILESTONE-->

---

## 5.27.0 (2024-04-07) {#5270}

- {{% badge color="fuchsia" icon="fa-fw fab fa-hackerrank" title=" " %}}0.121.0{{% /badge %}} This release requires a newer Hugo version.

- {{% badge style="note" title=" " %}}Change{{% /badge %}} If the theme is configured to to generate warnings or errors during build by setting `image.errorlevel` to either `warning` or `error` in your `hugo.toml`, it will now also generate output if a link fragment is not found in the target page.

- {{% badge style="note" title=" " %}}Change{{% /badge %}} The [dependency loader](basics/customization#own-shortcodes-with-javascript-dependencies) was made more versatile.

  The configuration in your `hugo.toml` does not require the `location` parameter anymore. If you still use it, the theme will work as before but will generate a warning. So you don't need to change anything, yet.

  With the new mechanism, your dependency loader now receives an additional `location` parameter instead that you can query to inject your dependencies in the desired location.

  By that you can now call the dependency mechanism in your own overriden partials by giving it a distinct `location` parameter. In addition your injected files can now be spread to multiple locations which wasn't previously possible.

- {{% badge style="info" icon="plus-circle" title=" " %}}New{{% /badge %}} Additional styling was added for the native HTML elements `<mark>` and `<kbd>`. To use them you must allow the [usage of HTML](https://gohugo.io/getting-started/configuration-markup/#rendererunsafe) in your `hugo.toml`. The [Markdown documentation](cont/markdown/#standard-and-extensions) was enhanced for this.

- {{% badge style="info" icon="plus-circle" title=" " %}}New{{% /badge %}} You now can scroll forward and backward thru all headings of a page by using <kbd>ALT</kbd> <kbd>🡑</kbd> and <kbd>ALT</kbd> <kbd>🡓</kbd>. This also works for the `PRINT` output format.

- {{% badge style="info" icon="plus-circle" title=" " %}}New{{% /badge %}} The breadcrumbs used in the topbar, search results and the taxonomy term lists are now using the pages frontmatter `linktitle` instead of `title` if set.

---

## 5.26.0 (2024-03-18) {#5260}

- {{% badge style="info" icon="plus-circle" title=" " %}}New{{% /badge %}} The lazy loading of images is now configurable by using the new `lazy` [image effect](cont/imageeffects). The default value hasn't changed in comparison to older versions, you don't need to change anything.

- {{% badge style="info" icon="plus-circle" title=" " %}}New{{% /badge %}} It is now possible to [adjust the max width of the main area](basics/customization#change-the-main-areas-max-width), eg. in case you want to use the full page width for your content.

- {{% badge style="info" icon="plus-circle" title=" " %}}New{{% /badge %}} Images and Codefences are now respecting [Hugo's Markdown attributes](https://gohugo.io/content-management/markdown-attributes/).

- {{% badge style="info" icon="plus-circle" title=" " %}}New{{% /badge %}} The theme has updated its Mermaid dependency to 10.6.0. This adds support for [block diagrams](shortcodes/mermaid#block-diagram).

- {{% badge style="info" icon="plus-circle" title=" " %}}New{{% /badge %}} This release fixes a long outstanding bug where the page wasn't repositioning correctly when going forward or backward in your browser history.

---

## 5.25.0 (2024-02-29) {#5250}

- {{% badge style="note" title=" " %}}Change{{% /badge %}} This release deprecates the [`attachments` shortcode](shortcodes/attachments) in favor of the new the [`resources` shortcode](shortcodes/resources).

  If you are using Hugo below {{% badge color="fuchsia" icon="fa-fw fab fa-hackerrank" title=" " %}}0.123.0{{% /badge %}}, you don't need to change anything as the old shortcode still works (but may generate warnings).

  Anyways, users are strongly advised to migrate as the `attachments` shortcode will not receive support anymore. Migration instructions are listed on the [`attachments` shortcode page](shortcodes/attachments#migration).

- {{% badge style="note" title=" " %}}Change{{% /badge %}} If you run Hugo with [GitInfo](https://gohugo.io/methods/page/gitinfo/) configured, the default page footer now prints out name, email address and date of the last commit. If you want to turn this off you either have to run Hugo without GitInfo (which is the default) or overwrite the `content-footer.html` partial.

## 5.24.0 (2024-02-28) {#5240}

- {{% badge color="fuchsia" icon="fa-fw fab fa-hackerrank" title=" " %}}0.112.4{{% /badge %}} This release requires a newer Hugo version.

- {{% badge style="note" title=" " %}}Change{{% /badge %}} The topbar button received a way to add text next to the icon. For this, the original `title` option was renamed to `hint` while the new `title` option is now displayed next to the icon.

- {{% badge style="note" title=" " %}}Change{{% /badge %}} The frontmatter option `menuTitle` is now deprecated in favor for Hugo's own `linkTitle`. You don't need to change anything as the old `menuTitle` option is still supported.

- {{% badge style="note" title=" " %}}Change{{% /badge %}} The light themes have a bit more contrast for content text and headings. Also the syntaxhighlighting was changed to the more colorful MonokaiLight. This brings the syntaxhighlightning in sync with the corresponding dark theme variants, which are using Monokai. If you dislike this, you can create your own color variant file as [described here](basics/branding#modify-shipped-variants).

- {{% badge style="info" icon="plus-circle" title=" " %}}New{{% /badge %}} If the theme can not resolve a link to a page or image, you can now generate warnings or errors during build by setting `link.errorlevel` or `image.errorlevel` to either `warning` or `error` in your `hugo.toml` respectively. By default this condition is silently ignored and the link is written as-is.

  Please note that a page link will generate false negatives if `uglyURLs=true` and it references an ordinary page before {{% badge color="fuchsia" icon="fa-fw fab fa-hackerrank" title=" " %}}0.123.0{{% /badge %}}.

  Please note that an image link will generate false negatives if the file resides in your `static` directory.

- {{% badge style="info" icon="plus-circle" title=" " %}}New{{% /badge %}} You now can configure additional options for every theme variant in your `hugo.toml`. This allows for optional [advanced functionality](basics/branding#theme-variant-advanced). You don't need to change anything as the old configuration options will still work (but may generate warnings now).

  The advanced functionality allows you to set an explicit name for a theme variant and now allows for multiple auto mode variants that adjust to the light/dark preference of your OS settings.

- {{% badge style="info" icon="plus-circle" title=" " %}}New{{% /badge %}} New partial for defining the heading. See [documentation](basics/customization#partials) for further reading.

- {{% badge style="info" icon="plus-circle" title=" " %}}New{{% /badge %}} Support for Hugo's built-in [`figure` shortcode](https://gohugo.io/content-management/shortcodes/#figure).

- {{% badge style="info" icon="plus-circle" title=" " %}}New{{% /badge %}} On taxonomy and term pages you can now use prev/next navigation as within the normal page structure.

- {{% badge style="info" icon="plus-circle" title=" " %}}New{{% /badge %}} In additiion to the existing [menu width customization](basics/customization#change-the-menu-width), it is now also possible to set the width of the menu flyout for small screen sizes with the `--MENU-WIDTH-S` CSS property.

- {{% badge style="info" icon="plus-circle" title=" " %}}New{{% /badge %}} Improvements for accessibility when tabbing thru the page for images, links and tab handles.

- {{% badge style="info" icon="plus-circle" title=" " %}}New{{% /badge %}} The `editURL` config parameter is now [overwritable in your pages frontmatter](cont/frontmatter). In addition it received more versatility by letting you control where to put the file path into the URL. This is achieved by replacing the variable `${FilePath}` in your URL by the pages file path. You don't need to change anything in your existing configuration as the old way without the replacement variable still works.

- {{% badge style="info" icon="plus-circle" title=" " %}}New{{% /badge %}} The themes [config](basics/configuration) and [frontmatter](cont/frontmatter) options received a comprehensive documentation update. In addition the theme switched from `config.toml` to `hugo.toml`.

- {{% badge style="info" icon="plus-circle" title=" " %}}New{{% /badge %}} Restored compatibility with Hugo versions {{% badge color="fuchsia" icon="fa-fw fab fa-hackerrank" title=" " %}}0.121.0{{% /badge %}} or higher for the [`highlight` shortcode](shortcodes/highlight). This does not change the minimum required Hugo version.

- {{% badge style="info" icon="plus-circle" title=" " %}}New{{% /badge %}} Restored compatibility with Hugo versions {{% badge color="fuchsia" icon="fa-fw fab fa-hackerrank" title=" " %}}0.123.0{{% /badge %}} or higher for theme specific [output formats](basics/customization) and handling of taxonomy and term titles. This does not change the minimum required Hugo version.

---

## 5.23.0 (2023-11-03) {#5230}

- {{% badge style="note" title=" " %}}Change{{% /badge %}} With {{% badge color="fuchsia" icon="fa-fw fab fa-hackerrank" title=" " %}}0.120.0{{% /badge %}} the author settings move into the `[params]` array in your `hugo.toml`. Because this collides with the previous way, the theme expected author information, it now adheres to Hugo standards and prints out a warning during built if something is wrong.

  Change your previous setting from

    {{< multiconfig file=hugo >}}
    [params]
      author = "Hugo"
    {{< /multiconfig >}}

  to

    {{< multiconfig file=hugo >}}
    [params]
      author.name = "Hugo"
    {{< /multiconfig >}}

- {{% badge style="note" title=" " %}}Change{{% /badge %}} Taxonomy [term pages](https://gohugo.io/content-management/taxonomies#add-custom-metadata-to-a-taxonomy-or-term) now add the breadcrumb for each listed page. If this gets too crowded for you, you can turn the breadcrumbs off in your `hugo.toml` by adding `disableTermBreadcrumbs=true`.

- {{% badge style="info" icon="plus-circle" title=" " %}}New{{% /badge %}} Taxonomy and term pages are now allowed to contain content. This is added inbetween the title and the page list.

- {{% badge style="info" icon="plus-circle" title=" " %}}New{{% /badge %}} It is now possible to print custom taxonomies anywhere in your page. [See the docs](cont/taxonomy#customization).

- {{% badge style="info" icon="plus-circle" title=" " %}}New{{% /badge %}} It is now possible to adjust the menu width for your whole site. [See the docs](basics/customization#change-the-menu-width).

- {{% badge style="info" icon="plus-circle" title=" " %}}New{{% /badge %}} This release adds social media meta tags for the Open Graph protocol and Twitter Cards to your site. [See the docs](basics/customization#social-media-meta-tags).

- {{% badge style="info" icon="plus-circle" title=" " %}}New{{% /badge %}} This release comes with additional sort options for the menu and the [`children` shortcode](shortcodes/children). Both will now accept the following values: `weight`, `title`, `linktitle`, `modifieddate`, `expirydate`, `publishdate`, `date`, `length` or `default` (adhering to Hugo's default sort order).

- {{% badge style="info" icon="plus-circle" title=" " %}}New{{% /badge %}} The theme now provides a mechanism to load further JavaScript dependencies defined by you only if it is needed. This comes in handy if you want to add own shortcodes that depend on additional JavaScript code to be loaded. [See the docs](basics/customization#own-shortcodes-with-javascript-dependencies).

- {{% badge style="info" icon="plus-circle" title=" " %}}New{{% /badge %}} The theme has updated its Mermaid dependency to 10.6.0. This adds support for the [xychart type](shortcodes/mermaid#xychart).

- {{% badge style="info" icon="plus-circle" title=" " %}}New{{% /badge %}} This release adds portable Markdown links.

  Previously it was not possible to use pure Markdown links in a configuration independend way to link to pages inside of your project. It always required you to know how your `uglyURLs` setting is, wheather you link to a page or page bundle and in case of relative links if your current page is a page or page bundle. (eg. `[generator](generator/index.html)` vs. `[generator](generator.html)`). This is a hassle as you have to change these links manually once you change your `uglyURLs` setting or change the type of a page.

  You could work around this by using the `relref` shortcode (eg `[generator]({{%/* relref "../generator" */%}})`) which works but results in non-portable Markdown.

  Now it's possible to use the same path of a call to `relref` in a plain Markdown link (eg `[generator](../generator)`). This is independend of any configuration settings or the page types involved in linking. Note, that this requires your links to be given without any extension, so `[generator](generator/index.html)` will work as before.

  The following types of linking are supported:

  | link                               | description                                 |
  | ---------------------------------- | ------------------------------------------- |
  | `[generator](en/basics/generator)` | absolute from your project root (multilang) |
  | `[generator](/en/basics/generator)`| absolute from your project root (multilang) |
  | `[generator](basics/generator)`    | absolute from your current language root    |
  | `[generator](/basics/generator)`   | absolute from your current language root    |
  | `[generator](./../generator)`      | relative from the current page              |
  | `[generator](../generator)`        | relative from the current page              |

---

## 5.22.0 (2023-10-02) {#5220}

- {{% badge style="note" title=" " %}}Change{{% /badge %}} This release fixes an issue where in unfortunate conditions DOM ids generated by Hugo may collide with DOM ids set by the theme. To avoid this, all theme DOM ids are now prefixed with `R-`.

  If you haven't modified anything, everything is fine. Otherwise you have to check your custom CSS rules and JavaScript code.

- {{% badge style="note" title=" " %}}Change{{% /badge %}} You can now have structural sections in the hierarchical menu without generating a page for it.

  This can come in handy, if content for such a section page doesn't make much sense to you. See [the documentation](cont/frontmatter#disable-section-pages) for how to do this.

  This feature may require you to make changes to your existing installation if you are already using _[shortcuts to pages inside of your project](cont/menushortcuts#shortcuts-to-pages-inside-of-your-project)_ with a _headless branch parent_.

  In this case it is advised to remove the `title` from the headless branch parent's frontmatter, as it will otherwise appear in your breadcrumbs.

- {{% badge style="info" icon="plus-circle" title=" " %}}New{{% /badge %}} It is now possible to overwrite the setting for `collapsibleMenu` of your `hugo.toml` inside of a page's frontmatter.

- {{% badge style="info" icon="plus-circle" title=" " %}}New{{% /badge %}} If a Mermaid graph is zoomable a button to reset the view is now added to the upper right corner. The button is only shown once the mouse is moved over the graph.

- {{% badge style="info" icon="plus-circle" title=" " %}}New{{% /badge %}} It is now possible to remove the root breadcrumb by setting `disableRootBreadcrumb=true` in your `hugo.toml`.

- {{% badge style="info" icon="plus-circle" title=" " %}}New{{% /badge %}} The output of the dedicated search page now displays the result's breadcrumb.

- {{% badge style="info" icon="plus-circle" title=" " %}}New{{% /badge %}} Table rows now change their background color on every even row.

- {{% badge style="info" icon="plus-circle" title=" " %}}New{{% /badge %}} Translation into Swahili. This language is not supported for search.

---

## 5.21.0 (2023-09-18) {#5210}

- {{% badge style="note" title=" " %}}Change{{% /badge %}} We made changes to the menu footer to improve alignment with the menu items in most cases. Care was taken not to break your existing overwritten footer. Anyways, if you have your `menu-footer.html` [partial overridden](basics/customization#partials), you may want to review the styling (eg. margins/paddings) of your partial.

- {{% badge style="info" icon="plus-circle" title=" " %}}New{{% /badge %}} This release comes with an awesome new feature, that allows you to customize your topbar buttons, change behavior, reorder them or define entirely new ones, unique to your installation. See [the documentation](basics/topbar) for further details.

- {{% badge style="info" icon="plus-circle" title=" " %}}New{{% /badge %}} The theme has updated its Swagger dependency to 5.7.2 for the [`openapi` shortcode](shortcodes/openapi). This brings support for OpenAPI Specification 3.1.

---

## 5.20.0 (2023-08-26) {#5200}

- {{% badge style="note" title=" " %}}Change{{% /badge %}} The theme has updated its Swagger dependency to 5.4.1 for the [`openapi` shortcode](shortcodes/openapi).

  With this comes a change in the light theme variants of `Relearn Bright`, `Relearn Light` and `Zen Light` by switching the syntaxhighlightning inside of openapi to a light scheme. This brings it more in sync with the code style used by the theme variants itself.

  Additionally, the syntaxhighlightning inside of openapi for printing was switched to a light scheme for all theme variants.

  If you dislike this change, you can revert this in your theme variants CSS by adding

    ````css
    --OPENAPI-CODE-theme: obsidian;
    --PRINT-OPENAPI-CODE-theme: obsidian;
    ````

- {{% badge style="note" title=" " %}}Change{{% /badge %}} For consistency reasons, we renamed the CSS variable `--MENU-SECTION-HR-color` to `--MENU-SECTION-SEPARATOR-color`. You don't need to change anything in your custom color stylesheet as the old name will be used as a fallback.

- {{% badge style="info" icon="plus-circle" title=" " %}}New{{% /badge %}} The theme variants `Zen Light` and `Zen Dark` now add more contrast between menu, topbar and content by adding thin borders.

  Those borders are now configurable by using the CSS variables `--MAIN-TOPBAR-BORDER-color`, `--MENU-BORDER-color`, `--MENU-TOPBAR-BORDER-color`, `--MENU-TOPBAR-SEPARATOR-color`, `--MENU-HEADER-SEPARATOR-color` and `--MENU-SECTION-ACTIVE-CATEGORY-BORDER-color`.

  For existing variants nothing has changed visually.

- {{% badge style="info" icon="plus-circle" title=" " %}}New{{% /badge %}} The default values for the [image effects](cont/markdown#image-effects) are [now configurable](cont/imageeffects) for your whole site via `hugo.toml` or for each page thru frontmatter.

- {{% badge style="info" icon="plus-circle" title=" " %}}New{{% /badge %}} This release fixes a long outstanding bug where Mermaid graphs could not be displayed if they were initially hidden - like in collapsed `expand` or inactive `tabs`.

- {{% badge style="info" icon="plus-circle" title=" " %}}New{{% /badge %}} Restored compatibility with Hugo versions lower than {{% badge color="fuchsia" icon="fa-fw fab fa-hackerrank" title=" " %}}0.111.0{{% /badge %}} for the [`highlight` shortcode](shortcodes/highlight). This does not change the minimum required Hugo version.

---

## 5.19.0 (2023-08-12) {#5190}

- {{% badge style="info" icon="plus-circle" title=" " %}}New{{% /badge %}} The [`highlight` shortcode](shortcodes/highlight) now accepts the new parameter `title`. This displays the code like a [single tab](shortcodes/tab). This is also available using codefences and makes it much easier to write nicer code samples.

- {{% badge style="info" icon="plus-circle" title=" " %}}New{{% /badge %}} The theme has added two new color variants `zen-light` and `zen-dark`. Check it out!

- {{% badge style="info" icon="plus-circle" title=" " %}}New{{% /badge %}} The theme now [dispatches the custom event](basics/customization#react-to-variant-switches-in-javascript) `themeVariantLoaded` on the `document` when the variant is fully loaded either initially or by switching the variant manually with the variant selector.

- {{% badge style="info" icon="plus-circle" title=" " %}}New{{% /badge %}} The theme has updated its Mermaid dependency to 10.3.1. This adds support for the [sankey diagram type](shortcodes/mermaid#sankey) and now comes with full support for YAML inside Mermaid graphs (previously, the theme ignored explicit Mermaid theme settings in YAML).

- {{% badge style="info" icon="plus-circle" title=" " %}}New{{% /badge %}} Translation into Hungarian.

---

## 5.18.0 (2023-07-27) {#5180}

- {{% badge style="note" title=" " %}}Change{{% /badge %}} The theme adds additional warnings for deprecated or now unsupported features.

- {{% badge style="note" title=" " %}}Change{{% /badge %}} There are visual improvements in displaying text links in your content aswell as to some other clickable areas in the theme. If you've overwritten some theme styles in your own CSS, keep this in mind.

- {{% badge style="info" icon="plus-circle" title=" " %}}New{{% /badge %}} Restored compatibility with Hugo {{% badge color="fuchsia" icon="fa-fw fab fa-hackerrank" title=" " %}}0.95.0{{% /badge %}} or higher. This does not change the minimum required Hugo version.

- {{% badge style="info" icon="plus-circle" title=" " %}}New{{% /badge %}} The [`siteparam` shortcode](shortcodes/siteparam) is now capable in displaying nested params aswell as supporting text formatting.

---

## 5.17.0 (2023-06-22) {#5170}

- {{% badge style="note" title=" " %}}Change{{% /badge %}} The default behavior for the copy-to-clipboard feature for code blocks has changed.

  The copy-to-clipboard button for code blocks will now only be displayed if the reader hovers the code block.

  If you dislike this new behavior you can turn it off and revert to the old behavior by adding `[params] disableHoverBlockCopyToClipBoard=true` to your `hugo.toml`.

- {{% badge style="info" icon="plus-circle" title=" " %}}New{{% /badge %}} Restored compatibility with Hugo {{% badge color="fuchsia" icon="fa-fw fab fa-hackerrank" title=" " %}}0.114.0{{% /badge %}} or higher. This does not change the minimum required Hugo version.

- {{% badge style="info" icon="plus-circle" title=" " %}}New{{% /badge %}} The new [`highlight` shortcode](shortcodes/highlight) replaces Hugo's default implementation and is fully compatible. So you don't need to change anything.

  In addition it offers some extensions. Currently only the `wrap` extension option is provided to control whether a code block should be wrapped or scrolled if to long to fit.

---

## 5.16.0 (2023-06-10) {#5160}

- {{% badge style="note" title=" " %}}Change{{% /badge %}} The theme now provides warnings for deprecated or now unsupported features. The warnings include hints how to fix them and an additional link to the documentation.

  `DEPRECATION` warnings mark features that still work but may be removed in the future.

  `UNSUPPORTED` warnings mark features that will not work anymore.

- {{% badge style="note" title=" " %}}Change{{% /badge %}} The 404 error page was revamped. Hopefully you will not see this very often.

- {{% badge style="info" icon="plus-circle" title=" " %}}New{{% /badge %}} The [`tabs` shortcode](shortcodes/tabs) and the [`tab` shortcode](shortcodes/tab) received some love and now align with their style, color, title and icon parameter to the other shortcodes.

  The visuals are now slightly different compared to previous versions. Most noteable, if you now display a single code block in a tab, its default styling will adapt to that of a code block but with a tab handle at the top.

  Additionally the `name` parameter was renamed to `title` but you don't need to change anything yet as the old name will be used as a fallback. Nevertheless you will get deprecation warnings while executing Hugo.

- {{% badge style="info" icon="plus-circle" title=" " %}}New{{% /badge %}} The theme now optionally supports [separate favicons](basics/branding#change-the-favicon) for light & dark mode.

---

## 5.15.0 (2023-05-29) {#5150}

- {{% badge style="note" title=" " %}}Change{{% /badge %}} Restored compatibility with Hugo {{% badge color="fuchsia" icon="fa-fw fab fa-hackerrank" title=" " %}}0.112.0{{% /badge %}} or higher. This does not change the minimum required Hugo version.

  The [`attachments` shortcode](shortcodes/attachments) has compatibility issues with newer Hugo versions. You must switch to leaf bundles or are locked to Hugo < `0.112.0` for now.

  It is [planned to refactor](https://github.com/McShelby/hugo-theme-relearn/issues/22) the `attchments` shortcode in the future. This will make it possible to use the shortcode in branch bundles again but not in simple pages anymore. This will most likely come with a breaking change.

- {{% badge style="note" title=" " %}}Change{{% /badge %}} The [`tabs` shortcode](shortcodes/tabs) has changed behavior if you haven't set the `groupid` parameter.

  Formerly all tab views without a `groupid` were treated as so they belong to the same group. Now, each tab view is treated as it was given a unique id.

- {{% badge style="info" icon="plus-circle" title=" " %}}New{{% /badge %}} The already known `tabs`has a new friend the [`tab` shortcode](shortcodes/tab) to make it easier to create a tab view in case you only need one single tab. Really handy if you want to flag your code examples with a language identifier.

  Additionally for such a use case, the whitespace between a tab outline and the code is removed if only a single code block is contained.

- {{% badge style="info" icon="plus-circle" title=" " %}}New{{% /badge %}} Besides the _tag_ [taxonomy](cont/taxonomy) the theme now also provides the _category_ taxonomy out of the box and shows them in the content footer of each page.

---

## 5.14.0 (2023-05-20) {#5140}

- {{% badge style="info" icon="plus-circle" title=" " %}}New{{% /badge %}} The taxonomy pages received some love in this release, making them better leverage available screen space and adding translation support for the taxonomy names.

  Hugo's default taxonmies `tags` and `categories` are already contained in the theme's i18n files. If you have self-defined taxonomies, you can add translations by adding them to your own i18n files. If you don't provide translations, the singualar and plural forms are taken as configured in your `hugo.toml`.

- {{% badge style="info" icon="plus-circle" title=" " %}}New{{% /badge %}} To give you more flexibility in customizing your article layout a new partial `content-header.html` is introduced.

  This came out of the requirement to customize the position of article tags, which by default are displayed above the title. A second requirement was to also show additional [taxonomies](https://gohugo.io/content-management/taxonomies/) not supported by the theme natively. While Hugo supports tags and categories by default, the theme only displays tags.

  So how to adjust the position of tags starting from the theme's default where tags are only shown above the title?

  1. Hide tags above title: Overwrite `content-header.html` with an empty file.
  2. Show tags between title and content: Overwrite `heading-post.html` and add `{{ partial "tags.html" . }}` to it.
  3. Show tags below content: Overwrite `content-footer.html` and add `{{ partial "tags.html" . }}` to it.

- {{% badge style="info" icon="plus-circle" title=" " %}}New{{% /badge %}} The new parameter `breadcrumbSeparator` is now available in your `hugo.toml` to change the - well - separator of the breadcrumb items. An appropriate default is in place if you do not configure anything.

---

## 5.13.0 (2023-05-17) {#5130}

- {{% badge style="note" title=" " %}}Change{{% /badge %}} The `swagger` shortcode was deprecated in favor for the  [`openapi` shortcode](shortcodes/openapi). You don't need to change anything yet as the old name will be used as a fallback. It is planned to remove the `swagger` shortcode in the next major release.

  Additionally, the implemantion of this shortcode was switched from RapiDoc to [SwaggerUI](https://github.com/swagger-api/swagger-ui).

---

## 5.12.0 (2023-05-04) {#5120}

- {{% badge style="note" title=" " %}}Change{{% /badge %}} In the effort to comply with WCAG standards, the implementation of the collapsible menu was changed (again). While Internet Explorer 11 has issues in displaying it, the functionality still works.

- {{% badge style="info" icon="plus-circle" title=" " %}}New{{% /badge %}} Support for the great [VSCode Front Matter extension](https://github.com/estruyf/vscode-front-matter) which provides on-premise CMS capabilties to Hugo.

  The theme provides Front Matter snippets for its shortcodes. Currently only English and German is supported. Put a reference into your `frontmatter.json` like this

  ````json {title="frontmatter.json"}
  {
    ...
    "frontMatter.extends": [
        "./vscode-frontmatter/snippets.en.json"
    ]
    ...
  }
  ````

- {{% badge style="info" icon="plus-circle" title=" " %}}New{{% /badge %}} Support for languages that are written right to left (like Arabic) is now complete and extended to the menu, the top navigation bar and print. You can experience this in the [pirate translation](/pir). This feature is not available in Internet Explorer 11.

- {{% badge style="info" icon="plus-circle" title=" " %}}New{{% /badge %}} The scrollbars are now colored according to their variant color scheme to better fit into the visuals.

---

## 5.11.0 (2023-02-07) {#5110}

- {{% badge style="note" title=" " %}}Change{{% /badge %}} The theme removed the popular [jQuery](https://jquery.com) library from its distribution.

  In case you made changes to the theme that are dependend on this library you can place a copy of jQuery into your `static/js` directory and load it from your own `layouts/partials/custom-header.html` like this:

  ````html {title="layouts/partials/custom-header.html"}
  <script src="{{"js/jquery.min.js"| relURL}}" defer></script>
  ````

- {{% badge style="note" title=" " %}}Change{{% /badge %}} [Mermaid](shortcodes/mermaid#parameter) diagrams can now be configured for pan and zoom on site-, page-level or individually for each graph.

  The default setting of `on`, in effect since 1.1.0, changed back to `off` as there was interference with scrolling on mobile and big pages.

- {{% badge style="note" title=" " %}}Change{{% /badge %}} The theme is now capable to visually [adapt to your OS's light/dark mode setting](basics/branding#adjust-to-os-settings).

  This is also the new default setting if you haven't configured `themeVariant` in your `hugo.toml`.

  Additionally you can configure the variants to be taken for light/dark mode with the new `themeVariantAuto` parameter.

  This is not supported for Internet Explorer 11, which still displays in the `relearn-light` variant.

- {{% badge style="note" title=" " %}}Change{{% /badge %}} The JavaScript code for handling image lightboxes (provided by [Featherlight](https://noelboss.github.io/featherlight)) was replaced by a CSS-only solution.

  This also changed the [lightbox effects](cont/markdown#lightbox) parameter from `featherlight=false` to `lightbox=false`. Nevertheless you don't need to change anything as the old name will be used as a fallback.

- {{% badge style="note" title=" " %}}Change{{% /badge %}} In the effort to comply with WCAG standards, the implementation of the [`expand` shortcode](shortcodes/expand) was changed. While Internet Explorer 11 has issues in displaying it, the functionality still works.

- {{% badge style="info" icon="plus-circle" title=" " %}}New{{% /badge %}} Translation into Czech. This language is not supported for search.

- {{% badge style="info" icon="plus-circle" title=" " %}}New{{% /badge %}} [GitHub releases](https://github.com/McShelby/hugo-theme-relearn/tags) are also now tagged for the main version (eg. `1.2.x`), major version (eg. `1.x`) and the latest (just `x`) release making it easier for you to pin the theme to a certain version.

---

## 5.10.0 (2023-01-25) {#5100}

- {{% badge style="info" icon="plus-circle" title=" " %}}New{{% /badge %}} The [`attachments`](shortcodes/attachments), [`badge`](shortcodes/badge), [`button`](shortcodes/button) and [`notice`](shortcodes/notice) shortcodes have a new parameter `color` to set arbitrary CSS color values.

  Additionally the `--ACCENT-color` brand color introduced in version 5.8.0 is now supported with these shortcodes.

---

## 5.9.0 (2022-12-23) {#590}

- {{% badge style="warning" title=" " %}}Breaking{{% /badge %}} With this version it is now possible to not only have sections on the first menu level but also pages.

  It was later discovered, that this causes pages only meant to be displayed in the `More` section of the menu and stored directly inside your `content` directory to now show up in the menu aswell.

  To [get rid](cont/menushortcuts#shortcuts-to-pages-inside-of-your-project) of this undesired behavior you have two choices:

  1. Make the page file a [headless branch bundle](https://gohugo.io/content-management/page-bundles/#headless-bundle) (contained in its own subdirectory and called `_index.md`) and add the following frontmatter configuration to the file (see exampleSite's `content/showcase/_index.en.md`). This causes its content to **not** be ontained in the sitemap.

      {{< multiconfig fm=true >}}
      title = "Showcase"
      [_build]
        render = "always"
        list = "never"
        publishResources = true
      {{< /multiconfig >}}

  2. Store the page file for below a parent headless branch bundle and add the following frontmatter to he **parent** (see exampleSite's `content/more/_index.en.md`). **Don't give this page a `title`** as this will cause it to be shown in the breadcrumbs - a thing you most likely don't want.

      {{< multiconfig fm=true >}}
      [_build]
        render = "never"
        list = "never"
        publishResources = false
      {{< /multiconfig >}}

      In this case, the file itself can be a branch bundle, leaf bundle or simple page (see exampleSite's `content/more/credits.en.md`). This causes its content to be contained in the sitemap.

      {{< multiconfig fm=true >}}
      title = "Credits"
      {{< /multiconfig >}}

- {{% badge style="note" title=" " %}}Change{{% /badge %}} The required folder name for the [`attachments` shortcode](shortcodes/attachments) was changed for leaf bundles.

  Previously, the attachments for leaf bundles in non-multilang setups were required to be in a `files` subdirectory. For page bundles and leaf bundles in multilang setups they were always required to be in a `_index.<LANGCODE>.files` or `index.<LANGCODE>.files` subdirectory accordingly.

  This added unnecessary complexity. So attachments for leaf bundles in non-multilang setups can now also reside in a `index.files` directory. Although the old `files` directory is now deprecated, if both directories are present, only the old `files` directory will be used for compatibility.

- {{% badge style="note" title=" " %}}Change{{% /badge %}} Absolute links prefixed with `http://` or `https://` are now opened in a separate browser tab.

  You can revert back to the old behavior by defining `externalLinkTarget="_self"` in the `params` section of your `hugo.toml`.

- {{% badge style="info" icon="plus-circle" title=" " %}}New{{% /badge %}} The theme now supports [Hugo's module system](https://gohugo.io/hugo-modules/).

---

## 5.8.0 (2022-12-08) {#580}

- {{% badge style="info" icon="plus-circle" title=" " %}}New{{% /badge %}} The new [`badge` shortcode](shortcodes/badge) is now available to add highly configurable markers to your content as you can see it on this page.

- {{% badge style="info" icon="plus-circle" title=" " %}}New{{% /badge %}} The new [`icon` shortcode](shortcodes/icon) simplyfies the usage of icons. This can even be combined with also new `badge` shortcode.

- {{% badge style="info" icon="plus-circle" title=" " %}}New{{% /badge %}} The theme now supports some of GFM (GitHub Flavored Markdown) syntax and Hugo Markdown extensions, namely [task lists](cont/markdown#tasks), [defintion lists](cont/markdown#definitions) and [footnotes](cont/markdown#footnotes).

- {{% badge style="info" icon="plus-circle" title=" " %}}New{{% /badge %}} A new color `--ACCENT-color` was introduced which is used for highlightning search results on the page. In case you simply don't care, you don't need to change anything in your variant stylesheet as the old `yellow` color is still used as default.

---

## 5.7.0 (2022-11-29) {#570}

- {{% badge style="note" title=" " %}}Change{{% /badge %}} The Korean language translation for this theme is now available with the language code `ko`. Formerly the country code `kr` was used instead.

- {{% badge style="info" icon="plus-circle" title=" " %}}New{{% /badge %}} The [`button` shortcode](shortcodes/button) can now also be used as a real button inside of HTML forms - although this is a pretty rare use case. The documentation was updated accordingly.

- {{% badge style="info" icon="plus-circle" title=" " %}}New{{% /badge %}} The search now supports the Korean language.

---

## 5.6.0 (2022-11-18) {#560}

- {{% badge style="info" icon="plus-circle" title=" " %}}New{{% /badge %}} This release introduces an additional dedicated search page. On this page, displayed search results have more space making it easier scanning thru large number of results.

  To activate this feature, you need to [configure it](basics/customization#activate-dedicated-search-page) in your `hugo.toml` as a new outputformat `searchpage` for the home page. If you don't configure it, no dedicated search page will be accessible and the theme works as before.

  You can access the search page by either clicking on the magnifier glass or pressing enter inside of the search box.

- {{% badge style="info" icon="plus-circle" title=" " %}}New{{% /badge %}} Keyboard handling for the TOC and search was improved.

  Pressing `CTRL+ALT+t` now will not only toggle the TOC overlay but also places the focus to the first heading on opening. Subsequently this makes it possible to easily select headings by using the `TAB` key.

  The search received its own brand new keyboard shortcut `CTRL+ALT+f`. This will focus the cursor inside of the the search box so you can immediately start your search by typing.

- {{% badge style="info" icon="plus-circle" title=" " %}}New{{% /badge %}} You are now able to turn off the generation of generator meta tags in your HTML head to hide the used versions of Hugo and this theme.

  To [configure this](basics/configuration) in your `hugo.toml` make sure to set Hugo's `disableHugoGeneratorInject=true` **and** also `[params] disableGeneratorVersion=true`, otherwise Hugo will generate a meta tag into your home page automagically.

- {{% badge style="info" icon="plus-circle" title=" " %}}New{{% /badge %}} Creation of your project gets a little bit faster with this release.

  This addresses increased build time with the 5.x releases. The theme now heavily caches partial results leading to improved performance. To further increase performance, unnecessary parts of the page are now skipped for creation of the print output (eg. menus, navigation bar, etc.).

---

## 5.5.0 (2022-11-06) {#550}

- {{% badge style="note" title=" " %}}Change{{% /badge %}} The way images are processed has changed. Now images are lazy loaded by default which speeds up page load on slow networks and/or big pages and also the print preview.

  For that the JavaScript code to handle the [lightbox and image effects](cont/markdown#image-effects) on the client side was removed in favour for static generation of those effects on the server.

  If you have used HTML directly in your Markdown files, this now has the downside that it doesn't respect the effect query parameter anymore. In this case you have to migrate all your HTML `img` URLs manually to the respective HTML attributes.

  | Old                                                    | New                                                             |
  |--------------------------------------------------------|-----------------------------------------------------------------|
  | `<img src="pic.png?width=20vw&classes=shadow,border">` | `<img src="pic.png" style="width:20vw;" class="shadow border">` |

---

## 5.4.0 (2022-11-01) {#540}

- {{% badge style="note" title=" " %}}Change{{% /badge %}} [With the proper settings](basics/customization#file-system) in your `hugo.toml` your page is now servable from the local file system using `file://` URLs.

  Please note that the searchbox will only work for this if you reconfigure your outputformat for the homepage in your `hugo.toml` from `json` to `search`. The now deprecated `json` outputformat still works as before, so there is no need to reconfigure your installation if it is only served from `http://` or `https://`.

- {{% badge style="note" title=" " %}}Change{{% /badge %}} The [`button` shortcode](shortcodes/button) has a new parameter `target` to set the destination frame/window for the URL to open. If not given, it defaults to a new window/tab for external URLs or is not set at all for internal URLs. Previously even internal URLs where opened in a new window/tab.

- {{% badge style="info" icon="plus-circle" title=" " %}}New{{% /badge %}} The [`math` shortcode](shortcodes/math) and [`mermaid` shortcode](shortcodes/mermaid) now also support the `align` parameter if codefence syntax is used.

- {{% badge style="info" icon="plus-circle" title=" " %}}New{{% /badge %}} Support for languages that are written right to left (like Arabic). This is only implemented for the content area but not the navigation sidebar. This feature is not available in Internet Explorer 11.

- {{% badge style="info" icon="plus-circle" title=" " %}}New{{% /badge %}} Translation into Finnish (Suomi).

---

## 5.3.0 (2022-10-07) {#530}

- {{% badge style="note" title=" " %}}Change{{% /badge %}} In the effort to comply with WCAG standards, the implementation of the collapsible menu was changed. The functionality of the new implementation does not work with old browsers (Internet Explorer 11).

- {{% badge style="info" icon="plus-circle" title=" " %}}New{{% /badge %}} [Image formatting](cont/markdown#css-classes) has two new classes to align images to the `left` or `right`. Additionally, the already existing `inline` option is now documented.

- {{% badge style="info" icon="plus-circle" title=" " %}}New{{% /badge %}} Printing for the [`swagger` shortcode](shortcodes/openapi) was optimized to expand sections that are usually closed in interactive mode. This requires [print support](basics/customization#activate-print-support) to be configured.

---

## 5.2.0 (2022-08-03) {#520}

- {{% badge style="note" title=" " %}}Change{{% /badge %}} If you've set `collapsibleMenu = true` in your `hugo.toml`, the menu will be expanded if a search term is found in a collapsed submenu. The menu will return to its initial collapse state once the search term does not match any submenus.

---

## 5.1.0 (2022-07-15) {#510}

- {{% badge color="fuchsia" icon="fa-fw fab fa-hackerrank" title=" " %}}0.95.0{{% /badge %}} This release requires a newer Hugo version.

- {{% badge style="note" title=" " %}}Change{{% /badge %}} Because the print preview URLs were non deterministic for normal pages in comparison to page bundles, this is now changed. Each print preview is now accessible by adding a `index.print.html` to the default URL.

  You can revert this behavior by overwriting the `print` output format setting in your `hugo.toml`to:

  {{< multiconfig file=hugo >}}
  [outputFormats]
    [outputFormats.print]
      name= "print"
      baseName = "index"
      path = "_print"
      isHTML = true
      mediaType = 'text/html'
      permalinkable = false
  {{< /multiconfig >}}

---

## 5.0.0 (2022-07-05) {#500}

- {{% badge style="warning" title=" " %}}Breaking{{% /badge %}} The theme changed how JavaScript and CSS dependencies are loaded to provide a better performance. In case you've added own JavaScript code that depends on the themes jQuery implementation, you have to put it into a separate `*.js` file (if not already) and add the `defer` keyword to the `script` element. Eg.

  ````html
  <script defer src="myscript.js"></script>
  ````

- {{% badge style="note" title=" " %}}Change{{% /badge %}} The way [archetypes](cont/archetypes) are used to generate output has changed. The new systems allows you, to redefine existing archetypes or even generate your own ones.

  Your existing markdown files will still work like before and therefore you don't need to change anything after the upgrade. Nevertheless, it is recommended to adapt your existing markdown files to the new way as follows:

  - for your home page, add the frontmatter parameter `archetype = "home"` and remove the leading heading

  - for all files containing the deprecated frontmatter parameter `chapter = true`, replace it with `archetype = "chapter"` and remove the leading headings

- {{% badge style="note" title=" " %}}Change{{% /badge %}} The frontmatter options `pre` / `post` were renamed to `menuPre` / `menuPost`. The old options will still be used if the new options aren't set. Therefore you don't need to change anything after the upgrade.

- {{% badge style="info" icon="plus-circle" title=" " %}}New{{% /badge %}} Adding new partials `heading-pre.html` / `heading-post.html` and according frontmatter options `headingPre` / `headingPost` to modify the way your page`s main heading gets styled.

- {{% badge style="info" icon="plus-circle" title=" " %}}New{{% /badge %}} The new shortcode `math` is available to add beautiful math and chemical formulae. See the [documentation](shortcodes/math) for available features. This feature will not work with Internet Explorer 11.

---

## 4.2.0 (2022-06-23) {#420}

- {{% badge style="warning" title=" " %}}Breaking{{% /badge %}} The second parameter for the [`include` shortcode](shortcodes/include) was switched in meaning and was renamed from `showfirstheading` to `hidefirstheading`. If you haven't used this parameter in your shortcode, the default behavior hasn't changed and you don't need to change anything.

  If you've used the second boolean parameter, you have to rename it and invert its value to achieve the same behavior.

- {{% badge style="note" title=" " %}}Change{{% /badge %}} Previously, if the [`tabs` shortcode](shortcodes/tabs) could not find a tab item because, the tabs ended up empty. Now the first tab is selected instead.

- {{% badge style="note" title=" " %}}Change{{% /badge %}} The `landingPageURL` was removed from `hugo.toml`. You can safely remove this as well from your configuration as it is not used anymore. The theme will detect the landing page URL automatically and will point to the project's homepage. If you want to support a different link, overwrite the `logo.html` partial.

- {{% badge style="info" icon="plus-circle" title=" " %}}New{{% /badge %}} All shortcodes can now be also called from your partials. Examples for this are added to the documentation of each shortcode.

---

## 4.1.0 (2022-06-12) {#410}

- {{% badge style="info" icon="plus-circle" title=" " %}}New{{% /badge %}} While fixing issues with the search functionality for non Latin languages, you can now [configure to have multiple languages on a single page](cont/i18n#search-with-mixed-language-support).

---

## 4.0.0 (2022-06-05) {#400}

- {{% badge style="warning" title=" " %}}Breaking{{% /badge %}} The `custom_css` config parameter was removed from the configuration. If used in an existing installation, it can be achieved by overriding the `custom-header.html` template in a much more generic manner.

- {{% badge style="warning" title=" " %}}Breaking{{% /badge %}} Because anchor hover color was not configurable without introducing more complexity to the variant stylesheets, we decided to remove `--MAIN-ANCHOR-color` instead. You don't need to change anything in your custom color stylesheet as the anchors now get their colors from `--MAIN-LINK-color` and `--MAIN-ANCHOR-HOVER-color` respectively.

- {{% badge style="info" icon="plus-circle" title=" " %}}New{{% /badge %}} All shortcodes now support named parameter. The positional parameter are still supported but will not be enhanced with new features, so you don't need to change anything in your installation.

  This applies to [`expand`](shortcodes/expand), [`include`](shortcodes/include), [`notice`](shortcodes/notice) and [`siteparam`](shortcodes/siteparam).

- {{% badge style="info" icon="plus-circle" title=" " %}}New{{% /badge %}} The [`button`](shortcodes/button) shortcode received some love and now has a parameter for the color style similar to other shortcodes.

- {{% badge style="info" icon="plus-circle" title=" " %}}New{{% /badge %}} New colors `--PRIMARY-color` and `--SECONDARY-color` were added to provide easier modification of your custom style. Shortcodes with a color style can now have `primary` or `secondary` as additional values.

  These two colors are the default for other, more specific color variables. You don't need to change anything in your existing custom color stylesheets as those variables get reasonable default values.

- {{% badge style="info" icon="plus-circle" title=" " %}}New{{% /badge %}} Translation into Polish. This language is not supported for search.

- {{% badge style="info" icon="plus-circle" title=" " %}}New{{% /badge %}} The documentation for all shortcodes were revised.

---

## 3.4.0 (2022-04-03) {#340}

- {{% badge style="warning" title=" " %}}Breaking{{% /badge %}} If you had previously overwritten the `custom-footer.html` partial to add visual elements below the content of your page, you have to move this content to the new partial `content-footer.html`. `custom-footer.html` was never meant to contain HTML other than additional styles and JavaScript.

- {{% badge style="info" icon="plus-circle" title=" " %}}New{{% /badge %}} If you prefer expandable/collapsible menu items, you can now set `collapsibleMenu=true` in your `hugo.toml`. This will add arrows to all menu items that contain sub menus. The menu will expand/collapse without navigation if you click on an arrow.

- {{% badge style="info" icon="plus-circle" title=" " %}}New{{% /badge %}} You can activate [print support](basics/customization#activate-print-support) in your `hugo.toml` to add the capability to print whole chapters or even the complete site.

- {{% badge style="info" icon="plus-circle" title=" " %}}New{{% /badge %}} Translation into Traditional Chinese.

---

## 3.3.0 (2022-03-28) {#330}

- {{% badge style="info" icon="plus-circle" title=" " %}}New{{% /badge %}} Introduction of new CSS variables to set the font. The theme distinguishes between `--MAIN-font` for all content text and `--CODE-font` for inline or block code. There are additional overrides for all headings. See the [theme variant generator](basics/generator) of the exampleSite for all available variables.

- {{% badge style="info" icon="plus-circle" title=" " %}}New{{% /badge %}} The new shortcode `swagger` is available to include a UI for REST OpenAPI specifications. See the [documentation](shortcodes/openapi) for available features. This feature will not work with Internet Explorer 11.

---

## 3.2.0 (2022-03-19) {#320}

- {{% badge color="fuchsia" icon="fa-fw fab fa-hackerrank" title=" " %}}0.93.0{{% /badge %}} This release requires a newer Hugo version.

- {{% badge style="note" title=" " %}}Change{{% /badge %}} In this release the Mermaid JavaScript library will only be loaded on demand if the page contains a Mermaid shortcode or is using Mermaid codefences. This changes the behavior of `disableMermaid` config option as follows: If a Mermaid shortcode or codefence is found, the option will be ignored and Mermaid will be loaded regardlessly.

  The option is still useful in case you are using scripting to set up your graph. In this case no shortcode or codefence is involved and the library is not loaded by default. In this case you can set `disableMermaid=false` in your frontmatter to force the library to be loaded. See the [theme variant generator](basics/generator) of the exampleSite for an example.

- {{% badge style="info" icon="plus-circle" title=" " %}}New{{% /badge %}} Additional color variant variable `--MERMAID-theme` to set the variant's Mermaid theme. This causes the Mermaid theme to switch with the color variant if it defers from the setting of the formerly selected color variant.

---

## 3.1.0 (2022-03-15) {#310}

- {{% badge style="info" icon="plus-circle" title=" " %}}New{{% /badge %}} [`attachment`](shortcodes/attachments) and [`notice`](shortcodes/notice) shortcodes have a new parameter to override the default icon. Allowed values are all [Font Awesome 5 Free](https://fontawesome.com/v5/search?m=free) icons.

---

## 3.0.0 (2022-02-22) {#300}

- {{% badge style="warning" title=" " %}}Breaking{{% /badge %}} We made changes to the menu footer. If you have your `menu-footer.html` [partial overridden](basics/customization#partials), you may have to review the styling (eg. margins/paddings) in your partial. For a reference take a look into the `menu-footer.html` partial that is coming with the exampleSite.

  This change was made to allow your own menu footer to be placed right after the so called prefooter that comes with the theme (containing the language switch and _Clear history_ functionality).

- {{% badge style="warning" title=" " %}}Breaking{{% /badge %}} We have changed the default colors from the original Learn theme (the purple menu header) to the Relearn defaults (the light green menu header) as used in the official documentation.

  This change will only affect your installation if you've not set the `themeVariant` parameter in your `hugo.toml`. [If you still want to use the Learn color variant](basics/branding#theme-variant), you have to explicitly set `themeVariant="learn"` in your `hugo.toml`.

  Note, that this will also affect your site if viewed with Internet Explorer 11 but in this case it can not be reconfigured as Internet Explorer does not support CSS variables.

- {{% badge style="note" title=" " %}}Change{{% /badge %}} Due to a bug, that we couldn't fix in a general manner for color variants, we decided to remove `--MENU-SEARCH-BOX-ICONS-color` and introduced `--MENU-SEARCH-color` instead. You don't need to change anything in your custom color stylesheet as the old name will be used as a fallback.

- {{% badge style="note" title=" " %}}Change{{% /badge %}} For consistency reasons, we renamed `--MENU-SEARCH-BOX-color` to `--MENU-SEARCH-BORDER-color`. You don't need to change anything in your custom color stylesheet as the old name will be used as a fallback.

- {{% badge style="info" icon="plus-circle" title=" " %}}New{{% /badge %}} With this release you are now capable to define your own _dark mode_ variants.

  To make this possible, we have introduced a lot more color variables you can use in [your color variants](basics/branding#theme-variant). Your old variants will still work and don't need to be changed as appropriate fallback values are used by the theme. Nevertheless, the new colors allow for much more customization.

  To see what's now possible, see the new variants `relearn-dark` and `neon` that are coming with this release.

- {{% badge style="info" icon="plus-circle" title=" " %}}New{{% /badge %}} To make the creation of new variants easier for you, we've added a new interactive [theme variant generator](basics/generator). This feature will not work with Internet Explorer 11.

- {{% badge style="info" icon="plus-circle" title=" " %}}New{{% /badge %}} You can now configure multiple color variants in your `hugo.toml`. In this case, the first variant is the default chosen on first view and a variant selector will be shown in the menu footer. See the [documentation](basics/branding#multiple-variants) for configuration.

  Note, that the new variant selector will not work with Internet Explorer 11 as it does not support CSS variables. Therefore, the variant selector will not be displayed with Internet Explorer 11.

---

## 2.9.0 (2021-11-19) {#290}

- {{% badge style="warning" title=" " %}}Breaking{{% /badge %}} This release removes the themes implementation of `ref`/`relref` in favor for Hugos standard implementation. This is because of inconsistencies with the themes implementation. In advantage, your project becomes standard compliant and exchanging this theme in your project to some other theme will be effortless.

  In a standard compliant form you must not link to the `*.md` file but to its logical name. You'll see, referencing other pages becomes much easier. All three types result in the same reference:

  | Type          | Non-Standard                     | Standard               |
  |---------------|----------------------------------|------------------------|
  | Branch bundle | `basics/configuration/_index.md` | `basics/configuration` |
  | Leaf bundle   | `basics/configuration/index.md`  | `basics/configuration` |
  | Page          | `basics/configuration.md`        | `basics/configuration` |

  If you've linked from a page of one language to a page of another language, conversion is a bit more difficult but [Hugo got you covered](https://gohugo.io/content-management/cross-references/#link-to-another-language-version) as well.

  Also, the old themes implementation allowed refs to non-existing content. This will cause Hugos implementation to show the error below and abort the generation. If your project relies on this old behavior, you can [reconfigure the error handling](https://gohugo.io/content-management/cross-references/#link-to-another-language-version) of Hugos implementation.

  In the best case your usage of the old implementation is already standard compliant and you don't need to change anything. You'll notice this very easily once you've started `hugo server` after an upgrade and no errors are written to the console.

  You may see errors on the console after the update in the form:

  ````shell
  ERROR 2021/11/19 22:29:10 [en] REF_NOT_FOUND: Ref "basics/configuration/_index.md": "hugo-theme-relearn\exampleSite\content\_index.en.md:19:22": page not found
  ````

  In this case, you must apply one of two options:

  1. Start up a text editor with regular expression support for search and replace. Search for `(ref\s+"[^"]*?)(?:/_index|/index)?(?:\.md)?(#[^"]*?)?"` and replace it by `$1$2"` in all `*.md` files. **This is the recommended choice**.

  2. Copy the old implementation files `theme/hugo-theme-relearn/layouts/shortcode/ref.html` and `theme/hugo-theme-relearn/layouts/shortcode/relref.html` to your own projects `layouts/shortcode/ref.html` and `layouts/shortcode/relref.html` respectively. **This is not recommended** as your project will still rely on non-standard behavior afterwards.

---

## 2.8.0 (2021-11-03) {#280}

- {{% badge style="note" title=" " %}}Change{{% /badge %}} Although never officially documented, this release removes the font `Novacento`/`Novecento`. If you use it in an overwritten CSS please replace it with `Work Sans`. This change was necessary as Novacento did not provide all Latin special characters and lead to mixed styled character text eg. for Czech.

- {{% badge style="info" icon="plus-circle" title=" " %}}New{{% /badge %}} The theme now supports favicons served from `static/images/` named as `favicon` or `logo` in SVG, PNG or ICO format [out of the box](basics/branding#change-the-favicon). An overridden partial `layouts/partials/favicon.html` may not be necessary anymore in most cases.

- {{% badge style="info" icon="plus-circle" title=" " %}}New{{% /badge %}} You can hide the table of contents menu for the whole site by setting the `disableToc` option in your `hugo.toml`. For an example see [the example configuration](basics/configuration).

---

## 2.7.0 (2021-10-24) {#270}

- {{% badge style="info" icon="plus-circle" title=" " %}}New{{% /badge %}} Optional second parameter for [`notice`](shortcodes/notice) shortcode to set title in box header.

---

## 2.6.0 (2021-10-21) {#260}

- {{% badge style="info" icon="plus-circle" title=" " %}}New{{% /badge %}} Your site can now be served from a subfolder if you set `baseURL` in your `hugo.toml`. See the [documentation](basics/customization#public-webserver-from-subdirectory) for a detailed example.

---

## 2.5.0 (2021-10-08) {#250}

- {{% badge style="note" title=" " %}}Change{{% /badge %}} New colors `--CODE-BLOCK-color` and `--CODE-BLOCK-BG-color` were added to provide a fallback for Hugos syntax highlighting in case no language was given or the language is unsupported. Ideally the colors are set to the same values as the ones from your chosen chroma style.

---

## 2.4.0 (2021-10-07) {#240}

- {{% badge style="note" title=" " %}}Change{{% /badge %}} Creation of customized stylesheets was simplified down to only contain the CSS variables. Everything else can and should be deleted from your custom stylesheet to assure everything works fine. For the predefined stylesheet variants, this change is already included.

- {{% badge style="info" icon="plus-circle" title=" " %}}New{{% /badge %}} Hidden pages are displayed by default in their according tags page. You can now turn off this behavior by setting `disableTagHiddenPages=true` in your `hugo.toml`.

- {{% badge style="info" icon="plus-circle" title=" " %}}New{{% /badge %}} You can define the expansion state of your menus for the whole site by setting the `alwaysopen` option in your `hugo.toml`. Please see further [documentation](cont/frontmatter#override-expand-state-rules-for-menu-entries) for possible values and default behavior.

- {{% badge style="info" icon="plus-circle" title=" " %}}New{{% /badge %}} New frontmatter `ordersectionsby` option to change immediate children sorting in menu and `children` shortcode. Possible values are `title` or `weight`.

- {{% badge style="info" icon="plus-circle" title=" " %}}New{{% /badge %}} Alternate content of a page is now advertised in the HTML meta tags. See [Hugo documentation](https://gohugo.io/templates/rss/#reference-your-rss-feed-in-head).

---

## 2.3.0 (2021-09-13) {#230}

- {{% badge color="fuchsia" icon="fa-fw fab fa-hackerrank" title=" " %}}0.81.0{{% /badge %}} This release requires a newer Hugo version.

- {{% badge style="info" icon="plus-circle" title=" " %}}New{{% /badge %}} Showcase multilanguage features by providing a documentation translation "fer us pirrrates". There will be no other translations besides the original English one and the Pirates one due to maintenance constraints.

---

## 2.2.0 (2021-09-09) {#220}

- {{% badge style="info" icon="plus-circle" title=" " %}}New{{% /badge %}} Hidden pages are displayed by default in the sitemap generated by Hugo and are therefore visible for search engine indexing. You can now turn off this behavior by setting `disableSeoHiddenPages=true` in your `hugo.toml`.

---

## 2.1.0 (2021-09-07) {#210}

- {{% badge color="fuchsia" icon="fa-fw fab fa-hackerrank" title=" " %}}0.69.0{{% /badge %}} This release requires a newer Hugo version.

- {{% badge style="note" title=" " %}}Change{{% /badge %}} In case the site's structure contains additional *.md files not part of the site (eg files that are meant to be included by site pages - see `CHANGELOG.md` in the exampleSite), they will now be ignored by the search.

- {{% badge style="info" icon="plus-circle" title=" " %}}New{{% /badge %}} Hidden pages are indexed for the site search by default. You can now turn off this behavior by setting `disableSearchHiddenPages=true` in your `hugo.toml`.

- {{% badge style="info" icon="plus-circle" title=" " %}}New{{% /badge %}} If a search term is found in an `expand` shortcode, the expand will be opened.

- {{% badge style="info" icon="plus-circle" title=" " %}}New{{% /badge %}} The menu will scroll the active item into view on load.

---

## 2.0.0 (2021-08-28) {#200}

- {{% badge style="note" title=" " %}}Change{{% /badge %}} Syntax highlighting was switched to the [built in Hugo mechanism](https://gohugo.io/content-management/syntax-highlighting/). You may need to configure a new stylesheet or decide to roll you own as described on in the Hugo documentation

- {{% badge style="note" title=" " %}}Change{{% /badge %}} In the predefined stylesheets there was a typo and `--MENU-HOME-LINK-HOVERED-color` must be changed to `--MENU-HOME-LINK-HOVER-color`. You don't need to change anything in your custom color stylesheet as the old name will be used as a fallback.

- {{% badge style="note" title=" " %}}Change{{% /badge %}} `--MENU-HOME-LINK-color` and `--MENU-HOME-LINK-HOVER-color` were missing in the documentation. You should add them to your custom stylesheets if you want to override the defaults.

- {{% badge style="note" title=" " %}}Change{{% /badge %}} Arrow navigation and `children` shortcode were ignoring setting for `ordersectionsby`. This is now changed and may result in different sorting order of your sub pages.

- {{% badge style="note" title=" " %}}Change{{% /badge %}} If hidden pages are accessed directly by typing their URL, they will be exposed in the menu.

- {{% badge style="note" title=" " %}}Change{{% /badge %}} A page without a `title` will be treated as `hidden=true`.

- {{% badge style="info" icon="plus-circle" title=" " %}}New{{% /badge %}} You can define the expansion state of your menus in the frontmatter. Please see further [documentation](cont/frontmatter#override-expand-state-rules-for-menu-entries) for possible values and default behavior.

- {{% badge style="info" icon="plus-circle" title=" " %}}New{{% /badge %}} New partials for defining pre/post content for menu items and the content. See [documentation](basics/customization#partials) for further reading.

- {{% badge style="info" icon="plus-circle" title=" " %}}New{{% /badge %}} Shortcode [`children`](shortcodes/children) with new parameter `containerstyle`.

- {{% badge style="info" icon="plus-circle" title=" " %}}New{{% /badge %}} New shortcode [`include`](shortcodes/include) to include arbitrary file content into a page.

---

## 1.2.0 (2021-07-26) {#120}

- {{% badge style="info" icon="plus-circle" title=" " %}}New{{% /badge %}} Shortcode [`expand`](shortcodes/expand) with new parameter to open on page load.

---

## 1.1.0 (2021-07-02) {#110}

- {{% badge style="warning" title=" " %}}Breaking{{% /badge %}} [Mermaid](shortcodes/mermaid) diagrams can now be panned and zoomed. This isn't configurable yet.

- {{% badge style="info" icon="plus-circle" title=" " %}}New{{% /badge %}} [`Mermaid`](shortcodes/mermaid#configuration) config options can be set in `hugo.toml`.

---

## 1.0.0 (2021-07-01) {#100}

- {{% badge color="fuchsia" icon="fa-fw fab fa-hackerrank" title=" " %}}0.65.0{{% /badge %}} The requirement for the Hugo version of this theme is the same as for the Learn theme version 2.5.0 on 2021-07-01.

- {{% badge style="info" icon="plus-circle" title=" " %}}New{{% /badge %}} Initial fork of the [Learn theme](https://github.com/matcornic/hugo-theme-learn) based on Learn 2.5.0 on 2021-07-01. This introduces no new features besides a global rename to `Relearn` and a new logo. For the reasons behind forking the Learn theme, see [this comment](https://github.com/matcornic/hugo-theme-learn/issues/442#issuecomment-907863495) in the Learn issues.

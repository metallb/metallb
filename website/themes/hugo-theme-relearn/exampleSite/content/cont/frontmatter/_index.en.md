+++
title = "Frontmatter"
weight = 2
+++

Each Hugo page **has to define** a [frontmatter](https://gohugo.io/content/front-matter/).

## All Frontmatter Options

The values reflect example options. The defaults can be taken from the [annotated example](#annotated-frontmatter-options) below.

{{< multiconfig fm=true >}}
{{% include "cont/frontmatter/frontmatter.toml" %}}
{{< /multiconfig >}}

## Annotated Frontmatter Options

````toml {title="toml"}
+++
{{% include "cont/frontmatter/frontmatter.toml" %}}+++
````

## Some Detailed Examples

### Add Icon to a Menu Entry

In the page frontmatter, add a `menuPre` param to insert any HTML code before the menu label. The example below uses the GitHub icon.

{{< multiconfig fm=true >}}
title = "GitHub repo"
menuPre = "<i class='fab fa-github'></i> "
{{< /multiconfig >}}

![Title with icon](frontmatter-icon.png?width=18.75rem)

### Ordering Sibling Menu/Page Entries

Hugo provides a [flexible way](https://gohugo.io/content/ordering/) to handle order for your pages.

The simplest way is to set `weight` parameter to a number.

{{< multiconfig fm=true >}}
title = "My page"
weight = 5
{{< /multiconfig >}}

### Using a Custom Title for Menu Entries

By default, the Relearn theme will use a page's `title` attribute for the menu item.

But a page's title has to be descriptive on its own while the menu is a hierarchy. Hugo adds the `linkTitle` parameter for that purpose:

For example (for a page named `content/install/linux.md`):

{{< multiconfig fm=true >}}
title = "Install on Linux"
linkTitle = "Linux"
{{< /multiconfig >}}

### Override Expand State Rules for Menu Entries

You can change how the theme expands menu entries on the side of the content with the `alwaysopen` setting on a per page basis. If `alwaysopen=false` for any given entry, its children will not be shown in the menu as long as it is not necessary for the sake of navigation.

The theme generates the menu based on the following rules:

- all parent entries of the active page including their siblings are shown regardless of any settings
- immediate children entries of the active page are shown regardless of any settings
- if not overridden, all other first level entries behave like they would have been given `alwaysopen=false`
- if not overridden, all other entries of levels besides the first behave like they would have been given `alwaysopen=true`
- all visible entries show their immediate children entries if `alwaysopen=true`; this proceeds recursively
- all remaining entries are not shown

You can see this feature in action on the example page for [children shortcode](shortcodes/children) and its children pages.

## Disable Section Pages

You may want to structure your pages in a hierachical way but don't want to generate pages for those sections? The theme got you covered.

To stay with the initial example: Suppose you want `level-one` appear in the sidebar but don't want to generate a page for it. So the entry in the sidebar should not be clickable but should show an expander.

For this, open `content/level-one/_index.md` and add the following frontmatter

{{< multiconfig fm=true >}}
collapsibleMenu = true
[_build]
  render = "never"
{{< /multiconfig >}}

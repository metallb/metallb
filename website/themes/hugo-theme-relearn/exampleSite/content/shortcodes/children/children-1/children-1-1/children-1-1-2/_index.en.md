+++
description = "This is a demo child page"
tags = ["children", "headless", "non-hidden"]
title = "page 1-1-2 (headless)"
[_build]
  render = "never"
+++

This is a headless child page.

While the heading is displayed in the theme for hierarchical views like the menu, the `children` shortcode, the chapter print feature and the breadcrumbs, its content will never be displayed and will not be accessible by search. Also its terms will not appear on the taxonomy pages.

## Subpages of this page

{{% children showhidden="true" %}}

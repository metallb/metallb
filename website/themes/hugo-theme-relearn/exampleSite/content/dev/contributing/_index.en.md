+++
description = "What to know if you want to contribute"
title = "Contributing"
+++

## Code Quality

A new release can happen at any time from the `main` branch of the [GitHub project](https://github.com/McShelby/hugo-theme-relearn) without further accknowledgment. This makes it necessary that, every pushed set of changesets into the `main` branch **must** be self-contained and correct, resulting in a releasable version.

Stay simple for the user by focusing on the mantra "convention over configuration".

At installation the site should work reasonable without (m)any configuration.

Stay close to the Hugo way.

Don't use npm or any preprocessing, our contributors may not be front-end developers.

Document new features in the exampleSite. This also contains entries to the [What's new](basics/migration) page.

Don't break existing features if you don't have to.

Remove reported issue from the browser's console.

Check for unnecessary whitespace and correct indention of your resulting HTML.

Be compatible to IE11, at least for main functionality, this means:

- test in IE11
- check caniuse.com
- don't use JavaScript arrow functions
- don't use JavaScript template literals
- don't use other fancy JavaScript ES5/6 stuff

## Conventional Commits

Write commit messages in the [conventional commit](https://www.conventionalcommits.org/en/v1.0.0/) format.

Following is an impomplete list of some of the used conventional commit types. Be creative.

| Common     | Feature    | Structure       | Shortcodes  |
|------------|------------|-----------------|-------------|
| build      | a11y       | favicon         | attachments |
| browser    | archetypes | search          | badge       |
| chore      | alias      | menu            | button      |
| docs       | generator  | history         | children    |
| shortcodes | i18n       | scrollbar       | expand      |
| theme      | mobile     | nav             | icon        |
|            | print      | toc             | include     |
|            | rss        | clipboard       | math        |
|            | variant    | syntaxhighlight | mermaid     |
|            |            | boxes           | notice      |
|            |            |                 | openapi     |
|            |            |                 | piratify    |
|            |            |                 | siteparam   |
|            |            |                 | tabs        |

+++
title = "Archetypes"
weight = 3
+++

Using the command: `hugo new [relative new content path]`, you can start a content file with the date and title automatically set. While this is a welcome feature, active writers need more: [archetypes](https://gohugo.io/content/archetypes/). These are preconfigured skeleton pages with default frontmatter.

The Relearn theme defines some few archetypes of pages but you are free to define new ones to your liking. All can be used at any level of the documentation, the only difference being the layout of the content.

## Predefined Archetypes

### Home {#archetypes-home}

A **Home** page is the starting page of your project. It's best to have only one page of this kind in your project.

![Home page](pages-home.png?width=60pc)

To create a home page, run the following command

````shell
hugo new --kind home _index.md
````

This leads to a file with the following content

````toml {title="_index.md"}
+++
archetype = "home"
title = "{{ replace .Name "-" " " | title }}"
+++

Lorem Ipsum.
````

### Chapter {#archetypes-chapter}

A **Chapter** displays a page meant to be used as introduction for a set of child pages. Commonly, it contains a simple title and a catch line to define content that can be found below it.

![Chapter page](pages-chapter.png?width=60pc)

To create a chapter page, run the following command

````shell
hugo new --kind chapter <name>/_index.md
````

This leads to a file with the following content

````toml {title="_index.md"}
+++
archetype = "chapter"
title = "{{ replace .Name "-" " " | title }}"
weight = 1
+++

Lorem Ipsum.
````

The `weight` number will be used to generate the subtitle of the chapter page, set the number to a consecutive value starting at 1 for each new chapter level.

### Default {#archetypes-default}

A **Default** page is any other content page. If you set an unknown archetype in your frontmatter, this archetype will be used to generate the page.

![Default page](pages-default.png?width=60pc)

To create a default page, run either one of the following commands

````shell
hugo new <chapter>/<name>/_index.md
````

or

````shell
hugo new <chapter>/<name>.md
````

This leads to a file with the following content

````toml {title="*.md"}
+++
title = "{{ replace .Name "-" " " | title }}"
+++

Lorem Ipsum.
````

## Self defined Archetypes

If you are in need of further archetypes you can define your own or even redefine existing ones.

### Template

Define a template file in your project at `archetypes/<kind>.md` and make sure it has at least the frontmatter parameter for that archetype like

````toml {title="&lt;kind&gt;.md"}
+++
archetype = "<kind>"
+++
````

Afterwards you can generate new content files of that kind with the following command

````shell
hugo new --kind <kind> <name>/_index.md
````

### Partials

To define how your archetypes are rendered, define corresponding partial files in your projects directory `layouts/partials/archetypes/<kind>`.

If you use an unknown archetype in your frontmatter, the `default` archetype will be used to generate the page.

Related to each archetype, several _hook_ partial files in the form of `<hook>.html` can be given inside each archetype directory. If a partial for a specific hook is missing, no output is generated for this hook.

The following hooks are used:

| Name                 | Notes       |
|----------------------|-------------|
| styleclass           | Defines a set of CSS classes to be added to the HTML's `<main>` element. You can use these classes to define own CSS rules in your `custom-header.html` |
| article              | Defines the HTML how to render your content |

Take a look at the existing archetypes of this theme to get an idea how to utilize it.

#### Output formats

Each hook file can be overridden of a specific [output format](https://gohugo.io/templates/output-formats/). Eg. if you define a new output format `PLAINTEXT` in your `hugo.toml`, you can add a file `layouts/partials/archetypes/default.plaintext.html` to change the way how normal content is written for that output format.

# Changelog

## 5.27.0 (2024-04-07)

### Enhancements

- [**feature**] theme: simplify title generation [#825](https://github.com/McShelby/hugo-theme-relearn/issues/825)
- [**feature**] theme: adjust to Hugo's build-in code [#824](https://github.com/McShelby/hugo-theme-relearn/issues/824)
- [**feature**] link: warn if fragment is not found [#823](https://github.com/McShelby/hugo-theme-relearn/issues/823)
- [**feature**] theme: add styling for selected HTML elements [#822](https://github.com/McShelby/hugo-theme-relearn/issues/822)
- [**feature**] a11y: improve search box [#821](https://github.com/McShelby/hugo-theme-relearn/issues/821)
- [**feature**][**change**] dependencies: make loader more versatile [#820](https://github.com/McShelby/hugo-theme-relearn/issues/820)
- [**feature**] nav: scroll to prev/next heading using keyboard shortcut [#819](https://github.com/McShelby/hugo-theme-relearn/issues/819)
- [**feature**] breadcrumb: use .LinkTitle instead of .Title if available [#816](https://github.com/McShelby/hugo-theme-relearn/issues/816)

### Fixes

- [**bug**] scrollbar: scroll bar in side menu gets stuck in dragged state on mobile [#808](https://github.com/McShelby/hugo-theme-relearn/issues/808)

---

## 5.26.2 (2024-03-18)

### Enhancements

- [**feature**] icons: use fixed width to ease layout [#812](https://github.com/McShelby/hugo-theme-relearn/issues/812)

### Fixes

- [**bug**] search: broken since version 5.26.1  [#813](https://github.com/McShelby/hugo-theme-relearn/issues/813)
- [**bug**] search: fix result links for pages in root [#810](https://github.com/McShelby/hugo-theme-relearn/issues/810)

---

## 5.26.1 (2024-03-17)

### Fixes

- [**bug**] mermaid: show reset button after pan [#807](https://github.com/McShelby/hugo-theme-relearn/issues/807)
- [**bug**] openapi: make it run for `file://` protocol [#806](https://github.com/McShelby/hugo-theme-relearn/issues/806)
- [**bug**] theme: fix relative path detection if `relativeURLs=false` [#804](https://github.com/McShelby/hugo-theme-relearn/issues/804)

---

## 5.26.0 (2024-03-16)

### Enhancements

- [**feature**] image: add lazy loading image effect option [#803](https://github.com/McShelby/hugo-theme-relearn/issues/803)
- [**feature**] render-hook: support Markdown attributes [#795](https://github.com/McShelby/hugo-theme-relearn/issues/795)
- [**feature**] theme: support full page width [#752](https://github.com/McShelby/hugo-theme-relearn/issues/752)

### Fixes

- [**bug**] clipboard: fix broken style if block code is in table [#790](https://github.com/McShelby/hugo-theme-relearn/issues/790)
- [**bug**] nav: browser back navigation does not jump to the correct position [#509](https://github.com/McShelby/hugo-theme-relearn/issues/509)

### Maintenance

- [**task**] build: update all available actions to nodejs 20 [#802](https://github.com/McShelby/hugo-theme-relearn/issues/802)
- [**task**] openapi: update swagger-ui to 5.11.10 [#798](https://github.com/McShelby/hugo-theme-relearn/issues/798)
- [**task**] mermaid: update to 10.9.0 [#797](https://github.com/McShelby/hugo-theme-relearn/issues/797)

---

## 5.25.0 (2024-02-29)

### Enhancements

- [**feature**][**change**] theme: print out GitInfo in page footer if configured [#786](https://github.com/McShelby/hugo-theme-relearn/issues/786)
- [**feature**][**change**] resources: new shortcode to deprecate attachments shortcode [#22](https://github.com/McShelby/hugo-theme-relearn/issues/22)

### Fixes

- [**bug**] swagger: fix compat warning [#787](https://github.com/McShelby/hugo-theme-relearn/issues/787)

---

## 5.24.3 (2024-02-28)

### Fixes

- [**bug**] theme: avoid crash on 404 if author settings want to warn [#785](https://github.com/McShelby/hugo-theme-relearn/issues/785)

---

## 5.24.2 (2024-02-24)

### Enhancements

- [**feature**] image: adjust to Hugo 0.123 [#777](https://github.com/McShelby/hugo-theme-relearn/issues/777)

### Fixes

- [**bug**] link: resolve fragments [#775](https://github.com/McShelby/hugo-theme-relearn/issues/775)

---

## 5.24.1 (2024-02-18)

### Enhancements

- [**feature**] link: make resolution reporting configurable [#774](https://github.com/McShelby/hugo-theme-relearn/issues/774)

---

## 5.24.0 (2024-02-17)

### Enhancements

- [**feature**] theme: compatibility with Hugo 0.123 [#771](https://github.com/McShelby/hugo-theme-relearn/issues/771)
- [**feature**] topbar: support editURL in frontmatter [#764](https://github.com/McShelby/hugo-theme-relearn/issues/764)
- [**feature**] menu: use --MENU-WIDTH-S to adjust mobile flyout [#761](https://github.com/McShelby/hugo-theme-relearn/issues/761)
- [**feature**] figure: support built-in shortcode [#746](https://github.com/McShelby/hugo-theme-relearn/issues/746)
- [**feature**] theme: make heading a template [#744](https://github.com/McShelby/hugo-theme-relearn/issues/744)
- [**feature**] taxonomy: make arrow nav browse thru terms [#742](https://github.com/McShelby/hugo-theme-relearn/issues/742)
- [**feature**] theme: switch from config.toml to hugo.toml [#741](https://github.com/McShelby/hugo-theme-relearn/issues/741)
- [**feature**] button: make non-interactive if used as mock [#740](https://github.com/McShelby/hugo-theme-relearn/issues/740)
- [**feature**][**change**] topbar: allow text for button [#739](https://github.com/McShelby/hugo-theme-relearn/issues/739)
- [**feature**] theme: run hugo demo site without warning [#736](https://github.com/McShelby/hugo-theme-relearn/issues/736)
- [**feature**] menu: make swipe handler passive [#735](https://github.com/McShelby/hugo-theme-relearn/issues/735)
- [**feature**] i18n: support standard Hugo options [#733](https://github.com/McShelby/hugo-theme-relearn/issues/733)
- [**feature**] a11y: show tab focus on images [#730](https://github.com/McShelby/hugo-theme-relearn/issues/730)
- [**feature**] a11y: improve discovering links on keyboard navigation [#726](https://github.com/McShelby/hugo-theme-relearn/issues/726)
- [**feature**][**change**] variant: increase contrast for light themes [#722](https://github.com/McShelby/hugo-theme-relearn/issues/722)
- [**feature**] theme: break build if minimum Hugo version is not matched [#719](https://github.com/McShelby/hugo-theme-relearn/issues/719)
- [**feature**] taxonomy: humanize term on missing term title [#713](https://github.com/McShelby/hugo-theme-relearn/issues/713)

### Fixes

- [**bug**] taxonomy: display translated title [#772](https://github.com/McShelby/hugo-theme-relearn/issues/772)
- [**bug**] highlight: fix codefence syntax in Hugo >= 0.121.0 [#749](https://github.com/McShelby/hugo-theme-relearn/issues/749)
- [**bug**] link: fix links to pages containing dots in their name [#748](https://github.com/McShelby/hugo-theme-relearn/issues/748)
- [**bug**] image: get resource images if link is prefixed with `./` [#747](https://github.com/McShelby/hugo-theme-relearn/issues/747)
- [**bug**] theme: switch dependency colors on OS color scheme change [#745](https://github.com/McShelby/hugo-theme-relearn/issues/745)
- [**bug**] clipboard: fix O(n²) buttons [#738](https://github.com/McShelby/hugo-theme-relearn/issues/738)
- [**bug**] button: fix whitespacing in FF [#737](https://github.com/McShelby/hugo-theme-relearn/issues/737)
- [**bug**] i18n: fix warning messages for zh-CN [#732](https://github.com/McShelby/hugo-theme-relearn/issues/732)
- [**bug**] mermaid: fix zoom button [#725](https://github.com/McShelby/hugo-theme-relearn/issues/725)
- [**bug**] theme: fix JS errors on `hugo --minifiy` [#724](https://github.com/McShelby/hugo-theme-relearn/issues/724)
- [**bug**] include: fix whitespacing in codefences [#723](https://github.com/McShelby/hugo-theme-relearn/issues/723)

---

## 5.23.2 (2023-11-03)

### Enhancements

- [**feature**] taxonomy: improve taxonomy page [#712](https://github.com/McShelby/hugo-theme-relearn/issues/712)
- [**feature**] taxonomy: warn on missing term title [#709](https://github.com/McShelby/hugo-theme-relearn/issues/709)

### Fixes

- [**bug**] taxonomy: fix sorting of terms on content pages [#710](https://github.com/McShelby/hugo-theme-relearn/issues/710)

---

## 5.23.1 (2023-10-30)

### Enhancements

- [**feature**] taxonomy: improve term page [#705](https://github.com/McShelby/hugo-theme-relearn/issues/705)

### Fixes

- [**bug**] variant: fix typo in chroma-learn.css [#708](https://github.com/McShelby/hugo-theme-relearn/issues/708)
- [**bug**] links: ignore local markdown links linking to files with extension [#707](https://github.com/McShelby/hugo-theme-relearn/issues/707)

---

## 5.23.0 (2023-10-29)

### Enhancements

- [**feature**] taxonomy: allow for content on term pages [#701](https://github.com/McShelby/hugo-theme-relearn/issues/701)
- [**feature**] theme: write full file path on warnings [#699](https://github.com/McShelby/hugo-theme-relearn/issues/699)
- [**feature**] theme: show anchor link and copy to clipboard button on mobile [#697](https://github.com/McShelby/hugo-theme-relearn/issues/697)
- [**feature**][**change**] config: adjust to changes in Hugo 0.120 [#693](https://github.com/McShelby/hugo-theme-relearn/issues/693)
- [**feature**] variants: add more contrast to neon [#692](https://github.com/McShelby/hugo-theme-relearn/issues/692)
- [**feature**] mermaid: only show zoom reset button if zoomed [#691](https://github.com/McShelby/hugo-theme-relearn/issues/691)
- [**feature**] menu: add additional sort options [#684](https://github.com/McShelby/hugo-theme-relearn/issues/684)
- [**feature**] theme: add social media meta information [#683](https://github.com/McShelby/hugo-theme-relearn/issues/683)
- [**feature**] theme: simplify additional JS dependencies [#682](https://github.com/McShelby/hugo-theme-relearn/issues/682)
- [**feature**] links: warn if ref/relref is used falsly [#681](https://github.com/McShelby/hugo-theme-relearn/issues/681)
- [**feature**] menu: make width configurable [#677](https://github.com/McShelby/hugo-theme-relearn/issues/677)
- [**feature**] tabs: use color for link of inactive tabs  [#675](https://github.com/McShelby/hugo-theme-relearn/issues/675)
- [**feature**] taxonomy: modularize term list generation [#671](https://github.com/McShelby/hugo-theme-relearn/issues/671)
- [**feature**] theme: remove warnings with `hugo --printI18nWarnings` [#670](https://github.com/McShelby/hugo-theme-relearn/issues/670)
- [**feature**] theme: implement portable linking [#377](https://github.com/McShelby/hugo-theme-relearn/issues/377)

### Fixes

- [**bug**] links: extra space before link text [#700](https://github.com/McShelby/hugo-theme-relearn/issues/700)
- [**bug**] mermaid: reset zoom correctly [#690](https://github.com/McShelby/hugo-theme-relearn/issues/690)
- [**bug**] theme: fix mobile layout for width=48rem [#676](https://github.com/McShelby/hugo-theme-relearn/issues/676)
- [**bug**] frontmatter: resemble documented shortcode style [#672](https://github.com/McShelby/hugo-theme-relearn/issues/672)
- [**bug**] taxonomy: display terms in pages if `removePathAccents=true` [#669](https://github.com/McShelby/hugo-theme-relearn/issues/669)

### Maintenance

- [**task**] mermaid: update mermaid to 10.6.0 [#703](https://github.com/McShelby/hugo-theme-relearn/issues/703)
- [**task**] openapi: update swagger-ui to 5.9.1 [#702](https://github.com/McShelby/hugo-theme-relearn/issues/702)

---

## 5.22.1 (2023-10-02)

### Enhancements

- [**feature**] i18n: add Swahili translation [#666](https://github.com/McShelby/hugo-theme-relearn/issues/666)
- [**feature**] math: hide unrendered math [#663](https://github.com/McShelby/hugo-theme-relearn/issues/663)
- [**feature**] tabs: improve a11y by removing duplicate hidden title [#662](https://github.com/McShelby/hugo-theme-relearn/issues/662)
- [**feature**] mermaid: improve zoom UX [#659](https://github.com/McShelby/hugo-theme-relearn/issues/659)

### Fixes

- [**bug**] variant: fix sidebar-flyout borders color for zen [#667](https://github.com/McShelby/hugo-theme-relearn/issues/667)
- [**bug**] clipboard: fix RTL location of tooltip [#661](https://github.com/McShelby/hugo-theme-relearn/issues/661)
- [**bug**] clipboard: ignore RTL for code [#660](https://github.com/McShelby/hugo-theme-relearn/issues/660)
- [**bug**] expand: fix aria-controls [#658](https://github.com/McShelby/hugo-theme-relearn/issues/658)
- [**bug**] theme: fix id generation for markdownified titles [#657](https://github.com/McShelby/hugo-theme-relearn/issues/657)
- [**bug**] mermaid: avoid graph bombing on hugo --minify [#656](https://github.com/McShelby/hugo-theme-relearn/issues/656)
- [**bug**] mermaid: fix width for some graphs [#655](https://github.com/McShelby/hugo-theme-relearn/issues/655)

---

## 5.22.0 (2023-09-26)

### Enhancements

- [**feature**] mermaid: add pan&zoom reset [#651](https://github.com/McShelby/hugo-theme-relearn/issues/651)
- [**feature**] markdown: add interlace color for tables [#648](https://github.com/McShelby/hugo-theme-relearn/issues/648)
- [**feature**] search: add breadcrumb to dedicated search results [#647](https://github.com/McShelby/hugo-theme-relearn/issues/647)
- [**feature**][**change**] menu: optionally disable index pages for sections [#642](https://github.com/McShelby/hugo-theme-relearn/issues/642)

### Fixes

- [**bug**] variants: restore generator zoom [#650](https://github.com/McShelby/hugo-theme-relearn/issues/650)
- [**bug**] clipboard: malused Fontawesome style [#649](https://github.com/McShelby/hugo-theme-relearn/issues/649)
- [**bug**][**change**] theme: avoid id collisions between headings and theme [#646](https://github.com/McShelby/hugo-theme-relearn/issues/646)
- [**bug**] theme: remove HTML validation errors [#644](https://github.com/McShelby/hugo-theme-relearn/issues/644)
- [**bug**] breadcrumb: remove superflous whitespace between items [#643](https://github.com/McShelby/hugo-theme-relearn/issues/643)

---

## 5.21.0 (2023-09-18)

### Enhancements

- [**feature**] topbar: make buttons configurable [#639](https://github.com/McShelby/hugo-theme-relearn/issues/639)
- [**feature**][**change**] menu: fix footer padding [#637](https://github.com/McShelby/hugo-theme-relearn/issues/637)

### Fixes

- [**bug**] breadcrumb: don't ignore spaces for separator [#636](https://github.com/McShelby/hugo-theme-relearn/issues/636)
- [**bug**] theme: fix snyk code issues [#633](https://github.com/McShelby/hugo-theme-relearn/issues/633)
- [**bug**] images: apply image effects to lightbox images [#631](https://github.com/McShelby/hugo-theme-relearn/issues/631)

### Maintenance

- [**task**] openapi: update to swagger 5.7.2 [#641](https://github.com/McShelby/hugo-theme-relearn/issues/641)

---

## 5.20.0 (2023-08-26)

### Enhancements

- [**feature**][**change**] theme: support for colored borders between menu and content [#626](https://github.com/McShelby/hugo-theme-relearn/issues/626)
- [**feature**] image: allow option to apply image effects globally [#623](https://github.com/McShelby/hugo-theme-relearn/issues/623)
- [**feature**][**change**] openapi: switch to light syntaxhighlightning where applicable [#621](https://github.com/McShelby/hugo-theme-relearn/issues/621)
- [**feature**] images: document usage of images with links [#576](https://github.com/McShelby/hugo-theme-relearn/issues/576)

### Fixes

- [**bug**] highlight: fix rendering for Hugo < 0.111 [#630](https://github.com/McShelby/hugo-theme-relearn/issues/630)
- [**bug**] search: remove link underline on dedicated search page [#627](https://github.com/McShelby/hugo-theme-relearn/issues/627)
- [**bug**] highlight: don't switch to block view if hl_inline=true [#618](https://github.com/McShelby/hugo-theme-relearn/issues/618)
- [**bug**] variant: minor adjustments to zen variants [#617](https://github.com/McShelby/hugo-theme-relearn/issues/617)
- [**bug**] mermaid: lazy render graph if it is initially hidden [#187](https://github.com/McShelby/hugo-theme-relearn/issues/187)

### Maintenance

- [**task**] openapi: update to swagger 5.4.1 [#620](https://github.com/McShelby/hugo-theme-relearn/issues/620)

---

## 5.19.0 (2023-08-12)

### Enhancements

- [**feature**] highlight: add title parameter [#616](https://github.com/McShelby/hugo-theme-relearn/issues/616)
- [**feature**] variant: signal variant switch as event [#614](https://github.com/McShelby/hugo-theme-relearn/issues/614)
- [**feature**] variant: add zen variant in light and dark [#613](https://github.com/McShelby/hugo-theme-relearn/issues/613)
- [**feature**] i18n: add Hungarian translation [#604](https://github.com/McShelby/hugo-theme-relearn/issues/604)
- [**feature**] mermaid: update to 10.3.0 [#601](https://github.com/McShelby/hugo-theme-relearn/issues/601)

### Fixes

- [**bug**] siteparam: avoid halt if param is a map/slice [#611](https://github.com/McShelby/hugo-theme-relearn/issues/611)
- [**bug**] mermaid: fix broken zoom since update to v10 [#608](https://github.com/McShelby/hugo-theme-relearn/issues/608)
- [**bug**] mermaid: variant generator diagram does not respond to events [#607](https://github.com/McShelby/hugo-theme-relearn/issues/607)
- [**bug**] print: avoid chroma leak for relearn-dark [#605](https://github.com/McShelby/hugo-theme-relearn/issues/605)

### Maintenance

- [**task**] mermaid: update to 10.3.1 [#610](https://github.com/McShelby/hugo-theme-relearn/issues/610)

---

## 5.18.0 (2023-07-27)

### Enhancements

- [**feature**][**change**] shortcodes: add more deprecation warnings [#598](https://github.com/McShelby/hugo-theme-relearn/issues/598)
- [**feature**][**change**] shortcodes: change `context` parameter to `page` if called as partial [#595](https://github.com/McShelby/hugo-theme-relearn/issues/595)
- [**feature**] siteparam: support nested parameters and text formatting [#590](https://github.com/McShelby/hugo-theme-relearn/issues/590)
- [**feature**][**change**] a11y: improve when tabbing thru links [#581](https://github.com/McShelby/hugo-theme-relearn/issues/581)

### Fixes

- [**bug**] openapi: inherit RTL setting from Hugo content [#600](https://github.com/McShelby/hugo-theme-relearn/issues/600)
- [**bug**] 404: fix display in RTL [#597](https://github.com/McShelby/hugo-theme-relearn/issues/597)
- [**bug**] highlight: fix position of copy-to-clipboard button in RTL [#594](https://github.com/McShelby/hugo-theme-relearn/issues/594)
- [**bug**] openapi: fix spelling [#593](https://github.com/McShelby/hugo-theme-relearn/issues/593)
- [**bug**] search: fix typo in output format [#591](https://github.com/McShelby/hugo-theme-relearn/issues/591)
- [**bug**] tabs: fix tab selection by groupid [#582](https://github.com/McShelby/hugo-theme-relearn/issues/582)
- [**bug**] theme: restore compat with Hugo 0.95.0 [#580](https://github.com/McShelby/hugo-theme-relearn/issues/580)
- [**bug**][**change**] theme: improve display of links [#577](https://github.com/McShelby/hugo-theme-relearn/issues/577)

---

## 5.17.1 (2023-06-22)

### Enhancements

- [**feature**][**change**] highlight: make copy to clipboard appear on hover [#574](https://github.com/McShelby/hugo-theme-relearn/issues/574)

---

## 5.17.0 (2023-06-22)

### Enhancements

- [**feature**] highlight: add configurable line breaks [#169](https://github.com/McShelby/hugo-theme-relearn/issues/169)

### Fixes

- [**bug**] theme: support Hugo 0.114.0 [#573](https://github.com/McShelby/hugo-theme-relearn/issues/573)
- [**bug**] taxonomy: fix number tags [#570](https://github.com/McShelby/hugo-theme-relearn/issues/570)
- [**bug**] highlight: improve copy to clipboard [#569](https://github.com/McShelby/hugo-theme-relearn/issues/569)

---

## 5.16.2 (2023-06-10)

### Enhancements

- [**feature**] theme: revamp 404 page [#566](https://github.com/McShelby/hugo-theme-relearn/issues/566)

---

## 5.16.1 (2023-06-09)

### Enhancements

- [**feature**] theme: add deprecation warnings [#565](https://github.com/McShelby/hugo-theme-relearn/issues/565)

### Fixes

- [**bug**] mermaid: allow for YAML frontmatter inside of graph [#564](https://github.com/McShelby/hugo-theme-relearn/issues/564)
- [**bug**] alias: fix redirect URLs in case of empty BaseURL [#562](https://github.com/McShelby/hugo-theme-relearn/issues/562)

---

## 5.16.0 (2023-06-08)

### Enhancements

- [**feature**] tabs: add title and icon option [#552](https://github.com/McShelby/hugo-theme-relearn/issues/552)
- [**feature**] shortcodes: add style option to mimic code box color scheme [#551](https://github.com/McShelby/hugo-theme-relearn/issues/551)
- [**feature**] tabs: support color options [#550](https://github.com/McShelby/hugo-theme-relearn/issues/550)
- [**feature**] favicon: add light & dark option for OS's prefered color scheme [#549](https://github.com/McShelby/hugo-theme-relearn/issues/549)

### Fixes

- [**bug**] icon: remove whitespace on start [#560](https://github.com/McShelby/hugo-theme-relearn/issues/560)
- [**bug**] shortcodes: avoid superflous margin at start and end of content [#558](https://github.com/McShelby/hugo-theme-relearn/issues/558)
- [**bug**] expand: fix html encoding of finishing content tag [#557](https://github.com/McShelby/hugo-theme-relearn/issues/557)
- [**bug**] icon: fix ouput "raw HTML omitted" with goldmark config unsafe=false [#555](https://github.com/McShelby/hugo-theme-relearn/issues/555)

---

## 5.15.2 (2023-05-29)

### Enhancements

- [**feature**] taxonomy: add support for category default taxonomy [#541](https://github.com/McShelby/hugo-theme-relearn/issues/541)

### Fixes

- [**bug**] attachments: work for Hugo < 0.112 [#546](https://github.com/McShelby/hugo-theme-relearn/issues/546)

---

## 5.15.1 (2023-05-25)

### Fixes

- [**bug**] shortcodes: intermediately use random ids instead of .Ordinal [#543](https://github.com/McShelby/hugo-theme-relearn/issues/543)

---

## 5.15.0 (2023-05-25)

### Enhancements

- [**feature**] tab: new shortcode to display single tab [#538](https://github.com/McShelby/hugo-theme-relearn/issues/538)
- [**feature**][**change**] tabs: treat groupid as unique if not set [#537](https://github.com/McShelby/hugo-theme-relearn/issues/537)
- [**feature**] expand: indent expanded content [#536](https://github.com/McShelby/hugo-theme-relearn/issues/536)
- [**feature**] notice: make boxes more prominent [#535](https://github.com/McShelby/hugo-theme-relearn/issues/535)

### Fixes

- [**bug**] attachments: fix build error since Hugo 0.112 [#540](https://github.com/McShelby/hugo-theme-relearn/issues/540)

### Maintenance

- [**task**] chore: update Mermaid to 9.4.3 [#534](https://github.com/McShelby/hugo-theme-relearn/issues/534)
- [**task**] mermaid: update to 10.2.0 [#499](https://github.com/McShelby/hugo-theme-relearn/issues/499)

---

## 5.14.3 (2023-05-20)

### Fixes

- [**bug**] tags: show taxonomy toc for standard installation [#533](https://github.com/McShelby/hugo-theme-relearn/issues/533)

---

## 5.14.2 (2023-05-20)

### Fixes

- [**bug**] tags: translate breadcrumb and title for taxonomy [#532](https://github.com/McShelby/hugo-theme-relearn/issues/532)

---

## 5.14.1 (2023-05-20)
*No changelog for this release.*

---

## 5.14.0 (2023-05-19)

### Enhancements

- [**feature**] tags: improve search index for tags [#531](https://github.com/McShelby/hugo-theme-relearn/issues/531)
- [**feature**] tags: increase readability of taxonomy pages [#530](https://github.com/McShelby/hugo-theme-relearn/issues/530)
- [**feature**] nav: make breadcrumb separator configurable [#529](https://github.com/McShelby/hugo-theme-relearn/issues/529)
- [**feature**] i18n: add translation for default taxonomies [#528](https://github.com/McShelby/hugo-theme-relearn/issues/528)
- [**feature**] theme: set appropriate defaults for all theme specific params [#516](https://github.com/McShelby/hugo-theme-relearn/issues/516)
- [**feature**] theme: allow to display tags below article [#513](https://github.com/McShelby/hugo-theme-relearn/issues/513)

### Fixes

- [**bug**] shortcode: make .context always a page [#527](https://github.com/McShelby/hugo-theme-relearn/issues/527)

---

## 5.13.2 (2023-05-17)

### Fixes

- [**bug**] print: enable print for pages with _build options [#522](https://github.com/McShelby/hugo-theme-relearn/issues/522)

---

## 5.13.1 (2023-05-16)

### Fixes

- [**bug**] openapi: allow toc to scroll page [#526](https://github.com/McShelby/hugo-theme-relearn/issues/526)

---

## 5.13.0 (2023-05-14)

### Enhancements

- [**feature**][**change**] openapi: replace implementation with swagger-ui [#523](https://github.com/McShelby/hugo-theme-relearn/issues/523)

### Fixes

- [**bug**] variant: avoid leaking shadows in neon print style [#524](https://github.com/McShelby/hugo-theme-relearn/issues/524)

---

## 5.12.6 (2023-05-04)

### Enhancements

- [**feature**] theme: better HTML titles and breadcrumbs for search and tag pages [#521](https://github.com/McShelby/hugo-theme-relearn/issues/521)

### Fixes

- [**bug**] menu: avoid hiding of expander on hover when active item has children [#520](https://github.com/McShelby/hugo-theme-relearn/issues/520)
- [**bug**] menu: showVisitedLinks not working for some theme variants [#518](https://github.com/McShelby/hugo-theme-relearn/issues/518)
- [**bug**] theme: fix resource URLs for 404 page on subdirectories [#515](https://github.com/McShelby/hugo-theme-relearn/issues/515)

---

## 5.12.5 (2023-03-28)

### Fixes

- [**bug**] expand: not properly exanded when used in bullet point list [#508](https://github.com/McShelby/hugo-theme-relearn/issues/508)

---

## 5.12.4 (2023-03-24)

### Fixes

- [**bug**] theme: disableExplicitIndexURLs param is not working as expected [#505](https://github.com/McShelby/hugo-theme-relearn/issues/505)

---

## 5.12.3 (2023-03-14)

### Fixes

- [**bug**] attachments: fix links if only one language is present [#503](https://github.com/McShelby/hugo-theme-relearn/issues/503)
- [**bug**] shortcodes: allow markdown for title and content [#502](https://github.com/McShelby/hugo-theme-relearn/issues/502)

---

## 5.12.2 (2023-03-03)

### Fixes

- [**bug**] menu: fix state for alwaysopen=false + collapsibleMenu=false [#498](https://github.com/McShelby/hugo-theme-relearn/issues/498)

---

## 5.12.1 (2023-02-26)

### Enhancements

- [**feature**] variant: add relearn bright theme [#493](https://github.com/McShelby/hugo-theme-relearn/issues/493)

### Fixes

- [**bug**] generator: fix setting of colors [#494](https://github.com/McShelby/hugo-theme-relearn/issues/494)

---

## 5.12.0 (2023-02-24)

### Enhancements

- [**feature**] frontmatter: support VSCode Front Matter extension [#481](https://github.com/McShelby/hugo-theme-relearn/issues/481)
- [**feature**] theme: make expand and image ids stable [#477](https://github.com/McShelby/hugo-theme-relearn/issues/477)
- [**feature**] variant: set scrollbar color to dark for dark variants [#471](https://github.com/McShelby/hugo-theme-relearn/issues/471)
- [**feature**] i18n: add full RTL support [#470](https://github.com/McShelby/hugo-theme-relearn/issues/470)
- [**feature**] piratify: fix some quirks, arrr [#469](https://github.com/McShelby/hugo-theme-relearn/issues/469)
- [**feature**][**change**] theme: optimization for huge screen sizes [#466](https://github.com/McShelby/hugo-theme-relearn/issues/466)

### Fixes

- [**bug**] i18n: write code ltr even for rtl languages [#492](https://github.com/McShelby/hugo-theme-relearn/issues/492)
- [**bug**] anchor: fix link in FF when served from file system [#482](https://github.com/McShelby/hugo-theme-relearn/issues/482)
- [**bug**] shortcodes: don't break build and render for invalid parameters [#480](https://github.com/McShelby/hugo-theme-relearn/issues/480)
- [**bug**] nav: restore scroll position on browser back [#476](https://github.com/McShelby/hugo-theme-relearn/issues/476)
- [**bug**] variant: avoid style leak for auto style [#473](https://github.com/McShelby/hugo-theme-relearn/issues/473)

### Maintenance

- [**task**] build: add imagebot [#485](https://github.com/McShelby/hugo-theme-relearn/issues/485)

---

## 5.11.2 (2023-02-07)

### Fixes

- [**bug**] tabs: nested tabs content is not displayed [#468](https://github.com/McShelby/hugo-theme-relearn/issues/468)

---

## 5.11.1 (2023-02-06)

### Fixes

- [**bug**] variant: include missing `theme-auto.css` in distribution [#467](https://github.com/McShelby/hugo-theme-relearn/issues/467)

---

## 5.11.0 (2023-02-05)

### Enhancements

- [**feature**] i18n: add Czech translation [#455](https://github.com/McShelby/hugo-theme-relearn/issues/455)
- [**feature**][**change**] lightbox: switch to CSS-only solution [#451](https://github.com/McShelby/hugo-theme-relearn/issues/451)
- [**feature**][**change**] variant: add support for `prefers-color-scheme` [#445](https://github.com/McShelby/hugo-theme-relearn/issues/445)
- [**feature**][**change**] expand: refactor for a11y [#339](https://github.com/McShelby/hugo-theme-relearn/issues/339)
- [**feature**][**change**] mermaid: make zoom configurable [#144](https://github.com/McShelby/hugo-theme-relearn/issues/144)

### Fixes

- [**bug**] swagger: avoid errors when using invalid rapi-doc fragment ids [#465](https://github.com/McShelby/hugo-theme-relearn/issues/465)
- [**bug**] search: fix oddities in keyboard handling [#463](https://github.com/McShelby/hugo-theme-relearn/issues/463)
- [**bug**] badge: fix text color for IE11 [#462](https://github.com/McShelby/hugo-theme-relearn/issues/462)
- [**bug**] mermaid: rerender graph if search term is present and variant is switched [#460](https://github.com/McShelby/hugo-theme-relearn/issues/460)
- [**bug**] tags: show tag on pages when tag has space [#459](https://github.com/McShelby/hugo-theme-relearn/issues/459)
- [**bug**] edit: remove double slash on root page link [#450](https://github.com/McShelby/hugo-theme-relearn/issues/450)

### Maintenance

- [**task**] build: add moving version tags [#453](https://github.com/McShelby/hugo-theme-relearn/issues/453)
- [**task**][**change**] theme: remove jQuery [#452](https://github.com/McShelby/hugo-theme-relearn/issues/452)
- [**task**] build: check for release notes before release [#448](https://github.com/McShelby/hugo-theme-relearn/issues/448)

---

## 5.10.2 (2023-01-25)

### Fixes

- [**bug**] nav: fix breadcrumb for huge installations [#446](https://github.com/McShelby/hugo-theme-relearn/issues/446)

---

## 5.10.1 (2023-01-25)

### Fixes

- [**bug**] print: fix image links with relative path [#444](https://github.com/McShelby/hugo-theme-relearn/issues/444)

---

## 5.10.0 (2023-01-25)

### Enhancements

- [**feature**] shortcodes: support for accent color [#440](https://github.com/McShelby/hugo-theme-relearn/issues/440)
- [**feature**] shortcodes: add color parameter where applicable [#438](https://github.com/McShelby/hugo-theme-relearn/issues/438)
- [**feature**] theme: announce translations as alternate links [#422](https://github.com/McShelby/hugo-theme-relearn/issues/422)

### Fixes

- [**bug**] nav: fix breadcrumbs for deeply nested sections [#442](https://github.com/McShelby/hugo-theme-relearn/issues/442)
- [**bug**] theme: improve whitespacing in tables [#441](https://github.com/McShelby/hugo-theme-relearn/issues/441)

---

## 5.9.4 (2023-01-23)

### Fixes

- [**bug**] variant: fix search icon and text color [#437](https://github.com/McShelby/hugo-theme-relearn/issues/437)

---

## 5.9.3 (2023-01-22)

### Fixes

- [**bug**] nav: fix left/right navigation for horizontal scrolling [#435](https://github.com/McShelby/hugo-theme-relearn/issues/435)
- [**bug**][**breaking**] theme: allow pages on top level [#434](https://github.com/McShelby/hugo-theme-relearn/issues/434)

### Maintenance

- [**task**] build: switch to wildcard version of actions [#428](https://github.com/McShelby/hugo-theme-relearn/issues/428)

---

## 5.9.2 (2022-12-30)

### Fixes

- [**bug**] search: apply dependency scripts for Hindi and Japanese [#427](https://github.com/McShelby/hugo-theme-relearn/issues/427)

---

## 5.9.1 (2022-12-23)

### Enhancements

- [**feature**] theme: make external link target configurable [#426](https://github.com/McShelby/hugo-theme-relearn/issues/426)

---

## 5.9.0 (2022-12-23)

### Enhancements

- [**feature**][**change**] theme: open external links in separate tab [#419](https://github.com/McShelby/hugo-theme-relearn/issues/419)
- [**feature**] theme: make it a Hugo module [#417](https://github.com/McShelby/hugo-theme-relearn/issues/417)

### Fixes

- [**bug**][**change**] attachments: fix incorrect links for defaultContentLanguageInSubdir=true [#425](https://github.com/McShelby/hugo-theme-relearn/issues/425)

---

## 5.8.1 (2022-12-11)

### Fixes

- [**bug**] theme: fix alias for home page if defaultContentLanguageInSubdir=true [#414](https://github.com/McShelby/hugo-theme-relearn/issues/414)

---

## 5.8.0 (2022-12-08)

### Enhancements

- [**feature**] icon: add new shortcode [#412](https://github.com/McShelby/hugo-theme-relearn/issues/412)
- [**feature**] theme: style and document markdown extensions [#411](https://github.com/McShelby/hugo-theme-relearn/issues/411)
- [**feature**] badge: add new shortcode [#410](https://github.com/McShelby/hugo-theme-relearn/issues/410)
- [**feature**] theme: add accent color [#409](https://github.com/McShelby/hugo-theme-relearn/issues/409)

### Fixes

- [**bug**] theme: fix spacing for tag flyout in FF [#413](https://github.com/McShelby/hugo-theme-relearn/issues/413)

---

## 5.7.0 (2022-11-29)

### Enhancements

- [**feature**] button: refactor for a11y [#372](https://github.com/McShelby/hugo-theme-relearn/issues/372)

### Fixes

- [**bug**] search: don't freeze browser on long search terms [#408](https://github.com/McShelby/hugo-theme-relearn/issues/408)
- [**bug**] search: fix searchbox placeholder color in FF and IE [#405](https://github.com/McShelby/hugo-theme-relearn/issues/405)
- [**bug**][**change**] i18n: rename Korean translation from country to lang code [#404](https://github.com/McShelby/hugo-theme-relearn/issues/404)

### Maintenance

- [**task**] search: update lunr languages to 1.10.0 [#403](https://github.com/McShelby/hugo-theme-relearn/issues/403)

---

## 5.6.6 (2022-11-23)

### Enhancements

- [**feature**] search: make build and js forgiving against config errors [#400](https://github.com/McShelby/hugo-theme-relearn/issues/400)

### Fixes

- [**bug**] variant: minor color adjustments [#402](https://github.com/McShelby/hugo-theme-relearn/issues/402)
- [**bug**] variant: fix generator for use of neon [#401](https://github.com/McShelby/hugo-theme-relearn/issues/401)

---

## 5.6.5 (2022-11-19)

### Fixes

- [**bug**] menu: relax usage of background color [#399](https://github.com/McShelby/hugo-theme-relearn/issues/399)

---

## 5.6.4 (2022-11-19)

### Fixes

- [**bug**] theme: make alias pages usable by file:// protocol [#398](https://github.com/McShelby/hugo-theme-relearn/issues/398)

---

## 5.6.3 (2022-11-19)

### Fixes

- [**bug**] theme: be compatible with Hugo >= 0.95.0 [#397](https://github.com/McShelby/hugo-theme-relearn/issues/397)

---

## 5.6.2 (2022-11-19)

### Fixes

- [**bug**] theme: build breaks sites without "output" section in config [#396](https://github.com/McShelby/hugo-theme-relearn/issues/396)

---

## 5.6.1 (2022-11-19)

### Fixes

- [**bug**] theme: fix image distortion [#395](https://github.com/McShelby/hugo-theme-relearn/issues/395)

---

## 5.6.0 (2022-11-18)

### Enhancements

- [**feature**] toc: improve keyboard handling [#390](https://github.com/McShelby/hugo-theme-relearn/issues/390)
- [**feature**] search: improve keyboard handling [#387](https://github.com/McShelby/hugo-theme-relearn/issues/387)
- [**feature**] search: add dedicated search page [#386](https://github.com/McShelby/hugo-theme-relearn/issues/386)
- [**feature**] theme: make creation of generator meta tag configurable [#383](https://github.com/McShelby/hugo-theme-relearn/issues/383)
- [**feature**] theme: increase build performance [#380](https://github.com/McShelby/hugo-theme-relearn/issues/380)

### Fixes

- [**bug**] mermaid: avoid leading whitespace [#394](https://github.com/McShelby/hugo-theme-relearn/issues/394)
- [**bug**] theme: fix build errors when referencing SVGs in markdown [#393](https://github.com/McShelby/hugo-theme-relearn/issues/393)
- [**bug**] variant: avoid neon to leak into IE11 fallback [#392](https://github.com/McShelby/hugo-theme-relearn/issues/392)
- [**bug**] theme: fix urls for file:// protocol in sitemap [#385](https://github.com/McShelby/hugo-theme-relearn/issues/385)
- [**bug**] theme: add id to h1 elements [#384](https://github.com/McShelby/hugo-theme-relearn/issues/384)
- [**bug**] rss: fix display of hidden subpages [#382](https://github.com/McShelby/hugo-theme-relearn/issues/382)
- [**bug**] nav: fix key navigation when pressing wrong modifiers [#379](https://github.com/McShelby/hugo-theme-relearn/issues/379)

### Maintenance

- [**task**] mermaid: update to version 9.2.2 [#391](https://github.com/McShelby/hugo-theme-relearn/issues/391)

---

## 5.5.3 (2022-11-10)

### Fixes

- [**bug**] tags: fix non-latin tag display on pages [#378](https://github.com/McShelby/hugo-theme-relearn/issues/378)

---

## 5.5.2 (2022-11-08)

### Fixes

- [**bug**] theme: fix typo in 404.html [#376](https://github.com/McShelby/hugo-theme-relearn/issues/376)
- [**bug**] theme: allow menu items and children to be served by file:// protocol [#375](https://github.com/McShelby/hugo-theme-relearn/issues/375)

---

## 5.5.1 (2022-11-07)

### Fixes

- [**bug**] theme: fix overflowing issue with anchors and tooltips [#364](https://github.com/McShelby/hugo-theme-relearn/issues/364)

---

## 5.5.0 (2022-11-06)

### Enhancements

- [**feature**][**change**] theme: optimize page load for images [#304](https://github.com/McShelby/hugo-theme-relearn/issues/304)

### Fixes

- [**bug**] theme: fix context in render hooks [#373](https://github.com/McShelby/hugo-theme-relearn/issues/373)
- [**bug**] print: make canonical URL absolute [#371](https://github.com/McShelby/hugo-theme-relearn/issues/371)

---

## 5.4.3 (2022-11-05)

### Enhancements

- [**feature**] history: refactor for a11y [#341](https://github.com/McShelby/hugo-theme-relearn/issues/341)

### Fixes

- [**bug**] theme: fix multilang links when site served from subdirectory [#370](https://github.com/McShelby/hugo-theme-relearn/issues/370)

---

## 5.4.2 (2022-11-05)

### Maintenance

- [**task**] build: change set-output to env vars [#348](https://github.com/McShelby/hugo-theme-relearn/issues/348)

---

## 5.4.1 (2022-11-05)

### Fixes

- [**bug**] mermaid: fix Gantt chart width [#365](https://github.com/McShelby/hugo-theme-relearn/issues/365)

---

## 5.4.0 (2022-11-01)

### Enhancements

- [**feature**] math: allow passing of parameters with codefence syntax [#363](https://github.com/McShelby/hugo-theme-relearn/issues/363)
- [**feature**] i18n: add Finnish translation [#361](https://github.com/McShelby/hugo-theme-relearn/issues/361)
- [**feature**] mermaid: allow passing of parameters with codefence syntax [#360](https://github.com/McShelby/hugo-theme-relearn/issues/360)
- [**feature**] i18n: support RTL [#357](https://github.com/McShelby/hugo-theme-relearn/issues/357)
- [**feature**][**change**] button: add option for target [#351](https://github.com/McShelby/hugo-theme-relearn/issues/351)
- [**feature**][**change**] theme: allow to be served by file:// protocol [#349](https://github.com/McShelby/hugo-theme-relearn/issues/349)

---

## 5.3.3 (2022-10-09)

### Fixes

- [**bug**] archetypes: fix frontmatter on home.md template [#346](https://github.com/McShelby/hugo-theme-relearn/issues/346)

---

## 5.3.2 (2022-10-08)

### Fixes

- [**bug**] nav: change defunct keyboard shortcuts [#344](https://github.com/McShelby/hugo-theme-relearn/issues/344)

---

## 5.3.1 (2022-10-08)

### Enhancements

- [**feature**] i18n: update Spanish translation [#343](https://github.com/McShelby/hugo-theme-relearn/issues/343)
- [**feature**] theme: option to align images [#327](https://github.com/McShelby/hugo-theme-relearn/issues/327)

---

## 5.3.0 (2022-10-07)

### Enhancements

- [**feature**] expander: improve whitespace between label and content [#338](https://github.com/McShelby/hugo-theme-relearn/issues/338)
- [**feature**] swagger: improve print version [#333](https://github.com/McShelby/hugo-theme-relearn/issues/333)

### Fixes

- [**bug**] print: fix links of subsections [#340](https://github.com/McShelby/hugo-theme-relearn/issues/340)
- [**bug**] theme: remove W3C validator errors [#337](https://github.com/McShelby/hugo-theme-relearn/issues/337)
- [**bug**] children: remove unused `page` parameter from docs [#336](https://github.com/McShelby/hugo-theme-relearn/issues/336)
- [**bug**] print: remove menu placeholder in Firefox [#335](https://github.com/McShelby/hugo-theme-relearn/issues/335)
- [**bug**] swagger: fix download button overflow [#334](https://github.com/McShelby/hugo-theme-relearn/issues/334)
- [**bug**][**change**] a11y: remove WCAG errors where applicable [#307](https://github.com/McShelby/hugo-theme-relearn/issues/307)

---

## 5.2.4 (2022-10-02)

### Fixes

- [**bug**] theme: remove HTML5 validator errors [#329](https://github.com/McShelby/hugo-theme-relearn/issues/329)

---

## 5.2.3 (2022-09-12)

### Fixes

- [**bug**] print: chapter pages overwrite font-size [#328](https://github.com/McShelby/hugo-theme-relearn/issues/328)

---

## 5.2.2 (2022-08-23)

### Fixes

- [**bug**] print: fix urls for uglyURLs=true [#322](https://github.com/McShelby/hugo-theme-relearn/issues/322)

---

## 5.2.1 (2022-08-05)

### Enhancements

- [**feature**] i18n: improve Japanese translation [#318](https://github.com/McShelby/hugo-theme-relearn/issues/318)

### Fixes

- [**bug**] nav: prev/next ignores ordersectionby [#320](https://github.com/McShelby/hugo-theme-relearn/issues/320)

### Maintenance

- [**task**] task: bump Hugo minimum requirement to 0.95 [#319](https://github.com/McShelby/hugo-theme-relearn/issues/319)

---

## 5.2.0 (2022-08-03)

### Enhancements

- [**feature**][**change**] menu: expand collapsed menus if search term is found in submenus [#312](https://github.com/McShelby/hugo-theme-relearn/issues/312)

### Fixes

- [**bug**] print: switch mermaid and swagger style before print [#316](https://github.com/McShelby/hugo-theme-relearn/issues/316)
- [**bug**] theme: fix chapter margins on big screens [#315](https://github.com/McShelby/hugo-theme-relearn/issues/315)

---

## 5.1.2 (2022-07-18)

### Fixes

- [**bug**] print: reset mermaid theme to light [#313](https://github.com/McShelby/hugo-theme-relearn/issues/313)
- [**bug**] mermaid: header is showing up in FF [#311](https://github.com/McShelby/hugo-theme-relearn/issues/311)

---

## 5.1.1 (2022-07-15)

### Fixes

- [**bug**] tags: don't count tags if page is hidden [#310](https://github.com/McShelby/hugo-theme-relearn/issues/310)

---

## 5.1.0 (2022-07-15)

### Enhancements

- [**feature**][**change**] print: make print url deterministic [#309](https://github.com/McShelby/hugo-theme-relearn/issues/309)
- [**feature**] theme: allow overriding partials for output formats [#308](https://github.com/McShelby/hugo-theme-relearn/issues/308)

---

## 5.0.3 (2022-07-07)

### Fixes

- [**bug**] ie11: no styles after rework of archetypes [#306](https://github.com/McShelby/hugo-theme-relearn/issues/306)

---

## 5.0.2 (2022-07-07)

### Fixes

- [**bug**] theme: load CSS if JS is disabled [#305](https://github.com/McShelby/hugo-theme-relearn/issues/305)

---

## 5.0.1 (2022-07-07)

### Enhancements

- [**feature**][**breaking**] theme: optimize loading of js and css [#303](https://github.com/McShelby/hugo-theme-relearn/issues/303)

---

## 5.0.0 (2022-07-05)

### Enhancements

- [**feature**][**change**] archetypes: modularize rendering [#300](https://github.com/McShelby/hugo-theme-relearn/issues/300)
- [**feature**] history: don't reload page when history gets cleared [#299](https://github.com/McShelby/hugo-theme-relearn/issues/299)
- [**feature**] menu: replace expander by fontawesome chevrons [#296](https://github.com/McShelby/hugo-theme-relearn/issues/296)
- [**feature**] theme: align content with topbar icon limits [#290](https://github.com/McShelby/hugo-theme-relearn/issues/290)
- [**feature**] button: allow for empty href [#288](https://github.com/McShelby/hugo-theme-relearn/issues/288)
- [**feature**] i18n: make Simplified Chinese the standard language for the `zn` code [#287](https://github.com/McShelby/hugo-theme-relearn/issues/287)
- [**feature**] clipboard: move head styles to stylesheet [#286](https://github.com/McShelby/hugo-theme-relearn/issues/286)
- [**feature**] math: add mathjax rendering [#235](https://github.com/McShelby/hugo-theme-relearn/issues/235)
- [**feature**] theme: allow for page heading modification [#139](https://github.com/McShelby/hugo-theme-relearn/issues/139)

### Fixes

- [**bug**] favicon: fix URL if site resides in subfolder [#302](https://github.com/McShelby/hugo-theme-relearn/issues/302)
- [**bug**] code: show copy-to-clipboard marker for blocklevel code [#298](https://github.com/McShelby/hugo-theme-relearn/issues/298)
- [**bug**] menu: make active expander visible on hover [#297](https://github.com/McShelby/hugo-theme-relearn/issues/297)
- [**bug**] print: disable arrow navigation [#294](https://github.com/McShelby/hugo-theme-relearn/issues/294)
- [**bug**] print: add missing page break after index or section [#292](https://github.com/McShelby/hugo-theme-relearn/issues/292)
- [**bug**] theme: use more space on wide screens [#291](https://github.com/McShelby/hugo-theme-relearn/issues/291)
- [**bug**] theme: fix size of chapter heading [#289](https://github.com/McShelby/hugo-theme-relearn/issues/289)

### Maintenance

- [**task**] chore: update RapiDoc 9.3.3 [#301](https://github.com/McShelby/hugo-theme-relearn/issues/301)
- [**task**] chore: update Mermaid 9.1.3 [#293](https://github.com/McShelby/hugo-theme-relearn/issues/293)

---

## 4.2.5 (2022-06-23)

### Fixes

- [**bug**] swagger: javascript code does not load in documentation [#285](https://github.com/McShelby/hugo-theme-relearn/issues/285)
- [**bug**] children: descriptions not working [#284](https://github.com/McShelby/hugo-theme-relearn/issues/284)
- [**bug**] print: fix empty page for shortcut links [#283](https://github.com/McShelby/hugo-theme-relearn/issues/283)

---

## 4.2.4 (2022-06-23)

### Fixes

- [**bug**] theme: fix url for logo and home button [#282](https://github.com/McShelby/hugo-theme-relearn/issues/282)

---

## 4.2.3 (2022-06-23)

### Fixes

- [**bug**][**breaking**] include: second parameter is ignored [#281](https://github.com/McShelby/hugo-theme-relearn/issues/281)

---

## 4.2.2 (2022-06-23)
*No changelog for this release.*

---

## 4.2.1 (2022-06-23)
*No changelog for this release.*

---

## 4.2.0 (2022-06-23)

### Enhancements

- [**feature**][**change**] tabs: don't change tab selection if panel does not contain item [#279](https://github.com/McShelby/hugo-theme-relearn/issues/279)
- [**feature**] shortcodes: convert to partials [#277](https://github.com/McShelby/hugo-theme-relearn/issues/277)

### Fixes

- [**bug**] swagger: avoid builtin syntax-highlightning [#280](https://github.com/McShelby/hugo-theme-relearn/issues/280)
- [**bug**] search: fix console message for missing lunr translations [#278](https://github.com/McShelby/hugo-theme-relearn/issues/278)
- [**bug**] tabs: fix wrapping when having many tabs [#272](https://github.com/McShelby/hugo-theme-relearn/issues/272)

---

## 4.1.1 (2022-06-18)

### Fixes

- [**bug**] notice: fix layout when content starts with heading [#275](https://github.com/McShelby/hugo-theme-relearn/issues/275)

---

## 4.1.0 (2022-06-12)

### Enhancements

- [**feature**] i18n: support multilang content [#271](https://github.com/McShelby/hugo-theme-relearn/issues/271)

---

## 4.0.5 (2022-06-12)

### Fixes

- [**bug**] i18n: Vietnamese language with wrong lang code [#270](https://github.com/McShelby/hugo-theme-relearn/issues/270)
- [**bug**] i18n: fix search for non western languages [#269](https://github.com/McShelby/hugo-theme-relearn/issues/269)

---

## 4.0.4 (2022-06-07)

### Enhancements

- [**feature**] theme: improve keyboard navigation for scrolling [#268](https://github.com/McShelby/hugo-theme-relearn/issues/268)

### Fixes

- [**bug**] swagger: adjust font-size for method buttons [#267](https://github.com/McShelby/hugo-theme-relearn/issues/267)
- [**bug**] menu: hide expander when only hidden subpages [#264](https://github.com/McShelby/hugo-theme-relearn/issues/264)
- [**bug**] theme: make compatible with Hugo 0.100.0 [#263](https://github.com/McShelby/hugo-theme-relearn/issues/263)

### Maintenance

- [**task**] swagger: update rapidoc to 9.3.2 [#266](https://github.com/McShelby/hugo-theme-relearn/issues/266)
- [**task**] mermaid: update to 9.1.1 [#265](https://github.com/McShelby/hugo-theme-relearn/issues/265)

---

## 4.0.3 (2022-06-05)

### Enhancements

- [**feature**] toc: add scrollbar [#262](https://github.com/McShelby/hugo-theme-relearn/issues/262)

---

## 4.0.2 (2022-06-05)

### Fixes

- [**bug**] theme: let browser scroll page on CTRL+f  [#242](https://github.com/McShelby/hugo-theme-relearn/issues/242)

---

## 4.0.1 (2022-06-05)
*No changelog for this release.*

---

## 4.0.0 (2022-06-05)

### Enhancements

- [**feature**] shortcodes: add named parameter if missing [#260](https://github.com/McShelby/hugo-theme-relearn/issues/260)
- [**feature**][**breaking**] theme: remove --MAIN-ANCHOR-color from stylesheet [#256](https://github.com/McShelby/hugo-theme-relearn/issues/256)
- [**feature**] i18n: add Italian translation [#254](https://github.com/McShelby/hugo-theme-relearn/issues/254)
- [**feature**] attachments: support for brand colors [#252](https://github.com/McShelby/hugo-theme-relearn/issues/252)
- [**feature**] notice: support for brand colors [#251](https://github.com/McShelby/hugo-theme-relearn/issues/251)
- [**feature**][**breaking**] config: remove custom_css [#248](https://github.com/McShelby/hugo-theme-relearn/issues/248)
- [**feature**] theme: use proper file extension for page-meta.go [#246](https://github.com/McShelby/hugo-theme-relearn/issues/246)
- [**feature**] variant: add support for brand color variables [#239](https://github.com/McShelby/hugo-theme-relearn/issues/239)
- [**feature**] i18n: add Polish translation [#237](https://github.com/McShelby/hugo-theme-relearn/issues/237)

### Fixes

- [**bug**] shortcodes: accept boolean parameters if given as string [#261](https://github.com/McShelby/hugo-theme-relearn/issues/261)
- [**bug**] print: adjust button and tab size [#259](https://github.com/McShelby/hugo-theme-relearn/issues/259)
- [**bug**] print: show Mermaid if requested in frontmatter [#255](https://github.com/McShelby/hugo-theme-relearn/issues/255)
- [**bug**] theme: adjust thin scrollbar slider [#244](https://github.com/McShelby/hugo-theme-relearn/issues/244)
- [**bug**] mobile: fix broken scrollbar [#243](https://github.com/McShelby/hugo-theme-relearn/issues/243)
- [**bug**] theme: fix display of tooltip for heading anchor  [#241](https://github.com/McShelby/hugo-theme-relearn/issues/241)

---

## 3.4.1 (2022-04-03)

### Fixes

- [**bug**] theme: fix IE11 incompatibilities [#234](https://github.com/McShelby/hugo-theme-relearn/issues/234)

---

## 3.4.0 (2022-04-03)

### Enhancements

- [**feature**] i18n: add Traditional Chinese translation [#233](https://github.com/McShelby/hugo-theme-relearn/issues/233)
- [**feature**] menu: expand/collapse menu items without navigation [#231](https://github.com/McShelby/hugo-theme-relearn/issues/231)
- [**feature**] print: add option to print whole chapter [#230](https://github.com/McShelby/hugo-theme-relearn/issues/230)
- [**feature**][**breaking**] theme: apply user supplied content footer below content [#229](https://github.com/McShelby/hugo-theme-relearn/issues/229)

### Fixes

- [**bug**] theme: scroll to heading on initial load [#232](https://github.com/McShelby/hugo-theme-relearn/issues/232)

---

## 3.3.0 (2022-03-28)

### Enhancements

- [**feature**] theme: add CSS font variables [#227](https://github.com/McShelby/hugo-theme-relearn/issues/227)
- [**feature**] swagger: add support for oas/swagger documentation [#226](https://github.com/McShelby/hugo-theme-relearn/issues/226)

### Fixes

- [**bug**] variant: make variant switch work on slow networks [#228](https://github.com/McShelby/hugo-theme-relearn/issues/228)

---

## 3.2.1 (2022-03-25)

### Fixes

- [**bug**] print: fix minor inconsistencies [#225](https://github.com/McShelby/hugo-theme-relearn/issues/225)
- [**bug**] print: show more than just the title page [#224](https://github.com/McShelby/hugo-theme-relearn/issues/224)
- [**bug**] theme: align content scrollbar to the right on big screens [#223](https://github.com/McShelby/hugo-theme-relearn/issues/223)

---

## 3.2.0 (2022-03-19)

### Enhancements

- [**feature**][**change**] mermaid: support differing themes for color variant switch [#219](https://github.com/McShelby/hugo-theme-relearn/issues/219)
- [**feature**] mermaid: load javascript on demand [#218](https://github.com/McShelby/hugo-theme-relearn/issues/218)

### Maintenance

- [**task**] mermaid: update to 8.14.0 [#220](https://github.com/McShelby/hugo-theme-relearn/issues/220)

---

## 3.1.1 (2022-03-16)

### Enhancements

- [**feature**] i18n: add Korean translation [#217](https://github.com/McShelby/hugo-theme-relearn/issues/217)

---

## 3.1.0 (2022-03-15)

### Enhancements

- [**feature**] notice: add icon parameter [#212](https://github.com/McShelby/hugo-theme-relearn/issues/212)
- [**feature**] mobile: remove breadcrumb ellipsis [#211](https://github.com/McShelby/hugo-theme-relearn/issues/211)

### Fixes

- [**bug**] theme: make storage of multiple Hugo sites on same server distinct [#214](https://github.com/McShelby/hugo-theme-relearn/issues/214)
- [**bug**] variant: switch breadcrumb color in Chrome [#213](https://github.com/McShelby/hugo-theme-relearn/issues/213)
- [**bug**] mobile: improve behavior of sidebar menu [#210](https://github.com/McShelby/hugo-theme-relearn/issues/210)

---

## 3.0.4 (2022-02-24)

### Enhancements

- [**feature**] theme: improve font loading [#201](https://github.com/McShelby/hugo-theme-relearn/issues/201)
- [**feature**][**change**] variant: fix inconsistent color variable naming  [#200](https://github.com/McShelby/hugo-theme-relearn/issues/200)

### Fixes

- [**bug**] variant: fix occasional fail when resetting generator [#208](https://github.com/McShelby/hugo-theme-relearn/issues/208)
- [**bug**] docs: don't move header on logo hover in IE11 [#207](https://github.com/McShelby/hugo-theme-relearn/issues/207)
- [**bug**] variant: avoid flash of menu header when non default variant is active [#206](https://github.com/McShelby/hugo-theme-relearn/issues/206)
- [**bug**] theme: fix wrong HTML closing tag order in chapters [#205](https://github.com/McShelby/hugo-theme-relearn/issues/205)
- [**bug**] theme: adjust breadcrumb and title for empty home page titles [#202](https://github.com/McShelby/hugo-theme-relearn/issues/202)

---

## 3.0.3 (2022-02-23)

### Enhancements

- [**feature**] tags: show tag count in taxonomy list [#195](https://github.com/McShelby/hugo-theme-relearn/issues/195)

### Fixes

- [**bug**] theme: remove Hugo build warning if page is not file based [#197](https://github.com/McShelby/hugo-theme-relearn/issues/197)
- [**bug**] tags: adhere to titleSeparator [#196](https://github.com/McShelby/hugo-theme-relearn/issues/196)
- [**bug**] theme: hide footer divider and variant selector in IE11 [#194](https://github.com/McShelby/hugo-theme-relearn/issues/194)

---

## 3.0.2 (2022-02-23)

### Enhancements

- [**feature**] tags: sort by name [#193](https://github.com/McShelby/hugo-theme-relearn/issues/193)

---

## 3.0.1 (2022-02-23)

### Enhancements

- [**feature**] children: set containerstyle automatically according to style [#192](https://github.com/McShelby/hugo-theme-relearn/issues/192)

### Fixes

- [**bug**] theme: revert fontawsome to version 5 for IE11 compat [#191](https://github.com/McShelby/hugo-theme-relearn/issues/191)

---

## 3.0.0 (2022-02-22)

### Enhancements

- [**feature**] variant: build a variant generator [#188](https://github.com/McShelby/hugo-theme-relearn/issues/188)
- [**feature**] nav: only show toc if the page has headings [#182](https://github.com/McShelby/hugo-theme-relearn/issues/182)
- [**feature**][**breaking**] theme: change default colors to Relearn defaults [#181](https://github.com/McShelby/hugo-theme-relearn/issues/181)
- [**feature**] variant: add a variant selector [#178](https://github.com/McShelby/hugo-theme-relearn/issues/178)
- [**feature**][**breaking**] menu: rework footer UX [#177](https://github.com/McShelby/hugo-theme-relearn/issues/177)
- [**feature**] theme: support for dark mode [#175](https://github.com/McShelby/hugo-theme-relearn/issues/175)
- [**feature**] docs: use light syntax highlightning theme [#174](https://github.com/McShelby/hugo-theme-relearn/issues/174)
- [**feature**] notice: tweak dull colors [#173](https://github.com/McShelby/hugo-theme-relearn/issues/173)
- [**feature**] theme: rework header UX [#151](https://github.com/McShelby/hugo-theme-relearn/issues/151)

### Fixes

- [**bug**] search: remove additional X in filled out search box in IE11 [#190](https://github.com/McShelby/hugo-theme-relearn/issues/190)
- [**bug**] clipboard: localize tooltips [#186](https://github.com/McShelby/hugo-theme-relearn/issues/186)
- [**bug**] print: hide sidebar on Mac [#183](https://github.com/McShelby/hugo-theme-relearn/issues/183)
- [**bug**] menu: fix scrollbar height [#180](https://github.com/McShelby/hugo-theme-relearn/issues/180)
- [**bug**][**change**] search: fix color change for icons on hover [#176](https://github.com/McShelby/hugo-theme-relearn/issues/176)

---

## 2.9.6 (2022-02-07)

### Fixes

- [**bug**] menu: remove debug output [#171](https://github.com/McShelby/hugo-theme-relearn/issues/171)

---

## 2.9.5 (2022-02-07)

### Fixes

- [**bug**] menu: let arrow navigation respect ordersectionsby configuration [#170](https://github.com/McShelby/hugo-theme-relearn/issues/170)

---

## 2.9.4 (2022-02-06)

### Fixes

- [**bug**] exampleSite: fix links in official documentation [#168](https://github.com/McShelby/hugo-theme-relearn/issues/168)

---

## 2.9.3 (2022-02-06)

### Fixes

- [**bug**] menu: invalid URL when the shortcut is an internal link [#163](https://github.com/McShelby/hugo-theme-relearn/issues/163)

---

## 2.9.2 (2021-11-26)

### Enhancements

- [**feature**] theme: add theme version info to head [#158](https://github.com/McShelby/hugo-theme-relearn/issues/158)

### Fixes

- [**bug**] theme: fix selection of *.ico files as favicons [#160](https://github.com/McShelby/hugo-theme-relearn/issues/160)

---

## 2.9.1 (2021-11-22)

### Fixes

- [**bug**] menu: fix significantly low performance for collecting of meta info [#157](https://github.com/McShelby/hugo-theme-relearn/issues/157)

---

## 2.9.0 (2021-11-19)

### Fixes

- [**bug**][**breaking**] relref: fix inconsistent behavior [#156](https://github.com/McShelby/hugo-theme-relearn/issues/156)
- [**bug**] search: make dropdown stick to search field when scrolling [#155](https://github.com/McShelby/hugo-theme-relearn/issues/155)
- [**bug**] menu: align long text properly [#154](https://github.com/McShelby/hugo-theme-relearn/issues/154)
- [**bug**] copyToClipBoard: add missing right border for inline code if `disableInlineCopyToClipBoard=true` [#153](https://github.com/McShelby/hugo-theme-relearn/issues/153)
- [**bug**] menu: show hidden sibling pages reliably [#152](https://github.com/McShelby/hugo-theme-relearn/issues/152)
- [**bug**] menu: bring active item in sight for large menus [#149](https://github.com/McShelby/hugo-theme-relearn/issues/149)

---

## 2.8.3 (2021-11-09)

### Fixes

- [**bug**] mermaid: let zoom reset to initial size [#145](https://github.com/McShelby/hugo-theme-relearn/issues/145)
- [**bug**] mermaid: remove whitespace from big graphs [#143](https://github.com/McShelby/hugo-theme-relearn/issues/143)

---

## 2.8.2 (2021-11-08)

### Fixes

- [**bug**] mermaid: always load javascript to avoid break if code fences are used [#142](https://github.com/McShelby/hugo-theme-relearn/issues/142)

---

## 2.8.1 (2021-11-04)

### Fixes

- [**bug**] search: don't break JS in multilang setup if search is disabled [#140](https://github.com/McShelby/hugo-theme-relearn/issues/140)

---

## 2.8.0 (2021-11-03)

### Enhancements

- [**feature**] toc: make disableTOC globally available via config.toml [#133](https://github.com/McShelby/hugo-theme-relearn/issues/133)
- [**feature**] mermaid: only load javascript if necessary [#95](https://github.com/McShelby/hugo-theme-relearn/issues/95)
- [**feature**][**change**] theme: switch font [#83](https://github.com/McShelby/hugo-theme-relearn/issues/83)
- [**feature**] theme: make favicon configurable [#2](https://github.com/McShelby/hugo-theme-relearn/issues/2)

### Fixes

- [**bug**] mermaid: assert that window.mermaid is actually mermaid [#136](https://github.com/McShelby/hugo-theme-relearn/issues/136)
- [**bug**] menu: remove usage of Hugos UniqueID [#131](https://github.com/McShelby/hugo-theme-relearn/issues/131)
- [**bug**] theme: reduce margin for children shortcode [#130](https://github.com/McShelby/hugo-theme-relearn/issues/130)
- [**bug**] theme: left-align h3 in chapters [#129](https://github.com/McShelby/hugo-theme-relearn/issues/129)
- [**bug**] theme: align copy link to clipboard [#128](https://github.com/McShelby/hugo-theme-relearn/issues/128)

---

## 2.7.0 (2021-10-24)

### Enhancements

- [**feature**] notice: support custom titles [#124](https://github.com/McShelby/hugo-theme-relearn/issues/124)

---

## 2.6.0 (2021-10-21)

### Fixes

- [**bug**] theme: generate correct links if theme served from subdirectory [#120](https://github.com/McShelby/hugo-theme-relearn/issues/120)

---

## 2.5.1 (2021-10-12)

### Fixes

- [**bug**] security: fix XSS for malicioius image URLs [#117](https://github.com/McShelby/hugo-theme-relearn/issues/117)

---

## 2.5.0 (2021-10-08)

### Enhancements

- [**feature**][**change**] syntax highlight: provide default colors for unknown languages [#113](https://github.com/McShelby/hugo-theme-relearn/issues/113)

### Fixes

- [**bug**] security: fix XSS for malicioius URLs [#114](https://github.com/McShelby/hugo-theme-relearn/issues/114)
- [**bug**] menu: write correct local shortcut links [#112](https://github.com/McShelby/hugo-theme-relearn/issues/112)

---

## 2.4.1 (2021-10-07)

### Fixes

- [**bug**] theme: remove runtime styles from print [#111](https://github.com/McShelby/hugo-theme-relearn/issues/111)

---

## 2.4.0 (2021-10-07)

### Enhancements

- [**feature**] lang: add vietnamese translation [#109](https://github.com/McShelby/hugo-theme-relearn/issues/109)
- [**feature**][**change**] theme: simplify stylesheet for color variants [#107](https://github.com/McShelby/hugo-theme-relearn/issues/107)
- [**feature**] hidden pages: remove from RSS feed, JSON, taxonomy etc [#102](https://github.com/McShelby/hugo-theme-relearn/issues/102)
- [**feature**] theme: announce alternative content in header [#101](https://github.com/McShelby/hugo-theme-relearn/issues/101)
- [**feature**] menu: frontmatter option to change sort predicate [#98](https://github.com/McShelby/hugo-theme-relearn/issues/98)
- [**feature**] menu: add default setting for menu expansion [#97](https://github.com/McShelby/hugo-theme-relearn/issues/97)
- [**feature**] theme: improve print style [#93](https://github.com/McShelby/hugo-theme-relearn/issues/93)
- [**feature**] theme: improve style [#92](https://github.com/McShelby/hugo-theme-relearn/issues/92)

### Fixes

- [**bug**] include: don't generate additional HTML if file should be displayed "as is" [#110](https://github.com/McShelby/hugo-theme-relearn/issues/110)
- [**bug**] attachments: fix broken links if multilang config is used [#105](https://github.com/McShelby/hugo-theme-relearn/issues/105)
- [**bug**] theme: fix sticky header to remove horizontal scrollbar [#82](https://github.com/McShelby/hugo-theme-relearn/issues/82)

### Maintenance

- [**task**] chore: update fontawesome [#94](https://github.com/McShelby/hugo-theme-relearn/issues/94)

---

## 2.3.2 (2021-09-20)

### Fixes

- [**bug**] docs: rename history pirate translation [#91](https://github.com/McShelby/hugo-theme-relearn/issues/91)

---

## 2.3.1 (2021-09-20)

### Fixes

- [**bug**] docs: rename english pirate translation to avoid crash on rendering [#90](https://github.com/McShelby/hugo-theme-relearn/issues/90)

---

## 2.3.0 (2021-09-13)

### Fixes

- [**bug**] theme: fix usage of section element [#88](https://github.com/McShelby/hugo-theme-relearn/issues/88)

### Maintenance

- [**task**] theme: ensure IE11 compatibility [#89](https://github.com/McShelby/hugo-theme-relearn/issues/89)
- [**task**] docs: Arrr! showcase multilang featurrre [#87](https://github.com/McShelby/hugo-theme-relearn/issues/87)

---

## 2.2.0 (2021-09-09)

### Enhancements

- [**feature**] sitemap: hide hidden pages from sitemap and SEO indexing [#85](https://github.com/McShelby/hugo-theme-relearn/issues/85)

### Fixes

- [**bug**] theme: fix showVisitedLinks in case Hugo is configured to modify relative URLs [#86](https://github.com/McShelby/hugo-theme-relearn/issues/86)

### Maintenance

- [**task**] theme: switch from data-vocabulary to schema [#84](https://github.com/McShelby/hugo-theme-relearn/issues/84)

---

## 2.1.0 (2021-09-07)

### Enhancements

- [**feature**] search: open expand if it contains search term [#80](https://github.com/McShelby/hugo-theme-relearn/issues/80)
- [**feature**] menu: scroll active item into view [#79](https://github.com/McShelby/hugo-theme-relearn/issues/79)
- [**feature**] search: disable search in hidden pages [#76](https://github.com/McShelby/hugo-theme-relearn/issues/76)
- [**feature**] search: improve readablility of index.json [#75](https://github.com/McShelby/hugo-theme-relearn/issues/75)
- [**feature**] search: increase performance [#74](https://github.com/McShelby/hugo-theme-relearn/issues/74)
- [**feature**] search: improve search context preview [#73](https://github.com/McShelby/hugo-theme-relearn/issues/73)

### Fixes

- [**bug**][**change**] search: hide non-site content [#81](https://github.com/McShelby/hugo-theme-relearn/issues/81)
- [**bug**] menu: always hide hidden sub pages [#77](https://github.com/McShelby/hugo-theme-relearn/issues/77)

---

## 2.0.0 (2021-08-28)

### Enhancements

- [**feature**] tabs: enhance styling [#65](https://github.com/McShelby/hugo-theme-relearn/issues/65)
- [**feature**] theme: improve readability [#64](https://github.com/McShelby/hugo-theme-relearn/issues/64)
- [**feature**] menu: show hidden pages if accessed directly [#60](https://github.com/McShelby/hugo-theme-relearn/issues/60)
- [**feature**][**change**] theme: treat pages without title as hidden [#59](https://github.com/McShelby/hugo-theme-relearn/issues/59)
- [**feature**] search: show search results if field gains focus [#58](https://github.com/McShelby/hugo-theme-relearn/issues/58)
- [**feature**] theme: add partial templates for pre/post menu entries [#56](https://github.com/McShelby/hugo-theme-relearn/issues/56)
- [**feature**] theme: make chapter archetype more readable [#55](https://github.com/McShelby/hugo-theme-relearn/issues/55)
- [**feature**] children: add parameter for container style [#53](https://github.com/McShelby/hugo-theme-relearn/issues/53)
- [**feature**] theme: make content a template [#50](https://github.com/McShelby/hugo-theme-relearn/issues/50)
- [**feature**] menu: control menu expansion with alwaysopen parameter [#49](https://github.com/McShelby/hugo-theme-relearn/issues/49)
- [**feature**] include: new shortcode to include other files [#43](https://github.com/McShelby/hugo-theme-relearn/issues/43)
- [**feature**] theme: adjust print styles [#35](https://github.com/McShelby/hugo-theme-relearn/issues/35)
- [**feature**][**change**] code highlighter: switch to standard hugo highlighter [#32](https://github.com/McShelby/hugo-theme-relearn/issues/32)

### Fixes

- [**bug**][**change**] arrow-nav: default sorting ignores ordersectionsby [#63](https://github.com/McShelby/hugo-theme-relearn/issues/63)
- [**bug**][**change**] children: default sorting ignores ordersectionsby [#62](https://github.com/McShelby/hugo-theme-relearn/issues/62)
- [**bug**][**change**] arrow-nav: fix broken links on (and below) hidden pages [#61](https://github.com/McShelby/hugo-theme-relearn/issues/61)
- [**bug**] theme: remove superfluous singular taxonomy from taxonomy title [#46](https://github.com/McShelby/hugo-theme-relearn/issues/46)
- [**bug**][**change**] theme: missing --MENU-HOME-LINK-HOVER-color in documentation [#45](https://github.com/McShelby/hugo-theme-relearn/issues/45)
- [**bug**] theme: fix home link when base URL has some path [#44](https://github.com/McShelby/hugo-theme-relearn/issues/44)

### Maintenance

- [**task**] docs: include changelog in exampleSite [#33](https://github.com/McShelby/hugo-theme-relearn/issues/33)

---

## 1.2.0 (2021-07-26)

### Enhancements

- [**feature**] theme: adjust copy-to-clipboard [#29](https://github.com/McShelby/hugo-theme-relearn/issues/29)
- [**feature**] attachments: adjust style between notice boxes and attachments [#28](https://github.com/McShelby/hugo-theme-relearn/issues/28)
- [**feature**] theme: adjust blockquote contrast [#27](https://github.com/McShelby/hugo-theme-relearn/issues/27)
- [**feature**] expand: add option to open on page load [#25](https://github.com/McShelby/hugo-theme-relearn/issues/25)
- [**feature**] expand: rework styling [#24](https://github.com/McShelby/hugo-theme-relearn/issues/24)
- [**feature**] attachments: sort output [#23](https://github.com/McShelby/hugo-theme-relearn/issues/23)
- [**feature**] notice: make restyling of notice boxes more robust [#20](https://github.com/McShelby/hugo-theme-relearn/issues/20)
- [**feature**] notice: fix contrast issues [#19](https://github.com/McShelby/hugo-theme-relearn/issues/19)
- [**feature**] notice: align box colors to common standards [#18](https://github.com/McShelby/hugo-theme-relearn/issues/18)
- [**feature**] notice: use distinct icons for notice box type [#17](https://github.com/McShelby/hugo-theme-relearn/issues/17)

### Fixes

- [**bug**] attachments: support i18n for attachment size [#21](https://github.com/McShelby/hugo-theme-relearn/issues/21)
- [**bug**] notice: support i18n for box labels [#16](https://github.com/McShelby/hugo-theme-relearn/issues/16)
- [**bug**] notice: support multiple blocks in one box [#15](https://github.com/McShelby/hugo-theme-relearn/issues/15)

### Maintenance

- [**task**] dependency: upgrade jquery to 3.6.0 [#30](https://github.com/McShelby/hugo-theme-relearn/issues/30)

---

## 1.1.1 (2021-07-04)

### Maintenance

- [**task**] theme: prepare for new hugo theme registration [#13](https://github.com/McShelby/hugo-theme-relearn/issues/13)

---

## 1.1.0 (2021-07-02)

### Enhancements

- [**feature**] mermaid: expose options in config.toml [#4](https://github.com/McShelby/hugo-theme-relearn/issues/4)

### Fixes

- [**bug**] mermaid: config option for CDN url not used [#12](https://github.com/McShelby/hugo-theme-relearn/issues/12)
- [**bug**] mermaid: only highlight text in HTML elements [#10](https://github.com/McShelby/hugo-theme-relearn/issues/10)
- [**bug**] mermaid: support pan & zoom for graphs [#9](https://github.com/McShelby/hugo-theme-relearn/issues/9)
- [**bug**] mermaid: code fences not always rendered [#6](https://github.com/McShelby/hugo-theme-relearn/issues/6)
- [**bug**] mermaid: search term on load may bomb chart [#5](https://github.com/McShelby/hugo-theme-relearn/issues/5)

### Maintenance

- [**task**] mermaid: update to 8.10.2 [#7](https://github.com/McShelby/hugo-theme-relearn/issues/7)

---

## 1.0.1 (2021-07-01)

### Maintenance

- [**task**] Prepare for hugo showcase [#3](https://github.com/McShelby/hugo-theme-relearn/issues/3)

---

## 1.0.0 (2021-07-01)

### Maintenance

- [**task**] Fork project [#1](https://github.com/McShelby/hugo-theme-relearn/issues/1)

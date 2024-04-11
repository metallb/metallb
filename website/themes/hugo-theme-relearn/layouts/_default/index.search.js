{{- partialCached "page-meta.hugo" . .RelPermalink }}
{{- $pages := slice }}
{{- range .Site.Pages }}
  {{- if and .Title .RelPermalink (or (ne (.Scratch.Get "relearnIsHiddenStem") true) (ne .Site.Params.disableSearchHiddenPages true) ) }}
    {{- $pages = $pages | append (dict
      "uri" (partial "relLangPrettyUglyURL.hugo" (dict "to" .))
      "title" (partial "pageHelper/title.hugo" (dict "page" .))
      "tags" .Params.tags
      "breadcrumb" (trim ((partial "breadcrumbs.html" (dict "page" . "dirOnly" true)) | plainify | htmlUnescape) "\n\r\t ")
      "description" .Description
      "content" (.Plain | htmlUnescape)
    ) }}
  {{- end }}
{{- end -}}
var relearn_search_index = {{ $pages | jsonify (dict "indent" "  ") }}
